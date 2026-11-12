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

import (
	"testing"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func init() {
	kindtest.RegisterSetup("ComputeNetworkAttachment", subnetSetup("computenetworkattachment"))
	kindtest.RegisterSetup("ComputeRegionInstanceTemplate", subnetSetup("computeregioninstancetemplate"))
}

// subnetSetup makes what the fixtures of a kind whose lower-case name is
// base name: a network ${NAME}-net with a subnet ${NAME}-subnet in
// us-central1, for the test's object and for its abandoned one (whose
// ${NAME} ends in -kept). The API refuses a network attachment or an
// instance template whose subnet does not exist. Live only: the fake does
// not check. Both are deleted after the test's own resources (t.Cleanup
// runs last-in first-out), and swept by name (hack/live/sweep.sh).
func subnetSetup(base string) kindtest.Setup {
	return func(t *testing.T, env *mockenv.Env) {
		if !mockenv.Live() {
			return
		}
		subnets, err := gcp.NewSubnetworksRESTClient(env.Ctx, env.GCPOptions()...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { subnets.Close() })
		for i, b := range []string{base, base + "-kept"} {
			network := newNetwork(t, env, b+"-net")
			name := mockenv.Name(b) + "-subnet"
			op, err := subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{
				Project: mockenv.Project,
				Region:  "us-central1",
				SubnetworkResource: &pb.Subnetwork{
					Name:        mockenv.Ptr(name),
					Network:     mockenv.Ptr(network),
					IpCidrRange: mockenv.Ptr([]string{"10.10.0.0/24", "10.11.0.0/24"}[i]),
				},
			})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil {
				t.Fatalf("making subnet %s: %v", name, err)
			}
			t.Logf("made the subnet %s in %s", name, network)
			t.Cleanup(func() {
				op, err := subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: mockenv.Project, Region: "us-central1", Subnetwork: name})
				if err == nil {
					err = op.Wait(env.Ctx)
				}
				if err != nil && !direct.IsNotFound(err) {
					t.Errorf("cleanup: deleting subnet %s: %v", name, err)
				}
			})
		}
	}
}
