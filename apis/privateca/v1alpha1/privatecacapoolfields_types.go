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
// decision 37 names it). Config Connector's PrivateCACAPool (DCL, or its
// direct controller) has no issuancePolicy.backdateDuration or
// issuancePolicy.baselineValues.nameConstraints; this kind owns those
// fields of a CA pool Config Connector's kind (or anything else) made. The
// fields keep the Certificate Authority Service API's names (CaPool in
// privateca v1); backdateDuration's description is Config Connector's
// generated one (apis/privateca/v1beta1/types.generated.go).

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/privatecarefs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var PrivateCACAPoolFieldsGVK = GroupVersion.WithKind("PrivateCACAPoolFields")

// PrivateCACAPoolFieldsSpec is the fields of a CA pool this object owns. A
// field left out is not owned: it is never read or written.
type PrivateCACAPoolFieldsSpec struct {
	// Immutable. The CA pool: Config Connector's PrivateCACAPool by name
	// (privateca.cnrm.cloud.google.com), or any CA pool by external, as
	// "projects/{{projectID}}/locations/{{location}}/caPools/{{caPoolID}}".
	// The pool must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *privatecarefs.PrivateCACAPoolRef `json:"resourceRef"`

	// The fields of the pool's issuance policy this object owns.
	// +required
	IssuancePolicy *CAPoolFields_IssuancePolicy `json:"issuancePolicy"`
}

// CAPoolFields_IssuancePolicy is the part of CaPool.IssuancePolicy this
// kind owns.
// +kubebuilder:validation:XValidation:rule="has(self.backdateDuration) || (has(self.baselineValues) && has(self.baselineValues.nameConstraints))",message="set backdateDuration or baselineValues.nameConstraints"
type CAPoolFields_IssuancePolicy struct {
	// Optional. The duration to backdate all certificates issued from this
	//  [CaPool][google.cloud.security.privateca.v1.CaPool]. If not set, the
	//  certificates will be issued with a not_before_time of the issuance time
	//  (i.e. the current time). If set, the certificates will be issued with a
	//  not_before_time of the issuance time minus the backdate_duration. The
	//  not_after_time will be adjusted to preserve the requested lifetime. The
	//  backdate_duration must be less than or equal to 48 hours.
	// A duration in seconds, such as "3600s".
	// +kcc:proto:field=google.cloud.security.privateca.v1.CaPool.IssuancePolicy.backdate_duration
	// +optional
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]{1,9})?s$`
	BackdateDuration *string `json:"backdateDuration,omitempty"`

	// The part of the pool's baseline X.509 values this object owns.
	// +optional
	BaselineValues *CAPoolFields_BaselineValues `json:"baselineValues,omitempty"`
}

// CAPoolFields_BaselineValues is the part of
// CaPool.IssuancePolicy.baseline_values (X509Parameters) this kind owns.
type CAPoolFields_BaselineValues struct {
	// The name constraints every certificate the pool issues carries
	// (RFC 5280 section 4.2.1.10), owned whole: each list replaces the
	// pool's.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.name_constraints
	// +optional
	NameConstraints *X509Parameters_NameConstraints `json:"nameConstraints,omitempty"`
}

// PrivateCACAPoolFieldsStatus defines the observed state.
type PrivateCACAPoolFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The CA pool whose fields this object sets, as
	// projects/{project}/locations/{location}/caPools/{caPool}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflprivatecacapoolfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// PrivateCACAPoolFields sets fields of a Certificate Authority Service CA
// pool that Config Connector's PrivateCACAPool has no field for
// (issuancePolicy.backdateDuration and
// issuancePolicy.baselineValues.nameConstraints), and only those: it
// patches the pool's issuance policy as the pool has it, with its own
// fields changed. Deleting it clears the fields it set; with
// deletion-policy abandon they stay.
// +k8s:openapi-gen=true
type PrivateCACAPoolFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   PrivateCACAPoolFieldsSpec   `json:"spec,omitempty"`
	Status PrivateCACAPoolFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// PrivateCACAPoolFieldsList contains a list of PrivateCACAPoolFields
type PrivateCACAPoolFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrivateCACAPoolFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrivateCACAPoolFields{}, &PrivateCACAPoolFieldsList{})
}
