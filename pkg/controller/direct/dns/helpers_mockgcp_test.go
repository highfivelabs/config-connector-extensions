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

package dns_test

// What the DNS tests share against the fake cloud and a real project
// (docs/live-runs.md): the namespace, the live switch, a VPC network of the
// run's own, and a wait on the cloud.

import (
	"maps"
	"os"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

// skipLiveWithoutDNS skips a live run unless E2E_DNS_ADMIN=true: the
// project must have dns.googleapis.com on and the live-test service account
// roles/dns.admin there (docs/live-runs.md), which $GCP_PROJECT did not
// have on 2026-11-10.
func skipLiveWithoutDNS(t *testing.T) {
	t.Helper()
	if mockenv.Live() && os.Getenv("E2E_DNS_ADMIN") != "true" {
		t.Skip("a live DNS test needs dns.googleapis.com on and roles/dns.admin for the live-test service account; set E2E_DNS_ADMIN=true where it has them (docs/live-runs.md)")
	}
}

// labels are mockenv.Labels() and extra.
func labels(extra map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, mockenv.Labels())
	maps.Copy(out, extra)
	return out
}

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
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
