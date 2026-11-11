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

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/iam/iamprincipalaccessboundarypolicy_fuzzer.generated.go
// (UPSTREAM.md), unchanged but for the harness: a test file here, run by
// TestIAMPrincipalAccessBoundaryPolicyFuzz instead of registered with Config
// Connector's fuzz runner, so the manager does not link the fuzz harness.

// +generated:fuzzer
// krm.group: iam.gcp.hfl.dev
// krm.version: v1beta1
// proto.message: google.iam.v3.PrincipalAccessBoundaryPolicy
// proto.service: google.iam.v3

package iam

import (
	"fmt"
	"testing"

	pb "cloud.google.com/go/iam/apiv3/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/fuzztesting"
)

// TestIAMPrincipalAccessBoundaryPolicyFuzz round-trips random policies
// through the spec and status mappers, as Config Connector's fuzz runner does.
func TestIAMPrincipalAccessBoundaryPolicyFuzz(t *testing.T) {
	f := iamPrincipalAccessBoundaryPolicyFuzzer()
	for seed := int64(0); seed < 1000; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			f.FuzzSpec(t, seed)
			f.FuzzStatus(t, seed)
		})
	}
}

func iamPrincipalAccessBoundaryPolicyFuzzer() fuzztesting.KRMFuzzer {
	f := fuzztesting.NewKRMTypedFuzzer(&pb.PrincipalAccessBoundaryPolicy{},
		IAMPrincipalAccessBoundaryPolicySpec_FromProto, IAMPrincipalAccessBoundaryPolicySpec_ToProto,
		IAMPrincipalAccessBoundaryPolicyObservedState_FromProto, IAMPrincipalAccessBoundaryPolicyObservedState_ToProto,
	)

	// Spec fields
	f.SpecField(".annotations")
	f.SpecField(".details")
	f.SpecField(".display_name")
	f.SpecField(".etag")

	// Status fields
	f.StatusField(".create_time")
	f.StatusField(".uid")
	f.StatusField(".update_time")

	// Identity / Special fields
	f.Unimplemented_Identity(".name")

	return f
}
