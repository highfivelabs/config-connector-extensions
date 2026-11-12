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

// ComputeNetworkEndpoint (zonal, our stable copy of Config Connector's
// alpha kind) end to end on the fake cloud or a real project
// (docs/live-runs.md). The case: a hybrid backend, a zonal group of
// NON_GCP_PRIVATE_IP_PORT endpoints (an address and port outside Google
// Cloud, reached over the network), which a cross-region internal load
// balancer uses. Such endpoints need no VM, so the test runs live for the
// cost of a network and a group. The group and its network are the run's
// own (mockenv.Name) and deleted at the end, with the endpoints in the
// group, the abandoned one included.

import (
	"strconv"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	upstreamcompute "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/iterator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const zonalEndpointZone = "us-central1-a"

func TestComputeNetworkEndpoint(t *testing.T) {
	env := mockenv.New(t, namespace)
	gvk := computev1beta1.ComputeNetworkEndpointGVK
	group := mockenv.Name("hybrid-backends")
	groupPath := "projects/" + mockenv.Project + "/zones/" + zonalEndpointZone + "/networkEndpointGroups/" + group
	negs, err := gcp.NewNetworkEndpointGroupsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { negs.Close() })

	network := newNetwork(t, env, "hybrid-net")
	op, err := negs.Insert(env.Ctx, &pb.InsertNetworkEndpointGroupRequest{
		Project: mockenv.Project,
		Zone:    zonalEndpointZone,
		NetworkEndpointGroupResource: &pb.NetworkEndpointGroup{
			Name:                mockenv.Ptr(group),
			NetworkEndpointType: mockenv.Ptr("NON_GCP_PRIVATE_IP_PORT"),
			Network:             mockenv.Ptr(network),
			DefaultPort:         mockenv.Ptr(int32(80)),
		},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making network endpoint group %s: %v", group, err)
	}
	t.Cleanup(func() {
		op, err := negs.Delete(env.Ctx, &pb.DeleteNetworkEndpointGroupRequest{
			Project: mockenv.Project, Zone: zonalEndpointZone, NetworkEndpointGroup: group,
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting network endpoint group %s: %v", group, err)
		}
	})

	newEp := func(name, ip, port string) *computev1beta1.ComputeNetworkEndpoint {
		return &computev1beta1.ComputeNetworkEndpoint{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1beta1.ComputeNetworkEndpointSpec{
				ProjectRef:              &refs.ProjectRef{External: mockenv.Project},
				Zone:                    zonalEndpointZone,
				NetworkEndpointGroupRef: &upstreamcompute.ComputeNetworkEndpointGroupRef{External: groupPath},
				IpAddress:               ip,
				ResourceID:              mockenv.Ptr(port),
			},
		}
	}
	cloudEndpoints := func(t *testing.T) []string {
		t.Helper()
		it := negs.ListNetworkEndpoints(env.Ctx, &pb.ListNetworkEndpointsNetworkEndpointGroupsRequest{
			Project: mockenv.Project, Zone: zonalEndpointZone, NetworkEndpointGroup: group,
		})
		var out []string
		for {
			e, err := it.Next()
			if err == iterator.Done {
				return out
			}
			if err != nil {
				t.Fatalf("listing the group's endpoints: %v", err)
			}
			out = append(out, e.GetNetworkEndpoint().GetIpAddress()+":"+strconv.Itoa(int(e.GetNetworkEndpoint().GetPort())))
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
	getObj := func(t *testing.T, name string) *computev1beta1.ComputeNetworkEndpoint {
		t.Helper()
		obj := &computev1beta1.ComputeNetworkEndpoint{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	waitCloud := func(t *testing.T, what string, cond func([]string) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			e := cloudEndpoints(t)
			if cond(e) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s: cloud endpoints %v", what, e)
			}
			time.Sleep(time.Second)
		}
	}

	first := newEp("onprem-a", "10.20.0.5", "8080")
	if err := env.Kube.Create(env.Ctx, first); err != nil {
		t.Fatal(err)
	}

	t.Run("create attaches the endpoint", func(t *testing.T) {
		env.WaitReady(gvk, namespace, first.Name)
		waitCloud(t, "the endpoint", func(e []string) bool { return has(e, "10.20.0.5:8080") })
	})

	t.Run("a resync attaches nothing", func(t *testing.T) {
		before := attaches()
		o := getObj(t, first.Name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, first.Name)
		if n := attaches() - before; n != 0 {
			t.Errorf("a resync attached %d more times", n)
		}
	})

	t.Run("a second endpoint joins the group", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newEp("onprem-b", "10.20.0.6", "8080")); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "onprem-b")
		waitCloud(t, "both endpoints", func(e []string) bool {
			return has(e, "10.20.0.5:8080") && has(e, "10.20.0.6:8080") && len(e) == 2
		})
	})

	t.Run("the CRD refuses a change", func(t *testing.T) {
		o := getObj(t, "onprem-b")
		o.Spec.IpAddress = "10.20.0.7"
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Errorf("changing the address: err = %v, want immutable", err)
		}
	})

	t.Run("delete detaches the endpoint", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, first.Name)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(first)
		waitCloud(t, "only the second endpoint", func(e []string) bool {
			return !has(e, "10.20.0.5:8080") && has(e, "10.20.0.6:8080")
		})
	})

	t.Run("abandon keeps the endpoint", func(t *testing.T) {
		k := newEp("onprem-kept", "10.20.0.8", "8080")
		k.Annotations = map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, k.Name)
		if err := env.Kube.Delete(env.Ctx, getObj(t, k.Name)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if e := cloudEndpoints(t); !has(e, "10.20.0.8:8080") {
			t.Errorf("cloud endpoints = %v, want the abandoned one kept", e)
		}
	})
}
