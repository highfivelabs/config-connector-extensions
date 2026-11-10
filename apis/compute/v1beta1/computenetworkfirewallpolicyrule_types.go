// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// Written here, as a stable kind (docs/design.md decision 13). Config
// Connector's ComputeNetworkFirewallPolicyRule is v1alpha1 and
// Terraform-only at the pin: no direct types, identity or controller. This
// kind has the shape of our ComputeRegionNetworkFirewallPolicyRule, which
// follows Config Connector's direct ComputeFirewallPolicyRule (the rule of an
// organization or folder policy; both share the Compute Engine API's
// FirewallPolicyRule and FirewallPolicyRuleMatcher messages), with Config
// Connector's ComputeNetworkFirewallPolicyRef for the policy and the
// network contexts that Config Connector's alpha CRD has.

import (
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/iamrefs"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeNetworkFirewallPolicyRuleGVK = GroupVersion.WithKind("ComputeNetworkFirewallPolicyRule")

// ComputeNetworkFirewallPolicyRuleSpec is one rule of a global network
// firewall policy. A rule is identified by its priority in the policy.
type ComputeNetworkFirewallPolicyRuleSpec struct {
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
	Match NetworkFirewallPolicyRuleMatch `json:"match"`

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
	TargetSecureTags []NetworkFirewallPolicyRuleSecureTag `json:"targetSecureTags,omitempty"`
}

// NetworkFirewallPolicyRuleMatch is what traffic a rule matches.
type NetworkFirewallPolicyRuleMatch struct {
	// Pairs of IP protocols and ports the rule matches.
	// +kubebuilder:validation:MinItems=1
	Layer4Configs []NetworkFirewallPolicyRuleLayer4Config `json:"layer4Configs"`

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
	SrcSecureTags []NetworkFirewallPolicyRuleSecureTag `json:"srcSecureTags,omitempty"`
	// The network context of the source (ingress rules): INTERNET,
	// NON_INTERNET, VPC_NETWORKS or INTRA_VPC.
	// +optional
	SrcNetworkContext *string `json:"srcNetworkContext,omitempty"`
	// The network context of the destination (egress rules): INTERNET or
	// NON_INTERNET.
	// +optional
	DestNetworkContext *string `json:"destNetworkContext,omitempty"`
}

// NetworkFirewallPolicyRuleLayer4Config is an IP protocol and its ports.
type NetworkFirewallPolicyRuleLayer4Config struct {
	// tcp, udp, icmp, esp, ah, ipip, sctp, or the protocol number.
	IPProtocol string `json:"ipProtocol"`
	// Ports or port ranges (80, 8000-8080), for tcp and udp. Unset means
	// every port.
	// +optional
	Ports []string `json:"ports,omitempty"`
}

// NetworkFirewallPolicyRuleSecureTag is a tag value.
type NetworkFirewallPolicyRuleSecureTag struct {
	// The tag value, as tagValues/{id}.
	// +kubebuilder:validation:Pattern=`^tagValues/[0-9]+$`
	Name string `json:"name"`
}

// ComputeNetworkFirewallPolicyRuleStatus defines the observed state.
type ComputeNetworkFirewallPolicyRuleStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The rule, as projects/{project}/global/firewallPolicies/{policy}/rules/{priority}
	// (not an API resource name: the API addresses a rule by its priority).
	ExternalRef *string `json:"externalRef,omitempty"`

	// How complex the rule is, counted by the API against the policy's limit.
	RuleTupleCount *int64 `json:"ruleTupleCount,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputenetworkfirewallpolicyrule;hflcomputenetworkfirewallpolicyrules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeNetworkFirewallPolicyRule is a rule of a global network firewall
// policy.
// +k8s:openapi-gen=true
type ComputeNetworkFirewallPolicyRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeNetworkFirewallPolicyRuleSpec   `json:"spec,omitempty"`
	Status ComputeNetworkFirewallPolicyRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeNetworkFirewallPolicyRuleList contains a list of ComputeNetworkFirewallPolicyRule
type ComputeNetworkFirewallPolicyRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeNetworkFirewallPolicyRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeNetworkFirewallPolicyRule{}, &ComputeNetworkFirewallPolicyRuleList{})
}
