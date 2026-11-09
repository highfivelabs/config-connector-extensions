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
// apis/networksecurity/v1alpha1/networksecuritysecurityprofile_identity_test.go
// (UPSTREAM.md); the organization case is added.

package v1alpha1

import (
	"testing"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
)

func TestNetworkSecuritySecurityProfileIdentity(t *testing.T) {
	tests := []struct {
		name     string
		external string
		want     *NetworkSecuritySecurityProfileIdentity
		wantErr  bool
	}{
		{
			name:     "basic",
			external: "projects/my-project/locations/us-central1/securityProfiles/my-profile",
			want: &NetworkSecuritySecurityProfileIdentity{
				Project:         "my-project",
				Location:        "us-central1",
				Securityprofile: "my-profile",
			},
		},
		{
			name:     "organization",
			external: "organizations/123456/locations/global/securityProfiles/my-profile",
			want: &NetworkSecuritySecurityProfileIdentity{
				Organization:    "123456",
				Location:        "global",
				Securityprofile: "my-profile",
			},
		},
		{
			name:     "invalid format",
			external: "invalid/my-project/locations/us-central1/securityProfiles/my-profile",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := &NetworkSecuritySecurityProfileIdentity{}
			err := got.FromExternal(tc.external)
			if (err != nil) != tc.wantErr {
				t.Errorf("FromExternal() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr {
				if *got != *tc.want {
					t.Errorf("FromExternal() = %+v, want %+v", *got, *tc.want)
				}
				if got.String() != tc.external {
					t.Errorf("String() = %v, want %v", got.String(), tc.external)
				}
			}
		})
	}
}

func TestNetworkSecuritySecurityProfileIdentity_Interfaces(t *testing.T) {
	var _ identity.IdentityV2 = &NetworkSecuritySecurityProfileIdentity{}
	var _ identity.Resource = &NetworkSecuritySecurityProfile{}
}
