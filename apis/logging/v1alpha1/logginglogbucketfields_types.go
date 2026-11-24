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
// decision 37 names it). Config Connector's LoggingLogBucket (DCL, or its
// direct controller) has no indexConfigs, restrictedFields or cmekSettings;
// this kind owns those fields of a log bucket Config Connector's kind (or
// anything else) made. The fields keep the Cloud Logging API's names
// (LogBucket in logging v2).

import (
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var LoggingLogBucketFieldsGVK = GroupVersion.WithKind("LoggingLogBucketFields")

// LoggingLogBucketFieldsSpec is the fields of a log bucket this object
// owns. A field left out is not owned: it is never read or written.
// +kubebuilder:validation:XValidation:rule="has(self.indexConfigs) || has(self.restrictedFields) || has(self.cmekSettings)",message="set at least one of indexConfigs, restrictedFields and cmekSettings"
type LoggingLogBucketFieldsSpec struct {
	// Immutable. The log bucket: Config Connector's LoggingLogBucket by
	// name (logging.cnrm.cloud.google.com), or any log bucket by external,
	// as "{{parent}}/locations/{{location}}/buckets/{{bucket}}" where the
	// parent is projects/{{projectID}}, folders/{{folderID}},
	// organizations/{{organizationID}} or billingAccounts/{{billingAccountID}}.
	// The bucket must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *loggingv1beta1.LoggingLogBucketRef `json:"resourceRef"`

	// The bucket's indexed fields, owned whole: the list replaces the
	// bucket's. An empty list removes every index.
	// +optional
	// +listType=atomic
	IndexConfigs []IndexConfig `json:"indexConfigs,omitempty"`

	// Fields of the log entries that only principals with the
	// fine-grained logging.fields.access permission may read, such as
	// "jsonPayload.user.email"; owned whole. An empty list restricts none.
	// +optional
	// +listType=atomic
	RestrictedFields []string `json:"restrictedFields,omitempty"`

	// The Cloud KMS key that encrypts the bucket's new log entries. The
	// bucket's Log Router service account must be able to use it, and the
	// key must be in the bucket's location. Once set it cannot be removed,
	// only changed to another key.
	// +optional
	CmekSettings *CmekSettings `json:"cmekSettings,omitempty"`
}

// IndexConfig is Cloud Logging's IndexConfig.
type IndexConfig struct {
	// The LogEntry field path to index, such as "jsonPayload.request.status".
	// Indexing a field whose values are not of the type makes no index.
	// +required
	FieldPath string `json:"fieldPath"`

	// The type of the index: INDEX_TYPE_STRING or INDEX_TYPE_INTEGER.
	// +required
	// +kubebuilder:validation:Enum=INDEX_TYPE_STRING;INDEX_TYPE_INTEGER
	Type string `json:"type"`
}

// CmekSettings is the part of Cloud Logging's CmekSettings a bucket takes.
type CmekSettings struct {
	// The key, as
	// "projects/{{projectID}}/locations/{{location}}/keyRings/{{keyRing}}/cryptoKeys/{{cryptoKey}}".
	// +required
	KmsKeyName string `json:"kmsKeyName"`
}

// LoggingLogBucketFieldsStatus defines the observed state.
type LoggingLogBucketFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The log bucket whose fields this object sets, as
	// {parent}/locations/{location}/buckets/{bucket}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The service account the Log Router uses with the bucket's CMEK key,
	// as the API reports it once cmekSettings is set.
	CmekServiceAccountID *string `json:"cmekServiceAccountID,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfllogginglogbucketfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// LoggingLogBucketFields sets fields of a Cloud Logging bucket that Config
// Connector's LoggingLogBucket has no field for (indexConfigs,
// restrictedFields, cmekSettings), and only those: it patches the bucket
// with an update mask naming them. Deleting it clears the index configs
// and restricted fields it set; the CMEK key stays (the API cannot remove
// it). With deletion-policy abandon everything stays.
// +k8s:openapi-gen=true
type LoggingLogBucketFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   LoggingLogBucketFieldsSpec   `json:"spec,omitempty"`
	Status LoggingLogBucketFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// LoggingLogBucketFieldsList contains a list of LoggingLogBucketFields
type LoggingLogBucketFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoggingLogBucketFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoggingLogBucketFields{}, &LoggingLogBucketFieldsList{})
}
