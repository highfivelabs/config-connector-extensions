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
// decision 37 names it). Config Connector's ComputeNetwork (Terraform, or
// its direct controller) has no BGP best path selection; this kind owns
// those fields of a VPC network Config Connector's kind (or anything else)
// made. The fields keep the Compute Engine API's names (Network in compute
// v1).

import (
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeNetworkFieldsGVK = GroupVersion.WithKind("ComputeNetworkFields")

// ComputeNetworkFieldsSpec is the fields of a VPC network this object owns.
type ComputeNetworkFieldsSpec struct {
	// Immutable. The network: Config Connector's ComputeNetwork by name
	// (compute.cnrm.cloud.google.com), or any network by external, as
	// "projects/{{projectID}}/global/networks/{{network}}". The network
	// must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *computerefs.ComputeNetworkRef `json:"resourceRef"`

	// The network's BGP best path selection (routingConfig). Config
	// Connector's routingMode is not here: it stays Config Connector's.
	// +required
	RoutingConfig *NetworkRoutingConfig `json:"routingConfig"`
}

// NetworkRoutingConfig is the part of Compute Engine's NetworkRoutingConfig
// this kind owns. A field left out is not owned: it is never read or
// written.
// +kubebuilder:validation:XValidation:rule="has(self.bgpBestPathSelectionMode) || has(self.bgpAlwaysCompareMed) || has(self.bgpInterRegionCost)",message="set at least one field of routingConfig"
// +kubebuilder:validation:XValidation:rule="!(has(self.bgpAlwaysCompareMed) || has(self.bgpInterRegionCost)) || (has(self.bgpBestPathSelectionMode) && self.bgpBestPathSelectionMode == 'STANDARD')",message="bgpAlwaysCompareMed and bgpInterRegionCost need bgpBestPathSelectionMode STANDARD"
type NetworkRoutingConfig struct {
	// How Cloud Routers in the network choose among routes learned for the
	// same prefix: LEGACY (the default) or STANDARD.
	// +optional
	// +kubebuilder:validation:Enum=LEGACY;STANDARD
	BgpBestPathSelectionMode *string `json:"bgpBestPathSelectionMode,omitempty"`

	// Compare the multi-exit discriminator of routes from different
	// neighbor ASNs. Only with STANDARD.
	// +optional
	BgpAlwaysCompareMed *bool `json:"bgpAlwaysCompareMed,omitempty"`

	// Whether the inter-region cost is added to a route's MED: DEFAULT
	// or ADD_COST_TO_MED. Only with STANDARD.
	// +optional
	// +kubebuilder:validation:Enum=DEFAULT;ADD_COST_TO_MED
	BgpInterRegionCost *string `json:"bgpInterRegionCost,omitempty"`
}

// ComputeNetworkFieldsStatus defines the observed state.
type ComputeNetworkFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The network whose fields this object sets, as
	// projects/{project}/global/networks/{network}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputenetworkfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeNetworkFields sets fields of a VPC network that Config
// Connector's ComputeNetwork has no field for (routingConfig's BGP best
// path selection), and only those: it patches the network with a body
// naming them. Deleting it leaves them as they are.
// +k8s:openapi-gen=true
type ComputeNetworkFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeNetworkFieldsSpec   `json:"spec,omitempty"`
	Status ComputeNetworkFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeNetworkFieldsList contains a list of ComputeNetworkFields
type ComputeNetworkFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeNetworkFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeNetworkFields{}, &ComputeNetworkFieldsList{})
}
