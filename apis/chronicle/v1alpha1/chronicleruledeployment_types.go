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

import (
	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ChronicleRuleDeploymentGVK = GroupVersion.WithKind("ChronicleRuleDeployment")

// ChronicleRuleDeploymentSpec is the deployment of a Google SecOps
// (Chronicle) rule: whether it runs, alerts or is archived, and how often
// it runs, as Terraform's google_chronicle_rule_deployment sets them. The
// deployment exists as long as its rule does; the object sets it and
// deleting the object leaves it as it is.
// +kubebuilder:validation:XValidation:rule="!(has(self.archived) && self.archived && has(self.enabled) && self.enabled)",message="an archived rule is not enabled: set enabled to false to archive it"
// +kubebuilder:validation:XValidation:rule="!(has(self.archived) && self.archived && has(self.alerting) && self.alerting)",message="an archived rule does not alert: the API turns alerting off when it archives a rule"
type ChronicleRuleDeploymentSpec struct {
	// The rule whose deployment this is: a Config Connector ChronicleRule,
	//  or external
	//  "projects/{project}/locations/{location}/instances/{instance}/rules/{rule}",
	//  the rule's ID without a revision. Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ruleRef is immutable"
	RuleRef *kccchroniclev1alpha1.ChronicleRuleRef `json:"ruleRef"`

	// Whether the rule runs continuously against incoming data. Left out,
	//  the deployment keeps what it has.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Whether the rule's detections are alerts. Left out, the deployment
	//  keeps what it has.
	// +optional
	Alerting *bool `json:"alerting,omitempty"`

	// Whether the rule is archived. Archiving needs the rule disabled
	//  (enabled false), and the API turns alerting off; an archived rule's
	//  enabled, alerting and runFrequency cannot change until it is
	//  unarchived (the controller unarchives before it changes them).
	// +optional
	Archived *bool `json:"archived,omitempty"`

	// How often the rule runs. LIVE and HOURLY are legacy values the API
	//  may store as LIVE_CUSTOMIZABLE and HOURLY_CUSTOMIZABLE (and HOURLY
	//  or DAILY as LIVE for a single-event rule); as Terraform does, the
	//  stored value is taken to match (status.observedState.runFrequency
	//  has it).
	// +optional
	// +kubebuilder:validation:Enum=LIVE;HOURLY;DAILY;LIVE_CUSTOMIZABLE;HOURLY_CUSTOMIZABLE
	RunFrequency *string `json:"runFrequency,omitempty"`

	// The schedule of a customizable run frequency (LIVE_CUSTOMIZABLE,
	//  HOURLY_CUSTOMIZABLE).
	// +optional
	ScheduleCustomizations *ChronicleRuleDeploymentScheduleCustomizations `json:"scheduleCustomizations,omitempty"`
}

// ChronicleRuleDeploymentScheduleCustomizations is the schedule of a
// customizable run frequency.
type ChronicleRuleDeploymentScheduleCustomizations struct {
	// Whether to add delays and runs so that enrichment is complete, at the
	//  cost of more late-arriving detections.
	// +optional
	EnsureEnrichmentCompleteness *bool `json:"ensureEnrichmentCompleteness,omitempty"`

	// How long to delay the rule's first run for late-arriving data, a
	//  duration such as "3600s".
	// +optional
	LateArrivingDataAdjustment *string `json:"lateArrivingDataAdjustment,omitempty"`
}

// ChronicleRuleDeploymentStatus defines the config connector machine state of ChronicleRuleDeployment
type ChronicleRuleDeploymentStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The deployment's name,
	//  projects/{project}/locations/{location}/instances/{instance}/rules/{rule}/deployment.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *ChronicleRuleDeploymentObservedState `json:"observedState,omitempty"`
}

// ChronicleRuleDeploymentObservedState is the deployment's output-only
// fields, as Terraform's computed attributes read them.
type ChronicleRuleDeploymentObservedState struct {
	// The run frequency the API stores (it may differ from a legacy
	//  spec.runFrequency).
	RunFrequency *string `json:"runFrequency,omitempty"`

	// When the deployment was last archived; empty unless it is archived.
	ArchiveTime *string `json:"archiveTime,omitempty"`

	// DEFAULT, LIMITED or PAUSED.
	ExecutionState *string `json:"executionState,omitempty"`

	// The rules this rule takes detections from (chained rules).
	ProducerRules []string `json:"producerRules,omitempty"`

	// The rules that take this rule's detections (chained rules).
	ConsumerRules []string `json:"consumerRules,omitempty"`

	// When the deployment's alerting last changed.
	LastAlertStatusChangeTime *string `json:"lastAlertStatusChangeTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflchronicleruledeployment;hflchronicleruledeployments
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ChronicleRuleDeployment is the Schema for the ChronicleRuleDeployment API
// +k8s:openapi-gen=true
type ChronicleRuleDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ChronicleRuleDeploymentSpec   `json:"spec,omitempty"`
	Status ChronicleRuleDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ChronicleRuleDeploymentList contains a list of ChronicleRuleDeployment
type ChronicleRuleDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChronicleRuleDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChronicleRuleDeployment{}, &ChronicleRuleDeploymentList{})
}
