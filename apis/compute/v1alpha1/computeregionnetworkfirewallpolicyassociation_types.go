// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here. The association of a regional network firewall policy with
// a VPC network is in no Config Connector code (the global network policy's
// association is Terraform-only; ComputeFirewallPolicyAssociation is for
// organization and folder policies). The fields follow Terraform's
// google_compute_region_network_firewall_policy_association (read, not
// copied) and the Compute Engine API's FirewallPolicyAssociation.

import (
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionNetworkFirewallPolicyAssociationGVK = GroupVersion.WithKind("ComputeRegionNetworkFirewallPolicyAssociation")

// ComputeRegionNetworkFirewallPolicyAssociationSpec attaches a regional
// network firewall policy to a VPC network. Every field is immutable: an
// association is added or removed, never changed.
type ComputeRegionNetworkFirewallPolicyAssociationSpec struct {
	// The policy to attach. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="firewallPolicyRef is immutable"
	FirewallPolicyRef ComputeRegionNetworkFirewallPolicyRef `json:"firewallPolicyRef"`

	// The VPC network the policy applies to. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="networkRef is immutable"
	NetworkRef computerefs.ComputeNetworkRef `json:"networkRef"`

	// The association's name in the policy. If not given, the
	// metadata.name is used. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`
}

// ComputeRegionNetworkFirewallPolicyAssociationStatus defines the observed state.
type ComputeRegionNetworkFirewallPolicyAssociationStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The association, as projects/{project}/regions/{region}/firewallPolicies/{policy}/associations/{name}
	// (not an API resource name: the API addresses an association by its
	// name in the policy).
	ExternalRef *string `json:"externalRef,omitempty"`

	// The policy's numeric ID, as the association reports it.
	FirewallPolicyID *string `json:"firewallPolicyId,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregionnetworkfirewallpolicyassociation;hflcomputeregionnetworkfirewallpolicyassociations
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionNetworkFirewallPolicyAssociation attaches a regional network
// firewall policy to a VPC network.
// +k8s:openapi-gen=true
type ComputeRegionNetworkFirewallPolicyAssociation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionNetworkFirewallPolicyAssociationSpec   `json:"spec,omitempty"`
	Status ComputeRegionNetworkFirewallPolicyAssociationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionNetworkFirewallPolicyAssociationList contains a list of ComputeRegionNetworkFirewallPolicyAssociation
type ComputeRegionNetworkFirewallPolicyAssociationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionNetworkFirewallPolicyAssociation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionNetworkFirewallPolicyAssociation{}, &ComputeRegionNetworkFirewallPolicyAssociationList{})
}
