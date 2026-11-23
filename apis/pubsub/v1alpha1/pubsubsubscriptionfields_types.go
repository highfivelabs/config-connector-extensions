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

// Modified by High Five Labs for config-connector-extensions: a field kind
// (docs/design.md section 6.6, pattern 2; decision 37 names it), written
// here. The export fields' comments (serviceAccountEmail,
// filenameDatetimeFormat, maxMessages, avroConfig.useTopicSchema) are
// copied from Config Connector apis/pubsub/v1beta1/types.generated.go
// (UPSTREAM.md), where the generator skipped them because Config
// Connector's own BigQueryConfig and CloudStorageConfig types exist without
// them. BigtableConfig is written here from the API's description (no
// Config Connector type has it). The message transforms are
// PubSubTopicFields' types.

package v1alpha1

import (
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var PubSubSubscriptionFieldsGVK = GroupVersion.WithKind("PubSubSubscriptionFields")

// PubSubSubscriptionFieldsSpec is the fields of a subscription this object
// owns. A field left unset is not owned: this object never sends it.
// +kubebuilder:validation:XValidation:rule="has(self.messageTransforms) || has(self.bigtableConfig) || has(self.bigqueryConfig) || has(self.cloudStorageConfig)",message="set at least one of messageTransforms, bigtableConfig, bigqueryConfig and cloudStorageConfig"
// +kubebuilder:validation:XValidation:rule="!has(self.bigtableConfig) || (!has(self.bigqueryConfig) && !has(self.cloudStorageConfig))",message="a subscription exports to one place: bigtableConfig goes with neither bigqueryConfig nor cloudStorageConfig"
type PubSubSubscriptionFieldsSpec struct {
	// Immutable. The subscription: Config Connector's PubSubSubscription by
	// name (pubsub.cnrm.cloud.google.com), or any subscription by external,
	// as "projects/{{projectID}}/subscriptions/{{subscriptionID}}". The
	// subscription must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *pubsubv1beta1.PubSubSubscriptionRef `json:"resourceRef"`

	// Transforms applied to messages delivered by the subscription, in
	// order.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.pubsub.v1.Subscription.message_transforms
	MessageTransforms []MessageTransform `json:"messageTransforms,omitempty"`

	// A Bigtable export: Pub/Sub writes the subscription's messages to a
	// Bigtable table. The subscription must not push or export elsewhere
	// (Config Connector's PubSubSubscription sets none of pushConfig's
	// pushEndpoint, bigqueryConfig and cloudStorageConfig): a subscription
	// delivers to one place.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.Subscription.bigtable_config
	BigtableConfig *BigtableConfig `json:"bigtableConfig,omitempty"`

	// Fields of the BigQuery export that Config Connector's
	// PubSubSubscription sets (its spec.bigqueryConfig): the export must
	// exist.
	// +optional
	BigqueryConfig *BigQueryConfigFields `json:"bigqueryConfig,omitempty"`

	// Fields of the Cloud Storage export that Config Connector's
	// PubSubSubscription sets (its spec.cloudStorageConfig): the export must
	// exist.
	// +optional
	CloudStorageConfig *CloudStorageConfigFields `json:"cloudStorageConfig,omitempty"`
}

// +kcc:proto=google.pubsub.v1.BigtableConfig
type BigtableConfig struct {
	// Required. The unique name of the table to write messages to, as
	// "projects/{{projectID}}/instances/{{instanceID}}/tables/{{tableID}}".
	// +required
	// +kcc:proto:field=google.pubsub.v1.BigtableConfig.table
	Table *string `json:"table"`

	// Optional. The app profile to use for the Bigtable writes. If not
	// specified, the "default" application profile is used. The app profile
	// must use single-cluster routing.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.BigtableConfig.app_profile_id
	AppProfileID *string `json:"appProfileId,omitempty"`

	// Optional. The service account to use to write to Bigtable. The
	// subscription creator or updater that specifies this field must have
	// `iam.serviceAccounts.actAs` permission on the service account. If not
	// specified, the Pub/Sub service agent,
	// service-{project_number}@gcp-sa-pubsub.iam.gserviceaccount.com, is
	// used.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.BigtableConfig.service_account_email
	ServiceAccountEmail *string `json:"serviceAccountEmail,omitempty"`

	// Optional. When true, write the subscription name, message_id,
	// publish_time, attributes, and ordering_key to additional columns in
	// the table under the pubsub_metadata column family.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.BigtableConfig.write_metadata
	WriteMetadata *bool `json:"writeMetadata,omitempty"`
}

// BigQueryConfigFields are fields of a BigQuery export that Config
// Connector's PubSubSubscription has no field for.
// +kcc:proto=google.pubsub.v1.BigQueryConfig
type BigQueryConfigFields struct {
	// Optional. The service account to use to write to BigQuery. The
	// subscription creator or updater that specifies this field must have
	// `iam.serviceAccounts.actAs` permission on the service account. If not
	// specified, the Pub/Sub [service
	// agent](https://cloud.google.com/iam/docs/service-agents),
	// service-{project_number}@gcp-sa-pubsub.iam.gserviceaccount.com, is used.
	// +required
	// +kcc:proto:field=google.pubsub.v1.BigQueryConfig.service_account_email
	ServiceAccountEmail *string `json:"serviceAccountEmail"`
}

// CloudStorageConfigFields are fields of a Cloud Storage export that
// Config Connector's PubSubSubscription has no field for.
// +kcc:proto=google.pubsub.v1.CloudStorageConfig
// +kubebuilder:validation:XValidation:rule="has(self.serviceAccountEmail) || has(self.filenameDatetimeFormat) || has(self.maxMessages) || has(self.avroConfig)",message="set at least one field of cloudStorageConfig"
type CloudStorageConfigFields struct {
	// Optional. The service account to use to write to Cloud Storage. The
	// subscription creator or updater that specifies this field must have
	// `iam.serviceAccounts.actAs` permission on the service account. If not
	// specified, the Pub/Sub
	// [service agent](https://cloud.google.com/iam/docs/service-agents),
	// service-{project_number}@gcp-sa-pubsub.iam.gserviceaccount.com, is used.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.CloudStorageConfig.service_account_email
	ServiceAccountEmail *string `json:"serviceAccountEmail,omitempty"`

	// Optional. User-provided format string specifying how to represent
	// datetimes in Cloud Storage filenames. See the [datetime format
	// guidance](https://cloud.google.com/pubsub/docs/create-cloudstorage-subscription#file_names).
	// +optional
	// +kcc:proto:field=google.pubsub.v1.CloudStorageConfig.filename_datetime_format
	FilenameDatetimeFormat *string `json:"filenameDatetimeFormat,omitempty"`

	// Optional. The maximum number of messages that can be written to a
	// Cloud Storage file before a new file is created. Min 1000 messages.
	// +optional
	// +kubebuilder:validation:Minimum=1000
	// +kcc:proto:field=google.pubsub.v1.CloudStorageConfig.max_messages
	MaxMessages *int64 `json:"maxMessages,omitempty"`

	// Fields of the export's Avro format, which Config Connector's
	// PubSubSubscription sets (its spec.cloudStorageConfig.avroConfig): the
	// export must write Avro.
	// +optional
	AvroConfig *CloudStorageConfigAvroConfigFields `json:"avroConfig,omitempty"`
}

// +kcc:proto=google.pubsub.v1.CloudStorageConfig.AvroConfig
type CloudStorageConfigAvroConfigFields struct {
	// Optional. When true, the output Cloud Storage file will be serialized
	// using the topic schema, if it exists.
	// +required
	// +kcc:proto:field=google.pubsub.v1.CloudStorageConfig.AvroConfig.use_topic_schema
	UseTopicSchema *bool `json:"useTopicSchema"`
}

// PubSubSubscriptionFieldsStatus defines the observed state.
type PubSubSubscriptionFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The subscription whose fields this object sets, as
	// projects/{project}/subscriptions/{subscription}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The subscription's state as the API answers it: ACTIVE, or
	// RESOURCE_ERROR when it cannot write to its export.
	State *string `json:"state,omitempty"`

	// The Bigtable export's state as the API answers it (ACTIVE, NOT_FOUND,
	// APP_PROFILE_MISCONFIGURED, PERMISSION_DENIED, SCHEMA_MISMATCH,
	// IN_TRANSIT_LOCATION_RESTRICTION, ...), when spec.bigtableConfig is
	// set.
	BigtableConfigState *string `json:"bigtableConfigState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflpubsubsubscriptionfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// PubSubSubscriptionFields sets fields of a Pub/Sub subscription that
// Config Connector's PubSubSubscription has no field for (messageTransforms,
// bigtableConfig, and a few fields of its BigQuery and Cloud Storage
// exports), and only those: it patches the subscription with an update
// mask naming them. Deleting it clears them.
// +k8s:openapi-gen=true
type PubSubSubscriptionFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   PubSubSubscriptionFieldsSpec   `json:"spec,omitempty"`
	Status PubSubSubscriptionFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// PubSubSubscriptionFieldsList contains a list of PubSubSubscriptionFields
type PubSubSubscriptionFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PubSubSubscriptionFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PubSubSubscriptionFields{}, &PubSubSubscriptionFieldsList{})
}
