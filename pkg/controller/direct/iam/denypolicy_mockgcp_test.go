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

package iam_test

// IAMDenyPolicy end to end on the fake cloud (pkg/test/mockenv; mockgcp
// serves IAM v2 deny policies unpatched) or a real project
// (docs/live-runs.md). The case: a landing zone's guardrail, a deny policy
// on a project (and on its folder and organization) that keeps everyone but
// the platform's own service account from deleting custom roles.
//
// A live run needs iam.denypolicies.* on the project, which only the Deny
// Admin role (roles/iam.denyAdmin) gives and which is granted on an
// organization; the live-test service account does not have it, so a live
// run skips the test unless E2E_IAM_DENY_ADMIN=true. The folder's and the
// organization's policies run in the fake cloud only.

import (
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/iam/apiv2"
	pb "cloud.google.com/go/iam/apiv2/iampb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestIAMDenyPolicy(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_IAM_DENY_ADMIN") != "true" {
		t.Skip("a live IAMDenyPolicy needs iam.denypolicies.* (roles/iam.denyAdmin, granted on the organization) for the live-test service account; set E2E_IAM_DENY_ADMIN=true where it has it (docs/live-runs.md)")
	}
	denyGVK := iamv1beta1.IAMDenyPolicyGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	platformSA := mockenv.ServiceAccount(t)
	attachment := func(parent string) string {
		return url.PathEscape("cloudresourcemanager.googleapis.com/" + parent)
	}
	policyName := func(parent, id string) string {
		return "policies/" + attachment(parent) + "/denypolicies/" + id
	}
	projectPolicy := func(name string) string { return policyName("projects/"+mockenv.Project, mockenv.Name(name)) }
	get := func(name string) (*pb.Policy, error) {
		return cloud.GetPolicy(env.Ctx, &pb.GetPolicyRequest{Name: name})
	}
	// Delete what the test made, the abandoned policy included, and what a
	// failed step left.
	t.Cleanup(func() {
		for _, name := range []string{"no-role-deletes", "kept"} {
			op, err := cloud.DeletePolicy(env.Ctx, &pb.DeletePolicyRequest{Name: projectPolicy(name)})
			if err == nil {
				_, err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting deny policy %s: %v", projectPolicy(name), err)
			}
		}
	})
	// puts counts the updates (PUT) of the policy with this ID.
	puts := func(id string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PUT ") && strings.Contains(r, "/denypolicies/"+id+"?") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *iamv1beta1.IAMDenyPolicy {
		t.Helper()
		obj := &iamv1beta1.IAMDenyPolicy{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	newPolicy := func(name string, permissions ...string) *iamv1beta1.IAMDenyPolicy {
		return &iamv1beta1.IAMDenyPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: iamv1beta1.IAMDenyPolicySpec{
				ProjectRef:  &refsv1beta1.ProjectRef{External: "projects/" + mockenv.Project},
				ResourceID:  mockenv.Ptr(mockenv.Name(name)),
				DisplayName: mockenv.Ptr("No custom role deletes"),
				Rules: []iamv1beta1.PolicyRule{{
					Description: mockenv.Ptr("only the platform deletes custom roles"),
					DenyRule: &iamv1beta1.DenyRule{
						DeniedPrincipals:    []string{"principalSet://goog/public:all"},
						ExceptionPrincipals: []string{"principal://iam.googleapis.com/projects/-/serviceAccounts/" + platformSA},
						DeniedPermissions:   permissions,
					},
				}},
			},
		}
	}

	if err := env.Kube.Create(env.Ctx, newPolicy("no-role-deletes", "iam.googleapis.com/roles.delete")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the project's deny policy", func(t *testing.T) {
		u := env.WaitReady(denyGVK, namespace, "no-role-deletes")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != projectPolicy("no-role-deletes") {
			t.Errorf("status.externalRef = %q, want %q", ref, projectPolicy("no-role-deletes"))
		}
		got, err := get(projectPolicy("no-role-deletes"))
		if err != nil {
			t.Fatalf("reading the deny policy: %v", err)
		}
		if got.GetDisplayName() != "No custom role deletes" || len(got.GetRules()) != 1 {
			t.Fatalf("cloud display name, rules = %q, %v", got.GetDisplayName(), got.GetRules())
		}
		rule := got.GetRules()[0]
		if rule.GetDescription() != "only the platform deletes custom roles" ||
			!slices.Equal(rule.GetDenyRule().GetDeniedPermissions(), []string{"iam.googleapis.com/roles.delete"}) ||
			!slices.Equal(rule.GetDenyRule().GetDeniedPrincipals(), []string{"principalSet://goog/public:all"}) ||
			len(rule.GetDenyRule().GetExceptionPrincipals()) != 1 {
			t.Errorf("cloud rule = %v", rule)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := puts(mockenv.Name("no-role-deletes"))
		o := getObj(t, "no-role-deletes")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(denyGVK, namespace, "no-role-deletes")
		if n := puts(mockenv.Name("no-role-deletes")) - before; n != 0 {
			t.Errorf("a resync updated the deny policy %d times", n)
		}
	})

	t.Run("a changed rule and display name are one update", func(t *testing.T) {
		before := puts(mockenv.Name("no-role-deletes"))
		o := getObj(t, "no-role-deletes")
		o.Spec.DisplayName = mockenv.Ptr("No custom role deletes or undeletes")
		o.Spec.Rules[0].DenyRule.DeniedPermissions = []string{"iam.googleapis.com/roles.delete", "iam.googleapis.com/roles.undelete"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := get(projectPolicy("no-role-deletes"))
			if err != nil {
				t.Fatal(err)
			}
			if got.GetDisplayName() == "No custom role deletes or undeletes" && len(got.GetRules()[0].GetDenyRule().GetDeniedPermissions()) == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("cloud display name, rules = %q, %v", got.GetDisplayName(), got.GetRules())
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(denyGVK, namespace, "no-role-deletes")
		if n := puts(mockenv.Name("no-role-deletes")) - before; n != 1 {
			t.Errorf("the change updated the deny policy %d times, want 1", n)
		}
	})

	t.Run("exactly one parent", func(t *testing.T) {
		for name, mutate := range map[string]func(*iamv1beta1.IAMDenyPolicy){
			"project-and-folder": func(o *iamv1beta1.IAMDenyPolicy) {
				o.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/123456"}
			},
			"none": func(o *iamv1beta1.IAMDenyPolicy) { o.Spec.ProjectRef = nil },
		} {
			o := newPolicy(name, "iam.googleapis.com/roles.delete")
			mutate(o)
			err := env.Kube.Create(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), "exactly one of projectRef, folderRef and organizationRef") {
				t.Errorf("creating with %s: got %v, want the API server to refuse it", name, err)
			}
		}
	})

	t.Run("the folder's and the organization's deny policies", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never touches a folder or an organization (docs/live-runs.md)")
		}
		for _, c := range []struct {
			name, parent string
			set          func(*iamv1beta1.IAMDenyPolicy)
		}{
			{"folder-guardrail", "folders/123456", func(o *iamv1beta1.IAMDenyPolicy) {
				o.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/123456"}
			}},
			{"org-guardrail", "organizations/987654", func(o *iamv1beta1.IAMDenyPolicy) {
				o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: "organizations/987654"}
			}},
		} {
			o := newPolicy(c.name, "iam.googleapis.com/roles.delete")
			o.Spec.ProjectRef = nil
			o.Spec.ResourceID = nil
			c.set(o)
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
			want := policyName(c.parent, c.name)
			u := env.WaitReady(denyGVK, namespace, c.name)
			if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
				t.Errorf("%s: status.externalRef = %q, want %q (the ID is metadata.name without resourceID)", c.name, ref, want)
			}
			if _, err := get(want); err != nil {
				t.Errorf("%s: reading the deny policy: %v", c.name, err)
			}
			if err := env.Kube.Delete(env.Ctx, getObj(t, c.name)); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(&iamv1beta1.IAMDenyPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: c.name}})
			if _, err := get(want); !direct.IsNotFound(err) {
				t.Errorf("%s: after delete, reading the deny policy: %v, want not found", c.name, err)
			}
		}
	})

	t.Run("a changed parent is refused", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("the new parent would be a folder")
		}
		o := getObj(t, "no-role-deletes")
		o.Spec.ProjectRef = nil
		o.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/123456"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(denyGVK, namespace, "no-role-deletes"); !strings.Contains(msg, "cannot change IAMDenyPolicy identity") {
			t.Errorf("Ready message = %q, want the change refused", msg)
		}
		o = getObj(t, "no-role-deletes")
		o.Spec.FolderRef = nil
		o.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: "projects/" + mockenv.Project}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(denyGVK, namespace, "no-role-deletes")
	})

	t.Run("delete deletes the deny policy", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "no-role-deletes")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&iamv1beta1.IAMDenyPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "no-role-deletes"}})
		if _, err := get(projectPolicy("no-role-deletes")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the deny policy: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the deny policy", func(t *testing.T) {
		kept := newPolicy("kept", "iam.googleapis.com/roles.delete")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(denyGVK, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, getObj(t, "kept")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&iamv1beta1.IAMDenyPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(projectPolicy("kept")); err != nil {
			t.Errorf("after abandon, reading the deny policy: %v, want it kept", err)
		}
	})
}
