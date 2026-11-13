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

// Modified by High Five Labs for config-connector-extensions: adapted from
// Config Connector's apis/compute/v1alpha1/computediskresourcepolicyattachment_identity_test.go
// (UPSTREAM.md), the zonal attachment's, for a regional disk; the
// reference's tests are left out (the reference is not copied).

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestComputeRegionDiskResourcePolicyAttachmentIdentity_FromExternal(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
		want    *ComputeRegionDiskResourcePolicyAttachmentIdentity
	}{
		{
			name: "valid reference",
			ref:  "projects/my-project/regions/us-central1/disks/my-disk/my-policy",
			want: &ComputeRegionDiskResourcePolicyAttachmentIdentity{
				Project: "my-project",
				Region:  "us-central1",
				Disk:    "my-disk",
				Name:    "my-policy",
			},
		},
		{
			name:    "invalid reference format",
			ref:     "invalid/format",
			wantErr: true,
		},
		{
			name:    "a zonal disk's attachment",
			ref:     "projects/my-project/zones/us-central1-a/disks/my-disk/my-policy",
			wantErr: true,
		},
		{
			name: "full url",
			ref:  "https://compute.googleapis.com/compute/v1/projects/my-project/regions/us-central1/disks/my-disk/my-policy",
			want: &ComputeRegionDiskResourcePolicyAttachmentIdentity{
				Project: "my-project",
				Region:  "us-central1",
				Disk:    "my-disk",
				Name:    "my-policy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &ComputeRegionDiskResourcePolicyAttachmentIdentity{}
			err := i.FromExternal(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FromExternal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, i); diff != "" {
					t.Errorf("FromExternal() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
