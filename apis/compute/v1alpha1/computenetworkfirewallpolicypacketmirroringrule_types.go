// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here. Config Connector has no packet mirroring rule kind at the
// pin and no pull request for one; Terraform's
// google_compute_network_firewall_policy_packet_mirroring_rule (Magic
// Modules' NetworkFirewallPolicyPacketMirroringRule.yaml, read, not copied)
// has these fields, but for tlsInspect, which the API refuses on a packet
// mirroring rule even as false (a live run, 2026-11-10: "TLS Inspection
// cannot be set for packet mirroring rule"). The shape follows our ComputeNetworkFirewallPolicyRule
// (compute.gcp.hfl.dev/v1beta1): a packet mirroring rule is the same
// FirewallPolicyRule message in the policy's packetMirroringRules, addressed
// by its priority, with Config Connector's ComputeNetworkFirewallPolicyRef
// for the policy.

import (
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeNetworkFirewallPolicyPacketMirroringRuleGVK = GroupVersion.WithKind("ComputeNetworkFirewallPolicyPacketMirroringRule")

// ComputeNetworkFirewallPolicyPacketMirroringRuleSpec is one packet
// mirroring rule of a global network firewall policy: which traffic of the
// policy's networks is copied to a mirroring endpoint group. A rule is
// identified by its priority in the policy.
// +kubebuilder:validation:XValidation:rule="self.action != 'mirror' || has(self.securityProfileGroup)",message="a mirror rule needs securityProfileGroup"
type ComputeNetworkFirewallPolicyPacketMirroringRuleSpec struct {
	// The project of the policy. Optional: the policy names its project.
	// When set, it must be the policy's project. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef,omitempty"`

	// The policy the rule belongs to: a Config Connector
	// ComputeNetworkFirewallPolicy by name, or
	// projects/{project}/global/firewallPolicies/{policy}. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="firewallPolicyRef is immutable"
	FirewallPolicyRef computev1beta1.ComputeNetworkFirewallPolicyRef `json:"firewallPolicyRef"`

	// The rule's priority, 0 to 2147483647: rules are evaluated from the
	// lowest number (the highest priority) to the highest, and a policy has
	// one packet mirroring rule at each priority. Immutable.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2147483647
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="priority is immutable"
	Priority int64 `json:"priority"`

	// What to do with matching traffic: mirror (to the mirroring endpoint
	// group of securityProfileGroup's custom mirroring profile),
	// do_not_mirror, or goto_next.
	// +kubebuilder:validation:Enum=mirror;do_not_mirror;goto_next
	Action string `json:"action"`

	// The direction the rule applies to: INGRESS or EGRESS.
	// +kubebuilder:validation:Enum=INGRESS;EGRESS
	Direction string `json:"direction"`

	// What traffic the rule matches.
	Match NetworkFirewallPolicyPacketMirroringRuleMatch `json:"match"`

	// An optional name for the rule; it is not an identifier and can change.
	// +optional
	RuleName *string `json:"ruleName,omitempty"`

	// An optional description of the rule.
	// +optional
	Description *string `json:"description,omitempty"`

	// Whether the rule is switched off. Unset means it is enforced.
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// The security profile group whose custom mirroring profile names
	// where mirrored packets go, as
	// //networksecurity.googleapis.com/{group's name} or the group's name
	// (organizations/{org}/locations/global/securityProfileGroups/{group} or
	// projects/{project}/locations/global/securityProfileGroups/{group}).
	// Only for action mirror, which needs it.
	// +optional
	SecurityProfileGroup *string `json:"securityProfileGroup,omitempty"`

	// Secure tags (tag values, tagValues/{id}) whose instances the rule
	// applies to. With none, the rule applies to every instance on the
	// policy's networks.
	// +optional
	TargetSecureTags []NetworkFirewallPolicyPacketMirroringRuleSecureTag `json:"targetSecureTags,omitempty"`
}

// NetworkFirewallPolicyPacketMirroringRuleMatch is what traffic a packet
// mirroring rule matches.
type NetworkFirewallPolicyPacketMirroringRuleMatch struct {
	// Pairs of IP protocols and ports the rule matches.
	// +kubebuilder:validation:MinItems=1
	Layer4Configs []NetworkFirewallPolicyPacketMirroringRuleLayer4Config `json:"layer4Configs"`

	// Source CIDR ranges.
	// +optional
	SrcIPRanges []string `json:"srcIPRanges,omitempty"`

	// Destination CIDR ranges.
	// +optional
	DestIPRanges []string `json:"destIPRanges,omitempty"`
}

// NetworkFirewallPolicyPacketMirroringRuleLayer4Config is an IP protocol
// and its ports.
type NetworkFirewallPolicyPacketMirroringRuleLayer4Config struct {
	// tcp, udp, icmp, esp, ah, ipip, sctp, all, or the protocol number.
	IPProtocol string `json:"ipProtocol"`
	// Ports or port ranges (80, 8000-8080), for tcp and udp. Unset means
	// every port.
	// +optional
	Ports []string `json:"ports,omitempty"`
}

// NetworkFirewallPolicyPacketMirroringRuleSecureTag is a tag value.
type NetworkFirewallPolicyPacketMirroringRuleSecureTag struct {
	// The tag value, as tagValues/{id}.
	// +kubebuilder:validation:Pattern=`^tagValues/[0-9]+$`
	Name string `json:"name"`
}

// ComputeNetworkFirewallPolicyPacketMirroringRuleStatus defines the
// observed state.
type ComputeNetworkFirewallPolicyPacketMirroringRuleStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The rule, as
	// projects/{project}/global/firewallPolicies/{policy}/packetMirroringRules/{priority}
	// (not an API resource name: the API addresses a rule by its priority).
	ExternalRef *string `json:"externalRef,omitempty"`

	// How complex the rule is, counted by the API against the policy's limit.
	RuleTupleCount *int64 `json:"ruleTupleCount,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputenetworkfirewallpolicypacketmirroringrule;hflcomputenetworkfirewallpolicypacketmirroringrules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeNetworkFirewallPolicyPacketMirroringRule is a packet mirroring rule
// of a global network firewall policy.
// +k8s:openapi-gen=true
type ComputeNetworkFirewallPolicyPacketMirroringRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeNetworkFirewallPolicyPacketMirroringRuleSpec   `json:"spec,omitempty"`
	Status ComputeNetworkFirewallPolicyPacketMirroringRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeNetworkFirewallPolicyPacketMirroringRuleList contains a list of ComputeNetworkFirewallPolicyPacketMirroringRule
type ComputeNetworkFirewallPolicyPacketMirroringRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeNetworkFirewallPolicyPacketMirroringRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeNetworkFirewallPolicyPacketMirroringRule{}, &ComputeNetworkFirewallPolicyPacketMirroringRuleList{})
}
