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

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector apis/iam/v1alpha1/iamaccesspolicy_types.go (UPSTREAM.md)
// as a stable kind (v1beta1, docs/design.md decision 26). Changed: the
// parent is one of organizationRef, folderRef and projectRef (decision 18;
// upstream's is an organization only); parent, location and resourceID are
// immutable (CEL, decision 20); etag is in status, not the spec; the details
// are upstream's type, imported; group, labels and short names are ours.

package v1beta1

import (
	kcciamv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var IAMAccessPolicyGVK = GroupVersion.WithKind("IAMAccessPolicy")

// IAMAccessPolicySpec defines the desired state of IAMAccessPolicy: an IAM
// v3 access policy of an organization, folder or project.
// +kubebuilder:validation:XValidation:rule="[has(self.organizationRef), has(self.folderRef), has(self.projectRef)].filter(x, x).size() == 1",message="exactly one of organizationRef, folderRef and projectRef is required"
type IAMAccessPolicySpec struct {
	// The organization the policy belongs to: exactly one of
	// organizationRef, folderRef and projectRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="organizationRef is immutable"
	OrganizationRef *refsv1beta1.OrganizationRef `json:"organizationRef,omitempty"`

	// The folder the policy belongs to: exactly one of organizationRef,
	// folderRef and projectRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="folderRef is immutable"
	FolderRef *refsv1beta1.FolderRefDeprecated `json:"folderRef,omitempty"`

	// The project the policy belongs to: exactly one of organizationRef,
	// folderRef and projectRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef,omitempty"`

	// The location of the policy (global, the default). Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location *string `json:"location,omitempty"`

	// The IAMAccessPolicy name. If not given, the metadata.name will be used. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// Optional. The description of the access policy. Must be less than
	//  or equal to 63 characters.
	// +kcc:proto:field=google.iam.v3.AccessPolicy.display_name
	// +optional
	// +kubebuilder:validation:MaxLength=63
	DisplayName *string `json:"displayName,omitempty"`

	// Optional. User defined annotations. See
	//  https://google.aip.dev/148#annotations for more details such as format and
	//  size limitations
	// +kcc:proto:field=google.iam.v3.AccessPolicy.annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Optional. The details for the access policy: its rules.
	// +kcc:proto:field=google.iam.v3.AccessPolicy.details
	// +optional
	Details *kcciamv1alpha1.AccessPolicyDetails `json:"details,omitempty"`
}

// IAMAccessPolicyStatus defines the config connector machine state of IAMAccessPolicy
type IAMAccessPolicyStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The policy's name,
	//  {organizations|folders|projects}/{id}/locations/{location}/accessPolicies/{policy}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The policy's etag, as the API last returned it.
	// +optional
	Etag *string `json:"etag,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *kcciamv1alpha1.IAMAccessPolicyObservedState `json:"observedState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfliamaccesspolicy;hfliamaccesspolicies
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// IAMAccessPolicy is an IAM v3 access policy of an organization, folder or
// project: Config Connector's IAMAccessPolicy with every parent.
// +k8s:openapi-gen=true
type IAMAccessPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   IAMAccessPolicySpec   `json:"spec,omitempty"`
	Status IAMAccessPolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// IAMAccessPolicyList contains a list of IAMAccessPolicy
type IAMAccessPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMAccessPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMAccessPolicy{}, &IAMAccessPolicyList{})
}
