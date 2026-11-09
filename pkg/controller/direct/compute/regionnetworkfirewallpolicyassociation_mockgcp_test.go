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

// ComputeRegionNetworkFirewallPolicyAssociation end to end on the fake
// cloud (pkg/test/mockenv, patch 0021) or a real project
// (docs/live-runs.md). The case: a regional policy attached to a VPC
// network of the run's own, as fabric's net-firewall-policy module does.
// The policy is the run's own object (policies take no labels, so the
// run's name prefix is how the sweeper finds it), the network is made and
// deleted by the test.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeRegionNetworkFirewallPolicyAssociation(t *testing.T) {
	policyGVK := computev1alpha1.ComputeRegionNetworkFirewallPolicyGVK
	assocGVK := computev1alpha1.ComputeRegionNetworkFirewallPolicyAssociationGVK
	env := mockenv.New(t, namespace)
	policies, err := gcp.NewRegionNetworkFirewallPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { policies.Close() })

	// The networks are made before the policy, so they are deleted after
	// it (cleanups run last first): a network with a policy attached
	// cannot be deleted.
	network := newNetwork(t, env, "assoc-net")
	other := newNetwork(t, env, "assoc-net2")

	const policyObj = "assoc-policy"
	policyName := mockenv.Name(policyObj)
	if err := env.Kube.Create(env.Ctx, newRegionFirewallPolicy(policyObj, "associations under test")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		op, err := policies.Delete(env.Ctx, &pb.DeleteRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: region, FirewallPolicy: policyName,
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting policy %s: %v", policyName, err)
		}
	})
	env.WaitReady(policyGVK, namespace, policyObj)

	cloudAssociation := func(name string) (*pb.FirewallPolicyAssociation, error) {
		return policies.GetAssociation(env.Ctx, &pb.GetAssociationRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: region, FirewallPolicy: policyName, Name: mockenv.Ptr(name),
		})
	}
	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/firewallPolicies/"+policyName+"/") &&
				(strings.Contains(r, "/addAssociation") || strings.Contains(r, "/removeAssociation")) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeRegionNetworkFirewallPolicyAssociation {
		t.Helper()
		obj := &computev1alpha1.ComputeRegionNetworkFirewallPolicyAssociation{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	newAssociation := func(name, networkPath string) *computev1alpha1.ComputeRegionNetworkFirewallPolicyAssociation {
		return &computev1alpha1.ComputeRegionNetworkFirewallPolicyAssociation{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1alpha1.ComputeRegionNetworkFirewallPolicyAssociationSpec{
				FirewallPolicyRef: computev1alpha1.ComputeRegionNetworkFirewallPolicyRef{Name: policyObj},
				NetworkRef:        computerefs.ComputeNetworkRef{External: networkPath},
				ResourceID:        mockenv.Ptr(mockenv.Name(name)),
			},
		}
	}

	const attached = "vpc"
	assocName := mockenv.Name(attached)
	if err := env.Kube.Create(env.Ctx, newAssociation(attached, network)); err != nil {
		t.Fatal(err)
	}

	t.Run("create attaches the policy to the network", func(t *testing.T) {
		u := env.WaitReady(assocGVK, namespace, attached)
		got, err := cloudAssociation(assocName)
		if err != nil {
			t.Fatalf("reading the association from the cloud: %v", err)
		}
		if !strings.HasSuffix(got.GetAttachmentTarget(), network) {
			t.Errorf("cloud attachment target = %q, want one ending in %q", got.GetAttachmentTarget(), network)
		}
		want := "projects/" + mockenv.Project + "/regions/" + region + "/firewallPolicies/" + policyName + "/associations/" + assocName
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		t.Logf("the association as the API returns it: %v", got)
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		o := getObj(t, attached)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(assocGVK, namespace, attached)
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("a second network gets its own association", func(t *testing.T) {
		const second = "vpc2"
		if err := env.Kube.Create(env.Ctx, newAssociation(second, other)); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(assocGVK, namespace, second)
		got, err := cloudAssociation(mockenv.Name(second))
		if err != nil || !strings.HasSuffix(got.GetAttachmentTarget(), other) {
			t.Errorf("the second association: %v, %v", got, err)
		}
		if first, err := cloudAssociation(assocName); err != nil || !strings.HasSuffix(first.GetAttachmentTarget(), network) {
			t.Errorf("the first association changed: %v, %v", first, err)
		}
		// Deleting one leaves the other.
		obj := getObj(t, second)
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		if _, err := cloudAssociation(mockenv.Name(second)); err == nil {
			t.Error("the deleted association is still in the policy")
		} else {
			// What the API answers for a name with no association: the
			// controller takes it, a 404, or a 400 that says so, as "not
			// there" (docs/live-runs.md).
			t.Logf("the answer for a name with no association: %v", err)
		}
		if _, err := cloudAssociation(assocName); err != nil {
			t.Errorf("deleting one association took the other: %v", err)
		}
	})

	t.Run("the policy, the network and the name cannot change", func(t *testing.T) {
		o := getObj(t, attached)
		o.Spec.NetworkRef = computerefs.ComputeNetworkRef{External: other}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "networkRef is immutable") {
			t.Errorf("changing the network: err = %v", err)
		}
		o = getObj(t, attached)
		o.Spec.FirewallPolicyRef = computev1alpha1.ComputeRegionNetworkFirewallPolicyRef{External: "projects/" + mockenv.Project + "/regions/" + region + "/firewallPolicies/other"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Errorf("changing the policy: err = %v", err)
		}
		o = getObj(t, attached)
		o.Spec.ResourceID = mockenv.Ptr("renamed")
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "resourceID is immutable") {
			t.Errorf("changing the name: err = %v", err)
		}
	})

	t.Run("an association removed out of band is made again", func(t *testing.T) {
		op, err := policies.RemoveAssociation(env.Ctx, &pb.RemoveAssociationRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: region, FirewallPolicy: policyName, Name: mockenv.Ptr(assocName),
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("removing the association out of band: %v", err)
		}
		o := getObj(t, attached)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { _, err := cloudAssociation(assocName); return err == nil })
		env.WaitReady(assocGVK, namespace, attached)
	})

	t.Run("a name taken for another network is an error, not a move", func(t *testing.T) {
		taken := newAssociation("taken", other)
		taken.Spec.ResourceID = mockenv.Ptr(assocName)
		if err := env.Kube.Create(env.Ctx, taken); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(assocGVK, namespace, "taken")
		if !strings.Contains(msg, network) {
			t.Errorf("the message does not name the network it is attached to: %q", msg)
		}
		if got, err := cloudAssociation(assocName); err != nil || !strings.HasSuffix(got.GetAttachmentTarget(), network) {
			t.Errorf("the first association moved: %v, %v", got, err)
		}
		// Taking the object away leaves the first one's association: the
		// two share a cloud name, so deleting "taken" would remove it; it
		// is abandoned first.
		o := getObj(t, "taken")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
		if _, err := cloudAssociation(assocName); err != nil {
			t.Errorf("deleting the object that could not take the name removed the association: %v", err)
		}
	})

	t.Run("delete detaches the policy", func(t *testing.T) {
		obj := getObj(t, attached)
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		if _, err := cloudAssociation(assocName); err == nil {
			t.Error("the association is still in the policy")
		}
	})

	t.Run("abandon keeps the association", func(t *testing.T) {
		kept := newAssociation("kept", network)
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(assocGVK, namespace, "kept")
		// The abandoned association is the test's to remove: a policy with
		// a network attached cannot be deleted.
		t.Cleanup(func() {
			op, err := policies.RemoveAssociation(env.Ctx, &pb.RemoveAssociationRegionNetworkFirewallPolicyRequest{
				Project: mockenv.Project, Region: region, FirewallPolicy: policyName, Name: mockenv.Ptr(mockenv.Name("kept")),
			})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) && !strings.Contains(err.Error(), "association") {
				t.Errorf("removing the abandoned association: %v", err)
			}
		})
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(kept)
		if _, err := cloudAssociation(mockenv.Name("kept")); err != nil {
			t.Errorf("the abandoned association is gone: %v", err)
		}
	})
}
