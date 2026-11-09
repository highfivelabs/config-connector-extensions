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

// What the compute tests share: a VPC network of the run's own, and a
// wait on the cloud. Both work against the fake cloud and a real project
// (docs/live-runs.md).

import (
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// newNetwork makes a custom-mode VPC network named mockenv.Name(base) and
// deletes it when the test ends, after what was made in it (cleanups run
// last first). It returns the network's path,
// projects/{project}/global/networks/{name}.
func newNetwork(t *testing.T, env *mockenv.Env, base string) string {
	t.Helper()
	networks, err := gcp.NewNetworksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	name := mockenv.Name(base)
	op, err := networks.Insert(env.Ctx, &pb.InsertNetworkRequest{
		Project: mockenv.Project,
		NetworkResource: &pb.Network{
			Name:                  mockenv.Ptr(name),
			AutoCreateSubnetworks: mockenv.Ptr(false),
		},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making network %s: %v", name, err)
	}
	t.Cleanup(func() {
		defer networks.Close()
		op, err := networks.Delete(env.Ctx, &pb.DeleteNetworkRequest{Project: mockenv.Project, Network: name})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting network %s: %v", name, err)
		}
	})
	return "projects/" + mockenv.Project + "/global/networks/" + name
}

// waitFor polls cond until it holds or mockenv.Timeout() passes.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the cloud")
		}
		time.Sleep(500 * time.Millisecond)
	}
}
