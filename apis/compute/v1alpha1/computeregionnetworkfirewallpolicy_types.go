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
// apis/compute/v1beta1/computenetworkfirewallpolicy_types.go (UPSTREAM.md).
// ComputeNetworkFirewallPolicy (global) becomes ComputeRegionNetworkFirewallPolicy;
// written here: spec.region, and status.regionNetworkFirewallPolicyId in place of
// networkFirewallPolicyId (Terraform's google_compute_region_network_firewall_policy
// names); projectRef is the v1beta1 ProjectRef our other kinds use; group,
// labels and short names.

package v1alpha1

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionNetworkFirewallPolicyGVK = GroupVersion.WithKind("ComputeRegionNetworkFirewallPolicy")

// ComputeRegionNetworkFirewallPolicySpec defines the desired state of ComputeRegionNetworkFirewallPolicy
// +kcc:spec:proto=google.cloud.compute.v1.FirewallPolicy
type ComputeRegionNetworkFirewallPolicySpec struct {
	/* An optional description of this resource. Provide this property when you create the resource. */
	// +optional
	Description *string `json:"description,omitempty"`

	// The project that this resource belongs to.
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The region of the firewall policy. Immutable.
	Region string `json:"region"`

	// The ComputeRegionNetworkFirewallPolicy name. If not given, the metadata.name will be used.
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`
}

// ComputeRegionNetworkFirewallPolicyStatus defines the config connector machine state of ComputeRegionNetworkFirewallPolicy
type ComputeRegionNetworkFirewallPolicyStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputeRegionNetworkFirewallPolicy's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* Creation timestamp in RFC3339 text format. */
	// +optional
	CreationTimestamp *string `json:"creationTimestamp,omitempty"`

	/* Fingerprint of the resource. This field is used internally during updates of this resource. */
	// +optional
	Fingerprint *string `json:"fingerprint,omitempty"`

	/* The unique identifier for the resource. This identifier is defined by the server. */
	// +optional
	RegionNetworkFirewallPolicyId *string `json:"regionNetworkFirewallPolicyId,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	/* Total count of all firewall policy rule tuples. A firewall policy can not exceed a set number of tuples. */
	// +optional
	RuleTupleCount *int64 `json:"ruleTupleCount,omitempty"`

	/* Server-defined URL for the resource. */
	// +optional
	SelfLink *string `json:"selfLink,omitempty"`

	/* Server-defined URL for this resource with the resource id. */
	// +optional
	SelfLinkWithId *string `json:"selfLinkWithId,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregionnetworkfirewallpolicy;hflcomputeregionnetworkfirewallpolicies
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionNetworkFirewallPolicy is the Schema for the ComputeRegionNetworkFirewallPolicy API
// +k8s:openapi-gen=true
// +kcc:proto=google.cloud.compute.v1.FirewallPolicy
type ComputeRegionNetworkFirewallPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionNetworkFirewallPolicySpec   `json:"spec,omitempty"`
	Status ComputeRegionNetworkFirewallPolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionNetworkFirewallPolicyList contains a list of ComputeRegionNetworkFirewallPolicy
type ComputeRegionNetworkFirewallPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionNetworkFirewallPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionNetworkFirewallPolicy{}, &ComputeRegionNetworkFirewallPolicyList{})
}
