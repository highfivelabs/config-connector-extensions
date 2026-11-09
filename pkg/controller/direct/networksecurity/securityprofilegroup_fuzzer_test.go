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
// pkg/controller/direct/networksecurity/networksecuritysecurityprofilegroup_fuzzer.go
// (UPSTREAM.md), unchanged but for the harness: a test file here, run by
// TestNetworkSecuritySecurityProfileGroupFuzz instead of registered with Config
// Connector's fuzz runner, so the manager does not link the fuzz harness.

// +tool:fuzz-gen
// proto.message: google.cloud.networksecurity.v1.SecurityProfileGroup

package networksecurity

import (
	"fmt"
	"testing"

	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/fuzztesting"
)

// TestNetworkSecuritySecurityProfileGroupFuzz round-trips random SecurityProfileGroup
// protos through the spec and status mappers, as Config Connector's fuzz
// runner does (pkg/fuzztesting/fuzztests).
func TestNetworkSecuritySecurityProfileGroupFuzz(t *testing.T) {
	f := securityProfileGroupFuzzer()
	for seed := int64(0); seed < 1000; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			f.FuzzSpec(t, seed)
			f.FuzzStatus(t, seed)
		})
	}
}

func securityProfileGroupFuzzer() fuzztesting.KRMFuzzer {
	f := fuzztesting.NewKRMTypedFuzzer(&pb.SecurityProfileGroup{},
		NetworkSecuritySecurityProfileGroupSpec_v1alpha1_FromProto, NetworkSecuritySecurityProfileGroupSpec_v1alpha1_ToProto,
		NetworkSecuritySecurityProfileGroupObservedState_v1alpha1_FromProto, NetworkSecuritySecurityProfileGroupObservedState_v1alpha1_ToProto,
	)

	f.SpecField(".description")
	f.SpecField(".labels")
	f.SpecField(".threat_prevention_profile")
	f.SpecField(".custom_mirroring_profile")
	f.SpecField(".custom_intercept_profile")
	f.SpecField(".url_filtering_profile")

	f.StatusField(".create_time")
	f.StatusField(".update_time")
	f.StatusField(".etag")
	f.StatusField(".data_path_id")

	f.Unimplemented_Identity(".name")

	return f
}
