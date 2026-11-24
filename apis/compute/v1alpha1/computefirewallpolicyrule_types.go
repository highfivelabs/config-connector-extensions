// Copyright 2024 Google LLC
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

package v1alpha1

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector's apis/compute/v1beta1/computefirewallpolicyrule_types.go
// (the direct ComputeFirewallPolicyRule: a rule of an organization's or a
// folder's firewall policy), so the schema is the one users of
// compute.cnrm.cloud.google.com/ComputeFirewallPolicyRule already write.
// Changed: group, labels and short names; the service accounts are typed
// as our other rule kinds' (iamrefs.IAMServiceAccountRef, the same
// schema); the policy reference and the match are not pointers (the same
// schema: both are required). Written here (foundation item 19), the
// fields upstream's mapper marks MISSING and the matcher fields it lacks:
// ruleName, securityProfileGroup, tlsInspect, targetSecureTags and
// match.srcSecureTags, srcNetworkContext, destNetworkContext and
// srcNetworks, with the names and shapes of our
// ComputeNetworkFirewallPolicyRule (compute.gcp.hfl.dev v1beta1).

import (
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/iamrefs"
	commonv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/common/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hflcomputev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

var ComputeFirewallPolicyRuleGVK = GroupVersion.WithKind("ComputeFirewallPolicyRule")

// +kcc:proto=google.cloud.compute.v1.FirewallPolicyRuleMatcherLayer4Config
type FirewallPolicyRuleMatcherLayer4Config struct {
	/* The IP protocol to which this rule applies. The protocol type is required when creating a firewall rule. This value can either be one of the following well known protocol strings (`tcp`, `udp`, `icmp`, `esp`, `ah`, `ipip`, `sctp`), or the IP protocol number. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcherLayer4Config.ip_protocol
	IPProtocol string `json:"ipProtocol"`

	/* An optional list of ports to which this rule applies. This field is only applicable for UDP or TCP protocol. Each entry must be either an integer or a range. If not specified, this rule applies to connections through any port. Example inputs include: ``. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcherLayer4Config.ports
	Ports []string `json:"ports,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.FirewallPolicyRuleMatcher
type FirewallPolicyRuleMatcher struct {
	/* Address groups which should be matched against the traffic destination. Maximum number of destination address groups is 10. Destination address groups is only supported in Egress rules. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.dest_address_groups
	DestAddressGroups []string `json:"destAddressGroups,omitempty"`

	/* Domain names that will be used to match against the resolved domain name of destination of traffic. Can only be specified if DIRECTION is egress. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.dest_fqdns
	DestFqdns []string `json:"destFqdns,omitempty"`

	/* CIDR IP address range. Maximum number of destination CIDR IP ranges allowed is 256. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.dest_ip_ranges
	DestIPRanges []string `json:"destIPRanges,omitempty"`

	/* The Unicode country codes whose IP addresses will be used to match against the source of traffic. Can only be specified if DIRECTION is egress. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.dest_region_codes
	DestRegionCodes []string `json:"destRegionCodes,omitempty"`

	/* Name of the Google Cloud Threat Intelligence list. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.dest_threat_intelligences
	DestThreatIntelligences []string `json:"destThreatIntelligences,omitempty"`

	/* Pairs of IP protocols and ports that the rule should match. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.layer4_configs
	Layer4Configs []FirewallPolicyRuleMatcherLayer4Config `json:"layer4Configs"`

	/* Address groups which should be matched against the traffic source. Maximum number of source address groups is 10. Source address groups is only supported in Ingress rules. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_address_groups
	SrcAddressGroups []string `json:"srcAddressGroups,omitempty"`

	/* Domain names that will be used to match against the resolved domain name of source of traffic. Can only be specified if DIRECTION is ingress. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_fqdns
	SrcFqdns []string `json:"srcFqdns,omitempty"`

	/* CIDR IP address range. Maximum number of source CIDR IP ranges allowed is 256. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_ip_ranges
	SrcIPRanges []string `json:"srcIPRanges,omitempty"`

	/* The Unicode country codes whose IP addresses will be used to match against the source of traffic. Can only be specified if DIRECTION is ingress. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_region_codes
	SrcRegionCodes []string `json:"srcRegionCodes,omitempty"`

	/* Name of the Google Cloud Threat Intelligence list. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_threat_intelligences
	SrcThreatIntelligences []string `json:"srcThreatIntelligences,omitempty"`

	// Written here. Secure tags (organization tag values, tagValues/{id})
	// the source must have (ingress rules only).
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_secure_tags
	// +optional
	SrcSecureTags []hflcomputev1beta1.NetworkFirewallPolicyRuleSecureTag `json:"srcSecureTags,omitempty"`

	// Written here. The network context of the source (ingress rules):
	// INTERNET, NON_INTERNET, VPC_NETWORKS or INTRA_VPC.
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_network_context
	// +optional
	SrcNetworkContext *string `json:"srcNetworkContext,omitempty"`

	// Written here. The network context of the destination (egress rules):
	// INTERNET or NON_INTERNET.
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.dest_network_context
	// +optional
	DestNetworkContext *string `json:"destNetworkContext,omitempty"`

	// Written here. The VPC networks the source must be in (ingress rules,
	// with srcNetworkContext VPC_NETWORKS), as
	// "projects/{{projectID}}/global/networks/{{network}}" or the network's
	// full URL.
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRuleMatcher.src_networks
	// +optional
	SrcNetworks []string `json:"srcNetworks,omitempty"`
}

// ComputeFirewallPolicyRuleSpec is one rule of a hierarchical firewall
// policy (an organization's or a folder's). A rule is identified by its
// priority in the policy.
// +kcc:spec:proto=google.cloud.compute.v1.FirewallPolicyRule
// +kubebuilder:validation:XValidation:rule="(self.action == 'apply_security_profile_group') == has(self.securityProfileGroup)",message="securityProfileGroup is set exactly when action is apply_security_profile_group"
// +kubebuilder:validation:XValidation:rule="!has(self.tlsInspect) || !self.tlsInspect || self.action == 'apply_security_profile_group'",message="tlsInspect needs action apply_security_profile_group"
type ComputeFirewallPolicyRuleSpec struct {
	/* The Action to perform when the client connection triggers the rule. Valid actions are "allow", "deny", "goto_next" and "apply_security_profile_group". */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.action
	// +kubebuilder:validation:Enum=allow;deny;goto_next;apply_security_profile_group
	Action string `json:"action"`

	/* An optional description for this resource. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.description
	Description *string `json:"description,omitempty"`

	/* The direction in which this rule applies. Possible values: INGRESS, EGRESS */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.direction
	// +kubebuilder:validation:Enum=INGRESS;EGRESS
	Direction string `json:"direction"`

	/* Denotes whether the firewall policy rule is disabled. When set to true, the firewall policy rule is not enforced and traffic behaves as if it did not exist. If this is unspecified, the firewall policy rule will be enabled. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.disabled
	Disabled *bool `json:"disabled,omitempty"`

	/* Denotes whether to enable logging for a particular rule. If logging is enabled, logs will be exported to the configured export destination in Stackdriver. Logs may be exported to BigQuery or Pub/Sub. Note: you cannot enable logging on "goto_next" rules. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.enable_logging
	EnableLogging *bool `json:"enableLogging,omitempty"`

	/* Immutable. The policy the rule belongs to: a Config Connector ComputeFirewallPolicy by name, or locations/global/firewallPolicies/{{firewallPolicyID}}. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="firewallPolicyRef is immutable"
	FirewallPolicyRef computev1beta1.ComputeFirewallPolicyRef `json:"firewallPolicyRef"`

	/* A match condition that incoming traffic is evaluated against. If it evaluates to true, the corresponding 'action' is enforced. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.match
	Match FirewallPolicyRuleMatcher `json:"match"`

	/* Immutable. An integer indicating the priority of a rule in the list. The priority must be a positive value between 0 and 2147483647. Rules are evaluated from highest to lowest priority where 0 is the highest priority and 2147483647 is the lowest prority. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.priority
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2147483647
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="priority is immutable"
	Priority int64 `json:"priority"`

	/* The networks the rule applies to; with none, every network under the policy's attachments. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.target_resources
	TargetResources []*computerefs.ComputeNetworkRef `json:"targetResources,omitempty"`

	/* Service accounts whose instances the rule applies to. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.target_service_accounts
	TargetServiceAccounts []*iamrefs.IAMServiceAccountRef `json:"targetServiceAccounts,omitempty"`

	// Written here. An optional name for the rule; it is not an identifier
	// and can change.
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.rule_name
	// +optional
	RuleName *string `json:"ruleName,omitempty"`

	// Written here. Secure tags (organization tag values, tagValues/{id})
	// whose instances the rule applies to. Not together with
	// targetServiceAccounts.
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.target_secure_tags
	// +optional
	TargetSecureTags []hflcomputev1beta1.NetworkFirewallPolicyRuleSecureTag `json:"targetSecureTags,omitempty"`

	// Written here. The security profile group a matching connection is
	// sent to, when action is apply_security_profile_group (and only then),
	// as
	// "//networksecurity.googleapis.com/organizations/{{organizationID}}/locations/global/securityProfileGroups/{{name}}"
	// or the group's https://networksecurity.googleapis.com/v1/... URL.
	// Cloud NGFW Enterprise inspects the traffic with it.
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.security_profile_group
	// +optional
	SecurityProfileGroup *string `json:"securityProfileGroup,omitempty"`

	// Written here. Whether the traffic is TLS-decrypted for inspection
	// (the network needs a TLS inspection policy on its firewall endpoint).
	// Only with action apply_security_profile_group. Unset means false.
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.tls_inspect
	// +optional
	TLSInspect *bool `json:"tlsInspect,omitempty"`
}

// +kcc:status:proto=google.cloud.compute.v1.FirewallPolicyRule
type ComputeFirewallPolicyRuleStatus struct {
	commonv1alpha1.CommonStatus `json:",inline"`
	/* Type of the resource. Always `compute#firewallPolicyRule` for firewall policy rules */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.kind
	Kind *string `json:"kind,omitempty"`

	/* Calculation of the complexity of a single firewall policy rule. */
	// +kcc:proto:field=google.cloud.compute.v1.FirewallPolicyRule.rule_tuple_count
	RuleTupleCount *int64 `json:"ruleTupleCount,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputefirewallpolicyrule;hflcomputefirewallpolicyrules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeFirewallPolicyRule is a rule of a hierarchical firewall policy,
// owning the whole rule: Config Connector's ComputeFirewallPolicyRule with
// the security profile group, TLS inspection, secure tags, rule name and
// network contexts. Use one kind or the other for a rule, never both.
// +k8s:openapi-gen=true
type ComputeFirewallPolicyRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComputeFirewallPolicyRuleSpec   `json:"spec,omitempty"`
	Status ComputeFirewallPolicyRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComputeFirewallPolicyRuleList contains a list of ComputeFirewallPolicyRule
type ComputeFirewallPolicyRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeFirewallPolicyRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeFirewallPolicyRule{}, &ComputeFirewallPolicyRuleList{})
}
