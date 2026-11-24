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

// Written here: a field kind (docs/design.md section 6.6, pattern 2;
// decision 37 names it). Config Connector's ComputeSecurityPolicy
// (Terraform, or its direct controller) has no Adaptive Protection
// threshold configs and no request body inspection size; this kind owns
// those fields of a Cloud Armor security policy Config Connector's kind
// (or anything else) made. The fields keep the Compute Engine API's names
// (SecurityPolicy in compute v1).

import (
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeSecurityPolicyFieldsGVK = GroupVersion.WithKind("ComputeSecurityPolicyFields")

// ComputeSecurityPolicyFieldsSpec is the fields of a security policy this
// object owns. A field left out is not owned: it is never read or written.
// +kubebuilder:validation:XValidation:rule="has(self.adaptiveProtectionConfig) || has(self.advancedOptionsConfig)",message="set adaptiveProtectionConfig or advancedOptionsConfig"
type ComputeSecurityPolicyFieldsSpec struct {
	// Immutable. The security policy: Config Connector's
	// ComputeSecurityPolicy by name (compute.cnrm.cloud.google.com), or any
	// global security policy by external, as
	// "projects/{{projectID}}/global/securityPolicies/{{name}}". The policy
	// must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *computev1beta1.ComputeSecurityPolicyRef `json:"resourceRef"`

	// The part of the policy's Adaptive Protection configuration this
	// object owns.
	// +optional
	AdaptiveProtectionConfig *SecurityPolicyFieldsAdaptiveProtectionConfig `json:"adaptiveProtectionConfig,omitempty"`

	// The part of the policy's advanced options this object owns.
	// +optional
	AdvancedOptionsConfig *SecurityPolicyFieldsAdvancedOptionsConfig `json:"advancedOptionsConfig,omitempty"`
}

// SecurityPolicyFieldsAdaptiveProtectionConfig is the part of
// SecurityPolicyAdaptiveProtectionConfig this kind owns.
type SecurityPolicyFieldsAdaptiveProtectionConfig struct {
	// The part of the Layer 7 DDoS defense configuration this object owns.
	// +required
	Layer7DdosDefenseConfig *SecurityPolicyFieldsLayer7DdosDefenseConfig `json:"layer7DdosDefenseConfig"`
}

// SecurityPolicyFieldsLayer7DdosDefenseConfig is the part of
// SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfig this kind
// owns.
type SecurityPolicyFieldsLayer7DdosDefenseConfig struct {
	// Adaptive Protection's detection and auto-deploy thresholds, each
	// named, owned whole: the list replaces the policy's (an empty list
	// removes them). Layer 7 DDoS defense (Config Connector's
	// adaptiveProtectionConfig.layer7DdosDefenseConfig.enable) must be on.
	// +required
	// +listType=atomic
	ThresholdConfigs []SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig `json:"thresholdConfigs"`
}

// SecurityPolicyFieldsAdvancedOptionsConfig is the part of
// SecurityPolicyAdvancedOptionsConfig this kind owns.
type SecurityPolicyFieldsAdvancedOptionsConfig struct {
	// The maximum request size chosen by the customer with Waf enabled.
	// Values supported are "8KB", "16KB, "32KB", "48KB" and "64KB".
	// Values are case insensitive.
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdvancedOptionsConfig.request_body_inspection_size
	// +required
	RequestBodyInspectionSize string `json:"requestBodyInspectionSize"`
}

// ComputeSecurityPolicyFieldsStatus defines the observed state.
type ComputeSecurityPolicyFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The security policy whose fields this object sets, as
	// projects/{project}/global/securityPolicies/{name}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputesecuritypolicyfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeSecurityPolicyFields sets fields of a Cloud Armor security policy
// that Config Connector's ComputeSecurityPolicy has no field for (Adaptive
// Protection's threshold configs and the WAF request body inspection
// size), and only those. Deleting it clears the fields it set; with
// deletion-policy abandon they stay.
// +k8s:openapi-gen=true
type ComputeSecurityPolicyFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeSecurityPolicyFieldsSpec   `json:"spec,omitempty"`
	Status ComputeSecurityPolicyFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeSecurityPolicyFieldsList contains a list of ComputeSecurityPolicyFields
type ComputeSecurityPolicyFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeSecurityPolicyFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeSecurityPolicyFields{}, &ComputeSecurityPolicyFieldsList{})
}
