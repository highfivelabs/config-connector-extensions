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

// ComputeRouterRoutePolicy end to end on the fake cloud (pkg/test/mockenv,
// patch 0012) or a real project (docs/live-runs.md). The case: an HA VPN's
// Cloud Router that exports only the on-premises prefixes, with a lower
// MED for one of them. The router and its network are the run's own
// (mockenv.Name) and deleted at the end, with the policies on the router,
// the abandoned one included.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeRouterRoutePolicy(t *testing.T) {
	const (
		namespace = "example"
		region    = "us-central1"
		name      = "export-onprem"
	)
	router := mockenv.Name("vpn-router")
	policyGVK := computev1alpha1.ComputeRouterRoutePolicyGVK
	env := mockenv.New(t, namespace)
	routers, err := gcp.NewRoutersRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { routers.Close() })

	// The router, in the fake cloud and as a Config Connector object (not
	// reconciled here; read to resolve routerRef by name).
	network := newNetwork(t, env, "vpn-net")
	op, err := routers.Insert(env.Ctx, &pb.InsertRouterRequest{
		Project: mockenv.Project, Region: region, RouterResource: &pb.Router{Name: mockenv.Ptr(router), Network: mockenv.Ptr(network)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := op.Wait(env.Ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		op, err := routers.Delete(env.Ctx, &pb.DeleteRouterRequest{Project: mockenv.Project, Region: region, Router: router})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting router %s: %v", router, err)
		}
	})
	r := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"region": region, "networkRef": map[string]any{"external": network},
	}}}
	r.SetGroupVersionKind(computev1alpha1.ComputeRouterGVK)
	r.SetNamespace(namespace)
	r.SetName(router)
	r.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, r); err != nil {
		t.Fatal(err)
	}

	newPolicy := func(name string) *computev1alpha1.ComputeRouterRoutePolicy {
		return &computev1alpha1.ComputeRouterRoutePolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1alpha1.ComputeRouterRoutePolicySpec{
				ProjectRef: &refsv1beta1.ProjectRef{External: mockenv.Project},
				Region:     region,
				RouterRef:  &computev1alpha1.ComputeRouterRef{Name: router},
				Type:       mockenv.Ptr("ROUTE_POLICY_TYPE_EXPORT"),
				Terms: []computev1alpha1.RoutePolicyTerm{
					{
						Priority: 1,
						Match:    &computev1alpha1.RoutePolicyExpr{Expression: "destination == '10.10.0.0/16'"},
						Actions:  []computev1alpha1.RoutePolicyExpr{{Expression: "med.set(100)"}, {Expression: "accept()"}},
					},
					{
						Priority: 2,
						Match:    &computev1alpha1.RoutePolicyExpr{Expression: "destination.inAnyRange(['10.0.0.0/8'])"},
						Actions:  []computev1alpha1.RoutePolicyExpr{{Expression: "accept()"}},
					},
				},
			},
		}
	}
	get := func(t *testing.T, name string) *pb.RoutePolicy {
		t.Helper()
		resp, err := routers.GetRoutePolicy(env.Ctx, &pb.GetRoutePolicyRouterRequest{
			Project: mockenv.Project, Region: region, Router: router, Policy: mockenv.Ptr(name),
		})
		if err != nil {
			t.Fatalf("reading the route policy from the fake cloud: %v", err)
		}
		return resp.GetResource()
	}
	writes := func() int {
		n := 0
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "POST ") && strings.Contains(req, "/routers/"+router+"/updateRoutePolicy") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T) *computev1alpha1.ComputeRouterRoutePolicy {
		t.Helper()
		obj := &computev1alpha1.ComputeRouterRoutePolicy{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	policy := newPolicy(name)
	if err := env.Kube.Create(env.Ctx, policy); err != nil {
		t.Fatal(err)
	}

	var fingerprint string
	t.Run("create puts the policy on the router", func(t *testing.T) {
		u := env.WaitReady(policyGVK, namespace, name)
		want := "projects/" + mockenv.Project + "/regions/" + region + "/routers/" + router + "/routePolicies/" + name
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		got := get(t, name)
		fingerprint = got.GetFingerprint()
		if got.GetType() != "ROUTE_POLICY_TYPE_EXPORT" || len(got.GetTerms()) != 2 || got.GetTerms()[0].GetMatch().GetExpression() != "destination == '10.10.0.0/16'" || len(got.GetTerms()[0].GetActions()) != 2 {
			t.Errorf("cloud route policy = %v", got)
		}
		if f, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "fingerprint"); f != fingerprint || f == "" {
			t.Errorf("status.observedState.fingerprint = %q, want %q", f, fingerprint)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		o := getObj(t)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(policyGVK, namespace, name)
		if writes() != before {
			t.Errorf("a resync wrote the policy %d more times", writes()-before)
		}
	})

	t.Run("a changed term replaces the policy", func(t *testing.T) {
		before := writes()
		o := getObj(t)
		o.Spec.Terms[0].Actions[0].Expression = "med.set(50)"
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(policyGVK, namespace, name)
		got := get(t, name)
		if got.GetTerms()[0].GetActions()[0].GetExpression() != "med.set(50)" {
			t.Errorf("cloud first action = %q, want med.set(50)", got.GetTerms()[0].GetActions()[0].GetExpression())
		}
		if got.GetFingerprint() == fingerprint {
			t.Errorf("the fingerprint did not change on update")
		}
		if writes()-before != 1 {
			t.Errorf("wrote the policy %d times, want once", writes()-before)
		}
	})

	t.Run("delete removes the policy", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, policy); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRouterRoutePolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := routers.GetRoutePolicy(env.Ctx, &pb.GetRoutePolicyRouterRequest{
			Project: mockenv.Project, Region: region, Router: router, Policy: mockenv.Ptr(name),
		})
		// The API's answer for a missing policy is a 400, not a 404.
		if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "The policy does not exist") {
			t.Errorf("reading the deleted route policy: got %v, want the API's 400 \"The policy does not exist\"", err)
		}
	})

	t.Run("abandon keeps the policy", func(t *testing.T) {
		k := newPolicy("kept")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(policyGVK, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		get(t, "kept")
	})
}
