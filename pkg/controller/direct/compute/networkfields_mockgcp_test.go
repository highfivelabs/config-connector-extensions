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

// ComputeNetworkFields and ComputeSubnetworkFields end to end on the fake
// cloud (pkg/test/mockenv, patch 0056) or a real project
// (docs/live-runs.md). The network and subnet are the run's own
// (mockenv.Name), cost nothing, and are deleted at the end.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// configConnectorObject makes a Config Connector object that is not
// reconciled here, with the status.selfLink its Terraform-based controller
// would write, for a field kind's resourceRef by name.
func configConnectorObject(t *testing.T, env *mockenv.Env, gvk schema.GroupVersionKind, namespace, name string, spec map[string]any, selfLink string) {
	t.Helper()
	u := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	if err := env.Kube.Create(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(u.Object, selfLink, "status", "selfLink"); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Status().Update(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
}

// forceResync makes the manager reconcile an object again and waits for it
// to be Ready.
func forceResync(t *testing.T, env *mockenv.Env, obj client.Object, gvk schema.GroupVersionKind) {
	t.Helper()
	if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		t.Fatal(err)
	}
	obj.SetAnnotations(map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()})
	if err := env.Kube.Update(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Second)
	env.WaitReady(gvk, obj.GetNamespace(), obj.GetName())
}

func patchesOf(env *mockenv.Env, path string) []string {
	var out []string
	for _, req := range env.Requests() {
		if strings.HasPrefix(req, "PATCH ") && (strings.Contains(req, path+"?") || strings.HasSuffix(req, path)) {
			out = append(out, req)
		}
	}
	return out
}

