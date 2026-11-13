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

// IAMPolicyBinding of an access policy to a resource, on the fake cloud
// (mockgcp patches 0045 and 0046). The case: the catalog's
// geap-agent-gateway module, a project's IAMAccessPolicy bound by name to
// the project and to an agent gateway, the CRD's target rule, and the
// policy and its bindings deleted at once. Live skips: the live-test
// account has no access policy role.

import (
	"strings"
	"testing"

	gcp "cloud.google.com/go/iam/apiv3"
	pb "cloud.google.com/go/iam/apiv3/iampb"
	kcciamv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	iamv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestIAMPolicyBindingOfAnAccessPolicy(t *testing.T) {
	if mockenv.Live() {
		t.Skip("binding an access policy needs iam.accessPolicies.* and iam.policyBindings.* on the project, which the live-test account does not have; live: pending those roles")
	}
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewPolicyBindingsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })
	policies, err := gcp.NewAccessPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { policies.Close() })

	parent := "projects/" + mockenv.Project
	policyName := parent + "/locations/global/accessPolicies/agents"
	bindingName := func(id string) string { return parent + "/locations/global/policyBindings/" + id }
	gateway := "//networkservices.googleapis.com/projects/" + mockenv.Project + "/locations/us-central1/agentGateways/agents"

	policy := &iamv1beta1.IAMAccessPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "agents"},
		Spec: iamv1beta1.IAMAccessPolicySpec{
			ProjectRef: &refsv1beta1.ProjectRef{External: mockenv.Project},
			Details: &kcciamv1alpha1.AccessPolicyDetails{Rules: []kcciamv1alpha1.AccessPolicyRule{{
				Effect:     mockenv.Ptr("DENY"),
				Principals: []string{"principalSet://goog/public:all"},
				Operation:  &kcciamv1alpha1.AccessPolicyRule_Operation{Permissions: []string{"networkservices.agentGateways.use"}},
			}}},
		},
	}
	if err := env.Kube.Create(env.Ctx, policy); err != nil {
		t.Fatal(err)
	}
	newBinding := func(name, resource string) *iamv1alpha1.IAMPolicyBinding {
		return &iamv1alpha1.IAMPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: iamv1alpha1.IAMPolicyBindingSpec{
				ProjectRef: &refsv1beta1.ProjectRef{External: mockenv.Project},
				Target:     &iamv1alpha1.PolicyBinding_Target{Resource: mockenv.Ptr(resource)},
				PolicyKind: mockenv.Ptr("ACCESS"),
				PolicyRef:  &iamv1alpha1.PolicyBindingPolicyRef{Name: "agents"},
			},
		}
	}
	toProject := newBinding("agents-project", "//cloudresourcemanager.googleapis.com/projects/"+mockenv.Project)
	toGateway := newBinding("agents-gateway", gateway)
	for _, b := range []*iamv1alpha1.IAMPolicyBinding{toProject, toGateway} {
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("an access policy binds by name to the project and to a gateway", func(t *testing.T) {
		env.WaitReady(iamv1beta1.IAMAccessPolicyGVK, namespace, "agents")
		for _, c := range []struct{ obj, resource string }{
			{"agents-project", "//cloudresourcemanager.googleapis.com/projects/" + mockenv.Project},
			{"agents-gateway", gateway},
		} {
			u := env.WaitReady(iamv1alpha1.IAMPolicyBindingGVK, namespace, c.obj)
			if ext, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ext != bindingName(c.obj) {
				t.Errorf("%s: status.externalRef = %q", c.obj, ext)
			}
			got, err := cloud.GetPolicyBinding(env.Ctx, &pb.GetPolicyBindingRequest{Name: bindingName(c.obj)})
			if err != nil {
				t.Fatalf("reading %s: %v", c.obj, err)
			}
			if got.GetTarget().GetResource() != c.resource || got.GetPolicy() != policyName || got.GetPolicyKind() != pb.PolicyBinding_ACCESS {
				t.Errorf("%s: the binding is %v", c.obj, got)
			}
			if kind, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "policyKind"); kind != "ACCESS" {
				t.Errorf("%s: status.observedState.policyKind = %q", c.obj, kind)
			}
		}
	})

	t.Run("the CRD takes exactly one target", func(t *testing.T) {
		both := newBinding("both", gateway)
		both.Spec.Target.PrincipalSet = mockenv.Ptr("//cloudresourcemanager.googleapis.com/projects/" + mockenv.Project)
		if err := env.Kube.Create(env.Ctx, both); err == nil || !strings.Contains(err.Error(), "exactly one of principalSet and resource") {
			t.Errorf("two targets: %v", err)
		}
	})

	t.Run("the policy and its bindings deleted at once all go", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, policy); err != nil {
			t.Fatal(err)
		}
		for _, b := range []*iamv1alpha1.IAMPolicyBinding{toProject, toGateway} {
			if err := env.Kube.Delete(env.Ctx, b); err != nil {
				t.Fatal(err)
			}
		}
		env.WaitGone(toProject)
		env.WaitGone(toGateway)
		env.WaitGone(policy)
		for _, id := range []string{"agents-project", "agents-gateway"} {
			if _, err := cloud.GetPolicyBinding(env.Ctx, &pb.GetPolicyBindingRequest{Name: bindingName(id)}); err == nil {
				t.Errorf("binding %s is still there", id)
			}
		}
		if _, err := policies.GetAccessPolicy(env.Ctx, &pb.GetAccessPolicyRequest{Name: policyName}); err == nil {
			t.Errorf("the policy is still there")
		}
	})
}
