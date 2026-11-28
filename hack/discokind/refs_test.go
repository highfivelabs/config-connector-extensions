// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

// The reference guard, the stricter rule of factory docs/design.md 5.4: a
// field that either source calls a reference is one. Every reference
// Crossplane's Upjet provider resolves on a generated kind's own
// Terraform resource, to a kind served here, is in kinds.yaml refs or
// waived with a reason (refWaivers); every waiver names such a reference.
// (The generator checks the other half: a refs entry Upjet also resolves
// names a kind that serves Upjet's target.) It reads kinds.yaml and the
// Upjet slice only, so it needs neither Discovery nor the network.

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestRefGuard(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "hack/discokind/kinds.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfgs []Config
	if err := yaml.UnmarshalStrict(b, &cfgs); err != nil {
		t.Fatal(err)
	}
	u, err := loadUpjetFacts(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cfgs {
		taken := map[string]string{} // normalized path -> how
		for p := range c.Refs {
			taken[normRefPath(p)] = "refs " + p
		}
		for p, why := range c.RefWaivers {
			if why == "" {
				t.Errorf("%s: refWaivers %s has no reason", c.Kind, p)
			}
			if how, both := taken[normRefPath(p)]; both {
				t.Errorf("%s: refWaivers %s is also %s", c.Kind, p, how)
			}
			taken[normRefPath(p)] = "waived"
		}
		upjet := map[string]bool{}
		var open []string
		for _, r := range upjetRefsOf(u, c.Kind, c.Terraform) {
			upjet[normRefPath(r.field)] = true
			if len(r.served) == 0 || isParentRef(c, r.field) {
				// Not served here, or the parent, which is a reference
				// already (projectRef).
				continue
			}
			if taken[normRefPath(r.field)] == "" && !nameAlias(c, r.field) {
				open = append(open, r.field+" → "+r.target+" ("+orNothing(r.served)+", "+r.at+")")
			}
		}
		sort.Strings(open)
		for _, o := range open {
			t.Errorf("%s: Upjet resolves %s; take it in kinds.yaml refs, or waive it in refWaivers with a reason", c.Kind, o)
		}
		for p := range c.RefWaivers {
			if !upjet[normRefPath(p)] {
				t.Errorf("%s: refWaivers %s is no reference Upjet resolves (a stale waiver)", c.Kind, p)
			}
		}
	}
}

// nameAlias: Upjet's field is a part of the name that kinds.yaml refs
// takes under the API's name: Terraform's dataplex_zone is an asset's
// zone. Listed here, each with the reason, rather than guessed.
func nameAlias(c Config, field string) bool {
	aliases := map[string]map[string]string{
		"DataplexAsset": {"dataplex_zone": "zone"},
	}
	p, ok := aliases[c.Kind][field]
	if !ok {
		return false
	}
	_, taken := c.Refs[p]
	return taken
}

// isParentRef: Upjet's field is the kind's parent (project), which the
// spec takes as its parent reference.
func isParentRef(c Config, field string) bool {
	for _, p := range c.Parents {
		if normRefPath(p) == normRefPath(field) {
			return true
		}
	}
	return false
}
