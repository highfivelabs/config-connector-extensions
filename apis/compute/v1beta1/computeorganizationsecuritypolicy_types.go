// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/compute/v1alpha1/computeorganizationsecuritypolicy_types.go (UPSTREAM.md) as a stable kind
// (v1beta1, docs/design.md decision 13). Changed: displayName, parent and type
// are immutable (CEL; upstream says so but its controller patches only the
// description, so a change was never applied); group, labels and short names
// are ours.

package v1beta1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeOrganizationSecurityPolicyGVK = GroupVersion.WithKind("ComputeOrganizationSecurityPolicy")

// ComputeOrganizationSecurityPolicySpec defines the desired state of ComputeOrganizationSecurityPolicy
// +kcc:spec:proto=google.cloud.compute.v1.SecurityPolicy
type ComputeOrganizationSecurityPolicySpec struct {
	/* A textual description for the organization security policy. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* Immutable. A textual name of the security policy. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="displayName is immutable"
	DisplayName string `json:"displayName"`

	/* Immutable. The parent of this OrganizationSecurityPolicy in the Cloud Resource Hierarchy.
	Format: organizations/{organization_id} or folders/{folder_id}. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="parent is immutable"
	Parent string `json:"parent"`

	/* Immutable. Optional. The policyId of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`

	/* Immutable. The type indicates the intended use of the security policy.
	For organization security policies, the only supported type
	is "FIREWALL". Default value: "FIREWALL" Possible values: ["FIREWALL"]. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type *string `json:"type,omitempty"`
}

// ComputeOrganizationSecurityPolicyStatus defines the config connector machine state of ComputeOrganizationSecurityPolicy
type ComputeOrganizationSecurityPolicyStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputeOrganizationSecurityPolicy's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* Fingerprint of this resource. This field is used internally during
	updates of this resource. */
	// +optional
	Fingerprint *string `json:"fingerprint,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	/* The unique identifier for the resource. This identifier is defined by the server. */
	// +optional
	PolicyId *string `json:"policyId,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeorganizationsecuritypolicy;hflcomputeorganizationsecuritypolicies
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeOrganizationSecurityPolicy is the Schema for the compute API
// +k8s:openapi-gen=true
type ComputeOrganizationSecurityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeOrganizationSecurityPolicySpec   `json:"spec,omitempty"`
	Status ComputeOrganizationSecurityPolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComputeOrganizationSecurityPolicyList contains a list of ComputeOrganizationSecurityPolicy
type ComputeOrganizationSecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeOrganizationSecurityPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeOrganizationSecurityPolicy{}, &ComputeOrganizationSecurityPolicyList{})
}
