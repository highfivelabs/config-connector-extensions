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

// Written here: a Cloud Storage object is in no Config Connector code or
// pull request. The shape follows the JSON API's Object and Terraform's
// google_storage_bucket_object (content inline), with the content also from
// a Kubernetes Secret or ConfigMap key, in the layout of Config Connector's
// direct kinds.

import (
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var StorageBucketObjectGVK = GroupVersion.WithKind("StorageBucketObject")

// StorageBucketObjectSpec is a small object in a bucket: a configuration
// file, a script, a seed document. Content is limited by the Kubernetes
// object's size (about 1 MiB); larger data belongs elsewhere.
type StorageBucketObjectSpec struct {
	// The bucket the object is in. Immutable.
	// +required
	BucketRef *storagev1beta1.StorageBucketRef `json:"bucketRef"`

	// The object's name, which may contain "/" (for example
	//  "config/app.yaml"). If not given, the metadata.name will be used.
	//  Immutable.
	ResourceID *string `json:"resourceID,omitempty"`

	// The object's content, as text. One of content and contentFrom.
	Content *string `json:"content,omitempty"`

	// The object's content, from a key of a Secret or ConfigMap in the
	//  object's namespace. One of content and contentFrom.
	ContentFrom *ContentSource `json:"contentFrom,omitempty"`

	// The Content-Type of the object's data.
	ContentType *string `json:"contentType,omitempty"`

	// The Cache-Control of the object's data.
	CacheControl *string `json:"cacheControl,omitempty"`

	// The Content-Disposition of the object's data.
	ContentDisposition *string `json:"contentDisposition,omitempty"`

	// The Content-Encoding of the object's data.
	ContentEncoding *string `json:"contentEncoding,omitempty"`

	// The Content-Language of the object's data.
	ContentLanguage *string `json:"contentLanguage,omitempty"`

	// User-provided metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ContentSource names one key of a Secret or ConfigMap.
type ContentSource struct {
	// A key of a Secret.
	SecretKeyRef *KeyRef `json:"secretKeyRef,omitempty"`

	// A key of a ConfigMap.
	ConfigMapKeyRef *KeyRef `json:"configMapKeyRef,omitempty"`
}

type KeyRef struct {
	// The Secret's or ConfigMap's name.
	Name string `json:"name"`

	// The key.
	Key string `json:"key"`
}

// StorageBucketObjectStatus defines the observed state.
type StorageBucketObjectStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The object, as projects/_/buckets/{bucket}/objects/{name}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the object as most recently observed in GCP.
	ObservedState *StorageBucketObjectObservedState `json:"observedState,omitempty"`
}

type StorageBucketObjectObservedState struct {
	// The content generation of the object; a new upload makes a new one.
	Generation *int64 `json:"generation,omitempty"`

	// The MD5 hash of the data, base64.
	MD5Hash *string `json:"md5Hash,omitempty"`

	// The CRC32C checksum of the data, base64.
	CRC32C *string `json:"crc32c,omitempty"`

	// The size of the data in bytes.
	Size *int64 `json:"size,omitempty"`

	// A URL to download the object's data.
	MediaLink *string `json:"mediaLink,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflstoragebucketobject;hflstoragebucketobjects
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// StorageBucketObject is an object in a Cloud Storage bucket, with its
// content from the spec, a Secret or a ConfigMap.
// +k8s:openapi-gen=true
type StorageBucketObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   StorageBucketObjectSpec   `json:"spec,omitempty"`
	Status StorageBucketObjectStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// StorageBucketObjectList contains a list of StorageBucketObject
type StorageBucketObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageBucketObject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StorageBucketObject{}, &StorageBucketObjectList{})
}
