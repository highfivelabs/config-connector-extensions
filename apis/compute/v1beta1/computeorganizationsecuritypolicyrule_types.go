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
// apis/compute/v1alpha1/computeorganizationsecuritypolicyrule_types.go (UPSTREAM.md) as a stable kind
// (v1beta1, docs/design.md decision 13). Changed: securityPolicyRef beside
// policyId, exactly one of them (CEL; the policy's ID is the server's, so
// policyId alone cannot name a policy made in the same apply), as the
// association has; the policy and the priority are immutable (CEL);
// direction and versionedExpr are enums (the values upstream lists);
// match.description is refused (CEL; the API has no such field); at least
// one layer4Config;
// status.externalRef and status.ruleTupleCount; group, labels and short
// names are ours.

package v1beta1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeOrganizationSecurityPolicyRuleGVK = GroupVersion.WithKind("ComputeOrganizationSecurityPolicyRule")

type OrganizationSecurityPolicyRuleMatchConfigLayer4Config struct {
	/* The IP protocol to which this rule applies. The protocol
	type is required when creating a firewall rule.
	This value can either be one of the following well
	known protocol strings (tcp, udp, icmp, esp, ah, ipip, sctp),
	or the IP protocol number. */
	IPProtocol string `json:"ipProtocol"`

	/* An optional list of ports to which this rule applies. This field
	is only applicable for UDP or TCP protocol. Each entry must be
	either an integer or a range. If not specified, this rule
	applies to connections through any port.

	Example inputs include: ["22"], ["80","443"], and
	["12345-12349"]. */
	// +optional
	Ports []string `json:"ports,omitempty"`
}

type OrganizationSecurityPolicyRuleMatchConfig struct {
	/* Destination IP address range in CIDR format. Required for
	EGRESS rules. */
	// +optional
	DestIPRanges []string `json:"destIpRanges,omitempty"`

	/* Pairs of IP protocols and ports that the rule should match. */
	// +required
	// +kubebuilder:validation:MinItems=1
	Layer4Config []OrganizationSecurityPolicyRuleMatchConfigLayer4Config `json:"layer4Config"`

	/* Source IP address range in CIDR format. Required for
	INGRESS rules. */
	// +optional
	SrcIPRanges []string `json:"srcIpRanges,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!has(self.description)",message="match.description is not in the Compute Engine API (beta or v1); leave it unset"
type OrganizationSecurityPolicyRuleMatch struct {
	/* The configuration options for matching the rule. */
	// +required
	Config *OrganizationSecurityPolicyRuleMatchConfig `json:"config"`

	/* A description of the rule. Refused: the Compute Engine API's
	SecurityPolicyRuleMatcher has no description (Magic Modules still lists
	it); kept so that upstream's schema is unchanged. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* Preconfigured versioned expression. For organization security policy rules,
	the only supported type is "FIREWALL". Default value: "FIREWALL" Possible values: ["FIREWALL"]. */
	// +optional
	// +kubebuilder:validation:Enum=FIREWALL
	VersionedExpr *string `json:"versionedExpr,omitempty"`
}

// +kcc:spec:proto=google.cloud.compute.v1.SecurityPolicyRule
// +kubebuilder:validation:XValidation:rule="has(self.policyId) != has(self.securityPolicyRef)",message="exactly one of policyId and securityPolicyRef is required"
// +kubebuilder:validation:XValidation:rule="has(self.policyId) == has(oldSelf.policyId) && (!has(self.policyId) || self.policyId == oldSelf.policyId)",message="policyId is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.securityPolicyRef) == has(oldSelf.securityPolicyRef) && (!has(self.securityPolicyRef) || self.securityPolicyRef == oldSelf.securityPolicyRef)",message="securityPolicyRef is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.resourceID) == has(oldSelf.resourceID) && (!has(self.resourceID) || self.resourceID == oldSelf.resourceID)",message="resourceID is immutable"
type ComputeOrganizationSecurityPolicyRuleSpec struct {
	/* The Action to perform when the client connection triggers the rule. Can currently be either
	"allow", "deny" or "goto_next". */
	Action string `json:"action"`

	/* A description of the rule. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* The direction in which this rule applies. If unspecified
	an INGRESS rule is created. Possible values: ["INGRESS", "EGRESS"]. */
	// +optional
	// +kubebuilder:validation:Enum=INGRESS;EGRESS
	Direction *string `json:"direction,omitempty"`

	/* Denotes whether to enable logging for a particular rule.
	If logging is enabled, logs will be exported to the
	configured export destination in Stackdriver. */
	// +optional
	EnableLogging *bool `json:"enableLogging,omitempty"`

	/* A match condition that incoming traffic is evaluated
	against. If it evaluates to true, the corresponding 'action' is
	enforced. */
	// +required
	Match *OrganizationSecurityPolicyRuleMatch `json:"match"`

	/* Immutable. The ID of the OrganizationSecurityPolicy this
	rule applies to. Exactly one of policyId or securityPolicyRef must be specified. */
	// +optional
	PolicyID *string `json:"policyId,omitempty"`

	/* Immutable. The OrganizationSecurityPolicy this rule applies to.
	Exactly one of policyId or securityPolicyRef must be specified. */
	// +optional
	SecurityPolicyRef *ComputeOrganizationSecurityPolicyRef `json:"securityPolicyRef,omitempty"`

	/* If set to true, the specified action is not enforced. */
	// +optional
	Preview *bool `json:"preview,omitempty"`

	/* Immutable. Optional. The priority of the resource. Used
	for creation and acquisition. When unset, the value of `metadata.name`
	is used as the default. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`

	/* A list of network resource URLs to which this rule applies.
	This field allows you to control which network's VMs get
	this rule. If this field is left blank, all VMs
	within the organization will receive the rule. */
	// +optional
	TargetResources []string `json:"targetResources,omitempty"`

	/* A list of service accounts indicating the sets of
	instances that are applied with this rule. */
	// +optional
	TargetServiceAccounts []string `json:"targetServiceAccounts,omitempty"`
}

type ComputeOrganizationSecurityPolicyRuleStatus struct {
	/* Conditions represent the latest available observation
	of the resource's current state. */
	// +optional
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* ObservedGeneration is the generation of the resource
	that was most recently observed by the Config Connector controller.
	If this is equal to metadata.generation, then that means that the
	current reported status reflects the most recent desired state of
	the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	/* The rule, as locations/global/securityPolicies/{policy}/rules/{priority}
	(not an API resource name: the API addresses a rule by its priority). */
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`

	/* How complex the rule is, counted by the API against the policy's limit. */
	// +optional
	RuleTupleCount *int64 `json:"ruleTupleCount,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeorganizationsecuritypolicyrule;hflcomputeorganizationsecuritypolicyrules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeOrganizationSecurityPolicyRule is the Schema for the compute API
// +k8s:openapi-gen=true
type ComputeOrganizationSecurityPolicyRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeOrganizationSecurityPolicyRuleSpec   `json:"spec,omitempty"`
	Status ComputeOrganizationSecurityPolicyRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeOrganizationSecurityPolicyRuleList contains a list of ComputeOrganizationSecurityPolicyRule
type ComputeOrganizationSecurityPolicyRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeOrganizationSecurityPolicyRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeOrganizationSecurityPolicyRule{}, &ComputeOrganizationSecurityPolicyRuleList{})
}
