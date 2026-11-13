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
// apis/compute/v1alpha1/computeregionperinstanceconfig_identity_test.go (UPSTREAM.md);
// the package is v1beta1, and the reference's tests are left out (the
// reference is not copied).

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

func TestComputeRegionPerInstanceConfigIdentity_FromExternal(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
		want    *ComputeRegionPerInstanceConfigIdentity
	}{
		{
			name: "valid reference",
			ref:  "projects/my-project/regions/us-central1/instanceGroupManagers/my-rigm/my-pic",
			want: &ComputeRegionPerInstanceConfigIdentity{
				Project:                    "my-project",
				Region:                     "us-central1",
				RegionInstanceGroupManager: "my-rigm",
				Name:                       "my-pic",
			},
		},
		{
			name: "full URL",
			ref:  "https://compute.googleapis.com/compute/v1/projects/my-project/regions/us-central1/instanceGroupManagers/my-rigm/my-pic",
			want: &ComputeRegionPerInstanceConfigIdentity{
				Project:                    "my-project",
				Region:                     "us-central1",
				RegionInstanceGroupManager: "my-rigm",
				Name:                       "my-pic",
			},
		},
		{
			name:    "invalid reference format",
			ref:     "invalid/format",
			wantErr: true,
		},
		{
			name:    "missing region",
			ref:     "projects/my-project/instanceGroupManagers/my-rigm/my-pic",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &ComputeRegionPerInstanceConfigIdentity{}
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

func TestComputeRegionPerInstanceConfigIdentity_StringAndParent(t *testing.T) {
	id := &ComputeRegionPerInstanceConfigIdentity{
		Project:                    "my-project",
		Region:                     "us-central1",
		RegionInstanceGroupManager: "my-rigm",
		Name:                       "my-pic",
	}

	wantStr := "projects/my-project/regions/us-central1/instanceGroupManagers/my-rigm/my-pic"
	if got := id.String(); got != wantStr {
		t.Errorf("String() = %q, want %q", got, wantStr)
	}

	wantParent := "projects/my-project/regions/us-central1/instanceGroupManagers/my-rigm"
	if got := id.ParentString(); got != wantParent {
		t.Errorf("ParentString() = %q, want %q", got, wantParent)
	}

	if got := id.Host(); got != "compute.googleapis.com" {
		t.Errorf("Host() = %q, want %q", got, "compute.googleapis.com")
	}
}

func TestComputeRegionPerInstanceConfig_GetIdentity(t *testing.T) {
	resourceID := "custom-pic"
	tests := []struct {
		name    string
		obj     *ComputeRegionPerInstanceConfig
		objects []runtime.Object
		want    *ComputeRegionPerInstanceConfigIdentity
		wantErr bool
	}{
		{
			name: "spec with resourceID and external IGM",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k8s-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Region: "us-central1",
					RegionInstanceGroupManagerRef: ComputeRegionInstanceGroupManagerRef{
						External: "projects/test-project/regions/us-central1/instanceGroupManagers/my-rigm",
					},
					ResourceID: &resourceID,
				},
			},
			want: &ComputeRegionPerInstanceConfigIdentity{
				Project:                    "test-project",
				Region:                     "us-central1",
				RegionInstanceGroupManager: "my-rigm",
				Name:                       "custom-pic",
			},
		},
		{
			name: "spec without resourceID (uses metadata.name) and short external IGM",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Region: "us-central1",
					RegionInstanceGroupManagerRef: ComputeRegionInstanceGroupManagerRef{
						External: "my-rigm",
					},
				},
			},
			want: &ComputeRegionPerInstanceConfigIdentity{
				Project:                    "test-project",
				Region:                     "us-central1",
				RegionInstanceGroupManager: "my-rigm",
				Name:                       "default-pic",
			},
		},
		{
			name: "spec with referenced ComputeInstanceGroupManager in same namespace",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Region: "us-central1",
					RegionInstanceGroupManagerRef: ComputeRegionInstanceGroupManagerRef{
						Name: "k8s-rigm",
					},
				},
			},
			objects: []runtime.Object{
				&computev1beta1.ComputeInstanceGroupManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "k8s-rigm",
						Namespace: "test-ns",
					},
					Spec: computev1beta1.ComputeInstanceGroupManagerSpec{
						Location: func() *string { s := "us-central1"; return &s }(),
						ProjectRef: apirefs.ProjectRef{
							External: "test-project",
						},
						ResourceID: func() *string { s := "actual-rigm"; return &s }(),
					},
				},
			},
			want: &ComputeRegionPerInstanceConfigIdentity{
				Project:                    "test-project",
				Region:                     "us-central1",
				RegionInstanceGroupManager: "actual-rigm",
				Name:                       "default-pic",
			},
		},
		{
			name: "missing region",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					RegionInstanceGroupManagerRef: ComputeRegionInstanceGroupManagerRef{
						External: "my-rigm",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing project",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					Region: "us-central1",
					RegionInstanceGroupManagerRef: ComputeRegionInstanceGroupManagerRef{
						External: "my-rigm",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing IGM",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Region: "us-central1",
				},
			},
			wantErr: true,
		},
		{
			name: "igm not found when reader is provided",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Region: "us-central1",
					RegionInstanceGroupManagerRef: ComputeRegionInstanceGroupManagerRef{
						Name: "nonexistent-rigm",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "igm invalid identity when reader is provided",
			obj: &ComputeRegionPerInstanceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-pic",
					Namespace: "test-ns",
				},
				Spec: ComputeRegionPerInstanceConfigSpec{
					ProjectRef: apirefs.ProjectRef{
						External: "test-project",
					},
					Region: "us-central1",
					RegionInstanceGroupManagerRef: ComputeRegionInstanceGroupManagerRef{
						Name: "invalid-rigm",
					},
				},
			},
			objects: []runtime.Object{
				&computev1beta1.ComputeInstanceGroupManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-rigm",
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
			s.AddKnownTypes(GroupVersion, &ComputeRegionPerInstanceConfig{})
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
