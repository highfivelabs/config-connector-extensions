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
// our ComputeRegionPerInstanceConfig identity test (computeregionperinstanceconfig_identity_test.go,
// itself copied from Config Connector's
// apis/compute/v1alpha1/computeregionperinstanceconfig_identity_test.go, UPSTREAM.md)
// for the zonal per-instance config, which Config Connector has only as a
// Terraform-based alpha kind: zones and a zonal instance group manager for
// regions and a regional one.

package v1beta1

import (
	"context"
	"testing"

	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	apirefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestComputePerInstanceConfigIdentity_FromExternal(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
		want    *ComputePerInstanceConfigIdentity
	}{
		{
			name: "valid reference",
			ref:  "projects/my-project/zones/us-central1-a/instanceGroupManagers/my-igm/my-pic",
			want: &ComputePerInstanceConfigIdentity{
				Project:              "my-project",
				Zone:                 "us-central1-a",
				InstanceGroupManager: "my-igm",
				Name:                 "my-pic",
			},
		},
		{
			name: "full URL",
			ref:  "https://compute.googleapis.com/compute/v1/projects/my-project/zones/us-central1-a/instanceGroupManagers/my-igm/my-pic",
			want: &ComputePerInstanceConfigIdentity{
				Project:              "my-project",
				Zone:                 "us-central1-a",
				InstanceGroupManager: "my-igm",
				Name:                 "my-pic",
			},
		},
		{
			name:    "invalid reference format",
			ref:     "invalid/format",
			wantErr: true,
		},
		{
			name:    "missing zone",
			ref:     "projects/my-project/instanceGroupManagers/my-igm/my-pic",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &ComputePerInstanceConfigIdentity{}
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

func TestComputePerInstanceConfigIdentity_StringAndParent(t *testing.T) {
	id := &ComputePerInstanceConfigIdentity{
		Project:              "my-project",
		Zone:                 "us-central1-a",
		InstanceGroupManager: "my-igm",
		Name:                 "my-pic",
	}

	wantStr := "projects/my-project/zones/us-central1-a/instanceGroupManagers/my-igm/my-pic"
	if got := id.String(); got != wantStr {
		t.Errorf("String() = %q, want %q", got, wantStr)
	}

	wantParent := "projects/my-project/zones/us-central1-a/instanceGroupManagers/my-igm"
	if got := id.ParentString(); got != wantParent {
		t.Errorf("ParentString() = %q, want %q", got, wantParent)
	}

	if got := id.Host(); got != "compute.googleapis.com" {
		t.Errorf("Host() = %q, want %q", got, "compute.googleapis.com")
	}
}

func TestComputePerInstanceConfig_GetIdentity(t *testing.T) {
	resourceID := "custom-pic"
	tests := []struct {
		name    string
		obj     *ComputePerInstanceConfig
		objects []runtime.Object
		want    *ComputePerInstanceConfigIdentity
		wantErr bool
	}{
		{
			name: "spec with resourceID and external IGM",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k8s-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Zone: "us-central1-a",
					InstanceGroupManagerRef: ComputeInstanceGroupManagerRef{
						External: "projects/test-project/zones/us-central1-a/instanceGroupManagers/my-igm",
					},
					ResourceID: &resourceID,
				},
			},
			want: &ComputePerInstanceConfigIdentity{
				Project:              "test-project",
				Zone:                 "us-central1-a",
				InstanceGroupManager: "my-igm",
				Name:                 "custom-pic",
			},
		},
		{
			name: "spec without resourceID (uses metadata.name) and short external IGM",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Zone: "us-central1-a",
					InstanceGroupManagerRef: ComputeInstanceGroupManagerRef{
						External: "my-igm",
					},
				},
			},
			want: &ComputePerInstanceConfigIdentity{
				Project:              "test-project",
				Zone:                 "us-central1-a",
				InstanceGroupManager: "my-igm",
				Name:                 "default-pic",
			},
		},
		{
			name: "spec with referenced ComputeInstanceGroupManager in same namespace",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Zone: "us-central1-a",
					InstanceGroupManagerRef: ComputeInstanceGroupManagerRef{
						Name: "k8s-igm",
					},
				},
			},
			objects: []runtime.Object{
				&computev1beta1.ComputeInstanceGroupManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "k8s-igm",
						Namespace: "test-ns",
					},
					Spec: computev1beta1.ComputeInstanceGroupManagerSpec{
						Location: func() *string { s := "us-central1-a"; return &s }(),
						ProjectRef: apirefs.ProjectRef{
							External: "test-project",
						},
						ResourceID: func() *string { s := "actual-igm"; return &s }(),
					},
				},
			},
			want: &ComputePerInstanceConfigIdentity{
				Project:              "test-project",
				Zone:                 "us-central1-a",
				InstanceGroupManager: "actual-igm",
				Name:                 "default-pic",
			},
		},
		{
			name: "missing zone",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					InstanceGroupManagerRef: ComputeInstanceGroupManagerRef{
						External: "my-igm",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing project",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					Zone: "us-central1-a",
					InstanceGroupManagerRef: ComputeInstanceGroupManagerRef{
						External: "my-igm",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing IGM",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Zone: "us-central1-a",
				},
			},
			wantErr: true,
		},
		{
			name: "igm not found when reader is provided",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Zone: "us-central1-a",
					InstanceGroupManagerRef: ComputeInstanceGroupManagerRef{
						Name: "nonexistent-igm",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "igm invalid identity when reader is provided",
			obj: &ComputePerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputePerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Zone: "us-central1-a",
					InstanceGroupManagerRef: ComputeInstanceGroupManagerRef{
						Name: "invalid-igm",
					},
				},
			},
			objects: []runtime.Object{
				&computev1beta1.ComputeInstanceGroupManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-igm",
						Namespace: "test-ns",
					},
					Spec: computev1beta1.ComputeInstanceGroupManagerSpec{
						// missing project and location
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			s.AddKnownTypes(GroupVersion, &ComputePerInstanceConfig{})
			s.AddKnownTypes(computev1beta1.GroupVersion, &computev1beta1.ComputeInstanceGroupManager{})
			cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(tt.objects...).Build()

			got, err := tt.obj.GetIdentity(context.TODO(), cl)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetIdentity() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("GetIdentity() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
