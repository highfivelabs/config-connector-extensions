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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// pull request #14418 (head b080f5dc), apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_types.go
// (UPSTREAM.md):
//   - group iam.gcp.hfl.dev, own short names, no cnrm.cloud.google.com
//     labels, the gap label and upstream-kind annotation;
//   - written here: spec.workloadIdentityPoolNamespaceRef, the namespace the
//     managed identity is in, which the PR's types leave out (its identity
//     cannot resolve the pool or namespace); and spec.attestationRules,
//     which the PR does not model (the API sets them with
//     setAttestationRules, as Terraform's attestation_rules does).

// API sources for IAMWorkloadIdentityPoolManagedIdentity, recorded by generate-types:
// +kcc:source:proto=https://github.com/googleapis/googleapis/blob/1765b559c42386788ff0c6412491277b4791107a/google/iam/v1/workload_identity_pool_managed_identity.proto
// +kcc:source:service-docs=https://cloud.google.com/iam/
// +kcc:source:resource-docs=https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools.namespaces.managedIdentities

package v1alpha1

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var IAMWorkloadIdentityPoolManagedIdentityGVK = GroupVersion.WithKind("IAMWorkloadIdentityPoolManagedIdentity")

// IAMWorkloadIdentityPoolManagedIdentitySpec defines the desired state of IAMWorkloadIdentityPoolManagedIdentity
// +kcc:spec:proto=google.iam.v1.WorkloadIdentityPoolManagedIdentity
// +kcc:required-from-proto
type IAMWorkloadIdentityPoolManagedIdentitySpec struct {
	// The project that this resource belongs to.
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The location of this resource.
	Location *string `json:"location"`

	// Written here (not in pull request #14418).

	// The workload identity pool namespace the managed identity is in.
	// +required
	WorkloadIdentityPoolNamespaceRef *IAMWorkloadIdentityPoolNamespaceRef `json:"workloadIdentityPoolNamespaceRef"`

	// The IAMWorkloadIdentityPoolManagedIdentity name. If not given, the metadata.name will be used.
	ResourceID *string `json:"resourceID,omitempty"`
	// Optional. A description of the managed identity. Cannot exceed 256 characters.
	// +kcc:proto:field=google.iam.v1.WorkloadIdentityPoolManagedIdentity.description
	Description *string `json:"description,omitempty"`

	// Optional. Whether the managed identity is disabled.
	// +kcc:proto:field=google.iam.v1.WorkloadIdentityPoolManagedIdentity.disabled
	Disabled *bool `json:"disabled,omitempty"`

	// Written here (not in pull request #14418).

	// The workloads that can receive this identity, at most 50. The list is
	// the whole set: a rule taken off it is removed, and an empty list
	// removes them all. Unset leaves the rules as they are.
	// +kubebuilder:validation:MaxItems=50
	AttestationRules []AttestationRule `json:"attestationRules,omitempty"`
}

// Written here, from the IAM v1 API's AttestationRule.

// AttestationRule allows one workload to receive a managed identity.
type AttestationRule struct {
	// A single workload operating on Google Cloud, for example
	//  "//compute.googleapis.com/projects/123/uid/zones/us-central1-a/instances/12345678".
	// +required
	GoogleCloudResource string `json:"googleCloudResource"`
}

// IAMWorkloadIdentityPoolManagedIdentityStatus defines the config connector machine state of IAMWorkloadIdentityPoolManagedIdentity
type IAMWorkloadIdentityPoolManagedIdentityStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the IAMWorkloadIdentityPoolManagedIdentity resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *IAMWorkloadIdentityPoolManagedIdentityObservedState `json:"observedState,omitempty"`
}

// IAMWorkloadIdentityPoolManagedIdentityObservedState is the state of the IAMWorkloadIdentityPoolManagedIdentity resource as most recently observed in GCP.
// +kcc:observedstate:proto=google.iam.v1.WorkloadIdentityPoolManagedIdentity
type IAMWorkloadIdentityPoolManagedIdentityObservedState struct {
	// Output only. The state of the managed identity.
	// +kcc:proto:field=google.iam.v1.WorkloadIdentityPoolManagedIdentity.state
	State *string `json:"state,omitempty"`

	// Output only. Time after which the managed identity will be permanently purged and cannot be recovered.
	// +kcc:proto:field=google.iam.v1.WorkloadIdentityPoolManagedIdentity.expire_time
	ExpireTime *string `json:"expireTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfliamworkloadidentitypoolmanagedidentity;hfliamworkloadidentitypoolmanagedidentities
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:metadata:annotations="gcp.hfl.dev/upstream-kind=iam.cnrm.cloud.google.com/IAMWorkloadIdentityPoolManagedIdentity"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// IAMWorkloadIdentityPoolManagedIdentity is the Schema for the IAMWorkloadIdentityPoolManagedIdentity API
// +k8s:openapi-gen=true
type IAMWorkloadIdentityPoolManagedIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   IAMWorkloadIdentityPoolManagedIdentitySpec   `json:"spec,omitempty"`
	Status IAMWorkloadIdentityPoolManagedIdentityStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// IAMWorkloadIdentityPoolManagedIdentityList contains a list of IAMWorkloadIdentityPoolManagedIdentity
type IAMWorkloadIdentityPoolManagedIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMWorkloadIdentityPoolManagedIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMWorkloadIdentityPoolManagedIdentity{}, &IAMWorkloadIdentityPoolManagedIdentityList{})
}