func TestComputeNetworkFields(t *testing.T) {
	const namespace = "example"
	gvk := krm.ComputeNetworkFieldsGVK
	env := mockenv.New(t, namespace)
	networkName := newNetwork(t, env, "nf")
	short := networkName[strings.LastIndex(networkName, "/")+1:]
	networks, err := gcp.NewNetworksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { networks.Close() })
	get := func(t *testing.T) *pb.NetworkRoutingConfig {
		t.Helper()
		n, err := networks.Get(env.Ctx, &pb.GetNetworkRequest{Project: mockenv.Project, Network: short})
		if err != nil {
			t.Fatalf("reading the network: %v", err)
		}
		return n.GetRoutingConfig()
	}
	patches := func() []string { return patchesOf(env, "/global/networks/"+short) }
	configConnectorObject(t, env, schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ComputeNetwork"},
		namespace, "network", map[string]any{"resourceID": short, "autoCreateSubnetworks": false}, "https://www.googleapis.com/compute/v1/"+networkName)

	newFields := func(name string, ref *computerefs.ComputeNetworkRef, rc *krm.NetworkRoutingConfig) *krm.ComputeNetworkFields {
		return &krm.ComputeNetworkFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec:       krm.ComputeNetworkFieldsSpec{ResourceRef: ref, RoutingConfig: rc},
		}
	}

	t.Run("the MED fields without STANDARD are refused by the CRD", func(t *testing.T) {
		bad := newFields("bad", &computerefs.ComputeNetworkRef{External: networkName}, &krm.NetworkRoutingConfig{BgpAlwaysCompareMed: mockenv.Ptr(true)})
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "need bgpBestPathSelectionMode STANDARD") {
			t.Errorf("creating MED fields without STANDARD: %v, want refused", err)
		}
	})

	obj := newFields("fields", &computerefs.ComputeNetworkRef{Name: "network"}, &krm.NetworkRoutingConfig{
		BgpBestPathSelectionMode: mockenv.Ptr("STANDARD"), BgpAlwaysCompareMed: mockenv.Ptr(true), BgpInterRegionCost: mockenv.Ptr("ADD_COST_TO_MED"),
	})
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the BGP fields, and only them", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != networkName {
			t.Errorf("status.externalRef = %q, want %q", ref, networkName)
		}
		rc := get(t)
		if rc.GetBgpBestPathSelectionMode() != "STANDARD" || !rc.GetBgpAlwaysCompareMed() || rc.GetBgpInterRegionCost() != "ADD_COST_TO_MED" {
			t.Errorf("routingConfig = %v", rc)
		}
		if rc.GetRoutingMode() != "REGIONAL" {
			t.Errorf("routingMode = %q, want REGIONAL, untouched", rc.GetRoutingMode())
		}
		if p := patches(); len(p) != 1 {
			t.Errorf("patches = %v, want one", p)
		}
	})

	t.Run("Terraform's routingMode update keeps them", func(t *testing.T) {
		// What Config Connector's ComputeNetwork (Terraform) sends when
		// routingMode changes: routingConfig with routingMode alone.
		op, err := networks.Patch(env.Ctx, &pb.PatchNetworkRequest{Project: mockenv.Project, Network: short,
			NetworkResource: &pb.Network{RoutingConfig: &pb.NetworkRoutingConfig{RoutingMode: mockenv.Ptr("GLOBAL")}}})
		waitOp(t, env, "Terraform's patch", op, err)
		rc := get(t)
		if rc.GetRoutingMode() != "GLOBAL" || rc.GetBgpBestPathSelectionMode() != "STANDARD" || !rc.GetBgpAlwaysCompareMed() {
			t.Errorf("after Terraform's patch routingConfig = %v", rc)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		forceResync(t, env, &krm.ComputeNetworkFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "fields"}}, gvk)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the network %d times", n)
		}
	})

	t.Run("a changed field is patched, and one left out is not owned", func(t *testing.T) {
		o := &krm.ComputeNetworkFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "fields"}, o); err != nil {
			t.Fatal(err)
		}
		o.Spec.RoutingConfig.BgpAlwaysCompareMed = nil
		o.Spec.RoutingConfig.BgpInterRegionCost = mockenv.Ptr("DEFAULT")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return get(t).GetBgpInterRegionCost() == "DEFAULT" })
		env.WaitReady(gvk, namespace, "fields")
		if !get(t).GetBgpAlwaysCompareMed() {
			t.Errorf("a field the spec no longer sets was changed")
		}
		if rc := get(t); rc.GetRoutingMode() != "GLOBAL" {
			t.Errorf("routingMode = %q, want Config Connector's GLOBAL", rc.GetRoutingMode())
		}
	})

	t.Run("a second object for the network is refused", func(t *testing.T) {
		dup := newFields("dup", &computerefs.ComputeNetworkRef{External: networkName}, &krm.NetworkRoutingConfig{BgpBestPathSelectionMode: mockenv.Ptr("LEGACY")})
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if get(t).GetBgpBestPathSelectionMode() != "STANDARD" {
			t.Errorf("the refused object changed the network")
		}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing network is waited for, never made", func(t *testing.T) {
		missing := "projects/" + mockenv.Project + "/global/networks/" + mockenv.Name("never-made")
		w := newFields("waits", &computerefs.ComputeNetworkRef{External: missing}, &krm.NetworkRoutingConfig{BgpBestPathSelectionMode: mockenv.Ptr("STANDARD")})
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing network waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete and abandon leave the fields", func(t *testing.T) {
		before := len(patches())
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		k := newFields("kept", &computerefs.ComputeNetworkRef{Name: "network"}, &krm.NetworkRoutingConfig{BgpBestPathSelectionMode: mockenv.Ptr("STANDARD")})
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("delete and abandon patched the network %d times", n)
		}
		if rc := get(t); rc.GetBgpBestPathSelectionMode() != "STANDARD" || rc.GetBgpInterRegionCost() != "DEFAULT" {
			t.Errorf("after delete routingConfig = %v", rc)
		}
	})
}

