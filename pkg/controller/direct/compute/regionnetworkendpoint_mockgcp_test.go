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

// ComputeRegionNetworkEndpoint end to end on the fake cloud
// (pkg/test/mockenv, patch 0011). The case: a regional external load
// balancer whose backend is an internet service, reached by domain name.

import (
	"strconv"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/iterator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	region    = "us-central1"
	group     = "internet-backends"
)

var gvk = computev1alpha1.ComputeRegionNetworkEndpointGVK

func newEndpoint(name, fqdn string, port int32) *computev1alpha1.ComputeRegionNetworkEndpoint {
	return &computev1alpha1.ComputeRegionNetworkEndpoint{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: computev1alpha1.ComputeRegionNetworkEndpointSpec{
			ProjectRef:                    &refsv1beta1.ProjectRef{External: mockenv.Project},
			Region:                        region,
			RegionNetworkEndpointGroupRef: &computev1alpha1.ComputeRegionNetworkEndpointGroupRef{Name: group},
			FQDN:                          mockenv.Ptr(fqdn),
			Port:                          port,
		},
	}
}

func TestComputeRegionNetworkEndpoint(t *testing.T) {
	env := mockenv.New(t, namespace)
	negs, err := gcp.NewRegionNetworkEndpointGroupsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { negs.Close() })

	// The group, in the fake cloud and as a Config Connector object (not
	// reconciled here; read to resolve regionNetworkEndpointGroupRef by name).
	op, err := negs.Insert(env.Ctx, &pb.InsertRegionNetworkEndpointGroupRequest{
		Project: mockenv.Project,
		Region:  region,
		NetworkEndpointGroupResource: &pb.NetworkEndpointGroup{
			Name:                mockenv.Ptr(group),
			NetworkEndpointType: mockenv.Ptr("INTERNET_FQDN_PORT"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := op.Wait(env.Ctx); err != nil {
		t.Fatal(err)
	}
	g := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"region": region, "networkEndpointType": "INTERNET_FQDN_PORT"}}}
	g.SetGroupVersionKind(computev1alpha1.ComputeRegionNetworkEndpointGroupGVK)
	g.SetNamespace(namespace)
	g.SetName(group)
	g.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, g); err != nil {
		t.Fatal(err)
	}

	cloudEndpoints := func(t *testing.T) []string {
		t.Helper()
		it := negs.ListNetworkEndpoints(env.Ctx, &pb.ListNetworkEndpointsRegionNetworkEndpointGroupsRequest{
			Project: mockenv.Project, Region: region, NetworkEndpointGroup: group,
		})
		var out []string
		for {
			e, err := it.Next()
			if err == iterator.Done {
				return out
			}
			if err != nil {
				t.Fatalf("listing the group's endpoints in the fake cloud: %v", err)
			}
			out = append(out, e.GetNetworkEndpoint().GetFqdn()+":"+strconv.Itoa(int(e.GetNetworkEndpoint().GetPort())))
		}
	}
	has := func(list []string, s string) bool {
		for _, x := range list {
			if x == s {
				return true
			}
		}
		return false
	}
	attaches := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/networkEndpointGroups/"+group+"/attachNetworkEndpoints") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeRegionNetworkEndpoint {
		t.Helper()
		obj := &computev1alpha1.ComputeRegionNetworkEndpoint{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	api := newEndpoint("api", "api.example.com", 443)
	if err := env.Kube.Create(env.Ctx, api); err != nil {
		t.Fatal(err)
	}

	t.Run("create attaches the endpoint", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "api")
		want := "projects/" + mockenv.Project + "/regions/" + region + "/networkEndpointGroups/" + group + "/networkEndpoints/api.example.com:443"
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		if e := cloudEndpoints(t); !has(e, "api.example.com:443") {
			t.Errorf("cloud endpoints = %v, want api.example.com:443", e)
		}
	})

	t.Run("a resync attaches nothing", func(t *testing.T) {
		before := attaches()
		o := getObj(t, "api")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "api")
		if attaches() != before {
			t.Errorf("a resync attached %d more times", attaches()-before)
		}
	})

	t.Run("a second endpoint joins the group", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newEndpoint("api-2", "api2.example.com", 443)); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "api-2")
		if e := cloudEndpoints(t); !has(e, "api.example.com:443") || !has(e, "api2.example.com:443") || len(e) != 2 {
			t.Errorf("cloud endpoints = %v, want both", e)
		}
	})

	t.Run("an endpoint cannot be changed", func(t *testing.T) {
		o := getObj(t, "api-2")
		o.Spec.Port = 8443
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(gvk, namespace, "api-2")
		if !strings.Contains(msg, "immutable") {
			t.Errorf("Ready message = %q, want it to say the endpoint is immutable", msg)
		}
		if e := cloudEndpoints(t); has(e, "api2.example.com:8443") {
			t.Errorf("the changed port was attached: %v", e)
		}
		// Back to its port, so it can be deleted.
		o = getObj(t, "api-2")
		o.Spec.Port = 443
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "api-2")
	})

	t.Run("delete detaches the endpoint", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, api); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRegionNetworkEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "api"}})
		if e := cloudEndpoints(t); has(e, "api.example.com:443") || !has(e, "api2.example.com:443") {
			t.Errorf("cloud endpoints after delete = %v, want only api2.example.com:443", e)
		}
	})

	t.Run("abandon keeps the endpoint", func(t *testing.T) {
		k := newEndpoint("kept", "kept.example.com", 443)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if e := cloudEndpoints(t); !has(e, "kept.example.com:443") {
			t.Errorf("cloud endpoints = %v, want the abandoned one kept", e)
		}
	})
}
