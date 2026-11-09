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

package compute_test

// ComputeRegionNetworkFirewallPolicy end to end on the fake cloud
// (pkg/test/mockenv, patch 0014). The case: a regional firewall policy for
// a VPC in one region, as fabric's net-firewall-policy module makes with
// region set.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func newRegionFirewallPolicy(name, description string) *computev1alpha1.ComputeRegionNetworkFirewallPolicy {
	return &computev1alpha1.ComputeRegionNetworkFirewallPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: computev1alpha1.ComputeRegionNetworkFirewallPolicySpec{
			ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
			Region:      region,
			Description: mockenv.Ptr(description),
		},
	}
}

func TestComputeRegionNetworkFirewallPolicy(t *testing.T) {
	policyGVK := computev1alpha1.ComputeRegionNetworkFirewallPolicyGVK
	env := mockenv.New(t, namespace)
	policies, err := gcp.NewRegionNetworkFirewallPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { policies.Close() })

	cloud := func(t *testing.T, name string) (*pb.FirewallPolicy, error) {
		t.Helper()
		return policies.Get(env.Ctx, &pb.GetRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: region, FirewallPolicy: name,
		})
	}
	patches := func(name string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/regions/"+region+"/firewallPolicies/"+name) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeRegionNetworkFirewallPolicy {
		t.Helper()
		obj := &computev1alpha1.ComputeRegionNetworkFirewallPolicy{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	policy := newRegionFirewallPolicy("vpc-us-central1", "regional rules for the shared VPC")
	if err := env.Kube.Create(env.Ctx, policy); err != nil {
		t.Fatal(err)
	}

	var fingerprint string
	t.Run("create makes the policy in the region", func(t *testing.T) {
		u := env.WaitReady(policyGVK, namespace, "vpc-us-central1")
		got, err := cloud(t, "vpc-us-central1")
		if err != nil {
			t.Fatalf("reading the policy from the fake cloud: %v", err)
		}
		if got.GetDescription() != "regional rules for the shared VPC" {
			t.Errorf("cloud description = %q", got.GetDescription())
		}
		if !strings.HasSuffix(got.GetRegion(), "/regions/"+region) {
			t.Errorf("cloud region = %q", got.GetRegion())
		}
		selfLink, _, _ := unstructured.NestedString(u.Object, "status", "selfLink")
		if !strings.HasSuffix(selfLink, "projects/"+mockenv.Project+"/regions/"+region+"/firewallPolicies/vpc-us-central1") {
			t.Errorf("status.selfLink = %q", selfLink)
		}
		if id, _, _ := unstructured.NestedString(u.Object, "status", "regionNetworkFirewallPolicyId"); id == "" || !strings.HasSuffix(got.GetSelfLinkWithId(), "/"+id) {
			t.Errorf("status.regionNetworkFirewallPolicyId = %q, cloud selfLinkWithId %q", id, got.GetSelfLinkWithId())
		}
		fingerprint, _, _ = unstructured.NestedString(u.Object, "status", "fingerprint")
		if fingerprint == "" || fingerprint != got.GetFingerprint() {
			t.Errorf("status.fingerprint = %q, cloud %q", fingerprint, got.GetFingerprint())
		}
		if n, _, _ := unstructured.NestedInt64(u.Object, "status", "ruleTupleCount"); n == 0 {
			t.Errorf("status.ruleTupleCount is not set")
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := patches("vpc-us-central1")
		o := getObj(t, "vpc-us-central1")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(policyGVK, namespace, "vpc-us-central1")
		if n := patches("vpc-us-central1") - before; n != 0 {
			t.Errorf("a resync patched the policy %d times", n)
		}
	})

	t.Run("a new description is patched once", func(t *testing.T) {
		before := patches("vpc-us-central1")
		o := getObj(t, "vpc-us-central1")
		o.Spec.Description = mockenv.Ptr("regional rules, reviewed")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool {
			got, err := cloud(t, "vpc-us-central1")
			return err == nil && got.GetDescription() == "regional rules, reviewed"
		})
		u := env.WaitReady(policyGVK, namespace, "vpc-us-central1")
		if n := patches("vpc-us-central1") - before; n != 1 {
			t.Errorf("patched %d times, want once", n)
		}
		if fp, _, _ := unstructured.NestedString(u.Object, "status", "fingerprint"); fp == fingerprint {
			t.Errorf("status.fingerprint did not change after the patch (%q)", fp)
		}
	})

	t.Run("removing the description clears it", func(t *testing.T) {
		o := getObj(t, "vpc-us-central1")
		o.Spec.Description = nil
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool {
			got, err := cloud(t, "vpc-us-central1")
			return err == nil && got.GetDescription() == ""
		})
		env.WaitReady(policyGVK, namespace, "vpc-us-central1")
	})

	t.Run("the region cannot change", func(t *testing.T) {
		o := getObj(t, "vpc-us-central1")
		o.Spec.Region = "europe-west1"
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(policyGVK, namespace, "vpc-us-central1")
		if !strings.Contains(msg, "cannot change") {
			t.Errorf("Ready message = %q, want it to say the identity cannot change", msg)
		}
		if _, err := policies.Get(env.Ctx, &pb.GetRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: "europe-west1", FirewallPolicy: "vpc-us-central1",
		}); err == nil {
			t.Errorf("a policy was made in europe-west1")
		}
		o = getObj(t, "vpc-us-central1")
		o.Spec.Region = region
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(policyGVK, namespace, "vpc-us-central1")
	})

	t.Run("delete removes the policy", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "vpc-us-central1")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRegionNetworkFirewallPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "vpc-us-central1"}})
		if _, err := cloud(t, "vpc-us-central1"); err == nil || !strings.Contains(err.Error(), "404") {
			t.Errorf("reading the deleted policy: got %v, want 404", err)
		}
	})

	t.Run("abandon keeps the policy", func(t *testing.T) {
		k := newRegionFirewallPolicy("kept", "kept")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(policyGVK, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if _, err := cloud(t, "kept"); err != nil {
			t.Errorf("the abandoned policy is gone: %v", err)
		}
	})
}

// waitFor polls cond for up to 30 seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the fake cloud")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
