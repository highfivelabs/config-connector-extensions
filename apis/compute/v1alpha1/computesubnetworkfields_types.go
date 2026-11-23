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
// decision 37 names it). Config Connector's ComputeSubnetwork (Terraform)
// has no allowSubnetCidrRoutesOverlap; this kind owns that field of a
// subnet Config Connector's kind (or anything else) made. The field keeps
// the Compute Engine API's name (Subnetwork in compute v1).

import (
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeSubnetworkFieldsGVK = GroupVersion.WithKind("ComputeSubnetworkFields")

// ComputeSubnetworkFieldsSpec is the fields of a subnet this object owns.
type ComputeSubnetworkFieldsSpec struct {
	// Immutable. The subnet: Config Connector's ComputeSubnetwork by name
	// (compute.cnrm.cloud.google.com), or any subnet by external, as
	// "projects/{{projectID}}/regions/{{region}}/subnetworks/{{subnetwork}}".
	// The subnet must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *computev1beta1.ComputeSubnetworkRef `json:"resourceRef"`

	// Whether the subnet's primary and secondary ranges may overlap (or
	// contain) the destination of a static or dynamic route that is
	// more specific, so that such a route is not refused.
	// +required
	AllowSubnetCidrRoutesOverlap *bool `json:"allowSubnetCidrRoutesOverlap"`
}

// ComputeSubnetworkFieldsStatus defines the observed state.
type ComputeSubnetworkFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The subnet whose fields this object sets, as
	// projects/{project}/regions/{region}/subnetworks/{subnetwork}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputesubnetworkfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeSubnetworkFields sets fields of a subnet that Config Connector's
// ComputeSubnetwork has no field for (allowSubnetCidrRoutesOverlap), and
// only those: it patches the subnet with a body naming them and its
// fingerprint. Deleting it leaves them as they are.
// +k8s:openapi-gen=true
type ComputeSubnetworkFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeSubnetworkFieldsSpec   `json:"spec,omitempty"`
	Status ComputeSubnetworkFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeSubnetworkFieldsList contains a list of ComputeSubnetworkFields
type ComputeSubnetworkFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeSubnetworkFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeSubnetworkFields{}, &ComputeSubnetworkFieldsList{})
}
