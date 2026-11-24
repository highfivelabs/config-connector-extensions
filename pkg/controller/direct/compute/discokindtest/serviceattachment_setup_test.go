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

package discokindtest

import (
	"testing"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func init() {
	kindtest.RegisterSetup("ComputeServiceAttachment", serviceAttachmentSetup("computeserviceattachment"))
}

// serviceAttachmentSetup makes what a service attachment's fixtures name,
// for the test's object and for its abandoned one (whose ${NAME} ends in
// -kept): a producer's internal passthrough load balancer in us-central1
// (network ${NAME}-net, subnet ${NAME}-ilb, a TCP health check, an
// INTERNAL backend service and the forwarding rule ${NAME}-fr) and a
// Private Service Connect NAT subnet ${NAME}-nat. A forwarding rule serves
// one attachment, so each object gets its own. The API refuses an
// attachment whose forwarding rule or NAT subnet does not exist; the fake
// (mockgcp) does not check, so this runs live only. Everything is deleted
// after the test's own resources (t.Cleanup runs last-in first-out) and
// swept by name (hack/live/sweep.sh).
func serviceAttachmentSetup(base string) kindtest.Setup {
	return func(t *testing.T, env *mockenv.Env) {
		if !mockenv.Live() {
			return
		}
		opts := env.GCPOptions()
		networks, err := gcp.NewNetworksRESTClient(env.Ctx, opts...)
		if err != nil {
			t.Fatal(err)
		}
		subnets, err := gcp.NewSubnetworksRESTClient(env.Ctx, opts...)
		if err != nil {
			t.Fatal(err)
		}
		healthChecks, err := gcp.NewRegionHealthChecksRESTClient(env.Ctx, opts...)
		if err != nil {
			t.Fatal(err)
		}
		backends, err := gcp.NewRegionBackendServicesRESTClient(env.Ctx, opts...)
		if err != nil {
			t.Fatal(err)
		}
		rules, err := gcp.NewForwardingRulesRESTClient(env.Ctx, opts...)
		if err != nil {
			t.Fatal(err)
		}
		// Registered first, so it runs after every delete below.
		t.Cleanup(func() {
			rules.Close()
			backends.Close()
			healthChecks.Close()
			subnets.Close()
			networks.Close()
		})
		const region = "us-central1"
		p := mockenv.Project
		regionPrefix := "projects/" + p + "/regions/" + region
		wait := func(what string, op *gcp.Operation, err error) {
			t.Helper()
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil {
				t.Fatalf("making %s: %v", what, err)
			}
		}
		cleanup := func(what string, del func() (*gcp.Operation, error)) {
			t.Cleanup(func() {
				op, err := del()
				if err == nil {
					err = op.Wait(env.Ctx)
				}
				if err != nil && !direct.IsNotFound(err) {
					t.Errorf("cleanup: deleting %s: %v", what, err)
				}
			})
		}
		for i, b := range []string{base, base + "-kept"} {
			name := mockenv.Name(b)

			network := name + "-net"
			op, err := networks.Insert(env.Ctx, &pb.InsertNetworkRequest{Project: p, NetworkResource: &pb.Network{
				Name: mockenv.Ptr(network), AutoCreateSubnetworks: mockenv.Ptr(false),
			}})
			wait("network "+network, op, err)
			cleanup("network "+network, func() (*gcp.Operation, error) {
				return networks.Delete(env.Ctx, &pb.DeleteNetworkRequest{Project: p, Network: network})
			})
			networkPath := "projects/" + p + "/global/networks/" + network

			ilb := name + "-ilb"
			op, err = subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{Project: p, Region: region, SubnetworkResource: &pb.Subnetwork{
				Name: mockenv.Ptr(ilb), Network: mockenv.Ptr(networkPath), IpCidrRange: mockenv.Ptr([]string{"10.20.0.0/24", "10.21.0.0/24"}[i]),
			}})
			wait("subnet "+ilb, op, err)
			cleanup("subnet "+ilb, func() (*gcp.Operation, error) {
				return subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: p, Region: region, Subnetwork: ilb})
			})

			nat := name + "-nat"
			op, err = subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{Project: p, Region: region, SubnetworkResource: &pb.Subnetwork{
				Name: mockenv.Ptr(nat), Network: mockenv.Ptr(networkPath), IpCidrRange: mockenv.Ptr([]string{"10.20.1.0/24", "10.21.1.0/24"}[i]),
				Purpose: mockenv.Ptr("PRIVATE_SERVICE_CONNECT"),
			}})
			wait("subnet "+nat, op, err)
			cleanup("subnet "+nat, func() (*gcp.Operation, error) {
				return subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: p, Region: region, Subnetwork: nat})
			})

			hc := name + "-hc"
			op, err = healthChecks.Insert(env.Ctx, &pb.InsertRegionHealthCheckRequest{Project: p, Region: region, HealthCheckResource: &pb.HealthCheck{
				Name: mockenv.Ptr(hc), Type: mockenv.Ptr("TCP"), TcpHealthCheck: &pb.TCPHealthCheck{Port: mockenv.Ptr(int32(80))},
			}})
			wait("health check "+hc, op, err)
			cleanup("health check "+hc, func() (*gcp.Operation, error) {
				return healthChecks.Delete(env.Ctx, &pb.DeleteRegionHealthCheckRequest{Project: p, Region: region, HealthCheck: hc})
			})

			bs := name + "-bs"
			op, err = backends.Insert(env.Ctx, &pb.InsertRegionBackendServiceRequest{Project: p, Region: region, BackendServiceResource: &pb.BackendService{
				Name: mockenv.Ptr(bs), LoadBalancingScheme: mockenv.Ptr("INTERNAL"), Protocol: mockenv.Ptr("TCP"),
				HealthChecks: []string{regionPrefix + "/healthChecks/" + hc},
			}})
			wait("backend service "+bs, op, err)
			cleanup("backend service "+bs, func() (*gcp.Operation, error) {
				return backends.Delete(env.Ctx, &pb.DeleteRegionBackendServiceRequest{Project: p, Region: region, BackendService: bs})
			})

			fr := name + "-fr"
			op, err = rules.Insert(env.Ctx, &pb.InsertForwardingRuleRequest{Project: p, Region: region, ForwardingRuleResource: &pb.ForwardingRule{
				Name: mockenv.Ptr(fr), LoadBalancingScheme: mockenv.Ptr("INTERNAL"), IPProtocol: mockenv.Ptr("TCP"),
				Ports: []string{"80"}, BackendService: mockenv.Ptr(regionPrefix + "/backendServices/" + bs),
				Network: mockenv.Ptr(networkPath), Subnetwork: mockenv.Ptr(regionPrefix + "/subnetworks/" + ilb),
				Labels: mockenv.Labels(),
			}})
			wait("forwarding rule "+fr, op, err)
			cleanup("forwarding rule "+fr, func() (*gcp.Operation, error) {
				return rules.Delete(env.Ctx, &pb.DeleteForwardingRuleRequest{Project: p, Region: region, ForwardingRule: fr})
			})
			t.Logf("made the producer %s in %s", fr, network)
		}
	}
}
