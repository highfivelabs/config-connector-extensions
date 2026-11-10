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

// IAMPrincipalAccessBoundaryPolicy end to end on the fake cloud
// (pkg/test/mockenv, mockgcp patch 0029). The case: an organization's
// boundary that lets its principals use resources only in the
// organization. A policy lives under an organization, which a live run
// never touches (docs/live-runs.md): live is pending the test folder and
// organization-level access, so a live run skips the test.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/iam/apiv3"
	pb "cloud.google.com/go/iam/apiv3/iampb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestIAMPrincipalAccessBoundaryPolicy(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a principal access boundary policy lives under an organization, which a live run never touches (docs/live-runs.md); live: pending, needs organization-level access")
	}
	const org = "123456789"
	gvk := iamv1beta1.IAMPrincipalAccessBoundaryPolicyGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewPrincipalAccessBoundaryPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	cloudName := func(id string) string {
		return "organizations/" + org + "/locations/global/principalAccessBoundaryPolicies/" + id
	}
	get := func(id string) (*pb.PrincipalAccessBoundaryPolicy, error) {
		return cloud.GetPrincipalAccessBoundaryPolicy(env.Ctx, &pb.GetPrincipalAccessBoundaryPolicyRequest{Name: cloudName(id)})
	}
	patches := func(id string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/principalAccessBoundaryPolicies/"+id) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *iamv1beta1.IAMPrincipalAccessBoundaryPolicy {
		t.Helper()
		obj := &iamv1beta1.IAMPrincipalAccessBoundaryPolicy{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*iamv1beta1.IAMPrincipalAccessBoundaryPolicy)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	newPolicy := func(name string) *iamv1beta1.IAMPrincipalAccessBoundaryPolicy {
		return &iamv1beta1.IAMPrincipalAccessBoundaryPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: iamv1beta1.IAMPrincipalAccessBoundaryPolicySpec{
				OrganizationRef: &refsv1beta1.OrganizationRef{External: "organizations/" + org},
				Location:        mockenv.Ptr("global"),
				DisplayName:     mockenv.Ptr("Only our organization"),
				Details: &iamv1beta1.PrincipalAccessBoundaryPolicyDetails{
					Rules: []iamv1beta1.PrincipalAccessBoundaryPolicyRule{{
						Description: mockenv.Ptr("the organization and everything in it"),
						Resources:   []string{"//cloudresourcemanager.googleapis.com/organizations/" + org},
						Effect:      mockenv.Ptr("ALLOW"),
					}},
				},
			},
		}
	}

	const boundary = "org-boundary"
	if err := env.Kube.Create(env.Ctx, newPolicy(boundary)); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the policy under the organization", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, boundary)
		got, err := get(boundary)
		if err != nil {
			t.Fatalf("reading the policy: %v", err)
		}
		if got.GetDisplayName() != "Only our organization" {
			t.Errorf("display name = %q", got.GetDisplayName())
		}
		rules := got.GetDetails().GetRules()
		if len(rules) != 1 || rules[0].GetEffect() != pb.PrincipalAccessBoundaryPolicyRule_ALLOW || len(rules[0].GetResources()) != 1 {
			t.Errorf("rules = %v", rules)
		}
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != cloudName(boundary) {
			t.Errorf("status.externalRef = %q, want %q", ref, cloudName(boundary))
		}
		if uid, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "uid"); uid == "" {
			t.Error("status.observedState.uid is not set")
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := patches(boundary)
		update(t, boundary, func(o *iamv1beta1.IAMPrincipalAccessBoundaryPolicy) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, boundary)
		if n := patches(boundary) - before; n != 0 {
			t.Errorf("a resync patched %d times", n)
		}
	})

	t.Run("changed rules and name are patched", func(t *testing.T) {
		update(t, boundary, func(o *iamv1beta1.IAMPrincipalAccessBoundaryPolicy) {
			o.Spec.DisplayName = mockenv.Ptr("Our organization and a partner")
			o.Spec.Details.Rules = append(o.Spec.Details.Rules, iamv1beta1.PrincipalAccessBoundaryPolicyRule{
				Description: mockenv.Ptr("a partner's project"),
				Resources:   []string{"//cloudresourcemanager.googleapis.com/projects/partner-project"},
				Effect:      mockenv.Ptr("ALLOW"),
			})
		})
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := get(boundary)
			if err == nil && len(got.GetDetails().GetRules()) == 2 && got.GetDisplayName() == "Our organization and a partner" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the policy was not patched: %v, %v", got, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
		env.WaitReady(gvk, namespace, boundary)
	})

	t.Run("the organization, location and ID cannot change", func(t *testing.T) {
		for _, c := range []struct {
			field  string
			change func(*iamv1beta1.IAMPrincipalAccessBoundaryPolicy)
		}{
			{"organizationRef", func(o *iamv1beta1.IAMPrincipalAccessBoundaryPolicy) {
				o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: "organizations/987654321"}
			}},
			{"location", func(o *iamv1beta1.IAMPrincipalAccessBoundaryPolicy) { o.Spec.Location = mockenv.Ptr("us-central1") }},
			{"resourceID", func(o *iamv1beta1.IAMPrincipalAccessBoundaryPolicy) { o.Spec.ResourceID = mockenv.Ptr("other") }},
		} {
			o := getObj(t, boundary)
			c.change(o)
			err := env.Kube.Update(env.Ctx, o)
			if c.field == "resourceID" {
				// resourceID was unset, so setting it is a change only CEL on
				// an existing value catches; the identity check refuses it.
				if err == nil {
					if msg := env.WaitNotReady(gvk, namespace, boundary); !strings.Contains(msg, "cannot change") {
						t.Errorf("setting resourceID: Ready message = %q", msg)
					}
					update(t, boundary, func(o *iamv1beta1.IAMPrincipalAccessBoundaryPolicy) { o.Spec.ResourceID = nil })
					env.WaitReady(gvk, namespace, boundary)
				}
				continue
			}
			if err == nil || !strings.Contains(err.Error(), c.field+" is immutable") {
				t.Errorf("changing %s: err = %v", c.field, err)
			}
		}
	})

	t.Run("delete removes the policy", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, boundary)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&iamv1beta1.IAMPrincipalAccessBoundaryPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: boundary}})
		if _, err := get(boundary); err == nil {
			t.Error("the policy is still there")
		}
	})

	t.Run("abandon keeps the policy", func(t *testing.T) {
		kept := newPolicy("kept")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(kept)
		if _, err := get("kept"); err != nil {
			t.Errorf("the abandoned policy is gone: %v", err)
		}
	})
}
