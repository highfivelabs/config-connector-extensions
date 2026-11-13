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

package v1beta1

// Copied from Config Connector's
// apis/chronicle/v1alpha1/chroniclereferencelist_types.go (UPSTREAM.md),
// with the instance its spec lacks (upstream's identity has {instance} but
// no field sets it). The leaf types and the observed state are upstream's,
// imported.

import (
	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ChronicleReferenceListGVK = GroupVersion.WithKind("ChronicleReferenceList")

// ChronicleReferenceListSpec is a Google SecOps (Chronicle) reference list
// (Terraform's google_chronicle_reference_list).
type ChronicleReferenceListSpec struct {
	// The project of the Google SecOps instance. Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The location of the Google SecOps instance, such as "us" or "eu".
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location string `json:"location"`

	// The ID of the Google SecOps instance (its customer ID), a UUID.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="instance is immutable"
	Instance string `json:"instance"`

	// The reference list's ID, which is also its display name and how
	// rules name it (%id). It starts with a letter and has only letters,
	// digits and underscores, so a metadata.name with a hyphen needs it.
	// When unset, metadata.name is used. Immutable.
	// +optional
	// +kubebuilder:validation:Pattern=`^[A-Za-z][A-Za-z0-9_]*$`
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// Required. A user-provided description of the reference list.
	// +kcc:proto:field=google.cloud.chronicle.v1.ReferenceList.description
	// +required
	Description *string `json:"description"`

	// Required. The entries of the reference list.
	//  When listed, they are returned in the order that was specified at creation
	//  or update. The combined size of the values of the reference list may not
	//  exceed 6MB.
	// +kcc:proto:field=google.cloud.chronicle.v1.ReferenceList.entries
	// +required
	Entries []kccchroniclev1alpha1.ReferenceListEntry `json:"entries"`

	// Required. The syntax type indicating how list entries should be
	// validated.
	// +kcc:proto:field=google.cloud.chronicle.v1.ReferenceList.syntax_type
	// +required
	// +kubebuilder:validation:Enum=REFERENCE_LIST_SYNTAX_TYPE_PLAIN_TEXT_STRING;REFERENCE_LIST_SYNTAX_TYPE_REGEX;REFERENCE_LIST_SYNTAX_TYPE_CIDR
	SyntaxType *string `json:"syntaxType"`

	// The scope info of the reference list.
	//  During reference list creation, if this field is not set, the reference
	//  list without scopes (an unscoped list) will be created for an unscoped
	//  user. For a scoped user, this field must be set. During reference list
	//  update, if scope_info is requested to be updated, this field must be set.
	// +kcc:proto:field=google.cloud.chronicle.v1.ReferenceList.scope_info
	// +optional
	ScopeInfo *kccchroniclev1alpha1.ScopeInfo `json:"scopeInfo,omitempty"`
}

type ChronicleReferenceListStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The reference list's name,
	//  projects/{project}/locations/{location}/instances/{instance}/referenceLists/{id}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *kccchroniclev1alpha1.ChronicleReferenceListObservedState `json:"observedState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflchroniclereferencelist;hflchroniclereferencelists
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ChronicleReferenceList is a Google SecOps (Chronicle) reference list in
// an instance: Config Connector's ChronicleReferenceList with the instance
// it lacks.
// +k8s:openapi-gen=true
type ChronicleReferenceList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ChronicleReferenceListSpec   `json:"spec,omitempty"`
	Status ChronicleReferenceListStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ChronicleReferenceListList contains a list of ChronicleReferenceList
type ChronicleReferenceListList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChronicleReferenceList `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChronicleReferenceList{}, &ChronicleReferenceListList{})
}
