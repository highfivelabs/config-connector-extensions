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

package iampolicy

import (
	"slices"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/genproto/googleapis/type/expr"
)

func policy(bs ...*iampb.Binding) *iampb.Policy { return &iampb.Policy{Version: 1, Bindings: bs} }

func bind(role string, members ...string) *iampb.Binding {
	return &iampb.Binding{Role: role, Members: members}
}

func TestSetMembersReplacesOneRoleOnly(t *testing.T) {
	p := policy(bind("roles/a", "user:x@e.com", "user:stray@e.com"), bind("roles/b", "user:keep@e.com"))
	if !SetMembers(p, "roles/a", nil, []string{"user:x@e.com", "group:g@e.com"}) {
		t.Fatal("expected a change: a stray member must go")
	}
	got, _ := Members(p, "roles/a", nil)
	if !slices.Equal(got, []string{"group:g@e.com", "user:x@e.com"}) {
		t.Errorf("roles/a members = %v", got)
	}
	if other, _ := Members(p, "roles/b", nil); !slices.Equal(other, []string{"user:keep@e.com"}) {
		t.Errorf("roles/b changed: %v", other)
	}
}

func TestSetMembersNoChange(t *testing.T) {
	p := policy(bind("roles/a", "user:Bob@e.com", "group:g@e.com"))
	// Order and the case of an email do not matter: the API lowercases.
	if SetMembers(p, "roles/a", nil, []string{"group:g@e.com", "user:bob@e.com"}) {
		t.Error("the same members in another order and case must not change the policy")
	}
}

func TestSetMembersCreatesAndRemoves(t *testing.T) {
	p := policy()
	if !SetMembers(p, "roles/a", nil, []string{"user:x@e.com"}) || len(p.Bindings) != 1 {
		t.Fatalf("binding not created: %v", p)
	}
	if !SetMembers(p, "roles/a", nil, nil) || len(p.Bindings) != 0 {
		t.Fatalf("no members must remove the binding: %v", p)
	}
	if SetMembers(p, "roles/a", nil, nil) {
		t.Error("removing a missing binding must not be a change")
	}
}

func TestConditionsAreSeparateBindings(t *testing.T) {
	cond := &Condition{Title: "soon", Expression: "request.time < timestamp('2030-01-01T00:00:00Z')"}
	p := policy(bind("roles/a", "user:plain@e.com"))
	if !SetMembers(p, "roles/a", cond, []string{"user:timed@e.com"}) {
		t.Fatal("expected a change")
	}
	if p.Version != 3 {
		t.Errorf("a conditional binding needs version 3, got %d", p.Version)
	}
	if plain, _ := Members(p, "roles/a", nil); !slices.Equal(plain, []string{"user:plain@e.com"}) {
		t.Errorf("the unconditional binding changed: %v", plain)
	}
	if !Remove(p, "roles/a", cond) || len(p.Bindings) != 1 || p.Bindings[0].Condition != nil {
		t.Errorf("removing the conditional binding left %v", p.Bindings)
	}
}

func TestDescriptionDriftIsCorrected(t *testing.T) {
	cond := &Condition{Title: "t", Description: "wanted", Expression: "true"}
	p := policy(&iampb.Binding{Role: "roles/a", Members: []string{"user:x@e.com"},
		Condition: &expr.Expr{Title: "t", Description: "edited", Expression: "true"}})
	if !SetMembers(p, "roles/a", cond, []string{"user:x@e.com"}) {
		t.Error("a changed condition description must be put back")
	}
}

func TestDuplicateBindingsMerge(t *testing.T) {
	p := policy(bind("roles/a", "user:x@e.com"), bind("roles/b", "user:y@e.com"), bind("roles/a", "user:z@e.com"))
	if !SetMembers(p, "roles/a", nil, []string{"user:x@e.com"}) {
		t.Fatal("expected a change")
	}
	if len(p.Bindings) != 2 {
		t.Errorf("duplicate bindings must collapse to one: %v", p.Bindings)
	}
}

func TestAddAndRemoveMembers(t *testing.T) {
	p := policy(bind("roles/a", "user:other@e.com"))
	if !AddMembers(p, "roles/a", nil, []string{"user:me@e.com"}) {
		t.Fatal("expected a change")
	}
	if m, _ := Members(p, "roles/a", nil); !slices.Equal(m, []string{"user:me@e.com", "user:other@e.com"}) {
		t.Errorf("add must keep the others: %v", m)
	}
	if AddMembers(p, "roles/a", nil, []string{"user:ME@e.com"}) {
		t.Error("adding a member already there is no change")
	}
	if !RemoveMembers(p, "roles/a", nil, []string{"user:me@e.com"}) {
		t.Fatal("expected a change")
	}
	if m, _ := Members(p, "roles/a", nil); !slices.Equal(m, []string{"user:other@e.com"}) {
		t.Errorf("remove must keep the others: %v", m)
	}
	if !RemoveMembers(p, "roles/a", nil, []string{"user:other@e.com"}) || len(p.Bindings) != 0 {
		t.Errorf("removing the last member must remove the binding: %v", p.Bindings)
	}
	if RemoveMembers(p, "roles/a", nil, []string{"user:other@e.com"}) {
		t.Error("removing from a missing binding is no change")
	}
}

func TestResolveExternal(t *testing.T) {
	cases := []struct{ kind, in, want string }{
		{"Project", "my-project", "projects/my-project"},
		{"Project", "projects/my-project", "projects/my-project"},
		{"Folder", "123", "folders/123"},
		{"StorageBucket", "b-1", "projects/_/buckets/b-1"},
		{"IAMServiceAccount", "sa@p.iam.gserviceaccount.com", "projects/p/serviceAccounts/sa@p.iam.gserviceaccount.com"},
		{"SecretManagerSecret", "projects/p/locations/europe-west1/secrets/s", "projects/p/locations/europe-west1/secrets/s"},
	}
	for _, c := range cases {
		rt, ok := Lookup(c.kind)
		if !ok {
			t.Fatalf("no %s", c.kind)
		}
		got, err := rt.ResolveExternal(c.in)
		if err != nil || got != c.want {
			t.Errorf("%s %q = %q, %v; want %q", c.kind, c.in, got, err, c.want)
		}
	}
	if rt, _ := Lookup("Folder"); true {
		if _, err := rt.ResolveExternal("projects/p"); err == nil {
			t.Error("a project is not a folder")
		}
	}
}

func TestEndpoint(t *testing.T) {
	sm, _ := Lookup("SecretManagerSecret")
	if got := sm.Endpoint("projects/p/secrets/s"); got != "secretmanager.googleapis.com" {
		t.Errorf("global secret host = %s", got)
	}
	if got := sm.Endpoint("projects/p/locations/europe-west1/secrets/s"); got != "secretmanager.europe-west1.rep.googleapis.com" {
		t.Errorf("regional secret host = %s", got)
	}
}
