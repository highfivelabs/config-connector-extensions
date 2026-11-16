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

// Written here: a field kind (docs/design.md section 6.6, pattern 2;
// decision 37 names it). Config Connector's KMSCryptoKey has no
// keyAccessJustificationsPolicy; this kind owns that one field of a key
// Config Connector's KMSCryptoKey (or anything else) made. The field keeps
// the Cloud KMS API's name (CryptoKey.keyAccessJustificationsPolicy).

import (
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var KMSCryptoKeyFieldsGVK = GroupVersion.WithKind("KMSCryptoKeyFields")

// KMSCryptoKeyFieldsSpec is the fields of a Cloud KMS key this object
// owns. Every other field of the key stays Config Connector's.
type KMSCryptoKeyFieldsSpec struct {
	// Immutable. The key: Config Connector's KMSCryptoKey by name
	// (kms.cnrm.cloud.google.com), or any key by external, as
	// "projects/{{projectID}}/locations/{{location}}/keyRings/{{keyRing}}/cryptoKeys/{{cryptoKey}}".
	// The key must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *kmsv1beta1.KMSCryptoKeyRef `json:"resourceRef"`

	// The key's Key Access Justifications policy: the reasons for which
	// Google may use the key. Owned whole: the list replaces the key's.
	// +required
	KeyAccessJustificationsPolicy *KeyAccessJustificationsPolicy `json:"keyAccessJustificationsPolicy"`
}

// KeyAccessJustificationsPolicy is Cloud KMS's KeyAccessJustificationsPolicy.
type KeyAccessJustificationsPolicy struct {
	// The access reasons allowed, as the API names them (for example
	// CUSTOMER_INITIATED_ACCESS, GOOGLE_INITIATED_SYSTEM_OPERATION). An
	// empty list allows none: every encrypt, decrypt and sign with the key
	// fails.
	// +optional
	AllowedAccessReasons []string `json:"allowedAccessReasons,omitempty"`
}

// KMSCryptoKeyFieldsStatus defines the observed state.
type KMSCryptoKeyFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The key whose fields this object sets, as
	// projects/{project}/locations/{location}/keyRings/{keyRing}/cryptoKeys/{cryptoKey}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflkmscryptokeyfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// KMSCryptoKeyFields sets fields of a Cloud KMS key that Config Connector's
// KMSCryptoKey has no field for (its keyAccessJustificationsPolicy), and
// only those: it patches the key with an update mask naming them.
// Deleting it clears them; with deletion-policy abandon they stay.
// +k8s:openapi-gen=true
type KMSCryptoKeyFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   KMSCryptoKeyFieldsSpec   `json:"spec,omitempty"`
	Status KMSCryptoKeyFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// KMSCryptoKeyFieldsList contains a list of KMSCryptoKeyFields
type KMSCryptoKeyFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KMSCryptoKeyFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KMSCryptoKeyFields{}, &KMSCryptoKeyFieldsList{})
}
