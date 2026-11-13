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

package networkservices_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"google.golang.org/api/iterator"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func init() {
	kindtest.RegisterSetup("NetworkServicesGateway", secureWebProxySetup)
}

// TestNetworkServicesGatewayOpenMesh runs the other shape of a gateway,
// OPEN_MESH (global, a scope, no network), from
// testdata/discokind/NetworkServicesGateway/open-mesh/; the generated
// TestNetworkServicesGateway runs a SECURE_WEB_GATEWAY.
func TestNetworkServicesGatewayOpenMesh(t *testing.T) {
	kindtest.RunCase(t, "NetworkServicesGateway", "open-mesh")
}

// secureWebProxySetup makes what a Secure Web Proxy's fixtures name, once
// for the test's object and its abandoned one (${KIND_NAME}): a network
// ${KIND_NAME}-net, a subnet ${KIND_NAME}-subnet in us-central1, the
// region's proxy-only subnet ${KIND_NAME}-proxy in that network, and a
// gateway security policy ${KIND_NAME}-gsp. The API refuses a gateway
// without them. Live only: the fake does not check.
//
// A SECURE_WEB_GATEWAY makes a Cloud Router (swg-autogen-router-...) with a
// Cloud NAT in its network and region, which the API does not delete with
// the last gateway (Terraform deletes it only when told to, with
// delete_swg_autogen_router_on_destroy); the cleanup deletes it before the
// subnets and the network. Cleanups run last-in first-out, so all of this
// goes after the test's gateways; the sweeper deletes a run's gateways,
// then its policies, routers, subnets and networks (hack/live/sweep.sh).
func secureWebProxySetup(t *testing.T, env *mockenv.Env) {
	if !mockenv.Live() {
		return
	}
	const region = "us-central1"
	base := mockenv.Name("networkservicesgateway")
	project := mockenv.Project
	// A cleanup gets its own context: env.Ctx may be done by then, and a
	// gateway's router and subnets take minutes to go.
	cleanupCtx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 20*time.Minute)
	}

	networks, err := gcp.NewNetworksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	subnets, err := gcp.NewSubnetworksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	routers, err := gcp.NewRoutersRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	network := base + "-net"
	op, err := networks.Insert(env.Ctx, &pb.InsertNetworkRequest{
		Project:         project,
		NetworkResource: &pb.Network{Name: mockenv.Ptr(network), AutoCreateSubnetworks: mockenv.Ptr(false)},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making the network %s: %v", network, err)
	}
	t.Logf("made the network %s", network)
	t.Cleanup(func() {
		defer networks.Close()
		ctx, cancel := cleanupCtx()
		defer cancel()
		op, err := networks.Delete(ctx, &pb.DeleteNetworkRequest{Project: project, Network: network})
		if err == nil {
			err = op.Wait(ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("cleanup: deleting the network %s: %v", network, err)
		}
	})
	networkPath := "projects/" + project + "/global/networks/" + network

	for _, s := range []*pb.Subnetwork{
		{Name: mockenv.Ptr(base + "-subnet"), IpCidrRange: mockenv.Ptr("10.20.0.0/24")},
		{Name: mockenv.Ptr(base + "-proxy"), IpCidrRange: mockenv.Ptr("10.20.2.0/23"),
			Purpose: mockenv.Ptr("REGIONAL_MANAGED_PROXY"), Role: mockenv.Ptr("ACTIVE")},
	} {
		s.Network = mockenv.Ptr(networkPath)
		name := s.GetName()
		op, err := subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{Project: project, Region: region, SubnetworkResource: s})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("making the subnet %s: %v", name, err)
		}
		t.Logf("made the subnet %s", name)
		t.Cleanup(func() {
			ctx, cancel := cleanupCtx()
			defer cancel()
			op, err := subnets.Delete(ctx, &pb.DeleteSubnetworkRequest{Project: project, Region: region, Subnetwork: name})
			if err == nil {
				err = op.Wait(ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting the subnet %s: %v", name, err)
			}
		})
	}

	// The routers the gateways made in the network, once they are gone.
	t.Cleanup(func() {
		defer subnets.Close()
		defer routers.Close()
		ctx, cancel := cleanupCtx()
		defer cancel()
		it := routers.List(ctx, &pb.ListRoutersRequest{Project: project, Region: region})
		for {
			r, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Errorf("cleanup: listing the routers in %s: %v", region, err)
				return
			}
			if !strings.HasSuffix(r.GetNetwork(), "/networks/"+network) {
				continue
			}
			op, err := routers.Delete(ctx, &pb.DeleteRouterRequest{Project: project, Region: region, Router: r.GetName()})
			if err == nil {
				err = op.Wait(ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting the router %s: %v", r.GetName(), err)
				continue
			}
			t.Logf("deleted the router %s the gateways made", r.GetName())
		}
	})

	// The gateway security policy, as the policy rule test makes its own.
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://networksecurity.googleapis.com/v1/"}
	parent := "projects/" + project + "/locations/" + region
	id := base + "-gsp"
	name := parent + "/gatewaySecurityPolicies/" + id
	gop, err := api.Do(env.Ctx, http.MethodPost, parent+"/gatewaySecurityPolicies", map[string]string{"gatewaySecurityPolicyId": id}, map[string]any{
		"description": "the policy of a config-connector-extensions gateway test",
	})
	if err == nil {
		_, err = api.Wait(env.Ctx, gop)
	}
	if err != nil {
		t.Fatalf("creating the gateway security policy %s: %v", name, err)
	}
	t.Logf("created the gateway security policy %s", name)
	t.Cleanup(func() {
		ctx, cancel := cleanupCtx()
		defer cancel()
		op, err := api.Do(ctx, http.MethodDelete, name, nil, nil)
		if err == nil {
			_, err = api.Wait(ctx, op)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("cleanup: deleting the gateway security policy %s: %v", name, err)
		}
	})
}
