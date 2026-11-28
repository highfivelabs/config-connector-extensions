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

package controller

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
)

// managerRole is the manager's ClusterRole in config/operator/rbac.yaml.
func managerRole(t *testing.T) *rbacv1.ClusterRole {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "operator", "rbac.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var role *rbacv1.ClusterRole
	dec := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		var r rbacv1.ClusterRole
		if err := dec.Decode(&r); err != nil {
			break
		}
		if r.Kind == "ClusterRole" && r.Name == "config-connector-extensions-manager" {
			role = &r
			break
		}
	}
	if role == nil {
		t.Fatal("config/operator/rbac.yaml has no ClusterRole config-connector-extensions-manager")
	}
	return role
}

// Every kind's group is in the manager's ClusterRole, so a new kind cannot
// ship without the rights to reconcile it.
func TestManagerRoleCoversEveryKind(t *testing.T) {
	role := managerRole(t)
	for _, k := range Kinds {
		var crud, status bool
		for _, rule := range role.Rules {
			if !slices.Contains(rule.APIGroups, k.gvk.Group) {
				continue
			}
			if slices.Contains(rule.Resources, "*") && slices.Contains(rule.Verbs, "create") && slices.Contains(rule.Verbs, "delete") {
				crud = true
			}
			if slices.Contains(rule.Resources, "*/status") && slices.Contains(rule.Verbs, "update") {
				status = true
			}
		}
		if !crud || !status {
			t.Errorf("ClusterRole config-connector-extensions-manager does not cover %s (objects: %v, status: %v)", k.gvk.Group, crud, status)
		}
	}
}

// Every kind IAMBinding and IAMMember can name by resourceRef.name
// (pkg/iampolicy's table) is readable by the manager: the resolver gets the
// object, and a group missing from the role is a 403 on a real install that
// envtest, which has no RBAC, never shows.
func TestManagerRoleReadsEveryIAMResourceKind(t *testing.T) {
	role := managerRole(t)
	for _, kind := range iampolicy.Kinds() {
		rt, _ := iampolicy.Lookup(kind)
		for _, gvk := range rt.GVKs {
			readable := false
			for _, rule := range role.Rules {
				if slices.Contains(rule.APIGroups, gvk.Group) && slices.Contains(rule.Resources, "*") && slices.Contains(rule.Verbs, "get") {
					readable = true
				}
			}
			if !readable {
				t.Errorf("ClusterRole config-connector-extensions-manager cannot get %s (IAM resourceRef kind %s)", gvk.Group, kind)
			}
		}
	}
}

// A kind that writes the API's secrets to a Secret (discokind
// SecretOutputs, docs/design.md decision 38) can: the manager's role
// creates and updates Secrets. envtest has no RBAC, so only this sees it.
func TestManagerRoleWritesSecretOutputs(t *testing.T) {
	role := managerRole(t)
	can := func(verb string) bool {
		for _, rule := range role.Rules {
			if slices.Contains(rule.APIGroups, "") && slices.Contains(rule.Resources, "secrets") && slices.Contains(rule.Verbs, verb) {
				return true
			}
		}
		return false
	}
	for _, k := range discokind.Kinds() {
		if len(k.SecretOutputs) == 0 {
			continue
		}
		for _, verb := range []string{"get", "create", "update"} {
			if !can(verb) {
				t.Errorf("ClusterRole config-connector-extensions-manager cannot %s Secrets, which %s writes its %v to", verb, k.GVK.Kind, k.SecretOutputs)
			}
		}
	}
}

// Every kind a generated kind's reference can name by name (discokind
// Kind.Refs) is readable by the manager, ours and Config Connector's, and
// every reference's target resolves: envtest has no RBAC, so a group
// missing here is a 403 only a real install shows.
func TestManagerRoleReadsEveryReferencedKind(t *testing.T) {
	role := managerRole(t)
	for _, k := range discokind.Kinds() {
		for _, r := range k.Refs {
			if !discokind.RefTargetKnown(r.Target) {
				t.Errorf("%s: %s names %s, which nothing resolves (pkg/discokind refs.go)", k.GVK.Kind, r.Path, r.Target)
				continue
			}
			for _, gvk := range discokind.RefTargetGVKs(r.Target) {
				readable := false
				for _, rule := range role.Rules {
					if slices.Contains(rule.APIGroups, gvk.Group) && slices.Contains(rule.Resources, "*") && slices.Contains(rule.Verbs, "get") {
						readable = true
					}
				}
				if !readable {
					t.Errorf("ClusterRole config-connector-extensions-manager cannot get %s (%s's %s names a %s)", gvk.Group, k.GVK.Kind, r.Path, r.Target)
				}
			}
		}
	}
}
