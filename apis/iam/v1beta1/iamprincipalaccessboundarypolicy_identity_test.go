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
// apis/iam/v1alpha1/iamprincipalaccessboundarypolicy_identity_generated_test.go (UPSTREAM.md), which
// controllerbuilder generated; unchanged but for the package and the
// generated-code marker (controller-gen does not regenerate it here).

// +generated:identity
// krm.group: iam.gcp.hfl.dev
// krm.version: v1beta1
// proto.message: google.iam.v3.PrincipalAccessBoundaryPolicy

package v1beta1

import (
	"testing"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
)

func TestIAMPrincipalAccessBoundaryPolicyIdentity_Conformance(t *testing.T) {
	identity.AssertConformance(t, identity.ConformanceCase[IAMPrincipalAccessBoundaryPolicyIdentity]{
		SampleExternal: "organizations/123456789/locations/us-central1/principalAccessBoundaryPolicies/my-principal-access-boundary-policy",
		WantHost:       "iam.googleapis.com",
		WantParent:     "organizations/123456789/locations/us-central1",
		WantIdentity: &IAMPrincipalAccessBoundaryPolicyIdentity{
			Organization:                  "123456789",
			Location:                      "us-central1",
			PrincipalAccessBoundaryPolicy: "my-principal-access-boundary-policy",
		},
		Ref: &IAMPrincipalAccessBoundaryPolicyRef{},
	})
}
