// Copyright 2025 Google LLC
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

// Modified by High Five Labs for config-connector-extensions: the rule's
// types (SecurityPolicyRule*) are copied from Config Connector
// apis/compute/v1beta1/computesecuritypolicy_types.go and types.generated.go
// (UPSTREAM.md), the rule of its opt-in direct ComputeSecurityPolicy, so a
// rule reads as spec.rule[] of Config Connector's global policy does.
// Changed: priority, the policy and the region are the kind's own fields;
// headerAction, redirectOptions and rateLimitOptions.exceedRedirectOptions
// are left out (the API takes them only on a global CLOUD_ARMOR policy);
// networkMatch (unreachable upstream, commented out) is uncommented with the
// API's and Terraform's names (srcIpRanges, destIpRanges); the lists are
// atomic. The kind, its reference and its status are written here.

package v1alpha1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionSecurityPolicyRuleGVK = GroupVersion.WithKind("ComputeRegionSecurityPolicyRule")

// ComputeRegionSecurityPolicyRef names a regional security policy. Exactly
// one of name and external is required.
// +kubebuilder:validation:XValidation:rule="has(self.name) != has(self.external)",message="exactly one of name and external is required"
type ComputeRegionSecurityPolicyRef struct {
	// The name of a ComputeRegionSecurityPolicy object
	// (compute.gcp.hfl.dev).
	// +optional
	Name string `json:"name,omitempty"`

	// The namespace of that object; the referring object's by default.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// A policy not managed by an object, as
	// projects/{project}/regions/{region}/securityPolicies/{name}.
	// +optional
	External string `json:"external,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.match) != has(self.networkMatch)",message="exactly one of match (a CLOUD_ARMOR policy's rule) and networkMatch (a CLOUD_ARMOR_NETWORK policy's) is required"
type ComputeRegionSecurityPolicyRuleSpec struct {
	// Immutable. The regional security policy the rule is in.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="securityPolicyRef is immutable"
	SecurityPolicyRef ComputeRegionSecurityPolicyRef `json:"securityPolicyRef"`

	// Immutable. The rule's priority, which names it in the policy: from 0
	// (evaluated first) to 2147483647 (the default rule's, evaluated last).
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2147483647
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="priority is immutable"
	Priority int64 `json:"priority"`

	// The Action to perform when the rule is matched. The following are the valid actions: - allow: allow access to target. - deny(STATUS): deny access to target, returns the HTTP response code specified. Valid values for `STATUS` are 403, 404, and 502. - rate_based_ban: limit client traffic to the configured threshold and ban the client if the traffic exceeds the threshold. Configure parameters for this action in RateLimitOptions. Requires rate_limit_options to be set. - throttle: limit client traffic to the configured threshold. Configure parameters for this action in rateLimitOptions. Requires rate_limit_options to be set for this.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRule.action
	Action string `json:"action"`

	// An optional description of this resource. Provide this property when you create the resource.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRule.description
	Description *string `json:"description,omitempty"`

	// A match condition that incoming traffic is evaluated against. If it evaluates to true, the corresponding 'action' is enforced.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRule.match
	Match *SecurityPolicyRuleMatcher `json:"match,omitempty"`

	// A match condition that incoming packets are evaluated against for CLOUD_ARMOR_NETWORK security policies. If it matches, the corresponding 'action' is enforced. The match criteria for a rule consists of built-in match fields (like 'srcIpRanges') and potentially multiple user-defined match fields ('userDefinedFields'). Field values may be extracted directly from the packet or derived from it (e.g. 'srcRegionCodes'). Some fields may not be present in every packet (e.g. 'srcPorts'). A user-defined field is only present if the base header is found in the packet and the entire field is in bounds. Each match field may specify which values can match it, listing one or more ranges, prefixes, or exact values that are considered a match for the field. A field value must be present in order to match a specified match field. If no match values are specified for a match field, then any field value is considered to match it, and it's not required to be present. For strings specifying '*' is also equivalent to match all. For a packet to match a rule, all specified match fields must match the corresponding field values derived from the packet.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRule.network_match
	NetworkMatch *SecurityPolicyRuleNetworkMatcher `json:"networkMatch,omitempty"`

	// Preconfigured WAF configuration to be applied for the rule. If the rule does not evaluate preconfigured WAF rules, i.e., if evaluatePreconfiguredWaf() is not used, this field will have no effect.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRule.preconfigured_waf_config
	PreconfiguredWafConfig *SecurityPolicyRulePreconfiguredWafConfig `json:"preconfiguredWafConfig,omitempty"`

	// If set to true, the specified action is not enforced.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRule.preview
	Preview *bool `json:"preview,omitempty"`

	// Must be specified if the action is "rate_based_ban" or "throttle". Cannot be specified for any other actions.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRule.rate_limit_options
	RateLimitOptions *SecurityPolicyRuleRateLimitOptions `json:"rateLimitOptions,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyRuleMatcher
type SecurityPolicyRuleMatcher struct {
	// The configuration options available when specifying versioned_expr. This field must be specified if versioned_expr is specified and cannot be specified if versioned_expr is not specified.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleMatcher.config
	Config *SecurityPolicyRuleMatcherConfig `json:"config,omitempty"`

	// User defined CEVAL expression. A CEVAL expression is used to specify match criteria such as origin.ip, source.region_code and contents in the request header. Expressions containing `evaluateThreatIntelligence` require Cloud Armor Managed Protection Plus tier and are not supported in Edge Policies nor in Regional Policies. Expressions containing `evaluatePreconfiguredExpr('sourceiplist-*')` require Cloud Armor Managed Protection Plus tier and are only supported in Global Security Policies.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleMatcher.expr
	Expr *SecurityPolicyRuleMatcherExpr `json:"expr,omitempty"`

	// Preconfigured versioned expression. If this field is specified, config must also be specified. Available preconfigured expressions along with their requirements are: SRC_IPS_V1 - must specify the corresponding src_ip_range field in config.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleMatcher.versioned_expr
	VersionedExpr *string `json:"versionedExpr,omitempty"`
}

type SecurityPolicyRuleMatcherExpr struct {
	// Textual representation of an expression in Common Expression Language syntax.
	// +optional
	Expression *string `json:"expression,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyRuleMatcherConfig
type SecurityPolicyRuleMatcherConfig struct {
	// CIDR IP address range. Maximum number of src_ip_ranges allowed is 10.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleMatcherConfig.src_ip_ranges
	SrcIPRanges []string `json:"srcIpRanges,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher
type SecurityPolicyRuleNetworkMatcher struct {
	// Destination IPv4/IPv6 addresses or CIDR prefixes, in standard text format.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.dest_ip_ranges
	DestIPRanges []string `json:"destIpRanges,omitempty"`

	// Destination port numbers for TCP/UDP/SCTP. Each element can be a 16-bit unsigned decimal number (e.g. "80") or range (e.g. "0-1023").
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.dest_ports
	DestPorts []string `json:"destPorts,omitempty"`

	// IPv4 protocol / IPv6 next header (after extension headers). Each element can be an 8-bit unsigned decimal number (e.g. "6"), range (e.g. "253-254"), or one of the following protocol names: "tcp", "udp", "icmp", "esp", "ah", "ipip", or "sctp".
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.ip_protocols
	IPProtocols []string `json:"ipProtocols,omitempty"`

	// BGP Autonomous System Number associated with the source IP address.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.src_asns
	SrcAsns []int64 `json:"srcAsns,omitempty"`

	// Source IPv4/IPv6 addresses or CIDR prefixes, in standard text format.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.src_ip_ranges
	SrcIPRanges []string `json:"srcIpRanges,omitempty"`

	// Source port numbers for TCP/UDP/SCTP. Each element can be a 16-bit unsigned decimal number (e.g. "80") or range (e.g. "0-1023").
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.src_ports
	SrcPorts []string `json:"srcPorts,omitempty"`

	// Two-letter ISO 3166-1 alpha-2 country code associated with the source IP address.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.src_region_codes
	SrcRegionCodes []string `json:"srcRegionCodes,omitempty"`

	// User-defined fields. Each element names a defined field and lists the matching values for that field.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcher.user_defined_fields
	UserDefinedFields []SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch `json:"userDefinedFields,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch
type SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch struct {
	// Name of the user-defined field, as given in the definition.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch.name
	Name string `json:"name"`

	// Matching values of the field. Each element can be a 32-bit unsigned decimal or hexadecimal (starting with "0x") number (e.g. "64") or range (e.g. "0x400-0x7ff").
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch.values
	Values []string `json:"values,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions
type SecurityPolicyRuleRateLimitOptions struct {
	// Can only be specified if the action for the rule is "rate_based_ban". If specified, determines the time (in seconds) the traffic will continue to be banned by the rate limit after the rate falls below the threshold.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.ban_duration_sec
	BanDurationSec *int32 `json:"banDurationSec,omitempty"`

	// Can only be specified if the action for the rule is "rate_based_ban". If specified, the key will be banned for the configured 'ban_duration_sec' when the number of requests that exceed the 'rate_limit_threshold' also exceed this 'ban_threshold'.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.ban_threshold
	BanThreshold *SecurityPolicyRuleRateLimitOptionsThreshold `json:"banThreshold,omitempty"`

	// Action to take for requests that are under the configured rate limit threshold. Valid option is "allow" only.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.conform_action
	ConformAction *string `json:"conformAction,omitempty"`

	// Determines the key to enforce the rate_limit_threshold on: ALL (the default when neither enforceOnKey nor enforceOnKeyConfigs is set), IP, HTTP_HEADER, XFF_IP, HTTP_COOKIE, HTTP_PATH, SNI, REGION_CODE, TLS_JA3_FINGERPRINT, USER_IP or TLS_JA4_FINGERPRINT.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.enforce_on_key
	EnforceOnKey *string `json:"enforceOnKey,omitempty"`

	// If specified, any combination of values of enforce_on_key_type/enforce_on_key_name is treated as the key on which ratelimit threshold/action is enforced. You can specify up to 3 enforce_on_key_configs. If enforce_on_key_configs is specified, enforce_on_key must not be specified.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.enforce_on_key_configs
	EnforceOnKeyConfigs []SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig `json:"enforceOnKeyConfigs,omitempty"`

	// Rate limit key name applicable only for the following key types: HTTP_HEADER -- Name of the HTTP header whose value is taken as the key value. HTTP_COOKIE -- Name of the HTTP cookie whose value is taken as the key value.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.enforce_on_key_name
	EnforceOnKeyName *string `json:"enforceOnKeyName,omitempty"`

	// Action to take for requests that are above the configured rate limit threshold: `deny(STATUS)`, where valid values for `STATUS` are 403, 404, 429, and 502.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.exceed_action
	ExceedAction *string `json:"exceedAction,omitempty"`

	// Threshold at which to begin ratelimiting.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptions.rate_limit_threshold
	RateLimitThreshold *SecurityPolicyRuleRateLimitOptionsThreshold `json:"rateLimitThreshold,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig
type SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig struct {
	// Rate limit key name applicable only for the following key types: HTTP_HEADER -- Name of the HTTP header whose value is taken as the key value. HTTP_COOKIE -- Name of the HTTP cookie whose value is taken as the key value.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig.enforce_on_key_name
	EnforceOnKeyName *string `json:"enforceOnKeyName,omitempty"`

	// Determines the key to enforce the rate_limit_threshold on, as enforceOnKey does.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig.enforce_on_key_type
	EnforceOnKeyType *string `json:"enforceOnKeyType,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptionsThreshold
type SecurityPolicyRuleRateLimitOptionsThreshold struct {
	// Number of HTTP(S) requests for calculating the threshold.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptionsThreshold.count
	Count *int32 `json:"count,omitempty"`

	// Interval over which the threshold is computed.
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyRuleRateLimitOptionsThreshold.interval_sec
	IntervalSec *int32 `json:"intervalSec,omitempty"`
}

type SecurityPolicyRulePreconfiguredWafConfig struct {
	// A list of exclusions to apply during preconfigured WAF evaluation.
	// +optional
	// +listType=atomic
	Exclusion []SecurityPolicyRulePreconfiguredWafConfigExclusion `json:"exclusion,omitempty"`
}

type SecurityPolicyRulePreconfiguredWafConfigExclusion struct {
	// A list of request cookie names whose value will be excluded from inspection during preconfigured WAF evaluation.
	// +optional
	// +listType=atomic
	RequestCookie []SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams `json:"requestCookie,omitempty"`

	// A list of request header names whose value will be excluded from inspection during preconfigured WAF evaluation.
	// +optional
	// +listType=atomic
	RequestHeader []SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams `json:"requestHeader,omitempty"`

	// A list of request query parameter names whose value will be excluded from inspection during preconfigured WAF evaluation. Note that the parameter can be in the query string or in the POST body.
	// +optional
	// +listType=atomic
	RequestQueryParam []SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams `json:"requestQueryParam,omitempty"`

	// A list of request URIs from the request line to be excluded from inspection during preconfigured WAF evaluation. When specifying this field, the query or fragment part should be excluded.
	// +optional
	// +listType=atomic
	RequestUri []SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams `json:"requestUri,omitempty"`

	// A list of target rule IDs under the WAF rule set to apply the preconfigured WAF exclusion. If omitted, it refers to all the rule IDs under the WAF rule set.
	// +optional
	// +listType=atomic
	TargetRuleIds []string `json:"targetRuleIds,omitempty"`

	// Target WAF rule set to apply the preconfigured WAF exclusion.
	// +required
	TargetRuleSet *string `json:"targetRuleSet,omitempty"`
}

type SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams struct {
	// The match operator for the field: EQUALS, STARTS_WITH, ENDS_WITH, CONTAINS or EQUALS_ANY.
	// +required
	Operator *string `json:"operator,omitempty"`

	// The value of the field.
	// +optional
	Value *string `json:"value,omitempty"`
}

type ComputeRegionSecurityPolicyRuleStatus struct {
	// Conditions represent the latest available observations of the
	// object's current state.
	// +optional
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most
	// recently observed by the controller.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The rule, as
	// projects/{project}/regions/{region}/securityPolicies/{policy}/rules/{priority}
	// (not an API resource name: the API addresses a rule by its priority).
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregionsecuritypolicyrule;hflcomputeregionsecuritypolicyrules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionSecurityPolicyRule is a rule of a regional Cloud Armor
// security policy (Terraform's google_compute_region_security_policy_rule):
// a regional external load balancer's (CLOUD_ARMOR, match) or a network
// edge policy's (CLOUD_ARMOR_NETWORK, networkMatch).
// +k8s:openapi-gen=true
type ComputeRegionSecurityPolicyRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionSecurityPolicyRuleSpec   `json:"spec,omitempty"`
	Status ComputeRegionSecurityPolicyRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionSecurityPolicyRuleList contains a list of ComputeRegionSecurityPolicyRule
type ComputeRegionSecurityPolicyRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionSecurityPolicyRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionSecurityPolicyRule{}, &ComputeRegionSecurityPolicyRuleList{})
}
