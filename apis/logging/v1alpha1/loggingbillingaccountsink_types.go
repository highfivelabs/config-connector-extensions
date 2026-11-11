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
// apis/logging/v1beta1/logginglogsink_types.go (UPSTREAM.md) as
// LoggingBillingAccountSink (docs/design.md decision 15). Changed: the
// parent is a required, immutable billingAccountRef instead of one of
// projectRef, folderRef and organizationRef; includeChildren (folders and
// organizations only) and uniqueWriterIdentity (a billing account's sink
// always has its own writer identity, as Terraform's
// google_logging_billing_account_sink sends) are dropped; the destination,
// BigQuery options and exclusions are Config Connector's own types, reused;
// status gains externalRef; group, labels and short names are ours.

package v1alpha1

import (
	billingv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/billing/v1alpha1"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var LoggingBillingAccountSinkGVK = GroupVersion.WithKind("LoggingBillingAccountSink")

// LoggingBillingAccountSinkSpec defines the desired state of LoggingBillingAccountSink
// +kcc:spec:proto=google.logging.v2.LogSink
type LoggingBillingAccountSinkSpec struct {
	/* Immutable. The billing account in which to create the sink, as billingAccounts/{id} or the bare ID. */
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="billingAccountRef is immutable"
	BillingAccountRef billingv1alpha1.BillingAccountRef `json:"billingAccountRef"`

	// +optional
	BigqueryOptions *loggingv1beta1.LogSinkBigQueryOptions `json:"bigqueryOptions,omitempty"`

	/* A description of this sink. The maximum length of the description is 8000 characters. */
	// +optional
	Description *string `json:"description,omitempty"`

	Destination loggingv1beta1.LogSinkDestination `json:"destination"`

	/* If set to True, then this sink is disabled and it does not export any log entries. */
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	/* Log entries that match any of the exclusion filters will not be exported. If a log entry is matched by both filter and one of exclusion's filters, it will not be exported. */
	// +optional
	Exclusions []loggingv1beta1.LogSinkExclusions `json:"exclusions,omitempty"`

	/* The filter to apply when exporting logs. Only log entries that match the filter are exported. */
	// +optional
	Filter *string `json:"filter,omitempty"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`
}

// LoggingBillingAccountSinkStatus defines the config connector machine state of LoggingBillingAccountSink
type LoggingBillingAccountSinkStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The sink's resource name, billingAccounts/{id}/sinks/{sink}.
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`

	/* The identity associated with this sink. This identity must be granted write access to the configured destination. */
	// +optional
	WriterIdentity *string `json:"writerIdentity,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflloggingbillingaccountsink;hflloggingbillingaccountsinks
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// LoggingBillingAccountSink is the Schema for the LoggingBillingAccountSink API
// +k8s:openapi-gen=true
type LoggingBillingAccountSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   LoggingBillingAccountSinkSpec   `json:"spec,omitempty"`
	Status LoggingBillingAccountSinkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// LoggingBillingAccountSinkList contains a list of LoggingBillingAccountSink
type LoggingBillingAccountSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoggingBillingAccountSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoggingBillingAccountSink{}, &LoggingBillingAccountSinkList{})
}
