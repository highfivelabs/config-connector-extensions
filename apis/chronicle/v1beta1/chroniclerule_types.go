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

package v1beta1

// Copied from Config Connector's apis/chronicle/v1alpha1/chroniclerule_types.go
// (UPSTREAM.md), with the instance its spec lacks (upstream's identity has
// {instance} but no field sets it, so its kind cannot resolve one), the
// etag moved to status (the API computes it), and Terraform's
// deletion_policy. The observed state is upstream's type, imported.

import (
	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ChronicleRuleGVK = GroupVersion.WithKind("ChronicleRule")

// ChronicleRuleSpec is a Google SecOps (Chronicle) detection rule
// (Terraform's google_chronicle_rule).
type ChronicleRuleSpec struct {
	// The project of the Google SecOps instance. Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The location of the Google SecOps instance, such as "us" or "eu".
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location string `json:"location"`

	// The ID of the Google SecOps instance (its customer ID), a UUID.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="instance is immutable"
	Instance string `json:"instance"`

	// The rule's ID (ru_...), which the API assigns when it creates the
	// rule. Set it to acquire a rule that exists; left out, a new rule is
	// created and status.externalRef records its ID. Immutable once set.
	// +optional
	// +kubebuilder:validation:Pattern=`^ru_[A-Za-z0-9-]+$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// The YARA-L content of the rule.
	// +kcc:proto:field=google.cloud.chronicle.v1.Rule.text
	// +required
	// +kubebuilder:validation:MinLength=1
	Text *string `json:"text"`

	// Resource name of the DataAccessScope bound to this rule.
	//  If reference lists are used in the rule, validations will be performed
	//  against this scope to ensure that the reference lists are compatible with
	//  both the user's and the rule's scopes.
	//  The scope should be in the format:
	//  `projects/{project}/locations/{location}/instances/{instance}/dataAccessScopes/{scope}`.
	// +kcc:proto:field=google.cloud.chronicle.v1.Rule.scope
	// +optional
	Scope *string `json:"scope,omitempty"`

	// What deleting the object does to a rule that has retrohunts or
	// detections (Terraform's deletion_policy). DEFAULT, the default:
	// the delete fails, and the object stays, until they are gone. FORCE:
	// they are deleted with the rule. The rule's deployment goes with the
	// rule either way.
	// +optional
	// +kubebuilder:validation:Enum=DEFAULT;FORCE
	DeletionPolicy *string `json:"deletionPolicy,omitempty"`
}

type ChronicleRuleStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The rule's name,
	//  projects/{project}/locations/{location}/instances/{instance}/rules/{rule}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The rule's etag, as the API last returned it.
	// +optional
	Etag *string `json:"etag,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *kccchroniclev1alpha1.ChronicleRuleObservedState `json:"observedState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflchroniclerule;hflchroniclerules
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ChronicleRule is a Google SecOps (Chronicle) detection rule in an
// instance: Config Connector's ChronicleRule with the instance it lacks.
// +k8s:openapi-gen=true
type ChronicleRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ChronicleRuleSpec   `json:"spec,omitempty"`
	Status ChronicleRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ChronicleRuleList contains a list of ChronicleRule
type ChronicleRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChronicleRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChronicleRule{}, &ChronicleRuleList{})
}
