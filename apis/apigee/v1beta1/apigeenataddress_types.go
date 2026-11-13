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

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/apigee/v1alpha1/apigeenataddress_types.go (UPSTREAM.md) as a stable kind
// (v1beta1, docs/design.md decision 26: Config Connector has it only as
// alpha). Changed: instanceId and resourceID immutable by CEL (the API has
// no update; upstream's controller ignores a change); status.externalRef;
// group, labels and short names are ours.

package v1beta1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ApigeeNATAddressGVK = GroupVersion.WithKind("ApigeeNATAddress")

type ApigeeNATAddressSpec struct {
	/* Immutable. The Apigee instance associated with the Apigee environment,
	in the format 'organizations/{{org_name}}/instances/{{instance_name}}'. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="instanceId is immutable"
	InstanceId string `json:"instanceId"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`
}

type ApigeeNATAddressStatus struct {
	/* Conditions represent the latest available observations of the
	   ApigeeNATAddress's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* A unique specifier for the ApigeeNATAddress resource in GCP:
	organizations/{{org}}/instances/{{instance}}/natAddresses/{{name}}. */
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`

	/* The allocated NAT IP address. */
	// +optional
	IpAddress *string `json:"ipAddress,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	/* State of the NAT IP address. */
	// +optional
	State *string `json:"state,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflapigeenataddress;hflapigeenataddresses
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ApigeeNATAddress is a NAT address reserved for an Apigee instance.
// +k8s:openapi-gen=true
type ApigeeNATAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ApigeeNATAddressSpec   `json:"spec,omitempty"`
	Status ApigeeNATAddressStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApigeeNATAddressList contains a list of ApigeeNATAddress
type ApigeeNATAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApigeeNATAddress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApigeeNATAddress{}, &ApigeeNATAddressList{})
}
