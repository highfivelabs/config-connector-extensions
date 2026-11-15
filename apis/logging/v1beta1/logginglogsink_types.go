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
// apis/logging/v1beta1/logginglogsink_types.go (UPSTREAM.md) as our own
// LoggingLogSink (docs/design.md decisions 19 and 26, section 10 question
// 7). Changed: interceptChildren (a folder's or organization's sink that
// keeps the logs it routes from its children's sinks), with the API's
// rules for it as CEL rules; a Cloud project as a destination
// (destination.projectRef), which an intercepting sink needs; exactly one
// of projectRef, folderRef and organizationRef (upstream falls back to the
// namespace's project), each immutable, as are resourceID, includeChildren
// and interceptChildren; exactly one destination; status gains
// externalRef; the BigQuery options and exclusion types and the other
// destination references are Config Connector's own; group, labels and
// short names are ours.

package v1beta1

import (
	bigqueryv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/bigquery/v1beta1"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var LoggingLogSinkGVK = GroupVersion.WithKind("LoggingLogSink")

// LoggingLogSinkDestination is where a sink routes its logs: exactly one of
// a BigQuery dataset, a log bucket, a Pub/Sub topic, a Cloud Storage
// bucket, or a Cloud project (Config Connector's has the first four).
// +kubebuilder:validation:XValidation:rule="(has(self.bigQueryDatasetRef) ? 1 : 0) + (has(self.loggingLogBucketRef) ? 1 : 0) + (has(self.pubSubTopicRef) ? 1 : 0) + (has(self.storageBucketRef) ? 1 : 0) + (has(self.projectRef) ? 1 : 0) == 1",message="exactly one of bigQueryDatasetRef, loggingLogBucketRef, pubSubTopicRef, storageBucketRef and projectRef is required"
type LoggingLogSinkDestination struct {
	// +optional
	BigQueryDatasetRef *bigqueryv1beta1.DatasetRef `json:"bigQueryDatasetRef,omitempty"`

	// +optional
	LoggingLogBucketRef *loggingv1beta1.LoggingLogBucketRef `json:"loggingLogBucketRef,omitempty"`

	// +optional
	PubSubTopicRef *pubsubv1beta1.PubSubTopicRef `json:"pubSubTopicRef,omitempty"`

	// +optional
	StorageBucketRef *storagev1beta1.StorageBucketRef `json:"storageBucketRef,omitempty"`

	/* A Cloud project, as the API's logging.googleapis.com/projects/{project}: the project's own sinks route what arrives. The destination of an intercepting sink (interceptChildren). */
	// +optional
	ProjectRef *refs.ProjectRef `json:"projectRef,omitempty"`
}

// LoggingLogSinkSpec defines the desired state of LoggingLogSink
// +kcc:spec:proto=google.logging.v2.LogSink
// +kubebuilder:validation:XValidation:rule="(has(self.projectRef) ? 1 : 0) + (has(self.folderRef) ? 1 : 0) + (has(self.organizationRef) ? 1 : 0) == 1",message="exactly one of projectRef, folderRef and organizationRef is required"
// +kubebuilder:validation:XValidation:rule="!has(self.interceptChildren) || !self.interceptChildren || has(self.folderRef) || has(self.organizationRef)",message="interceptChildren needs a folderRef or organizationRef parent"
// +kubebuilder:validation:XValidation:rule="!has(self.interceptChildren) || !self.interceptChildren || (has(self.includeChildren) && self.includeChildren)",message="interceptChildren needs includeChildren true"
// +kubebuilder:validation:XValidation:rule="!has(self.interceptChildren) || !self.interceptChildren || has(self.destination.projectRef)",message="interceptChildren needs a Cloud project destination (destination.projectRef)"
// +kubebuilder:validation:XValidation:rule="(has(self.includeChildren) && self.includeChildren) == (has(oldSelf.includeChildren) && oldSelf.includeChildren)",message="includeChildren is immutable"
// +kubebuilder:validation:XValidation:rule="(has(self.interceptChildren) && self.interceptChildren) == (has(oldSelf.interceptChildren) && oldSelf.interceptChildren)",message="interceptChildren is immutable"
type LoggingLogSinkSpec struct {
	// +optional
	BigqueryOptions *loggingv1beta1.LogSinkBigQueryOptions `json:"bigqueryOptions,omitempty"`

	/* A description of this sink. The maximum length of the description is 8000 characters. */
	// +optional
	Description *string `json:"description,omitempty"`

	Destination LoggingLogSinkDestination `json:"destination"`

	/* If set to True, then this sink is disabled and it does not export any log entries. */
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	/* Log entries that match any of the exclusion filters will not be exported. If a log entry is matched by both filter and one of exclusion's filters, it will not be exported. */
	// +optional
	Exclusions []loggingv1beta1.LogSinkExclusions `json:"exclusions,omitempty"`

	/* The filter to apply when exporting logs. Only log entries that match the filter are exported. */
	// +optional
	Filter *string `json:"filter,omitempty"`

	/* Immutable. The folder in which to create the sink. Exactly one of projectRef, folderRef and organizationRef is required. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="folderRef is immutable"
	FolderRef *refs.FolderRef `json:"folderRef,omitempty"`

	/* Immutable. Whether or not to include children organizations in the sink export. If true, logs associated with child projects are also exported; otherwise only logs relating to the provided organization are included. */
	// +optional
	IncludeChildren *bool `json:"includeChildren,omitempty"`

	/* Immutable. Whether the sink intercepts its children's logs: logs it matches are not routed by the non-_Required sinks of the folders and projects under it. Only for a folder's or organization's sink, which must have includeChildren true and a Cloud project destination (destination.projectRef). */
	// +optional
	InterceptChildren *bool `json:"interceptChildren,omitempty"`

	/* Immutable. The organization in which to create the sink. Exactly one of projectRef, folderRef and organizationRef is required. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="organizationRef is immutable"
	OrganizationRef *refs.OrganizationRef `json:"organizationRef,omitempty"`

	/* Immutable. The project in which to create the sink. Exactly one of projectRef, folderRef and organizationRef is required. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refs.ProjectRef `json:"projectRef,omitempty"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	/* Whether or not to create a unique identity associated with this sink. If false (the default), then the writer_identity used is serviceAccount:cloud-logs@system.gserviceaccount.com. If true, then a unique service account is created and used for this sink. If you wish to publish logs across projects, you must set unique_writer_identity to true. A folder's or organization's sink always has a unique identity (it is sent as true). */
	// +optional
	UniqueWriterIdentity *bool `json:"uniqueWriterIdentity,omitempty"`
}

// LoggingLogSinkStatus defines the config connector machine state of LoggingLogSink
type LoggingLogSinkStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The sink's resource name: projects/{project}/sinks/{sink},
	// folders/{folder}/sinks/{sink} or organizations/{organization}/sinks/{sink}.
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`

	/* The identity associated with this sink. This identity must be granted write access to the configured destination. */
	// +optional
	WriterIdentity *string `json:"writerIdentity,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfllogginglogsink;hfllogginglogsinks
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// LoggingLogSink is the Schema for the LoggingLogSink API
// +k8s:openapi-gen=true
type LoggingLogSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   LoggingLogSinkSpec   `json:"spec,omitempty"`
	Status LoggingLogSinkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// LoggingLogSinkList contains a list of LoggingLogSink
type LoggingLogSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoggingLogSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoggingLogSink{}, &LoggingLogSinkList{})
}
