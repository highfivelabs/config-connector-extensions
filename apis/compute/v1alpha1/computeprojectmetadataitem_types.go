// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0
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

package v1alpha1

// Written here after Terraform's google_compute_project_metadata_item
// (read, not copied): one key of a project's common instance metadata.
// Config Connector's ComputeProjectMetadata (Terraform-based) owns the
// whole metadata; no Config Connector code or pull request has one key.

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeProjectMetadataItemGVK = GroupVersion.WithKind("ComputeProjectMetadataItem")

// ComputeProjectMetadataItemSpec is one key of a project's common instance
// metadata; the project's other keys are left as they are.
type ComputeProjectMetadataItemSpec struct {
	// The project whose metadata this key is in. Immutable.
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The metadata key, for example enable-oslogin. If not given, the
	//  metadata.name is used. Immutable.
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`

	// The value of the key.
	Value string `json:"value"`
}

// ComputeProjectMetadataItemStatus defines the config connector machine state of ComputeProjectMetadataItem
type ComputeProjectMetadataItemStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The key, as projects/{project}/commonInstanceMetadata/{key}. The API
	//  has no resource name for one key; this one is ours.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeprojectmetadataitem;hflcomputeprojectmetadataitems
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeProjectMetadataItem is the Schema for the ComputeProjectMetadataItem API
// +k8s:openapi-gen=true
type ComputeProjectMetadataItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeProjectMetadataItemSpec   `json:"spec,omitempty"`
	Status ComputeProjectMetadataItemStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeProjectMetadataItemList contains a list of ComputeProjectMetadataItem
type ComputeProjectMetadataItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeProjectMetadataItem `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeProjectMetadataItem{}, &ComputeProjectMetadataItemList{})
}
