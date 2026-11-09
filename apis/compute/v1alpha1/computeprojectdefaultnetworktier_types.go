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

// Written here after Terraform's google_compute_project_default_network_tier
// (read, not copied): a project's default network tier, the tier new
// instances and forwarding rules get when they name none. Config
// Connector has no kind or pull request for it. The setting always exists
// (PREMIUM until changed); one object per project manages it.

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeProjectDefaultNetworkTierGVK = GroupVersion.WithKind("ComputeProjectDefaultNetworkTier")

// ComputeProjectDefaultNetworkTierSpec is a project's default network tier.
type ComputeProjectDefaultNetworkTierSpec struct {
	// The project whose default network tier this is. Immutable.
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The default network tier: PREMIUM or STANDARD.
	// +kubebuilder:validation:Enum=PREMIUM;STANDARD
	NetworkTier string `json:"networkTier"`
}

// ComputeProjectDefaultNetworkTierStatus defines the config connector machine state of ComputeProjectDefaultNetworkTier
type ComputeProjectDefaultNetworkTierStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The setting, as projects/{project}/defaultNetworkTier. The API has no
	//  resource name for it; this one is ours.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeprojectdefaultnetworktier;hflcomputeprojectdefaultnetworktiers
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeProjectDefaultNetworkTier is the Schema for the ComputeProjectDefaultNetworkTier API
// +k8s:openapi-gen=true
type ComputeProjectDefaultNetworkTier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeProjectDefaultNetworkTierSpec   `json:"spec,omitempty"`
	Status ComputeProjectDefaultNetworkTierStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeProjectDefaultNetworkTierList contains a list of ComputeProjectDefaultNetworkTier
type ComputeProjectDefaultNetworkTierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeProjectDefaultNetworkTier `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeProjectDefaultNetworkTier{}, &ComputeProjectDefaultNetworkTierList{})
}
