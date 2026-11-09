// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here. A rule of a regional network firewall policy is in no
// Config Connector code: its ComputeFirewallPolicyRule is the rule of an
// organization or folder policy (compute.firewallPolicies.addRule), and the
// global network policy's rule is Terraform-only. The match fields follow
// that kind (its ComputeFirewallPolicyRule types, which share the Compute
// Engine API's FirewallPolicyRule and FirewallPolicyRuleMatcher messages);
// the rest follows Terraform's
// google_compute_region_network_firewall_policy_rule (read, not copied).

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/iamrefs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionNetworkFirewallPolicyRuleGVK = GroupVersion.WithKind("ComputeRegionNetworkFirewallPolicyRule")

// ComputeRegionNetworkFirewallPolicyRuleSpec is one rule of a regional
// network firewall policy. A rule is identified by its priority in the
// policy.
type ComputeRegionNetworkFirewallPolicyRuleSpec struct {
	// The policy the rule belongs to. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="firewallPolicyRef is immutable"
	FirewallPolicyRef ComputeRegionNetworkFirewallPolicyRef `json:"firewallPolicyRef"`

	// The rule's priority, 0 to 2147483647: rules are evaluated from the
	// lowest number (the highest priority) to the highest, and a policy has
	// one rule at each priority. Immutable.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2147483647
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="priority is immutable"
	Priority int64 `json:"priority"`

	// What to do when the rule matches: allow, deny, goto_next, or
	// apply_security_profile_group.
	Action string `json:"action"`

	// The direction the rule applies to: INGRESS or EGRESS.
	// +kubebuilder:validation:Enum=INGRESS;EGRESS
	Direction string `json:"direction"`

	// What traffic the rule matches.
	Match RegionFirewallPolicyRuleMatch `json:"match"`

	// An optional name for the rule; it is not an identifier and can change.
	// +optional
	RuleName *string `json:"ruleName,omitempty"`

	// An optional description of the rule.
	// +optional
	Description *string `json:"description,omitempty"`

	// Whether the rule is switched off. Unset means it is enforced.
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// Whether matches are logged. Not allowed on a goto_next rule.
	// +optional
	EnableLogging *bool `json:"enableLogging,omitempty"`

	// Service accounts whose instances the rule applies to. Not together
	// with targetSecureTags. With neither, the rule applies to every
	// instance on the policy's networks.
	// +optional
	TargetServiceAccounts []iamrefs.IAMServiceAccountRef `json:"targetServiceAccounts,omitempty"`

	// Secure tags (tag values, tagValues/{id}) whose instances the rule
	// applies to. Not together with targetServiceAccounts.
	// +optional
	TargetSecureTags []RegionFirewallPolicyRuleSecureTag `json:"targetSecureTags,omitempty"`
}

// RegionFirewallPolicyRuleMatch is what traffic a rule matches.
type RegionFirewallPolicyRuleMatch struct {
	// Pairs of IP protocols and ports the rule matches.
	// +kubebuilder:validation:MinItems=1
	Layer4Configs []RegionFirewallPolicyRuleLayer4Config `json:"layer4Configs"`

	// +optional
	SrcIPRanges []string `json:"srcIPRanges,omitempty"`
	// +optional
	DestIPRanges []string `json:"destIPRanges,omitempty"`
	// Address groups for the source (ingress rules only).
	// +optional
	SrcAddressGroups []string `json:"srcAddressGroups,omitempty"`
	// Address groups for the destination (egress rules only).
	// +optional
	DestAddressGroups []string `json:"destAddressGroups,omitempty"`
	// Domain names for the source (ingress rules only).
	// +optional
	SrcFqdns []string `json:"srcFqdns,omitempty"`
	// Domain names for the destination (egress rules only).
	// +optional
	DestFqdns []string `json:"destFqdns,omitempty"`
	// Country codes for the source (ingress rules only).
	// +optional
	SrcRegionCodes []string `json:"srcRegionCodes,omitempty"`
	// Country codes for the destination (egress rules only).
	// +optional
	DestRegionCodes []string `json:"destRegionCodes,omitempty"`
	// Google Cloud Threat Intelligence lists for the source.
	// +optional
	SrcThreatIntelligences []string `json:"srcThreatIntelligences,omitempty"`
	// Google Cloud Threat Intelligence lists for the destination.
	// +optional
	DestThreatIntelligences []string `json:"destThreatIntelligences,omitempty"`
	// Secure tags (tagValues/{id}) the source must have.
	// +optional
	SrcSecureTags []RegionFirewallPolicyRuleSecureTag `json:"srcSecureTags,omitempty"`
}

// RegionFirewallPolicyRuleLayer4Config is an IP protocol and its ports.
type RegionFirewallPolicyRuleLayer4Config struct {
	// tcp, udp, icmp, esp, ah, ipip, sctp, or the protocol number.
	IPProtocol string `json:"ipProtocol"`
	// Ports or port ranges (80, 8000-8080), for tcp and udp. Unset means
	// every port.
	// +optional
	Ports []string `json:"ports,omitempty"`
}

// RegionFirewallPolicyRuleSecureTag is a tag value.
type RegionFirewallPolicyRuleSecureTag struct {
	// The tag value, as tagValues/{id}.
	// +kubebuilder:validation:Pattern=`^tagValues/[0-9]+$`
	Name string `json:"name"`
}

// ComputeRegionNetworkFirewallPolicyRuleStatus defines the observed state.
type ComputeRegionNetworkFirewallPolicyRuleStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The rule, as projects/{project}/regions/{region}/firewallPolicies/{policy}/rules/{priority}
	// (not an API resource name: the API addresses a rule by its priority).
	ExternalRef *string `json:"externalRef,omitempty"`

	// How complex the rule is, counted by the API against the policy's limit.
	RuleTupleCount *int64 `json:"ruleTupleCount,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregionnetworkfirewallpolicyrule;hflcomputeregionnetworkfirewallpolicyrules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionNetworkFirewallPolicyRule is a rule of a regional network
// firewall policy.
// +k8s:openapi-gen=true
type ComputeRegionNetworkFirewallPolicyRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionNetworkFirewallPolicyRuleSpec   `json:"spec,omitempty"`
	Status ComputeRegionNetworkFirewallPolicyRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionNetworkFirewallPolicyRuleList contains a list of ComputeRegionNetworkFirewallPolicyRule
type ComputeRegionNetworkFirewallPolicyRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionNetworkFirewallPolicyRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionNetworkFirewallPolicyRule{}, &ComputeRegionNetworkFirewallPolicyRuleList{})
}