func TestComputeSubnetworkFields(t *testing.T) {
	const (
		namespace = "example"
		region    = "us-central1"
	)
	gvk := krm.ComputeSubnetworkFieldsGVK
	env := mockenv.New(t, namespace)
	subnets, err := gcp.NewSubnetworksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { subnets.Close() })
	network := newNetwork(t, env, "sf-net")
	short := mockenv.Name("sf")
	op, err := subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{Project: mockenv.Project, Region: region, SubnetworkResource: &pb.Subnetwork{
		Name: mockenv.Ptr(short), Network: mockenv.Ptr(network), IpCidrRange: mockenv.Ptr("10.20.0.0/24"),
	}})
	waitOp(t, env, "making the subnet", op, err)
	cleanupOp(t, env, "subnet "+short, func() (*gcp.Operation, error) {
		return subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: mockenv.Project, Region: region, Subnetwork: short})
	})
	subnetName := "projects/" + mockenv.Project + "/regions/" + region + "/subnetworks/" + short
	get := func(t *testing.T) *pb.Subnetwork {
		t.Helper()
		s, err := subnets.Get(env.Ctx, &pb.GetSubnetworkRequest{Project: mockenv.Project, Region: region, Subnetwork: short})
		if err != nil {
			t.Fatalf("reading the subnet: %v", err)
		}
		return s
	}
	patches := func() []string { return patchesOf(env, "/subnetworks/"+short) }
	configConnectorObject(t, env, schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ComputeSubnetwork"},
		namespace, "subnet", map[string]any{
			"resourceID": short, "region": region, "ipCidrRange": "10.20.0.0/24",
			"networkRef": map[string]any{"external": network},
		}, "https://www.googleapis.com/compute/v1/"+subnetName)

	newFields := func(name string, ref *computev1beta1.ComputeSubnetworkRef, allow bool) *krm.ComputeSubnetworkFields {
		return &krm.ComputeSubnetworkFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec:       krm.ComputeSubnetworkFieldsSpec{ResourceRef: ref, AllowSubnetCidrRoutesOverlap: mockenv.Ptr(allow)},
		}
	}
	obj := newFields("fields", &computev1beta1.ComputeSubnetworkRef{Name: "subnet"}, true)
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the field with the fingerprint", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != subnetName {
			t.Errorf("status.externalRef = %q, want %q", ref, subnetName)
		}
		if !get(t).GetAllowSubnetCidrRoutesOverlap() {
			t.Errorf("allowSubnetCidrRoutesOverlap is not set")
		}
		if p := patches(); len(p) != 1 {
			t.Errorf("patches = %v, want one", p)
		}
	})

	t.Run("Terraform's secondary range update keeps it", func(t *testing.T) {
		// What Config Connector's ComputeSubnetwork (Terraform) sends when
		// its secondary ranges change: the ranges and the fingerprint.
		op, err := subnets.Patch(env.Ctx, &pb.PatchSubnetworkRequest{Project: mockenv.Project, Region: region, Subnetwork: short,
			SubnetworkResource: &pb.Subnetwork{
				SecondaryIpRanges: []*pb.SubnetworkSecondaryRange{{RangeName: mockenv.Ptr("pods"), IpCidrRange: mockenv.Ptr("10.21.0.0/16")}},
				Fingerprint:       get(t).Fingerprint,
			}})
		waitOp(t, env, "Terraform's patch", op, err)
		s := get(t)
		if len(s.GetSecondaryIpRanges()) != 1 || !s.GetAllowSubnetCidrRoutesOverlap() {
			t.Errorf("after Terraform's patch: secondary ranges %v, allowSubnetCidrRoutesOverlap %v", s.GetSecondaryIpRanges(), s.GetAllowSubnetCidrRoutesOverlap())
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		forceResync(t, env, &krm.ComputeSubnetworkFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "fields"}}, gvk)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the subnet %d times", n)
		}
	})

	t.Run("a field changed outside is set again", func(t *testing.T) {
		op, err := subnets.Patch(env.Ctx, &pb.PatchSubnetworkRequest{Project: mockenv.Project, Region: region, Subnetwork: short,
			SubnetworkResource: &pb.Subnetwork{AllowSubnetCidrRoutesOverlap: mockenv.Ptr(false), Fingerprint: get(t).Fingerprint}})
		waitOp(t, env, "changing the field outside", op, err)
		forceResync(t, env, &krm.ComputeSubnetworkFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "fields"}}, gvk)
		if !get(t).GetAllowSubnetCidrRoutesOverlap() {
			t.Errorf("after a resync allowSubnetCidrRoutesOverlap is false")
		}
	})

	t.Run("a second object for the subnet is refused", func(t *testing.T) {
		dup := newFields("dup", &computev1beta1.ComputeSubnetworkRef{External: subnetName}, false)
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if !get(t).GetAllowSubnetCidrRoutesOverlap() {
			t.Errorf("the refused object changed the subnet")
		}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing subnet is waited for, never made", func(t *testing.T) {
		missing := "projects/" + mockenv.Project + "/regions/" + region + "/subnetworks/" + mockenv.Name("never-made")
		w := newFields("waits", &computev1beta1.ComputeSubnetworkRef{External: missing}, true)
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing subnet waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete and abandon leave the field", func(t *testing.T) {
		before := len(patches())
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		k := newFields("kept", &computev1beta1.ComputeSubnetworkRef{Name: "subnet"}, true)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("delete and abandon patched the subnet %d times", n)
		}
		if !get(t).GetAllowSubnetCidrRoutesOverlap() {
			t.Errorf("after delete allowSubnetCidrRoutesOverlap is false")
		}
	})
}
