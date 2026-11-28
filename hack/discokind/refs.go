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

// References (kinds.yaml refs; docs/design.md section 2.4): a string field
// the API fills from another resource gets a <field>Ref beside it, which
// pkg/discokind resolves. Each is checked against Crossplane's Upjet
// provider (hack/upjet/provider-upjet-gcp.json): where Upjet resolves the
// same field, its target must be served by the kind named here, and the
// Terraform resource is recorded as the reference's provenance.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// RefConfig is one kinds.yaml refs entry.
type RefConfig struct {
	// Target is the kind the reference names (ComputeNetwork).
	Target string `json:"target"`
	// Format is how the field takes the target's value
	// (pkg/discokind.Ref.Format): "", id or serviceAccountName.
	Format string `json:"format,omitempty"`
}

var refFormats = map[string]bool{"": true, "id": true, "serviceAccountName": true}

// upjetFacts is the part of the Upjet slice the generator reads.
type upjetFacts struct {
	Entries []struct {
		Ours struct {
			OursKinds []string `json:"oursKinds"`
		} `json:"ours"`
		Facts struct {
			Terraform  string `json:"terraform"`
			References []struct {
				Field  string `json:"field"`
				Target string `json:"target"`
				At     string `json:"at"`
			} `json:"references"`
		} `json:"facts"`
		TargetsOurs map[string][]string `json:"targetsOurs"`
	} `json:"entries"`
}

// upjetRef is one reference Upjet resolves on a kind's Terraform resource.
type upjetRef struct {
	field, target, at string
	// served are the kinds here that serve the target.
	served []string
}

func loadUpjetFacts(root string) (*upjetFacts, error) {
	b, err := os.ReadFile(filepath.Join(root, "hack/upjet/provider-upjet-gcp.json"))
	if err != nil {
		return nil, err
	}
	u := &upjetFacts{}
	return u, json.Unmarshal(b, u)
}

// upjetRefsOf are the references Upjet resolves on the kind's own
// Terraform resource (kinds.yaml terraform).
func upjetRefsOf(u *upjetFacts, kind, terraform string) []upjetRef {
	if u == nil || terraform == "" {
		return nil
	}
	var out []upjetRef
	for _, e := range u.Entries {
		if e.Facts.Terraform != terraform || !contains(e.Ours.OursKinds, kind) {
			continue
		}
		for _, r := range e.Facts.References {
			out = append(out, upjetRef{field: r.Field, target: r.Target, at: r.At, served: e.TargetsOurs[r.Target]})
		}
	}
	return out
}

var listMarkerRE = regexp.MustCompile(`\[\]`)

// normRefPath compares a spec path with a Terraform one: lower case, no
// underscores or list markers, a plural's s dropped.
func normRefPath(p string) string {
	segs := strings.Split(listMarkerRE.ReplaceAllString(p, ""), ".")
	for i, s := range segs {
		s = strings.ToLower(strings.ReplaceAll(s, "_", ""))
		segs[i] = strings.TrimSuffix(s, "s")
	}
	return strings.Join(segs, ".")
}

// builtRef is a kinds.yaml reference as the generator worked it out.
type builtRef struct {
	path   string
	list   bool
	target string
	format string
	upjet  string
}

