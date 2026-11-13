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

// IAMAccessPolicy end to end on the fake cloud (pkg/test/mockenv, mockgcp
// patch 0045). The case: the catalog's geap-agent-gateway module, a
// project's access policy that denies a permission to everyone but the
// project's own principals, its rules changed, a change made by hand put
// back, an organization's policy, the CRD's parent rule, delete and
// abandon. Live skips: the live-test account has no access policy role.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/iam/apiv3"
	pb "cloud.google.com/go/iam/apiv3/iampb"
	kcciamv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestIAMAccessPolicy(t *testing.T) {
	if mockenv.Live() {
		t.Skip("an access policy needs iam.accessPolicies.* (roles/iam.accessPolicyAdmin), which the live-test account does not have; live: pending that role")
	}
	gvk := iamv1beta1.IAMAccessPolicyGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewAccessPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	projectName := func(id string) string {
		return "projects/" + mockenv.Project + "/locations/global/accessPolicies/" + id
	}
	get := func(name string) (*pb.AccessPolicy, error) {
		return cloud.GetAccessPolicy(env.Ctx, &pb.GetAccessPolicyRequest{Name: name})
	}
	patches := func(id string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/accessPolicies/"+id) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *iamv1beta1.IAMAccessPolicy {
		t.Helper()
		obj := &iamv1beta1.IAMAccessPolicy{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*iamv1beta1.IAMAccessPolicy)) {
		t.Helper()
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, name)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		}); err != nil {
			t.Fatal(err)
		}
	}
	rule := func(permission string) kcciamv1alpha1.AccessPolicyRule {
		return kcciamv1alpha1.AccessPolicyRule{
			Description:        mockenv.Ptr("only the project's own principals"),
			Effect:             mockenv.Ptr("DENY"),
			Principals:         []string{"principalSet://goog/public:all"},
			ExcludedPrincipals: []string{"principalSet://cloudresourcemanager.googleapis.com/projects/" + mockenv.Project},
			Operation:          &kcciamv1alpha1.AccessPolicyRule_Operation{Permissions: []string{permission}},
		}
	}
	newPolicy := func(name string) *iamv1beta1.IAMAccessPolicy {
		return &iamv1beta1.IAMAccessPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: iamv1beta1.IAMAccessPolicySpec{
				ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
				DisplayName: mockenv.Ptr("Agents call only our tools"),
				Annotations: map[string]string{"made-by": "config-connector-extensions"},
				Details: &kcciamv1alpha1.AccessPolicyDetails{
					Rules: []kcciamv1alpha1.AccessPolicyRule{rule("networkservices.agentGateways.use")},
				},
			},
		}
	}

	const agents = "agents"
	if err := env.Kube.Create(env.Ctx, newPolicy(agents)); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the project's policy", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, agents)
		if ext, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ext != projectName(agents) {
			t.Errorf("status.externalRef = %q, want %q", ext, projectName(agents))
		}
		got, err := get(projectName(agents))
		if err != nil {
			t.Fatalf("reading the policy: %v", err)
		}
		rules := got.GetDetails().GetRules()
		if got.GetDisplayName() != "Agents call only our tools" || len(rules) != 1 || rules[0].GetEffect() != pb.AccessPolicyRule_DENY ||
			len(rules[0].GetExcludedPrincipals()) != 1 || rules[0].GetOperation().GetPermissions()[0] != "networkservices.agentGateways.use" {
			t.Errorf("the policy is %v", got)
		}
		if uid, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "uid"); uid == "" {
			t.Errorf("status.observedState.uid is not set: %v", u.Object["status"])
		}
	})

	t.Run("a resync with nothing changed writes nothing", func(t *testing.T) {
		before := patches(agents)
		update(t, agents, func(o *iamv1beta1.IAMAccessPolicy) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, agents)
		if n := patches(agents); n != before {
			t.Errorf("a resync patched the policy (%d patches, was %d)", n, before)
		}
	})

	t.Run("a changed rule is sent in one update", func(t *testing.T) {
		before := patches(agents)
		update(t, agents, func(o *iamv1beta1.IAMAccessPolicy) {
			o.Spec.Details.Rules = append(o.Spec.Details.Rules, rule("networkservices.agentGateways.get"))
		})
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := get(projectName(agents))
			if err == nil && len(got.GetDetails().GetRules()) == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the policy's rules did not change: %v (%v)", got, err)
			}
			time.Sleep(200 * time.Millisecond)
		}
		env.WaitReady(gvk, namespace, agents)
		if n := patches(agents) - before; n != 1 {
			t.Errorf("%d updates, want 1", n)
		}
	})

	t.Run("a change made by hand is put back", func(t *testing.T) {
		got, err := get(projectName(agents))
		if err != nil {
			t.Fatal(err)
		}
		got.DisplayName = "changed by hand"
		op, err := cloud.UpdateAccessPolicy(env.Ctx, &pb.UpdateAccessPolicyRequest{AccessPolicy: got})
		if err == nil {
			_, err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		update(t, agents, func(o *iamv1beta1.IAMAccessPolicy) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := get(projectName(agents))
			if err == nil && got.GetDisplayName() == "Agents call only our tools" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the display name was not put back: %v (%v)", got, err)
			}
			time.Sleep(200 * time.Millisecond)
		}
	})

	t.Run("an organization's policy", func(t *testing.T) {
		const org = "123456789"
		p := newPolicy("org-agents")
		p.Spec.ProjectRef = nil
		p.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: "organizations/" + org}
		if err := env.Kube.Create(env.Ctx, p); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "org-agents")
		name := "organizations/" + org + "/locations/global/accessPolicies/org-agents"
		if _, err := get(name); err != nil {
			t.Errorf("reading %s: %v", name, err)
		}
		if err := env.Kube.Delete(env.Ctx, getObj(t, "org-agents")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(p)
		if _, err := get(name); err == nil {
			t.Errorf("%s is still there", name)
		}
	})

	t.Run("the CRD needs exactly one parent", func(t *testing.T) {
		p := newPolicy("two-parents")
		p.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: "organizations/123456789"}
		if err := env.Kube.Create(env.Ctx, p); err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef, folderRef and projectRef") {
			t.Errorf("two parents: %v", err)
		}
	})

	t.Run("abandon keeps the policy", func(t *testing.T) {
		kept := newPolicy("kept")
		kept.Annotations = map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, getObj(t, "kept")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(kept)
		if _, err := get(projectName("kept")); err != nil {
			t.Errorf("the abandoned policy is gone: %v", err)
		}
	})

	t.Run("delete removes the policy", func(t *testing.T) {
		o := getObj(t, agents)
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
		if _, err := get(projectName(agents)); err == nil {
			t.Errorf("the policy is still there")
		}
	})
}
