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

// Written here, after Config Connector's zonal ComputeNetworkEndpoint
// (pkg/clients/generated/apis/compute/v1alpha1/computenetworkendpoint_types.go:
// projectRef, the group, ipAddress, instanceRef) for a regional group, with
// the fields Terraform's google_compute_region_network_endpoint has and the
// zonal kind lacks: port, fqdn, clientDestinationPort. No Config Connector
// code or pull request has a regional network endpoint.

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionNetworkEndpointGVK = GroupVersion.WithKind("ComputeRegionNetworkEndpoint")

// ComputeRegionNetworkEndpointSpec is one endpoint in a regional network
// endpoint group. Every field is immutable: an endpoint is attached or
// detached, never changed.
type ComputeRegionNetworkEndpointSpec struct {
	// The project that this resource belongs to.
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The region of the network endpoint group.
	Region string `json:"region"`

	// The regional network endpoint group the endpoint is in.
	// +required
	RegionNetworkEndpointGroupRef *ComputeRegionNetworkEndpointGroupRef `json:"regionNetworkEndpointGroupRef"`

	// The port of the endpoint.
	// +kubebuilder:validation:Minimum=1
	Port int32 `json:"port"`

	// The IPv4 address of the endpoint (INTERNET_IP_PORT and
	//  GCE_VM_IP_PORTMAP groups).
	IPAddress *string `json:"ipAddress,omitempty"`

	// The fully qualified domain name of the endpoint (INTERNET_FQDN_PORT
	//  groups).
	FQDN *string `json:"fqdn,omitempty"`

	// The client destination port (GCE_VM_IP_PORTMAP groups).
	ClientDestinationPort *int32 `json:"clientDestinationPort,omitempty"`

	// The VM the endpoint is on (GCE_VM_IP_PORTMAP groups), as its name in
	//  the group's project and zone, or its URL.
	Instance *string `json:"instance,omitempty"`
}

// ComputeRegionNetworkEndpointStatus defines the observed state.
type ComputeRegionNetworkEndpointStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The endpoint, as
	//  projects/{project}/regions/{region}/networkEndpointGroups/{group}/networkEndpoints/{ipAddress or fqdn}:{port}
	//  (not an API resource name: endpoints have none).
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregionnetworkendpoint;hflcomputeregionnetworkendpoints
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionNetworkEndpoint is one endpoint (an internet address, a
// domain name or a VM port) in a regional network endpoint group, such as a
// load balancer backend.
// +k8s:openapi-gen=true
type ComputeRegionNetworkEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionNetworkEndpointSpec   `json:"spec,omitempty"`
	Status ComputeRegionNetworkEndpointStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionNetworkEndpointList contains a list of ComputeRegionNetworkEndpoint
type ComputeRegionNetworkEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionNetworkEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionNetworkEndpoint{}, &ComputeRegionNetworkEndpointList{})
}
