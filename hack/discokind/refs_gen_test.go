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

import (
	"encoding/json"
	"strings"
	"testing"
)

// refDiscovery is a standard collection with a required network, a list of
// subnets, a list of consumers each with a network, and a key struct
// shared ($ref Key) at two paths.
func refDiscovery() *Discovery {
	str := &Schema{Type: "string"}
	get := &Method{HTTPMethod: "GET", Path: "v1/{+name}", FlatPath: "v1/projects/{projectsId}/things/{thingsId}"}
	return &Discovery{
		RootURL: "https://x.googleapis.com/",
		Resources: map[string]*DRes{"projects": {Resources: map[string]*DRes{"things": {Methods: map[string]*Method{
			"create": {HTTPMethod: "POST", Parameters: map[string]*Param{"thingId": {Location: "query", Type: "string"}}, Request: &Schema{Ref: "Thing"}},
			"get":    get,
			"patch":  {HTTPMethod: "PATCH"},
			"delete": {HTTPMethod: "DELETE"},
		}}}}},
		Schemas: map[string]*Schema{
			"Key": {Properties: map[string]*Schema{"kmsKeyName": str}},
			"Thing": {Properties: map[string]*Schema{
				"network":     {Type: "string", Description: "Required. Immutable. The network."},
				"natSubnets":  {Type: "array", Items: str},
				"consumers":   {Type: "array", Items: &Schema{Type: "object", Properties: map[string]*Schema{"networkUrl": str, "limit": {Type: "integer"}}}},
				"diskKey":     {Ref: "Key"},
				"sourceKey":   {Ref: "Key"},
				"size":        {Type: "integer"},
				"state":       {Type: "string", Description: "Output only. The state."},
				"displayName": str,
			}},
		},
	}
}

func refConfig() *Config {
	return &Config{Kind: "XThing", Group: "x.gcp.hfl.dev", Version: "v1alpha1", Discovery: "x.v1", Resource: "projects.things",
		Parents: []string{"project"}, Terraform: "google_x_thing", Refs: map[string]RefConfig{
			"network":                {Target: "ComputeNetwork"},
			"natSubnets":             {Target: "ComputeSubnetwork"},
			"consumers[].networkUrl": {Target: "ComputeNetwork"},
		}}
}