// buildRefs checks a kind's refs against its schema and Upjet, and works
// out each one's shape and provenance.
func buildRefs(c *Config, d *Discovery, s *Schema, nameParts []string, u *upjetFacts) ([]builtRef, error) {
	ups := upjetRefsOf(u, c.Kind, c.Terraform)
	var out []builtRef
	paths := make([]string, 0, len(c.Refs))
	for p := range c.Refs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		rc := c.Refs[p]
		if rc.Target == "" {
			return nil, fmt.Errorf("refs: %s has no target", p)
		}
		if !refFormats[rc.Format] {
			return nil, fmt.Errorf("refs: %s: format %q is not one of \"\", id, serviceAccountName", p, rc.Format)
		}
		if contains(nameParts, p) {
			return nil, fmt.Errorf("refs: %s is a part of the name, which stays an ID", p)
		}
		if c.Skip[p] != "" || contains(c.Secrets, p) || contains(c.InputOnly, p) {
			return nil, fmt.Errorf("refs: %s is skipped, a secret or input-only", p)
		}
		f, parent := fieldAtPath(d, s, p)
		if f == nil || isOutput(f) {
			return nil, fmt.Errorf("refs: %s is not a writable field of the spec", p)
		}
		list := false
		switch {
		case f.Type == "string" && f.Format == "" && len(f.Enum) == 0:
		case f.Type == "array" && f.Items != nil && f.Items.Type == "string" && f.Items.Format == "" && f.Items.Ref == "":
			list = true
		default:
			return nil, fmt.Errorf("refs: %s is not a string or a list of strings", p)
		}
		leaf := p[strings.LastIndex(p, ".")+1:]
		refName := refFieldName(leaf, list)
		if parent != nil && parent.Properties[refName] != nil {
			return nil, fmt.Errorf("refs: %s: the API has a field %s already", p, refName)
		}
		b := builtRef{path: p, list: list, target: rc.Target, format: rc.Format}
		for _, ur := range ups {
			if normRefPath(ur.field) != normRefPath(p) {
				continue
			}
			// The stricter rule: Upjet resolves this field too, so the two
			// must name the same resource.
			if !contains(ur.served, rc.Target) {
				return nil, fmt.Errorf("refs: %s names %s, but Upjet resolves %s from %s, which %s serves here (%s)",
					p, rc.Target, ur.field, ur.target, orNothing(ur.served), ur.at)
			}
			b.upjet = ur.target
		}
		out = append(out, b)
	}
	return out, nil
}

func orNothing(l []string) string {
	if len(l) == 0 {
		return "no kind"
	}
	return strings.Join(l, ", ")
}

// fieldAtPath is the schema of the field at a dotted spec path under s,
// through objects, $refs and lists of objects ([] after a part), and the
// schema of the object that holds it; nil if there is none.
func fieldAtPath(d *Discovery, s *Schema, path string) (*Schema, *Schema) {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		name, list := strings.CutSuffix(part, "[]")
		if s != nil && s.Ref != "" {
			s = d.Schemas[s.Ref]
		}
		if s == nil {
			return nil, nil
		}
		holder := s
		s = s.Properties[name]
		if s == nil {
			return nil, nil
		}
		if i == len(parts)-1 {
			if list {
				return nil, nil
			}
			return s, holder
		}
		if list {
			if s.Type != "array" || s.Items == nil {
				return nil, nil
			}
			s = s.Items
		} else if s.Type == "array" {
			return nil, nil
		}
	}
	return nil, nil
}

// refFieldName is pkg/discokind.RefFieldName (the generator does not
// import the controller's package): field + "Ref", or for a list of
// strings its singular + "Refs".
func refFieldName(field string, list bool) string {
	if !list {
		return field + "Ref"
	}
	return singularField(field) + "Refs"
}

// singularField is pkg/discokind.SingularField.
func singularField(s string) string {
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 3:
		return strings.TrimSuffix(s, "ies") + "y"
	case strings.HasSuffix(s, "sses"), strings.HasSuffix(s, "xes"), strings.HasSuffix(s, "ches"), strings.HasSuffix(s, "shes"):
		return strings.TrimSuffix(s, "es")
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// refComment is the doc comment of a reference field.
func refComment(r builtRef, field string) string {
	what := "the value " + field + " takes"
	switch {
	case r.format == "id":
		what = "the ID " + field + " takes, or the resource's full name"
	case r.target == "IAMServiceAccount":
		what = "the service account's email"
	}
	an := "a"
	if strings.ContainsAny(r.target[:1], "AEIOU") {
		an = "an"
	}
	one := "A reference to the " + r.target + " that fills " + field
	if r.list {
		one = "References to the " + r.target + "s that fill " + field
	}
	return comment(one + ", in its place: external (" + what + "), or the name (and namespace) of " + an + " " + r.target + " object, this repository's kind of that name first, then Config Connector's. Set " + field + " or this, not both.")
}
