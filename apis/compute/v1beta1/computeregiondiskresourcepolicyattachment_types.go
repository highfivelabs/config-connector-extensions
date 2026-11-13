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
// Config Connector's apis/compute/v1alpha1/computediskresourcepolicyattachment_types.go
// (UPSTREAM.md), the zonal attachment, for a regional disk. Config Connector
// has ComputeRegionDiskResourcePolicyAttachment only as a Terraform-based
// alpha kind (pkg/clients/generated/apis/compute/v1alpha1/
// computeregiondiskresourcepolicyattachment_types.go), whose field names
// this keeps: diskRef, projectRef, region, resourceID. Changed: a stable
// kind (v1beta1, docs/design.md decision 26); the spec immutable by CEL (an
// attachment is made and removed, never changed); group, labels and short
// names are ours.

package v1beta1

import (
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionDiskResourcePolicyAttachmentGVK = GroupVersion.WithKind("ComputeRegionDiskResourcePolicyAttachment")

// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="the spec is immutable"
type ComputeRegionDiskResourcePolicyAttachmentSpec struct {
	/* The regional disk: Config Connector's ComputeDisk by name (a regional
	one), or `external` as projects/{project}/regions/{region}/disks/{disk}. */
	// +required
	DiskRef computev1beta1.ComputeDiskRef `json:"diskRef"`

	/* The project that this resource belongs to. */
	// +required
	ProjectRef refs.ProjectRef `json:"projectRef"`

	/* Immutable. A reference to the region where the disk resides. */
	// +required
	Region string `json:"region"`

	/* Immutable. Optional. The name of the resource policy to attach, in the
	disk's region. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`
}

type ComputeRegionDiskResourcePolicyAttachmentStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputeRegionDiskResourcePolicyAttachment's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregiondiskresourcepolicyattachment;hflcomputeregiondiskresourcepolicyattachments
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionDiskResourcePolicyAttachment is the Schema for the ComputeRegionDiskResourcePolicyAttachment API
// +k8s:openapi-gen=true
type ComputeRegionDiskResourcePolicyAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionDiskResourcePolicyAttachmentSpec   `json:"spec,omitempty"`
	Status ComputeRegionDiskResourcePolicyAttachmentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionDiskResourcePolicyAttachmentList contains a list of ComputeRegionDiskResourcePolicyAttachment
type ComputeRegionDiskResourcePolicyAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionDiskResourcePolicyAttachment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionDiskResourcePolicyAttachment{}, &ComputeRegionDiskResourcePolicyAttachmentList{})
}
