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
// decision 37 names it). Config Connector's PrivateCACertificateTemplate
// (DCL, or its direct controller) has no maximumLifetime or
// predefinedValues.nameConstraints; this kind owns those fields of a
// certificate template Config Connector's kind (or anything else) made.
// The fields keep the Certificate Authority Service API's names
// (CertificateTemplate in privateca v1).

import (
	privatecav1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var PrivateCACertificateTemplateFieldsGVK = GroupVersion.WithKind("PrivateCACertificateTemplateFields")

// PrivateCACertificateTemplateFieldsSpec is the fields of a certificate
// template this object owns. A field left out is not owned: it is never
// read or written.
// +kubebuilder:validation:XValidation:rule="has(self.maximumLifetime) || (has(self.predefinedValues) && has(self.predefinedValues.nameConstraints))",message="set maximumLifetime or predefinedValues.nameConstraints"
type PrivateCACertificateTemplateFieldsSpec struct {
	// Immutable. The certificate template: Config Connector's
	// PrivateCACertificateTemplate by name (privateca.cnrm.cloud.google.com),
	// or any template by external, as
	// "projects/{{projectID}}/locations/{{location}}/certificateTemplates/{{certificateTemplateID}}".
	// The template must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *privatecav1beta1.PrivateCACertificateTemplateRef `json:"resourceRef"`

	// Optional. The maximum lifetime allowed for issued certificates that
	// use this template. If the issuing CA pool's issuance policy has a
	// maximum lifetime, the shorter of the two applies; a certificate
	// outliving its issuing authority is cut to the authority's
	// expiry. A duration in seconds, such as "2592000s".
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateTemplate.maximum_lifetime
	// +optional
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]{1,9})?s$`
	MaximumLifetime *string `json:"maximumLifetime,omitempty"`

	// The part of the template's predefined X.509 values this object owns.
	// +optional
	PredefinedValues *CertificateTemplateFields_PredefinedValues `json:"predefinedValues,omitempty"`
}

// CertificateTemplateFields_PredefinedValues is the part of
// CertificateTemplate.predefined_values (X509Parameters) this kind owns.
type CertificateTemplateFields_PredefinedValues struct {
	// The name constraints every certificate issued with the template
	// carries (RFC 5280 section 4.2.1.10), owned whole: each list
	// replaces the template's.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.name_constraints
	// +optional
	NameConstraints *X509Parameters_NameConstraints `json:"nameConstraints,omitempty"`
}

// PrivateCACertificateTemplateFieldsStatus defines the observed state.
type PrivateCACertificateTemplateFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The certificate template whose fields this object sets, as
	// projects/{project}/locations/{location}/certificateTemplates/{certificateTemplate}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflprivatecacertificatetemplatefields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// PrivateCACertificateTemplateFields sets fields of a Certificate
// Authority Service certificate template that Config Connector's
// PrivateCACertificateTemplate has no field for (maximumLifetime and
// predefinedValues.nameConstraints), and only those. Deleting it clears
// the fields it set; with deletion-policy abandon they stay.
// +k8s:openapi-gen=true
type PrivateCACertificateTemplateFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   PrivateCACertificateTemplateFieldsSpec   `json:"spec,omitempty"`
	Status PrivateCACertificateTemplateFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// PrivateCACertificateTemplateFieldsList contains a list of PrivateCACertificateTemplateFields
type PrivateCACertificateTemplateFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrivateCACertificateTemplateFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrivateCACertificateTemplateFields{}, &PrivateCACertificateTemplateFieldsList{})
}
