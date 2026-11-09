// Copyright 2026 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// pkg/controller/direct/networksecurity/networksecurityaddressgroup_fuzzer.go
// (UPSTREAM.md), unchanged but for the harness: a test file here, run by
// TestNetworkSecurityAddressGroupFuzz instead of registered with Config
// Connector's fuzz runner, so the manager does not link the fuzz harness.

// +tool:fuzz-gen
// proto.message: google.cloud.networksecurity.v1.AddressGroup

package networksecurity

import (
	"fmt"
	"testing"

	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/fuzztesting"
)

// TestNetworkSecurityAddressGroupFuzz round-trips random AddressGroup protos
// through the spec and status mappers, as Config Connector's fuzz runner
// does (pkg/fuzztesting/fuzztests).
func TestNetworkSecurityAddressGroupFuzz(t *testing.T) {
	f := addressGroupFuzzer()
	for seed := int64(0); seed < 1000; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			f.FuzzSpec(t, seed)
			f.FuzzStatus(t, seed)
		})
	}
}

func addressGroupFuzzer() fuzztesting.KRMFuzzer {
	f := fuzztesting.NewKRMTypedFuzzer(&pb.AddressGroup{},
		NetworkSecurityAddressGroupSpec_v1alpha1_FromProto, NetworkSecurityAddressGroupSpec_v1alpha1_ToProto,
		NetworkSecurityAddressGroupObservedState_v1alpha1_FromProto, NetworkSecurityAddressGroupObservedState_v1alpha1_ToProto,
	)

	f.SpecField(".description")
	f.SpecField(".labels")
	f.SpecField(".type")
	f.SpecField(".items")
	f.SpecField(".capacity")
	f.SpecField(".purpose")

	f.StatusField(".create_time")
	f.StatusField(".update_time")
	f.StatusField(".self_link")

	f.Unimplemented_Identity(".name")

	return f
}