func refUpjet(t *testing.T, target string) *upjetFacts {
	t.Helper()
	u := &upjetFacts{}
	err := json.Unmarshal([]byte(`{"entries":[{"ours":{"oursKinds":["XThing"]},"facts":{"terraform":"google_x_thing","references":[
		{"field":"network","target":"google_compute_network","at":"x.go:1"},
		{"field":"nat_subnets","target":"`+target+`","at":"x.go:2"}]},
		"targetsOurs":{"google_compute_network":["ComputeNetwork"],"google_compute_subnetwork":["ComputeSubnetwork"],"google_compute_address":["ComputeAddress"]}}]}`), u)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// TestRefsRender: a reference sits beside its field (a list's as
// <singular>Refs), one of the two at most (exactly one for a required
// field), immutable as the field is; the registry records each with its
// provenance from Upjet.
func TestRefsRender(t *testing.T) {
	k, err := build(refConfig(), refDiscovery(), refUpjet(t, "google_compute_subnetwork"))
	if err != nil {
		t.Fatal(err)
	}
	types := string(k.typesFile())
	for _, want := range []string{
		"NetworkRef *commonv1alpha1.ResourceRef `json:\"networkRef,omitempty\"`",
		"NatSubnetRefs []commonv1alpha1.ResourceRef `json:\"natSubnetRefs,omitempty\"`",
		"NetworkURLRef *commonv1alpha1.ResourceRef `json:\"networkUrlRef,omitempty\"`",
		`rule="has(self.network) != has(self.networkRef)",message="exactly one of network and networkRef is required"`,
		`rule="!(has(self.natSubnets) && has(self.natSubnetRefs))"`,
		`rule="!(has(self.networkUrl) && has(self.networkUrlRef))"`,
		`rule="self == oldSelf",message="networkRef is immutable"`,
		`commonv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/common/v1alpha1"`,
	} {
		if !strings.Contains(types, want) {
			t.Errorf("the types lack %s", want)
		}
	}
	// The spec-level rules come before the spec's type, after the parents'.
	if i, j := strings.Index(types, "exactly one of network and networkRef"), strings.Index(types, "type XThingSpec struct"); i < 0 || i > j {
		t.Error("the network rule is not on the spec's type")
	}
	if strings.Contains(types, "// +kubebuilder:validation:Required\n\tNetwork ") || strings.Contains(types, "+kubebuilder:validation:Required\n// +kubebuilder:validation:XValidation:rule=\"self == oldSelf\",message=\"network is immutable\"") {
		t.Error("network is still required on its own")
	}
	reg := string(registryFile([]*built{k}))
	for _, want := range []string{
		`{Path: "consumers[].networkUrl", Target: "ComputeNetwork"},`,
		`{Path: "natSubnets", List: true, Target: "ComputeSubnetwork", Upjet: "google_compute_subnetwork"},`,
		`{Path: "network", Target: "ComputeNetwork", Upjet: "google_compute_network"},`,
	} {
		if !strings.Contains(reg, want) {
			t.Errorf("the registry lacks %s", want)
		}
	}
}

// TestRefsRefused: the generator refuses a reference Upjet disagrees with
// (the stricter rule), and one that is not a writable string field.
func TestRefsRefused(t *testing.T) {
	if _, err := build(refConfig(), refDiscovery(), refUpjet(t, "google_compute_address")); err == nil ||
		!strings.Contains(err.Error(), "natSubnets names ComputeSubnetwork, but Upjet resolves nat_subnets from google_compute_address") {
		t.Errorf("an Upjet disagreement: err = %v", err)
	}
	for name, tc := range map[string]struct {
		ref  string
		rc   RefConfig
		want string
	}{
		"an integer":       {"size", RefConfig{Target: "X"}, "not a string"},
		"an output":        {"state", RefConfig{Target: "X"}, "not a writable field"},
		"missing":          {"nope.x", RefConfig{Target: "X"}, "not a writable field"},
		"no target":        {"displayName", RefConfig{}, "no target"},
		"a bad format":     {"displayName", RefConfig{Target: "X", Format: "url"}, "format"},
		"half of a shared": {"diskKey.kmsKeyName", RefConfig{Target: "KMSCryptoKey"}, "shared"},
	} {
		t.Run(name, func(t *testing.T) {
			c := refConfig()
			c.Refs = map[string]RefConfig{tc.ref: tc.rc}
			k, err := build(c, refDiscovery(), nil)
			if err == nil && tc.want == "shared" {
				err = sharedRefsErr(k)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
	c := refConfig()
	c.Refs = map[string]RefConfig{"diskKey.kmsKeyName": {Target: "KMSCryptoKey"}, "sourceKey.kmsKeyName": {Target: "KMSCryptoKey"}}
	k, err := build(c, refDiscovery(), nil)
	if err == nil {
		err = sharedRefsErr(k)
	}
	if err != nil {
		t.Errorf("a shared struct referenced at both paths: %v", err)
	}
}

// sharedRefsErr writes the spec as typesFile does and returns checkRefs'
// verdict.
func sharedRefsErr(k *built) error {
	g := &typeGen{k: k, defs: map[string]string{}, inStack: map[string]bool{}, secrets: map[string]bool{},
		skipped: map[string]bool{}, refPaths: map[string][]string{}, refs: map[string]builtRef{}, refApplied: map[string]bool{}}
	for _, r := range k.refs {
		g.refs[r.path] = r
	}
	g.structDef(k.c.Kind+"Spec", k.schema, true, "")
	return g.checkRefs()
}
