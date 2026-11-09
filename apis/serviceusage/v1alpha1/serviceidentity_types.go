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

// Modified by High Five Labs for config-connector-extensions: scaffolded by
// hack/scaffold-kind.sh (Config Connector's controllerbuilder generate-types
// from google.api.serviceusage.v1beta1.ServiceIdentity, with
// hack/scaffold/serviceusage/overlay.proto), then edited after Config
// Connector's apis/serviceusage/v1beta1/serviceidentity_types.go
// (UPSTREAM.md): the scaffold's projectRef and location (the proto names no
// parent) are replaced by folderRef or organizationRef; status keeps
// upstream's top-level email beside the generated observedState, and adds
// externalRef.

// API sources for ServiceIdentity, recorded by generate-types:
// +kcc:source:proto=https://github.com/googleapis/googleapis/blob/1765b559c42386788ff0c6412491277b4791107a/google/api/serviceusage/v1beta1/resources.proto
// +kcc:source:service-docs=https://cloud.google.com/service-usage/
// +kcc:source:resource-docs=https://cloud.google.com/service-usage/docs/reference/rest/v1beta1/services/generateServiceIdentity

package v1alpha1

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ServiceIdentityGVK = GroupVersion.WithKind("ServiceIdentity")

// ServiceIdentitySpec defines the desired state of ServiceIdentity
// +kcc:spec:proto=google.api.serviceusage.v1beta1.ServiceIdentity
// +kubebuilder:validation:XValidation:rule="has(self.organizationRef) != has(self.folderRef)",message="exactly one of organizationRef and folderRef is required"
type ServiceIdentitySpec struct {
	// The service whose identity this is, such as osconfig.googleapis.com.
	//  If not given, the metadata.name will be used. Immutable.
	ResourceID *string `json:"resourceID,omitempty"`

	// The folder whose service identity this is. Immutable.
	// +optional
	FolderRef *refsv1beta1.FolderRefDeprecated `json:"folderRef,omitempty"`

	// The organization whose service identity this is. Immutable.
	// +optional
	OrganizationRef *refsv1beta1.OrganizationRef `json:"organizationRef,omitempty"`
}

// ServiceIdentityStatus defines the config connector machine state of ServiceIdentity
type ServiceIdentityStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The email address of the service account that a service producer would use
	//  to access consumer resources.
	// +kcc:proto:field=google.api.serviceusage.v1beta1.ServiceIdentity.email
	Email *string `json:"email,omitempty"`

	// The identity's name, folders/{id}/services/{service} or
	//  organizations/{id}/services/{service}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *ServiceIdentityObservedState `json:"observedState,omitempty"`
}

// ServiceIdentityObservedState is the state of the ServiceIdentity resource as most recently observed in GCP.
// +kcc:observedstate:proto=google.api.serviceusage.v1beta1.ServiceIdentity
type ServiceIdentityObservedState struct {
	// The email address of the service account that a service producer would use
	//  to access consumer resources.
	// +kcc:proto:field=google.api.serviceusage.v1beta1.ServiceIdentity.email
	Email *string `json:"email,omitempty"`

	// The unique and stable id of the service account.
	//  https://cloud.google.com/iam/reference/rest/v1/projects.serviceAccounts#ServiceAccount
	// +kcc:proto:field=google.api.serviceusage.v1beta1.ServiceIdentity.unique_id
	UniqueID *string `json:"uniqueID,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflserviceidentity;hflserviceidentities
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ServiceIdentity is the Schema for the ServiceIdentity API
// +k8s:openapi-gen=true
type ServiceIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ServiceIdentitySpec   `json:"spec,omitempty"`
	Status ServiceIdentityStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ServiceIdentityList contains a list of ServiceIdentity
type ServiceIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceIdentity{}, &ServiceIdentityList{})
}
