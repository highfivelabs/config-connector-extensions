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
// apis/compute/v1alpha1/computeorganizationsecuritypolicyassociation_types.go (UPSTREAM.md) as a stable
// kind (v1beta1, docs/design.md decision 13). Changed: exactly one of policyId
// and securityPolicyRef (CEL; upstream errors in the identity when none is
// set) and the spec immutable (CEL; upstream's controller refuses any change);
// the reference names our ComputeOrganizationSecurityPolicy; group, labels
// and short names are ours. Added 2026-11-15 (the builder's RECPF answer, a
// stable Cloud Armor association): excludedFolders and excludedProjects, the
// v1 API's fields that Terraform's
// google_compute_organization_security_policy_association and fabric's
// cloud_armor_policy carry, refused on a project's association (CEL, as
// fabric's project module refuses them).

package v1beta1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeOrganizationSecurityPolicyAssociationGVK = GroupVersion.WithKind("ComputeOrganizationSecurityPolicyAssociation")

// +kcc:spec:proto=google.cloud.compute.v1.SecurityPolicyAssociation
// +kubebuilder:validation:XValidation:rule="has(self.policyId) != has(self.securityPolicyRef)",message="exactly one of policyId and securityPolicyRef is required"
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="the spec is immutable"
// +kubebuilder:validation:XValidation:rule="!self.attachmentId.startsWith('projects/') || (!has(self.excludedFolders) && !has(self.excludedProjects))",message="exclusions are not supported for a project's association"
type ComputeOrganizationSecurityPolicyAssociationSpec struct {
	/* Immutable. The resource that the security policy is attached to: organizations/{organization}, folders/{folder} or projects/{project number}. */
	AttachmentID string `json:"attachmentId"`

	/* Immutable. Folders under the attachment the policy does not apply to, as folders/{folder}. Not on a project's association. */
	// +optional
	ExcludedFolders []string `json:"excludedFolders,omitempty"`

	/* Immutable. Projects under the attachment the policy does not apply to, as projects/{project number}. Not on a project's association. */
	// +optional
	ExcludedProjects []string `json:"excludedProjects,omitempty"`

	/* Immutable. The security policy ID of the association. Exactly one of policyId or securityPolicyRef must be specified. */
	// +optional
	PolicyID *string `json:"policyId,omitempty"`

	/* Immutable. The security policy that this association belongs to. Exactly one of policyId or securityPolicyRef must be specified. */
	// +optional
	SecurityPolicyRef *ComputeOrganizationSecurityPolicyRef `json:"securityPolicyRef,omitempty"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`
}

// +kcc:status:proto=google.cloud.compute.v1.SecurityPolicyAssociation
type ComputeOrganizationSecurityPolicyAssociationStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputeOrganizationSecurityPolicyAssociation's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* The display name of the security policy of the association. */
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeorganizationsecuritypolicyassociation;hflcomputeorganizationsecuritypolicyassociations
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeOrganizationSecurityPolicyAssociation is the Schema for the compute API
// +k8s:openapi-gen=true
type ComputeOrganizationSecurityPolicyAssociation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeOrganizationSecurityPolicyAssociationSpec   `json:"spec,omitempty"`
	Status ComputeOrganizationSecurityPolicyAssociationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComputeOrganizationSecurityPolicyAssociationList contains a list of ComputeOrganizationSecurityPolicyAssociation
type ComputeOrganizationSecurityPolicyAssociationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeOrganizationSecurityPolicyAssociation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeOrganizationSecurityPolicyAssociation{}, &ComputeOrganizationSecurityPolicyAssociationList{})
}
