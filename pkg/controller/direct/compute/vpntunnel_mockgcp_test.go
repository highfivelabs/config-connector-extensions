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

// ComputeVPNTunnel end to end on the fake cloud (pkg/test/mockenv, patch
// 0031) or a real project (docs/live-runs.md). The case: an HA VPN tunnel
// to an external peer gateway with a chosen cipher suite, the field Config
// Connector's ComputeVPNTunnel lacks. The network, HA VPN gateway, external
// peer gateway and Cloud Router are the run's own (mockenv.Name), made
// through the API and referenced by external, and deleted at the end after
// the tunnels.

import (
	"maps"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	kccv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1/secret"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeVPNTunnel(t *testing.T) {
	const ns = "vpn"
	env := mockenv.New(t, ns)
	ctx := env.Ctx
	opts := env.GCPOptions()
	wait := func(t *testing.T, op *gcp.Operation, err error, what string) {
		t.Helper()
		if err == nil {
			err = op.Wait(ctx)
		}
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	gateways, err := gcp.NewVpnGatewaysRESTClient(ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	peers, err := gcp.NewExternalVpnGatewaysRESTClient(ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	routers, err := gcp.NewRoutersRESTClient(ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	tunnels, err := gcp.NewVpnTunnelsRESTClient(ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { gateways.Close(); peers.Close(); routers.Close(); tunnels.Close() })

	// The tunnel's surroundings, made through the API; cleanups run last
	// first, so these go after the tunnels.
	network := newNetwork(t, env, "vpn-net")
	gw, peer, router := mockenv.Name("vpn-gw"), mockenv.Name("vpn-peer"), mockenv.Name("vpn-router")
	op, err := gateways.Insert(ctx, &pb.InsertVpnGatewayRequest{Project: mockenv.Project, Region: region, VpnGatewayResource: &pb.VpnGateway{
		Name: mockenv.Ptr(gw), Network: mockenv.Ptr(network), Labels: mockenv.Labels(),
	}})
	wait(t, op, err, "making the HA VPN gateway")
	t.Cleanup(func() {
		op, err := gateways.Delete(ctx, &pb.DeleteVpnGatewayRequest{Project: mockenv.Project, Region: region, VpnGateway: gw})
		if err == nil {
			err = op.Wait(ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting HA VPN gateway %s: %v", gw, err)
		}
	})
	op, err = peers.Insert(ctx, &pb.InsertExternalVpnGatewayRequest{Project: mockenv.Project, ExternalVpnGatewayResource: &pb.ExternalVpnGateway{
		Name:           mockenv.Ptr(peer),
		RedundancyType: mockenv.Ptr("SINGLE_IP_INTERNALLY_REDUNDANT"),
		Interfaces:     []*pb.ExternalVpnGatewayInterface{{Id: mockenv.Ptr(uint32(0)), IpAddress: mockenv.Ptr("203.0.113.10")}},
		Labels:         mockenv.Labels(),
	}})
	wait(t, op, err, "making the external peer gateway")
	t.Cleanup(func() {
		op, err := peers.Delete(ctx, &pb.DeleteExternalVpnGatewayRequest{Project: mockenv.Project, ExternalVpnGateway: peer})
		if err == nil {
			err = op.Wait(ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting external VPN gateway %s: %v", peer, err)
		}
	})
	op, err = routers.Insert(ctx, &pb.InsertRouterRequest{Project: mockenv.Project, Region: region, RouterResource: &pb.Router{
		Name: mockenv.Ptr(router), Network: mockenv.Ptr(network), Bgp: &pb.RouterBgp{Asn: mockenv.Ptr(uint32(64514))},
	}})
	wait(t, op, err, "making the Cloud Router")
	t.Cleanup(func() {
		op, err := routers.Delete(ctx, &pb.DeleteRouterRequest{Project: mockenv.Project, Region: region, Router: router})
		if err == nil {
			err = op.Wait(ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting router %s: %v", router, err)
		}
	})

	tunnelName, keptName := mockenv.Name("vpn-tunnel"), mockenv.Name("vpn-kept")
	for _, n := range []string{tunnelName, keptName} {
		t.Cleanup(func() {
			op, err := tunnels.Delete(ctx, &pb.DeleteVpnTunnelRequest{Project: mockenv.Project, Region: region, VpnTunnel: n})
			if err == nil {
				err = op.Wait(ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting VPN tunnel %s: %v", n, err)
			}
		})
	}

	if err := env.Kube.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "vpn-secret"},
		StringData: map[string]string{"sharedSecret": "a-shared-secret-for-the-test"},
	}); err != nil {
		t.Fatal(err)
	}
	gvk := computev1beta1.ComputeVPNTunnelGVK
	newTunnel := func(name, resourceID string) *computev1beta1.ComputeVPNTunnel {
		return &computev1beta1.ComputeVPNTunnel{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
			Spec: computev1beta1.ComputeVPNTunnelSpec{
				ProjectRef:                   &refsv1beta1.ProjectRef{External: mockenv.Project},
				Region:                       region,
				ResourceID:                   mockenv.Ptr(resourceID),
				VpnGatewayRef:                &kccv1beta1.ComputeVPNGatewayRef{External: "projects/" + mockenv.Project + "/regions/" + region + "/vpnGateways/" + gw},
				VpnGatewayInterface:          mockenv.Ptr(int64(0)),
				PeerExternalGatewayRef:       &kccv1beta1.ComputeExternalVPNGatewayRef{External: "projects/" + mockenv.Project + "/global/externalVpnGateways/" + peer},
				PeerExternalGatewayInterface: mockenv.Ptr(int64(0)),
				RouterRef:                    &computev1alpha1.ComputeRouterRef{External: "projects/" + mockenv.Project + "/regions/" + region + "/routers/" + router},
				SharedSecret: secret.Legacy{ValueFrom: &secret.LegacyValueFrom{
					SecretKeyRef: &secret.LegacyValueFromSecretKeyRef{Name: "vpn-secret", Key: "sharedSecret"},
				}},
				CipherSuite: &computev1beta1.VPNTunnelCipherSuite{
					Phase1: &computev1beta1.VPNTunnelPhase1Algorithms{
						Encryption: []string{"AES-CBC-256"},
						Integrity:  []string{"HMAC-SHA2-256-128"},
						Prf:        []string{"PRF-HMAC-SHA2-256"},
						Dh:         []string{"Group-14"},
					},
					Phase2: &computev1beta1.VPNTunnelPhase2Algorithms{
						Encryption: []string{"AES-CBC-256"},
						Integrity:  []string{"HMAC-SHA2-256-128"},
						Pfs:        []string{"Group-14"},
					},
				},
				Labels: mockenv.Labels(),
			},
		}
	}
	cloudTunnel := func(t *testing.T, name string) *pb.VpnTunnel {
		t.Helper()
		got, err := tunnels.Get(ctx, &pb.GetVpnTunnelRequest{Project: mockenv.Project, Region: region, VpnTunnel: name})
		if err != nil {
			t.Fatalf("reading VPN tunnel %s: %v", name, err)
		}
		return got
	}
	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if !strings.HasPrefix(r, "GET ") && strings.Contains(r, "/vpnTunnels") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1beta1.ComputeVPNTunnel {
		t.Helper()
		obj := &computev1beta1.ComputeVPNTunnel{}
		if err := env.Kube.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	tunnel := newTunnel("tunnel", tunnelName)
	if err := env.Kube.Create(ctx, tunnel); err != nil {
		t.Fatal(err)
	}

	t.Run("create sends the cipher suite", func(t *testing.T) {
		u := env.WaitReady(gvk, ns, "tunnel")
		want := "projects/" + mockenv.Project + "/regions/" + region + "/vpnTunnels/" + tunnelName
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		if h, _, _ := unstructured.NestedString(u.Object, "status", "sharedSecretHash"); h == "" {
			t.Errorf("status.sharedSecretHash is empty")
		}
		got := cloudTunnel(t, tunnelName)
		p1, p2 := got.GetCipherSuite().GetPhase1(), got.GetCipherSuite().GetPhase2()
		if strings.Join(p1.GetEncryption(), ",") != "AES-CBC-256" || strings.Join(p1.GetDh(), ",") != "Group-14" ||
			strings.Join(p1.GetPrf(), ",") != "PRF-HMAC-SHA2-256" || strings.Join(p2.GetPfs(), ",") != "Group-14" ||
			strings.Join(p2.GetIntegrity(), ",") != "HMAC-SHA2-256-128" {
			t.Errorf("cloud cipher suite = %v, want the spec's", got.GetCipherSuite())
		}
		if !strings.HasSuffix(got.GetRouter(), "/routers/"+router) || !strings.HasSuffix(got.GetPeerExternalGateway(), "/externalVpnGateways/"+peer) {
			t.Errorf("cloud router %q and peer %q, want %s and %s", got.GetRouter(), got.GetPeerExternalGateway(), router, peer)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		o := getObj(t, "tunnel")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, ns, "tunnel")
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("labels change in place", func(t *testing.T) {
		o := getObj(t, "tunnel")
		want := maps.Clone(o.Spec.Labels)
		want["tier"] = "edge"
		o.Spec.Labels = want
		if err := env.Kube.Update(ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return maps.Equal(cloudTunnel(t, tunnelName).GetLabels(), want) })
		if id := cloudTunnel(t, tunnelName).GetId(); id == 0 {
			t.Errorf("the tunnel has no id")
		}
	})

	t.Run("the cipher suite cannot be changed", func(t *testing.T) {
		o := getObj(t, "tunnel")
		o.Spec.CipherSuite.Phase2.Encryption = []string{"AES-GCM-16-256"}
		err := env.Kube.Update(ctx, o)
		if err == nil || !strings.Contains(err.Error(), "cipherSuite is immutable") {
			t.Errorf("update error = %v, want cipherSuite is immutable", err)
		}
	})

	t.Run("delete removes the tunnel", func(t *testing.T) {
		if err := env.Kube.Delete(ctx, tunnel); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeVPNTunnel{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "tunnel"}})
		if _, err := tunnels.Get(ctx, &pb.GetVpnTunnelRequest{Project: mockenv.Project, Region: region, VpnTunnel: tunnelName}); !direct.IsNotFound(err) {
			t.Errorf("reading the deleted tunnel: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the tunnel", func(t *testing.T) {
		k := newTunnel("kept", keptName)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, ns, "kept")
		if err := env.Kube.Delete(ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		cloudTunnel(t, keptName)
	})
}
