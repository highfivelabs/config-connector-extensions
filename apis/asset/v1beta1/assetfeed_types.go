// Copyright 2025 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/asset/v1beta1/assetfeed_types.go, with FeedOutputConfig and Expr
// from apis/asset/v1beta1/types.generated.go (UPSTREAM.md). Changed:
// folderRef is refs.FolderRefDeprecated, the type refs.FolderRef aliases
// (controller-gen does not take aliases); exactly one parent and the
// parent and resourceID immutable, by CEL (docs/design.md decision 20;
// upstream refuses a change in the identity); group, labels and short
// names are ours.

package v1beta1

import (
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	refv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var AssetFeedGVK = GroupVersion.WithKind("AssetFeed")

// Parent defines the parent field for AssetFeed
type AssetFeedParent struct {
	// The project the feed belongs to. Exactly one of projectRef,
	// folderRef and organizationRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refv1beta1.ProjectRef `json:"projectRef,omitempty"`
	// The organization the feed belongs to. Exactly one of projectRef,
	// folderRef and organizationRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="organizationRef is immutable"
	OrganizationRef *refv1beta1.OrganizationRef `json:"organizationRef,omitempty"`
	// The folder the feed belongs to. Exactly one of projectRef, folderRef
	// and organizationRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="folderRef is immutable"
	FolderRef *refv1beta1.FolderRefDeprecated `json:"folderRef,omitempty"`
}

// +kcc:proto=google.cloud.asset.v1.PubsubDestination
type PubsubDestination struct {
	// The name of the Pub/Sub topic to publish to.
	//  Example: `projects/PROJECT_ID/topics/TOPIC_ID`.
	// +kcc:proto:field=google.cloud.asset.v1.PubsubDestination.topic
	TopicRef *pubsubv1beta1.PubSubTopicRef `json:"topicRef,omitempty"`
}

// +kcc:proto=google.cloud.asset.v1.FeedOutputConfig
type FeedOutputConfig struct {
	// Destination on Pub/Sub.
	// +kcc:proto:field=google.cloud.asset.v1.FeedOutputConfig.pubsub_destination
	PubsubDestination *PubsubDestination `json:"pubsubDestination,omitempty"`
}

// +kcc:proto=google.type.Expr
type Expr struct {
	// Textual representation of an expression in Common Expression Language
	//  syntax.
	// +kcc:proto:field=google.type.Expr.expression
	Expression *string `json:"expression,omitempty"`

	// Optional. Title for the expression, i.e. a short string describing
	//  its purpose. This can be used e.g. in UIs which allow to enter the
	//  expression.
	// +kcc:proto:field=google.type.Expr.title
	Title *string `json:"title,omitempty"`

	// Optional. Description of the expression. This is a longer text which
	//  describes the expression, e.g. when hovered over it in a UI.
	// +kcc:proto:field=google.type.Expr.description
	Description *string `json:"description,omitempty"`

	// Optional. String indicating the location of the expression for error
	//  reporting, e.g. a file name and a position in the file.
	// +kcc:proto:field=google.type.Expr.location
	Location *string `json:"location,omitempty"`
}

// AssetFeedSpec defines the desired state of AssetFeed
// +kcc:spec:proto=google.cloud.asset.v1.Feed
// +kubebuilder:validation:XValidation:rule="(has(self.organizationRef) ? 1 : 0) + (has(self.folderRef) ? 1 : 0) + (has(self.projectRef) ? 1 : 0) == 1",message="exactly one of organizationRef, folderRef and projectRef is required"
type AssetFeedSpec struct {
	Parent AssetFeedParent `json:",inline"`

	// The AssetFeed name. If not given, the metadata.name will be used.
	// Immutable.
	// +kcc:proto:field=google.cloud.asset.v1.Feed.name
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// A list of the full names of the assets to receive updates. You must specify
	//  either or both of asset_names and asset_types. Only asset updates matching
	//  specified asset_names or asset_types are exported to the feed.
	//  Example:
	//  `//compute.googleapis.com/projects/my_project_123/zones/zone1/instances/instance1`.
	//  For a list of the full names for supported asset types, see [Resource
	//  name format](/asset-inventory/docs/resource-name-format).
	// +kcc:proto:field=google.cloud.asset.v1.Feed.asset_names
	AssetNames []string `json:"assetNames,omitempty"`

	// A list of types of the assets to receive updates. You must specify either
	//  or both of asset_names and asset_types. Only asset updates matching
	//  specified asset_names or asset_types are exported to the feed.
	//  Example: `"compute.googleapis.com/Disk"`
	//
	//  For a list of all supported asset types, see
	//  [Supported asset types](/asset-inventory/docs/supported-asset-types).
	// +kcc:proto:field=google.cloud.asset.v1.Feed.asset_types
	AssetTypes []string `json:"assetTypes,omitempty"`

	// Asset content type. If not specified, no content but the asset name and
	//  type will be returned.
	// +kcc:proto:field=google.cloud.asset.v1.Feed.content_type
	ContentType *string `json:"contentType,omitempty"`

	// Required. Feed output configuration defining where the asset updates are
	//  published to.
	// +required
	// +kcc:proto:field=google.cloud.asset.v1.Feed.feed_output_config
	FeedOutputConfig *FeedOutputConfig `json:"feedOutputConfig"`

	// A condition which determines whether an asset update should be published.
	//  If specified, an asset will be returned only when the expression evaluates
	//  to true.
	//  When set, `expression` field in the `Expr` must be a valid [CEL expression]
	//  (https://github.com/google/cel-spec) on a TemporalAsset with name
	//  `temporal_asset`. Example: a Feed with expression ("temporal_asset.deleted
	//  == true") will only publish Asset deletions. Other fields of `Expr` are
	//  optional.
	//
	//  See our [user
	//  guide](https://cloud.google.com/asset-inventory/docs/monitoring-asset-changes-with-condition)
	//  for detailed instructions.
	// +kcc:proto:field=google.cloud.asset.v1.Feed.condition
	Condition *Expr `json:"condition,omitempty"`

	// A list of relationship types to output, for example:
	//  `INSTANCE_TO_INSTANCEGROUP`. This field should only be specified if
	//  content_type=RELATIONSHIP.
	//  * If specified:
	//  it outputs specified relationship updates on the [asset_names] or the
	//  [asset_types]. It returns an error if any of the [relationship_types]
	//  doesn't belong to the supported relationship types of the [asset_names] or
	//  [asset_types], or any of the [asset_names] or the [asset_types] doesn't
	//  belong to the source types of the [relationship_types].
	//  * Otherwise:
	//  it outputs the supported relationships of the types of [asset_names] and
	//  [asset_types] or returns an error if any of the [asset_names] or the
	//  [asset_types] has no replationship support.
	//  See [Introduction to Cloud Asset
	//  Inventory](https://cloud.google.com/asset-inventory/docs/overview)
	//  for all supported asset types and relationship types.
	// +kcc:proto:field=google.cloud.asset.v1.Feed.relationship_types
	RelationshipTypes []string `json:"relationshipTypes,omitempty"`
}

// AssetFeedStatus defines the config connector machine state of AssetFeed
type AssetFeedStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the AssetFeed resource in GCP: the feed's
	// name as Google Cloud gives it, a project named by its number.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflassetfeed;hflassetfeeds
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// AssetFeed is the Schema for the AssetFeed API
// +k8s:openapi-gen=true
type AssetFeed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   AssetFeedSpec   `json:"spec,omitempty"`
	Status AssetFeedStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// AssetFeedList contains a list of AssetFeed
type AssetFeedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AssetFeed `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AssetFeed{}, &AssetFeedList{})
}
