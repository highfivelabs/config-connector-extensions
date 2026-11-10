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
// pkg/controller/direct/compute/computeorganizationsecuritypolicyassociation_fuzzer.go (UPSTREAM.md). Unchanged but for
// the harness: a test file here, run by its own test function instead of
// registered with Config Connector's fuzz runner, so the manager does not link
// the fuzz harness; and the mapper names (organizationsecuritypolicy_mapper.go).

// +tool:fuzz-gen
// proto.message: google.cloud.compute.v1.SecurityPolicyAssociation
// api.group: compute.gcp.hfl.dev

package compute

import (
	"fmt"
	"testing"

	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/fuzztesting"
)

// TestComputeOrganizationSecurityPolicyAssociationFuzz round-trips random protos through the spec and status
// mappers, as Config Connector's fuzz runner does (pkg/fuzztesting/fuzztests).
func TestComputeOrganizationSecurityPolicyAssociationFuzz(t *testing.T) {
	f := computeOrganizationSecurityPolicyAssociationFuzzer()
	for seed := int64(0); seed < 1000; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			f.FuzzSpec(t, seed)
			f.FuzzStatus(t, seed)
		})
	}
}

func computeOrganizationSecurityPolicyAssociationFuzzer() fuzztesting.KRMFuzzer {
	f := fuzztesting.NewKRMTypedFuzzer(&pb.SecurityPolicyAssociation{},
		ComputeOrganizationSecurityPolicyAssociationSpec_v1beta1_FromProto, ComputeOrganizationSecurityPolicyAssociationSpec_v1beta1_ToProto,
		ComputeOrganizationSecurityPolicyAssociationStatus_v1beta1_FromProto, ComputeOrganizationSecurityPolicyAssociationStatus_v1beta1_ToProto,
	)

	// Field Comparison & Mapping Documentation:
	//
	// KRM Spec Fields:
	// - attachmentId -> f.SpecField(".attachment_id")
	// - policyId     -> f.SpecField(".security_policy_id")
	// - resourceID   -> f.SpecField(".name")
	//
	// KRM Status Fields:
	// - displayName  -> f.StatusField(".display_name")
	//
	// Unimplemented Proto Fields:
	// - excluded_folders  -> f.Unimplemented_NotYetTriaged(".excluded_folders")
	// - excluded_projects -> f.Unimplemented_NotYetTriaged(".excluded_projects")
	// - short_name        -> f.Unimplemented_NotYetTriaged(".short_name")

	// Spec fields
	f.SpecField(".attachment_id")
	f.SpecField(".security_policy_id")
	f.SpecField(".name")

	// Status fields
	f.StatusField(".display_name")

	// Unimplemented / Not Yet Triaged fields
	f.Unimplemented_NotYetTriaged(".excluded_folders")
	f.Unimplemented_NotYetTriaged(".excluded_projects")
	f.Unimplemented_NotYetTriaged(".short_name")

	return f
}
