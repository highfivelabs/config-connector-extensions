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
)

// Every kind's group is in the manager's ClusterRole, so a new kind cannot
// ship without the rights to reconcile it.
func TestManagerRoleCoversEveryKind(t *testing.T) {
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
		if r.Kind == "ClusterRole" && r.Name == "gaps-gcp-manager" {
			role = &r
			break
		}
	}
	if role == nil {
		t.Fatal("config/operator/rbac.yaml has no ClusterRole gaps-gcp-manager")
	}
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
			t.Errorf("ClusterRole gaps-gcp-manager does not cover %s (objects: %v, status: %v)", k.gvk.Group, crud, status)
		}
	}
}
