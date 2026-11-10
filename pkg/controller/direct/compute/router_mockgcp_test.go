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

// ComputeRouter end to end on the fake cloud (pkg/test/mockenv, patch 0027)
// or a real project (docs/live-runs.md). The case: a Cloud Router for an
// on-premises link, made whole from one object: custom advertisements, an
// interface, a BGP peer on it authenticated with an MD5 key from a Secret,
// and a Cloud NAT. Then the router is changed, a peer added behind its back
// is taken off, the Secret is rotated, and the router is deleted (and one
// abandoned). The network and routers are the run's own (mockenv.Name).

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1/secret"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/protobuf/encoding/protojson"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeRouter(t *testing.T) {
	const (
		namespace = "example"
		region    = "us-central1"
		name      = "onprem-router"
		keyValue  = "first-md5-key-value"
		rotated   = "second-md5-key-value"
	)
	routerGVK := computev1alpha1.ComputeRouterGVK
	router := mockenv.Name("router")
	env := mockenv.New(t, namespace)
	routers, err := gcp.NewRoutersRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { routers.Close() })
	network := newNetwork(t, env, "router-net")

	// Deletes a router the test made, if the test did not (cleanups run
	// before the network's).
	cleanupRouter := func(name string) {
		t.Cleanup(func() {
			op, err := routers.Delete(env.Ctx, &pb.DeleteRouterRequest{Project: mockenv.Project, Region: region, Router: name})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting router %s: %v", name, err)
			}
		})
	}
	cleanupRouter(router)

	keySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "bgp-md5"},
		Data:       map[string][]byte{"key": []byte(keyValue)},
	}
	if err := env.Kube.Create(env.Ctx, keySecret); err != nil {
		t.Fatal(err)
	}

	newRouter := func(objName, resourceID string) *computev1alpha1.ComputeRouter {
		return &computev1alpha1.ComputeRouter{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   namespace,
				Name:        objName,
				Annotations: map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project},
			},
			Spec: computev1alpha1.ComputeRouterSpec{
				ResourceID:  mockenv.Ptr(resourceID),
				Region:      region,
				NetworkRef:  computerefs.ComputeNetworkRef{External: network},
				Description: mockenv.Ptr("on-premises link"),
				BGP: &computev1alpha1.RouterBGP{
					Asn:                64514,
					AdvertiseMode:      mockenv.Ptr("CUSTOM"),
					AdvertisedGroups:   []string{"ALL_SUBNETS"},
					AdvertisedIPRanges: []computev1alpha1.RouterAdvertisedIPRange{{Range: "10.200.0.0/16", Description: mockenv.Ptr("shared services")}},
				},
				MD5AuthenticationKeys: []computev1alpha1.RouterMD5AuthenticationKey{
					{Name: "onprem-key", KeyRef: secret.LegacyValueFromSecretKeyRef{Name: "bgp-md5", Key: "key"}},
				},
				Interfaces: []computev1alpha1.RouterInterface{
					{Name: "onprem-if", IPRange: mockenv.Ptr("169.254.10.1/30")},
				},
				BGPPeers: []computev1alpha1.RouterBGPPeer{{
					Name:                     "onprem-peer",
					InterfaceName:            "onprem-if",
					PeerAsn:                  65010,
					PeerIPAddress:            mockenv.Ptr("169.254.10.2"),
					AdvertisedRoutePriority:  mockenv.Ptr(int64(100)),
					MD5AuthenticationKeyName: mockenv.Ptr("onprem-key"),
				}},
				NATs: []computev1alpha1.RouterNAT{{
					Name:                          "egress",
					NatIpAllocateOption:           mockenv.Ptr("AUTO_ONLY"),
					SourceSubnetworkIpRangesToNat: "ALL_SUBNETWORKS_ALL_IP_RANGES",
					LogConfig:                     &computev1beta1.RouternatLogConfig{Enable: true, Filter: "ERRORS_ONLY"},
				}},
			},
		}
	}
	get := func(t *testing.T, name string) *pb.Router {
		t.Helper()
		r, err := routers.Get(env.Ctx, &pb.GetRouterRequest{Project: mockenv.Project, Region: region, Router: name})
		if err != nil {
			t.Fatalf("reading router %s from the cloud: %v", name, err)
		}
		return r
	}
	// writes counts the controller's updates of the router (PUT); the
	// test's own patch is a PATCH.
	writes := func() int {
		n := 0
		for _, req := range env.Requests() {
			method, url, _ := strings.Cut(req, " ")
			path, _, _ := strings.Cut(url, "?")
			if method == "PUT" && strings.HasSuffix(path, "/routers/"+router) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T) *computev1alpha1.ComputeRouter {
		t.Helper()
		obj := &computev1alpha1.ComputeRouter{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	resync := func(t *testing.T) {
		t.Helper()
		o := getObj(t)
		o.Annotations[k8s.InternalForceReconcileAnnotation] = time.Now().String()
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(routerGVK, namespace, name)
	}
	names := func(n int, name func(int) string) []string {
		var out []string
		for i := 0; i < n; i++ {
			out = append(out, name(i))
		}
		return out
	}

	obj := newRouter(name, router)
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	var keysVersion string
	t.Run("create makes the whole router", func(t *testing.T) {
		u := env.WaitReady(routerGVK, namespace, name)
		got := get(t, router)
		if got.GetDescription() != "on-premises link" || got.GetBgp().GetAsn() != 64514 || got.GetBgp().GetAdvertiseMode() != "CUSTOM" || len(got.GetBgp().GetAdvertisedIpRanges()) != 1 {
			t.Errorf("cloud router = %v", got)
		}
		if len(got.Interfaces) != 1 || got.Interfaces[0].GetName() != "onprem-if" || got.Interfaces[0].GetIpRange() != "169.254.10.1/30" {
			t.Errorf("cloud interfaces = %v", got.Interfaces)
		}
		if len(got.BgpPeers) != 1 || got.BgpPeers[0].GetName() != "onprem-peer" || got.BgpPeers[0].GetPeerAsn() != 65010 ||
			got.BgpPeers[0].GetMd5AuthenticationKeyName() != "onprem-key" || got.BgpPeers[0].GetAdvertisedRoutePriority() != 100 {
			t.Errorf("cloud BGP peers = %v", got.BgpPeers)
		}
		if len(got.Nats) != 1 || got.Nats[0].GetName() != "egress" || got.Nats[0].GetLogConfig().GetFilter() != "ERRORS_ONLY" {
			t.Errorf("cloud NATs = %v", got.Nats)
		}
		if len(got.Md5AuthenticationKeys) != 1 || got.Md5AuthenticationKeys[0].GetName() != "onprem-key" {
			t.Errorf("cloud MD5 keys = %v", got.Md5AuthenticationKeys)
		}
		// The API never returns a key's value (patch 0027 in the fake).
		if raw, _ := protojson.Marshal(got); strings.Contains(string(raw), keyValue) {
			t.Errorf("the cloud returned the MD5 key value")
		}
		keysVersion, _, _ = unstructured.NestedString(u.Object, "status", "md5AuthenticationKeysVersion")
		if keysVersion == "" {
			t.Errorf("status.md5AuthenticationKeysVersion is empty")
		}
		if link, _, _ := unstructured.NestedString(u.Object, "status", "selfLink"); !strings.HasSuffix(link, "/routers/"+router) {
			t.Errorf("status.selfLink = %q", link)
		}
		if raw, _ := u.MarshalJSON(); strings.Contains(string(raw), keyValue) {
			t.Errorf("the object carries the MD5 key value")
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		resync(t)
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote the router %d times; the cloud has %v", n, get(t, router))
		}
	})

	t.Run("a changed peer and a removed NAT are a write each", func(t *testing.T) {
		before := writes()
		o := getObj(t)
		o.Spec.BGPPeers[0].AdvertisedRoutePriority = mockenv.Ptr(int64(200))
		o.Spec.BGPPeers[0].Bfd = &computev1alpha1.RouterBGPPeerBfd{SessionInitializationMode: mockenv.Ptr("PASSIVE")}
		o.Spec.NATs = nil
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool {
			got := get(t, router)
			return len(got.Nats) == 0 && len(got.BgpPeers) == 1 && got.BgpPeers[0].GetAdvertisedRoutePriority() == 200
		})
		env.WaitReady(routerGVK, namespace, name)
		got := get(t, router)
		if got.BgpPeers[0].GetBfd().GetSessionInitializationMode() != "PASSIVE" || got.BgpPeers[0].GetMd5AuthenticationKeyName() != "onprem-key" {
			t.Errorf("cloud BGP peer = %v", got.BgpPeers[0])
		}
		if len(got.Interfaces) != 1 || len(got.Md5AuthenticationKeys) != 1 {
			t.Errorf("the update lost an interface or a key: %v", got)
		}
		// The API takes one feature per update: the peers, then the NATs.
		if n := writes() - before; n != 2 {
			t.Errorf("wrote the router %d times, want twice", n)
		}
	})

	t.Run("a peer added behind its back is taken off", func(t *testing.T) {
		got := get(t, router)
		extra := &pb.RouterInterface{Name: mockenv.Ptr("stray-if"), IpRange: mockenv.Ptr("169.254.20.1/30")}
		peer := &pb.RouterBgpPeer{Name: mockenv.Ptr("stray-peer"), InterfaceName: mockenv.Ptr("stray-if"), PeerAsn: mockenv.Ptr(uint32(65020)), PeerIpAddress: mockenv.Ptr("169.254.20.2")}
		op, err := routers.Patch(env.Ctx, &pb.PatchRouterRequest{
			Project: mockenv.Project, Region: region, Router: router,
			RouterResource: &pb.Router{Interfaces: append(got.Interfaces, extra), BgpPeers: append(got.BgpPeers, peer)},
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("adding a peer by hand: %v", err)
		}
		if n := len(get(t, router).BgpPeers); n != 2 {
			t.Fatalf("the hand-made peer is not there: %d peers", n)
		}
		resync(t)
		// Live, taking them off is two updates (the peers, then the
		// interfaces); wait for the cloud rather than for Ready, which the
		// object already had.
		waitFor(t, func() bool {
			got := get(t, router)
			return len(got.BgpPeers) == 1 && len(got.Interfaces) == 1
		})
		got = get(t, router)
		if p := names(len(got.BgpPeers), func(i int) string { return got.BgpPeers[i].GetName() }); len(p) != 1 || p[0] != "onprem-peer" {
			t.Errorf("cloud BGP peers = %v, want [onprem-peer]", p)
		}
		if in := names(len(got.Interfaces), func(i int) string { return got.Interfaces[i].GetName() }); len(in) != 1 || in[0] != "onprem-if" {
			t.Errorf("cloud interfaces = %v, want [onprem-if]", in)
		}
	})

	t.Run("a rotated Secret writes the key again", func(t *testing.T) {
		before := writes()
		s := &corev1.Secret{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "bgp-md5"}, s); err != nil {
			t.Fatal(err)
		}
		s.Data["key"] = []byte(rotated)
		if err := env.Kube.Update(env.Ctx, s); err != nil {
			t.Fatal(err)
		}
		resync(t)
		// Wait for the status to change rather than for Ready, which the
		// object already had.
		var u *unstructured.Unstructured
		waitFor(t, func() bool {
			u = env.WaitReady(routerGVK, namespace, name)
			v, _, _ := unstructured.NestedString(u.Object, "status", "md5AuthenticationKeysVersion")
			return v != keysVersion
		})
		v, _, _ := unstructured.NestedString(u.Object, "status", "md5AuthenticationKeysVersion")
		if v == "" || v == keysVersion {
			t.Errorf("status.md5AuthenticationKeysVersion = %q, was %q; want a new one", v, keysVersion)
		}
		if n := writes() - before; n != 1 {
			t.Errorf("wrote the router %d times, want once", n)
		}
		if raw, _ := u.MarshalJSON(); strings.Contains(string(raw), rotated) {
			t.Errorf("the object carries the MD5 key value")
		}
	})

	t.Run("delete removes the router", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRouter{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := routers.Get(env.Ctx, &pb.GetRouterRequest{Project: mockenv.Project, Region: region, Router: router})
		if !direct.IsNotFound(err) {
			t.Errorf("reading the deleted router: got %v, want not found", err)
		}
	})

	t.Run("abandon keeps the router", func(t *testing.T) {
		kept := mockenv.Name("kept-router")
		cleanupRouter(kept)
		k := newRouter("kept", kept)
		k.Spec.BGPPeers, k.Spec.Interfaces, k.Spec.MD5AuthenticationKeys = nil, nil, nil
		k.Annotations["cnrm.cloud.google.com/deletion-policy"] = "abandon"
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(routerGVK, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := get(t, kept); len(got.Nats) != 1 {
			t.Errorf("the abandoned router lost its NAT: %v", got)
		}
	})
}
