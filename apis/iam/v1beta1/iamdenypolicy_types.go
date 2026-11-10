// Copyright 2025 Google LLC
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
// apis/iam/v1alpha1/iamdenypolicy_types.go (UPSTREAM.md) as a stable kind
// (v1beta1, docs/design.md decision 13). Changed: exactly one of projectRef,
// folderRef and organizationRef (a CEL rule; upstream errors in the
// controller when none is set and takes the first when several are); the
// commented-out fields upstream left out are dropped; folderRef is
// refs.FolderRefDeprecated, the type refs.FolderRef aliases (controller-gen
// does not take aliases); group, labels and short names are ours.

package v1beta1

import (
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var IAMDenyPolicyGVK = GroupVersion.WithKind("IAMDenyPolicy")

// IAMDenyPolicySpec defines the desired state of IAMDenyPolicy
// +kcc:spec:proto=google.iam.v2.Policy
// +kubebuilder:validation:XValidation:rule="(has(self.projectRef) ? 1 : 0) + (has(self.folderRef) ? 1 : 0) + (has(self.organizationRef) ? 1 : 0) == 1",message="exactly one of projectRef, folderRef and organizationRef is required"
type IAMDenyPolicySpec struct {
	// The project that this resource belongs to. Exactly one of projectRef,
	//  folderRef and organizationRef. Immutable.
	// +optional
	ProjectRef *refs.ProjectRef `json:"projectRef,omitempty"`

	// The folder that this resource belongs to. Exactly one of projectRef,
	//  folderRef and organizationRef. Immutable.
	// +optional
	FolderRef *refs.FolderRefDeprecated `json:"folderRef,omitempty"`

	// The organization that this resource belongs to. Exactly one of
	//  projectRef, folderRef and organizationRef. Immutable.
	// +optional
	OrganizationRef *refs.OrganizationRef `json:"organizationRef,omitempty"`

	// The IAMDenyPolicy name. If not given, the metadata.name will be used.
	//  Immutable.
	ResourceID *string `json:"resourceID,omitempty"`

	// A user-specified description of the `Policy`. This value can be up to 63
	//  characters.
	// +kcc:proto:field=google.iam.v2.Policy.display_name
	DisplayName *string `json:"displayName,omitempty"`

	// A list of rules that specify the behavior of the `Policy`. All of the rules
	//  should be of the `kind` specified in the `Policy`.
	// +kcc:proto:field=google.iam.v2.Policy.rules
	Rules []PolicyRule `json:"rules,omitempty"`
}

// IAMDenyPolicyStatus defines the config connector machine state of IAMDenyPolicy
type IAMDenyPolicyStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the IAMDenyPolicy resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfliamdenypolicy;hfliamdenypolicies
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// IAMDenyPolicy is the Schema for the IAMDenyPolicy API
// +k8s:openapi-gen=true
type IAMDenyPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   IAMDenyPolicySpec   `json:"spec,omitempty"`
	Status IAMDenyPolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// IAMDenyPolicyList contains a list of IAMDenyPolicy
type IAMDenyPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMDenyPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMDenyPolicy{}, &IAMDenyPolicyList{})
}
