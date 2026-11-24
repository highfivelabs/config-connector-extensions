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
// here; its field types (IngestionDataSourceSettings*, PlatformLogsSettings,
// MessageTransform, JavaScriptUDF) are copied from Config Connector
// apis/pubsub/v1beta1/types.generated.go (UPSTREAM.md), where they are
// generated but commented out as unreachable (Config Connector's PubSubTopic
// has neither field). Changed: the JSON names are the API's (clientId,
// tenantId, subscriptionId, clusterId, identityPoolId, javascriptUdf: the
// generator capitalized ID); the lists are atomic; the empty format
// messages are objects that take no fields.

package v1alpha1

import (
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var PubSubTopicFieldsGVK = GroupVersion.WithKind("PubSubTopicFields")

// PubSubTopicFieldsSpec is the fields of a topic this object owns. A field
// left unset is not owned: this object never sends it.
// +kubebuilder:validation:XValidation:rule="has(self.ingestionDataSourceSettings) || has(self.messageTransforms)",message="set at least one of ingestionDataSourceSettings and messageTransforms"
type PubSubTopicFieldsSpec struct {
	// Immutable. The topic: Config Connector's PubSubTopic by name
	// (pubsub.cnrm.cloud.google.com), or any topic by external, as
	// "projects/{{projectID}}/topics/{{topicID}}". The topic must exist;
	// this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *pubsubv1beta1.PubSubTopicRef `json:"resourceRef"`

	// Settings for an import topic: where Pub/Sub ingests messages from
	// (Amazon Kinesis, Cloud Storage, Azure Event Hubs, Amazon MSK or
	// Confluent Cloud), and its platform logs.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.Topic.ingestion_data_source_settings
	IngestionDataSourceSettings *IngestionDataSourceSettings `json:"ingestionDataSourceSettings,omitempty"`

	// Transforms applied to messages published to the topic, in order.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.pubsub.v1.Topic.message_transforms
	MessageTransforms []MessageTransform `json:"messageTransforms,omitempty"`
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings
// +kubebuilder:validation:XValidation:rule="[has(self.awsKinesis), has(self.cloudStorage), has(self.azureEventHubs), has(self.awsMsk), has(self.confluentCloud)].filter(x, x).size() <= 1",message="set at most one ingestion source"
type IngestionDataSourceSettings struct {
	// Optional. Amazon Kinesis Data Streams.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.aws_kinesis
	AwsKinesis *IngestionDataSourceSettings_AwsKinesis `json:"awsKinesis,omitempty"`

	// Optional. Cloud Storage.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.cloud_storage
	CloudStorage *IngestionDataSourceSettings_CloudStorage `json:"cloudStorage,omitempty"`

	// Optional. Azure Event Hubs.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.azure_event_hubs
	AzureEventHubs *IngestionDataSourceSettings_AzureEventHubs `json:"azureEventHubs,omitempty"`

	// Optional. Amazon MSK.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.aws_msk
	AwsMsk *IngestionDataSourceSettings_AwsMsk `json:"awsMsk,omitempty"`

	// Optional. Confluent Cloud.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.confluent_cloud
	ConfluentCloud *IngestionDataSourceSettings_ConfluentCloud `json:"confluentCloud,omitempty"`

	// Optional. Platform Logs settings. If unset, no Platform Logs will be
	//  generated.
	// +optional
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.platform_logs_settings
	PlatformLogsSettings *PlatformLogsSettings `json:"platformLogsSettings,omitempty"`
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.AwsKinesis
type IngestionDataSourceSettings_AwsKinesis struct {
	// Required. The Kinesis stream ARN to ingest data from.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsKinesis.stream_arn
	StreamArn *string `json:"streamArn,omitempty"`

	// Required. The Kinesis consumer ARN to used for ingestion in Enhanced
	//  Fan-Out mode. The consumer must be already created and ready to be used.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsKinesis.consumer_arn
	ConsumerArn *string `json:"consumerArn,omitempty"`

	// Required. AWS role ARN to be used for Federated Identity authentication
	//  with Kinesis. Check the Pub/Sub docs for how to set up this role and the
	//  required permissions that need to be attached to it.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsKinesis.aws_role_arn
	AwsRoleArn *string `json:"awsRoleArn,omitempty"`

	// Required. The GCP service account to be used for Federated Identity
	//  authentication with Kinesis (via a `AssumeRoleWithWebIdentity` call for
	//  the provided role). The `aws_role_arn` must be set up with
	//  `accounts.google.com:sub` equals to this service account number.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsKinesis.gcp_service_account
	GcpServiceAccount *string `json:"gcpServiceAccount,omitempty"`
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.AwsMsk
type IngestionDataSourceSettings_AwsMsk struct {
	// Required. The Amazon Resource Name (ARN) that uniquely identifies the
	//  cluster.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsMsk.cluster_arn
	ClusterArn *string `json:"clusterArn,omitempty"`

	// Required. The name of the topic in the Amazon MSK cluster that Pub/Sub
	//  will import from.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsMsk.topic
	Topic *string `json:"topic,omitempty"`

	// Required. AWS role ARN to be used for Federated Identity authentication
	//  with Amazon MSK. Check the Pub/Sub docs for how to set up this role and
	//  the required permissions that need to be attached to it.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsMsk.aws_role_arn
	AwsRoleArn *string `json:"awsRoleArn,omitempty"`

	// Required. The GCP service account to be used for Federated Identity
	//  authentication with Amazon MSK (via a `AssumeRoleWithWebIdentity` call
	//  for the provided role). The `aws_role_arn` must be set up with
	//  `accounts.google.com:sub` equals to this service account number.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AwsMsk.gcp_service_account
	GcpServiceAccount *string `json:"gcpServiceAccount,omitempty"`
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs
type IngestionDataSourceSettings_AzureEventHubs struct {
	// Optional. Name of the resource group within the azure subscription.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs.resource_group
	ResourceGroup *string `json:"resourceGroup,omitempty"`

	// Optional. The name of the Event Hubs namespace.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs.namespace
	Namespace *string `json:"namespace,omitempty"`

	// Optional. The name of the Event Hub.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs.event_hub
	EventHub *string `json:"eventHub,omitempty"`

	// Optional. The client id of the Azure application that is being used to
	//  authenticate Pub/Sub.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs.client_id
	ClientID *string `json:"clientId,omitempty"`

	// Optional. The tenant id of the Azure application that is being used to
	//  authenticate Pub/Sub.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs.tenant_id
	TenantID *string `json:"tenantId,omitempty"`

	// Optional. The Azure subscription id.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs.subscription_id
	SubscriptionID *string `json:"subscriptionId,omitempty"`

	// Optional. The GCP service account to be used for Federated Identity
	//  authentication.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.AzureEventHubs.gcp_service_account
	GcpServiceAccount *string `json:"gcpServiceAccount,omitempty"`
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage
// +kubebuilder:validation:XValidation:rule="[has(self.textFormat), has(self.avroFormat), has(self.pubsubAvroFormat)].filter(x, x).size() <= 1",message="set at most one of textFormat, avroFormat and pubsubAvroFormat"
type IngestionDataSourceSettings_CloudStorage struct {
	// Optional. Cloud Storage bucket. The bucket name must be without any
	//  prefix like "gs://". See the [bucket naming requirements]
	//  (https://cloud.google.com/storage/docs/buckets#naming).
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.bucket
	Bucket *string `json:"bucket,omitempty"`

	// Optional. Data from Cloud Storage will be interpreted as text.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.text_format
	TextFormat *IngestionDataSourceSettings_CloudStorage_TextFormat `json:"textFormat,omitempty"`

	// Optional. Data from Cloud Storage will be interpreted in Avro format.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.avro_format
	AvroFormat *IngestionDataSourceSettings_CloudStorage_AvroFormat `json:"avroFormat,omitempty"`

	// Optional. It will be assumed data from Cloud Storage was written via
	//  [Cloud Storage
	//  subscriptions](https://cloud.google.com/pubsub/docs/cloudstorage).
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.pubsub_avro_format
	PubsubAvroFormat *IngestionDataSourceSettings_CloudStorage_PubSubAvroFormat `json:"pubsubAvroFormat,omitempty"`

	// Optional. Only objects with a larger or equal creation timestamp will be
	//  ingested (RFC 3339).
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.minimum_object_create_time
	MinimumObjectCreateTime *string `json:"minimumObjectCreateTime,omitempty"`

	// Optional. Glob pattern used to match objects that will be ingested. If
	//  unset, all objects will be ingested. See the [supported
	//  patterns](https://cloud.google.com/storage/docs/json_api/v1/objects/list#list-objects-and-prefixes-using-glob).
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.match_glob
	MatchGlob *string `json:"matchGlob,omitempty"`
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.AvroFormat
type IngestionDataSourceSettings_CloudStorage_AvroFormat struct {
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.PubSubAvroFormat
type IngestionDataSourceSettings_CloudStorage_PubSubAvroFormat struct {
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.TextFormat
type IngestionDataSourceSettings_CloudStorage_TextFormat struct {
	// Optional. When unset, '\n' is used.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.CloudStorage.TextFormat.delimiter
	Delimiter *string `json:"delimiter,omitempty"`
}

// +kcc:proto=google.pubsub.v1.IngestionDataSourceSettings.ConfluentCloud
type IngestionDataSourceSettings_ConfluentCloud struct {
	// Required. The address of the bootstrap server. The format is url:port.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.ConfluentCloud.bootstrap_server
	BootstrapServer *string `json:"bootstrapServer,omitempty"`

	// Required. The id of the cluster.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.ConfluentCloud.cluster_id
	ClusterID *string `json:"clusterId,omitempty"`

	// Required. The name of the topic in the Confluent Cloud cluster that
	//  Pub/Sub will import from.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.ConfluentCloud.topic
	Topic *string `json:"topic,omitempty"`

	// Required. The id of the identity pool to be used for Federated Identity
	//  authentication with Confluent Cloud.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.ConfluentCloud.identity_pool_id
	IdentityPoolID *string `json:"identityPoolId,omitempty"`

	// Required. The GCP service account to be used for Federated Identity
	//  authentication with `identity_pool_id`.
	// +kcc:proto:field=google.pubsub.v1.IngestionDataSourceSettings.ConfluentCloud.gcp_service_account
	GcpServiceAccount *string `json:"gcpServiceAccount,omitempty"`
}

// +kcc:proto=google.pubsub.v1.PlatformLogsSettings
type PlatformLogsSettings struct {
	// Optional. The minimum severity level of Platform Logs that will be
	// written: DISABLED, DEBUG, INFO, WARNING or ERROR.
	// +kubebuilder:validation:Enum=SEVERITY_UNSPECIFIED;DISABLED;DEBUG;INFO;WARNING;ERROR
	// +kcc:proto:field=google.pubsub.v1.PlatformLogsSettings.severity
	Severity *string `json:"severity,omitempty"`
}

// +kcc:proto=google.pubsub.v1.MessageTransform
type MessageTransform struct {
	// Optional. JavaScript User Defined Function. If multiple JavaScriptUDF's
	//  are specified on a resource, each must have a unique `function_name`.
	// +kcc:proto:field=google.pubsub.v1.MessageTransform.javascript_udf
	JavascriptUdf *JavaScriptUdf `json:"javascriptUdf,omitempty"`

	// Optional. This field is deprecated, use the `disabled` field to disable
	//  transforms.
	// +kcc:proto:field=google.pubsub.v1.MessageTransform.enabled
	Enabled *bool `json:"enabled,omitempty"`

	// Optional. If true, the transform is disabled and will not be applied to
	//  messages. Defaults to `false`.
	// +kcc:proto:field=google.pubsub.v1.MessageTransform.disabled
	Disabled *bool `json:"disabled,omitempty"`
}

// +kcc:proto=google.pubsub.v1.JavaScriptUDF
type JavaScriptUdf struct {
	// Required. Name of the JavasScript function that should applied to Pub/Sub
	//  messages.
	// +required
	// +kcc:proto:field=google.pubsub.v1.JavaScriptUDF.function_name
	FunctionName *string `json:"functionName"`

	// Required. JavaScript code that contains a function `function_name`
	// taking (message, metadata) and returning the message to publish
	// ({data, attributes}) or null to filter it out.
	// +required
	// +kcc:proto:field=google.pubsub.v1.JavaScriptUDF.code
	Code *string `json:"code"`
}

// PubSubTopicFieldsStatus defines the observed state.
type PubSubTopicFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The topic whose fields this object sets, as
	// projects/{project}/topics/{topic}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The topic's state as the API answers it: ACTIVE, or
	// INGESTION_RESOURCE_ERROR when an import topic cannot reach its source.
	State *string `json:"state,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflpubsubtopicfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// PubSubTopicFields sets fields of a Pub/Sub topic that Config Connector's
// PubSubTopic has no field for (ingestionDataSourceSettings,
// messageTransforms), and only those: it patches the topic with an update
// mask naming them. Deleting it clears them.
// +k8s:openapi-gen=true
type PubSubTopicFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   PubSubTopicFieldsSpec   `json:"spec,omitempty"`
	Status PubSubTopicFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// PubSubTopicFieldsList contains a list of PubSubTopicFields
type PubSubTopicFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PubSubTopicFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PubSubTopicFields{}, &PubSubTopicFieldsList{})
}
