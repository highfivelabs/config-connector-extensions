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

// Written here: a workload identity pool provider key is in no Config
// Connector code or pull request, nor in Terraform (which has only the
// workforce-pool analogue). The shape follows the IAM v1 API's
// WorkloadIdentityPoolProviderKey, in the layout of Config Connector's
// direct kinds.

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var IAMWorkloadIdentityPoolProviderKeyGVK = GroupVersion.WithKind("IAMWorkloadIdentityPoolProviderKey")

// IAMWorkloadIdentityPoolProviderKeySpec defines the desired state of
// IAMWorkloadIdentityPoolProviderKey. Every field is immutable: Google makes
// the key pair once.
// +kcc:spec:proto=google.iam.v1.WorkloadIdentityPoolProviderKey
type IAMWorkloadIdentityPoolProviderKeySpec struct {
	// The project that this resource belongs to.
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The location of this resource ("global").
	Location *string `json:"location"`

	// The workload identity pool provider the key is for.
	// +required
	WorkloadIdentityPoolProviderRef *IAMWorkloadIdentityPoolProviderRef `json:"workloadIdentityPoolProviderRef"`

	// The key's ID. If not given, the metadata.name will be used.
	ResourceID *string `json:"resourceID,omitempty"`

	// Required. The purpose of the key: ENCRYPTION (of SAML assertions).
	// +required
	Use string `json:"use"`

	// Required. The key's specification.
	// +required
	KeyData *IAMWorkloadIdentityPoolProviderKeyKeyData `json:"keyData"`
}

type IAMWorkloadIdentityPoolProviderKeyKeyData struct {
	// Required. The specification of the key: RSA_2048, RSA_3072 or RSA_4096.
	// +required
	KeySpec string `json:"keySpec"`
}

// IAMWorkloadIdentityPoolProviderKeyStatus defines the observed state.
type IAMWorkloadIdentityPoolProviderKeyStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the IAMWorkloadIdentityPoolProviderKey resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the key as most recently observed in GCP.
	ObservedState *IAMWorkloadIdentityPoolProviderKeyObservedState `json:"observedState,omitempty"`
}

type IAMWorkloadIdentityPoolProviderKeyObservedState struct {
	// The state of the key: ACTIVE or DELETED.
	State *string `json:"state,omitempty"`

	// Time after which a deleted key is purged.
	ExpireTime *string `json:"expireTime,omitempty"`

	// The public half of the key, to configure in the identity provider.
	KeyData *IAMWorkloadIdentityPoolProviderKeyKeyDataObservedState `json:"keyData,omitempty"`
}

type IAMWorkloadIdentityPoolProviderKeyKeyDataObservedState struct {
	// The format of the key, RSA_X509_PEM.
	Format *string `json:"format,omitempty"`

	// The key, in that format.
	Key *string `json:"key,omitempty"`

	// Earliest time the key is valid.
	NotBeforeTime *string `json:"notBeforeTime,omitempty"`

	// Latest time the key is valid.
	NotAfterTime *string `json:"notAfterTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfliamworkloadidentitypoolproviderkey;hfliamworkloadidentitypoolproviderkeys
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// IAMWorkloadIdentityPoolProviderKey is a key Google holds for a workload
// identity pool provider; its public half encrypts the provider's SAML
// assertions.
// +k8s:openapi-gen=true
type IAMWorkloadIdentityPoolProviderKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   IAMWorkloadIdentityPoolProviderKeySpec   `json:"spec,omitempty"`
	Status IAMWorkloadIdentityPoolProviderKeyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// IAMWorkloadIdentityPoolProviderKeyList contains a list of IAMWorkloadIdentityPoolProviderKey
type IAMWorkloadIdentityPoolProviderKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMWorkloadIdentityPoolProviderKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMWorkloadIdentityPoolProviderKey{}, &IAMWorkloadIdentityPoolProviderKeyList{})
}
