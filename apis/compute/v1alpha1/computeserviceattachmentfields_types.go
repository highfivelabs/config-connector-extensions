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
// decision 37 names it). Config Connector's ComputeServiceAttachment (DCL)
// has no reconcileConnections or propagatedConnectionLimit; this kind owns
// those two fields of a service attachment Config Connector's kind (or
// anything else) made. The fields keep the Compute Engine API's names
// (ServiceAttachment in compute v1).

import (
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeServiceAttachmentFieldsGVK = GroupVersion.WithKind("ComputeServiceAttachmentFields")

// ComputeServiceAttachmentFieldsSpec is the fields of a Private Service
// Connect service attachment this object owns. A field left out is not
// owned: it is never read or written.
// +kubebuilder:validation:XValidation:rule="has(self.reconcileConnections) || has(self.propagatedConnectionLimit)",message="set at least one of reconcileConnections and propagatedConnectionLimit"
type ComputeServiceAttachmentFieldsSpec struct {
	// Immutable. The service attachment: Config Connector's
	// ComputeServiceAttachment by name (compute.cnrm.cloud.google.com), or
	// any service attachment by external, as
	// "projects/{{projectID}}/regions/{{region}}/serviceAttachments/{{name}}".
	// The service attachment must exist; this object never creates or
	// deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *refs.ComputeServiceAttachmentRef `json:"resourceRef"`

	// Whether a change of the consumer accept or reject lists also applies
	// to the endpoints already connected: true, and an endpoint whose
	// project or network leaves the accept list (or joins the reject list)
	// is disconnected, and a pending one now accepted is connected; false,
	// and a list change affects only endpoints that connect later.
	// +optional
	ReconcileConnections *bool `json:"reconcileConnections,omitempty"`

	// The number of consumer spokes a connected Private Service Connect
	// endpoint may be propagated to through Network Connectivity Center,
	// per endpoint (the API's default is 250).
	// +optional
	// +kubebuilder:validation:Minimum=0
	PropagatedConnectionLimit *int32 `json:"propagatedConnectionLimit,omitempty"`
}

// ComputeServiceAttachmentFieldsStatus defines the observed state.
type ComputeServiceAttachmentFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The service attachment whose fields this object sets, as
	// projects/{project}/regions/{region}/serviceAttachments/{name}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeserviceattachmentfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeServiceAttachmentFields sets fields of a Private Service Connect
// service attachment that Config Connector's ComputeServiceAttachment has
// no field for (reconcileConnections, propagatedConnectionLimit), and only
// those: it patches the service attachment with a body naming them.
// Deleting it leaves them as they are.
// +k8s:openapi-gen=true
type ComputeServiceAttachmentFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeServiceAttachmentFieldsSpec   `json:"spec,omitempty"`
	Status ComputeServiceAttachmentFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeServiceAttachmentFieldsList contains a list of ComputeServiceAttachmentFields
type ComputeServiceAttachmentFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeServiceAttachmentFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeServiceAttachmentFields{}, &ComputeServiceAttachmentFieldsList{})
}
