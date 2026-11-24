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
	"strings"
	"testing"
)

// secretDiscovery is a standard collection whose resource has a secret
// the API returns (token), a writable boolean (disabled) and a string.
func secretDiscovery() *Discovery {
	get := &Method{HTTPMethod: "GET", Path: "v1/{+name}", FlatPath: "v1/projects/{projectsId}/things/{thingsId}"}
	return &Discovery{
		RootURL: "https://x.googleapis.com/",
		Resources: map[string]*DRes{"projects": {Resources: map[string]*DRes{"things": {Methods: map[string]*Method{
			"create": {HTTPMethod: "POST", Parameters: map[string]*Param{"thingId": {Location: "query", Type: "string"}}, Request: &Schema{Ref: "Thing"}},
			"get":    get,
			"patch":  {HTTPMethod: "PATCH"},
			"delete": {HTTPMethod: "DELETE"},
		}}}}},
		Schemas: map[string]*Schema{"Thing": {Properties: map[string]*Schema{
			"name":        {Type: "string"},
			"token":       {Type: "string", Description: "Output only. The token."},
			"state":       {Type: "string", Description: "Output only. The state."},
			"disabled":    {Type: "boolean"},
			"displayName": {Type: "string"},
		}}},
	}
}

// TestSecretOutputs: the generator takes a top-level read-only string as a
// secret output, leaves it out of observedState, gives the spec its
// secretOutput, and refuses what is not one (decision 38).
func TestSecretOutputs(t *testing.T) {
	base := func() *Config {
		return &Config{Kind: "XThing", Group: "x.gcp.hfl.dev", Version: "v1alpha1", Discovery: "x.v1", Resource: "projects.things", Parents: []string{"project"}}
	}
	c := base()
	c.SecretOutputs, c.CreateOnlyOutputs, c.DisableBeforeDelete = []string{"token"}, []string{"token"}, "disabled"
	k, err := build(c, secretDiscovery())
	if err != nil {
		t.Fatal(err)
	}
	types := string(k.typesFile())
	if !strings.Contains(types, "SecretOutput *XThingSecretOutput `json:\"secretOutput,omitempty\"`") {
		t.Error("the spec has no secretOutput")
	}
	if strings.Contains(types, "Token *string") {
		t.Error("observedState has the secret output")
	}
	if !strings.Contains(types, "State *string") {
		t.Error("observedState lost a read-only field that is not a secret")
	}
	reg := string(registryFile([]*built{k}))
	for _, want := range []string{`SecretOutputs: []string{"token"}`, `CreateOnlyOutputs: []string{"token"}`, `DisableBeforeDelete: "disabled"`} {
		if !strings.Contains(reg, want) {
			t.Errorf("the registry lacks %s", want)
		}
	}

	for name, tc := range map[string]struct {
		set  func(*Config)
		want string
	}{
		"a writable field":          {func(c *Config) { c.SecretOutputs = []string{"displayName"} }, "secretOutputs: displayName"},
		"a missing field":           {func(c *Config) { c.SecretOutputs = []string{"nope"} }, "secretOutputs: nope"},
		"create-only, not a secret": {func(c *Config) { c.CreateOnlyOutputs = []string{"state"} }, "createOnlyOutputs: state"},
		"disable on a string":       {func(c *Config) { c.DisableBeforeDelete = "displayName" }, "disableBeforeDelete: displayName"},
		"disable on an output":      {func(c *Config) { c.DisableBeforeDelete = "state" }, "disableBeforeDelete: state"},
	} {
		c := base()
		tc.set(c)
		if _, err := build(c, secretDiscovery()); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}
