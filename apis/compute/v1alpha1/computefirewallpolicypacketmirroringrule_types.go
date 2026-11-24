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

// Written here: Config Connector has no packet mirroring rule kind. The
// hierarchical policy's rule (firewallPolicies.addPacketMirroringRule, in
// the compute beta API only) after our
// ComputeNetworkFirewallPolicyPacketMirroringRule (compute.gcp.hfl.dev
// v1beta1), whose match, secure tag and layer 4 types it shares; the policy
// is Config Connector's ComputeFirewallPolicyRef.

import (
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hflcomputev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

var ComputeFirewallPolicyPacketMirroringRuleGVK = GroupVersion.WithKind("ComputeFirewallPolicyPacketMirroringRule")

// ComputeFirewallPolicyPacketMirroringRuleSpec is one packet mirroring rule
// of a hierarchical firewall policy (an organization's or a folder's):
// which traffic of the networks under the policy's attachments is copied
// to a mirroring endpoint group. A rule is identified by its priority in
// the policy.
// +kubebuilder:validation:XValidation:rule="self.action != 'mirror' || has(self.securityProfileGroup)",message="a mirror rule needs securityProfileGroup"
type ComputeFirewallPolicyPacketMirroringRuleSpec struct {
	// The policy the rule belongs to: a Config Connector
	// ComputeFirewallPolicy by name, or
	// locations/global/firewallPolicies/{policy}. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="firewallPolicyRef is immutable"
	FirewallPolicyRef computev1beta1.ComputeFirewallPolicyRef `json:"firewallPolicyRef"`

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
	Match hflcomputev1beta1.NetworkFirewallPolicyPacketMirroringRuleMatch `json:"match"`

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
	// (organizations/{org}/locations/global/securityProfileGroups/{group}).
	// Only for action mirror, which needs it.
	// +optional
	SecurityProfileGroup *string `json:"securityProfileGroup,omitempty"`

	// Secure tags (tag values, tagValues/{id}) whose instances the rule
	// applies to. With none, the rule applies to every instance under the
	// policy's attachments.
	// +optional
	TargetSecureTags []hflcomputev1beta1.NetworkFirewallPolicyPacketMirroringRuleSecureTag `json:"targetSecureTags,omitempty"`
}

// ComputeFirewallPolicyPacketMirroringRuleStatus defines the observed
// state.
type ComputeFirewallPolicyPacketMirroringRuleStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The rule, as
	// locations/global/firewallPolicies/{policy}/packetMirroringRules/{priority}
	// (not an API resource name: the API addresses a rule by its priority).
	ExternalRef *string `json:"externalRef,omitempty"`

	// How complex the rule is, counted by the API against the policy's limit.
	RuleTupleCount *int64 `json:"ruleTupleCount,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputefirewallpolicypacketmirroringrule;hflcomputefirewallpolicypacketmirroringrules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeFirewallPolicyPacketMirroringRule is a packet mirroring rule of a
// hierarchical firewall policy.
// +k8s:openapi-gen=true
type ComputeFirewallPolicyPacketMirroringRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeFirewallPolicyPacketMirroringRuleSpec   `json:"spec,omitempty"`
	Status ComputeFirewallPolicyPacketMirroringRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeFirewallPolicyPacketMirroringRuleList contains a list of ComputeFirewallPolicyPacketMirroringRule
type ComputeFirewallPolicyPacketMirroringRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeFirewallPolicyPacketMirroringRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeFirewallPolicyPacketMirroringRule{}, &ComputeFirewallPolicyPacketMirroringRuleList{})
}
