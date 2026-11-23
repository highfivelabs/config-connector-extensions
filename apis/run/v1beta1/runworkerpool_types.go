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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// apis/run/v1alpha1/runworkerpool_types.go and, for InstanceSplit,
// InstanceSplitStatus and WorkerPoolScaling, apis/run/v1alpha1/types.generated.go
// (UPSTREAM.md): Config Connector's RunWorkerPool is alpha only, so ours is
// its stable equivalent (docs/design.md decision 26, application item 6):
//   - group run.gcp.hfl.dev at v1beta1, own short names, no
//     cnrm.cloud.google.com labels, the gap label and upstream-kind
//     annotation; projectRef, location and resourceID immutable by CEL
//     (decision 20);
//   - the template's containers, volumes, VPC access and service mesh, the
//     Binary Authorization settings and the conditions are this package's
//     (RunService's) types, which are upstream's worker pool types;
//   - written here, from the Cloud Run v2 API: the template's labels, and
//     (in the shared BinaryAuthorization) binaryAuthorization.policy.

package v1beta1

import (
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	k8sv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var RunWorkerPoolGVK = GroupVersion.WithKind("RunWorkerPool")

// RunWorkerPoolSpec defines the desired state of RunWorkerPool
// +kcc:spec:proto=google.cloud.run.v2.WorkerPool
type RunWorkerPoolSpec struct {
	// The project that this resource belongs to.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refs.ProjectRef `json:"projectRef"`

	// The location of this resource.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location *string `json:"location"`

	// The RunWorkerPool name. If not given, the metadata.name will be used.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// Optional. Unstructured key value map that can be used to organize and
	//  categorize objects. User-provided labels are shared with Google's billing
	//  system, so they can be used to filter, or break down billing charges by
	//  team, component, environment, state, etc. For more information, visit
	//  https://cloud.google.com/resource-manager/docs/creating-managing-labels or
	//  https://cloud.google.com/run/docs/configuring/labels.
	Labels map[string]string `json:"labels,omitempty"`

	// User-provided description of the WorkerPool. This field currently has a
	//  512-character limit.
	Description *string `json:"description,omitempty"`

	// Optional. Unstructured key value map that may be set by external tools to
	//  store and arbitrary metadata. They are not queryable and should be
	//  preserved when modifying objects.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Arbitrary identifier for the API client.
	Client *string `json:"client,omitempty"`

	// Arbitrary version identifier for the API client.
	ClientVersion *string `json:"clientVersion,omitempty"`

	// Optional. The launch stage as defined by [Google Cloud Platform
	//   Launch Stages](https://cloud.google.com/terms/launch-stages).
	LaunchStage *string `json:"launchStage,omitempty"`

	// Optional. Settings for the Binary Authorization feature.
	BinaryAuthorization *BinaryAuthorization `json:"binaryAuthorization,omitempty"`

	// Required. The template used to create revisions for this WorkerPool.
	Template *WorkerPoolRevisionTemplate `json:"template,omitempty"`

	// Optional. Specifies how to distribute instances over a collection of
	//  Revisions belonging to the WorkerPool. If instance split is empty or not
	//  provided, defaults to 100% instances assigned to the latest `Ready`
	//  Revision.
	InstanceSplits []InstanceSplit `json:"instanceSplits,omitempty"`

	// Optional. Specifies worker-pool-level scaling settings
	Scaling *WorkerPoolScaling `json:"scaling,omitempty"`

	// Optional. One or more custom audiences that you want this worker pool to support.
	//  Specify each custom audience as the full URL in a string.
	CustomAudiences []string `json:"customAudiences,omitempty"`
}

// RunWorkerPoolStatus defines the config connector machine state of RunWorkerPool
type RunWorkerPoolStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []k8sv1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the RunWorkerPool resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *RunWorkerPoolObservedState `json:"observedState,omitempty"`
}

// RunWorkerPoolObservedState is the state of the RunWorkerPool resource as most recently observed in GCP.
// +kcc:observedstate:proto=google.cloud.run.v2.WorkerPool
type RunWorkerPoolObservedState struct {
	// Output only. Server assigned unique identifier for the trigger. The value
	//  is a UUID4 string and guaranteed to remain unchanged until the resource is
	//  deleted.
	Uid *string `json:"uid,omitempty"`

	// Output only. A number that monotonically increases every time the user
	//  modifies the desired state.
	Generation *int64 `json:"generation,omitempty"`

	// Output only. The creation time.
	CreateTime *string `json:"createTime,omitempty"`

	// Output only. The last-modified time.
	UpdateTime *string `json:"updateTime,omitempty"`

	// Output only. The deletion time. It is only populated as a response to a
	//  Delete request.
	DeleteTime *string `json:"deleteTime,omitempty"`

	// Output only. For a deleted resource, the time after which it will be
	//  permanently deleted.
	ExpireTime *string `json:"expireTime,omitempty"`

	// Output only. Email address of the authenticated creator.
	Creator *string `json:"creator,omitempty"`

	// Output only. Email address of the last authenticated modifier.
	LastModifier *string `json:"lastModifier,omitempty"`

	// Required. The template used to create revisions for this WorkerPool.
	Template *RevisionTemplateObservedState `json:"template,omitempty"`

	// Output only. The generation of this WorkerPool currently serving traffic.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// Output only. The Condition of this WorkerPool, containing its readiness
	//  status, and detailed error information in case it did not reach a serving
	//  state.
	TerminalCondition *RunCondition `json:"terminalCondition,omitempty"`

	// Output only. The Conditions of all other associated sub-resources. They
	//  contain additional diagnostics information in case the WorkerPool does not
	//  reach its Serving state.
	ObservedConditions []RunCondition `json:"observedConditions,omitempty"`

	// Output only. Name of the latest revision that is serving traffic.
	LatestReadyRevision *string `json:"latestReadyRevision,omitempty"`

	// Output only. Name of the last created revision.
	LatestCreatedRevision *string `json:"latestCreatedRevision,omitempty"`

	// Output only. Detailed status information for corresponding instance splits.
	InstanceSplitStatuses []InstanceSplitStatus `json:"instanceSplitStatuses,omitempty"`

	// Output only. One or more custom audiences that you want this worker pool to support.
	CustomAudiences []string `json:"customAudiences,omitempty"`

	// Output only. Reserved for future use.
	SatisfiesPzs *bool `json:"satisfiesPzs,omitempty"`

	// Output only. Returns true if the WorkerPool is currently being acted upon
	//  by the system to bring it into the desired state.
	Reconciling *bool `json:"reconciling,omitempty"`

	// Output only. A system-generated fingerprint for this version of the
	//  resource. May be used to detect modification conflict during updates.
	Etag *string `json:"etag,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.WorkerPoolRevisionTemplate
type WorkerPoolRevisionTemplate struct {
	// Optional. The unique name for the revision. If this field is omitted, it
	//  will be automatically generated based on the WorkerPool name.
	Revision *string `json:"revision,omitempty"`

	// Written here: Optional. Unstructured key value map that can be used to
	//  organize and categorize objects. User-provided labels are shared with
	//  Google's billing system, so they can be used to filter, or break down
	//  billing charges by team, component, environment, state, etc. Cloud Run
	//  API v2 does not support labels with `run.googleapis.com`,
	//  `cloud.googleapis.com`, `serving.knative.dev`, or
	//  `autoscaling.knative.dev` namespaces, and they will be rejected.
	Labels map[string]string `json:"labels,omitempty"`

	// Optional. Unstructured key value map that may be set by external tools to
	//  store and arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Optional. VPC Access configuration to use for this Revision.
	VPCAccess *VPCAccess `json:"vpcAccess,omitempty"`

	// Optional. Email address of the IAM service account associated with the
	//  revision of the service.
	ServiceAccountRef *refs.IAMServiceAccountRef `json:"serviceAccountRef,omitempty"`

	// Holds list of the containers that defines the unit of execution for this
	//  Revision.
	Containers []Container `json:"containers,omitempty"`

	// Optional. A list of Volumes to make available to containers.
	Volumes []Volume `json:"volumes,omitempty"`

	// A reference to a customer managed encryption key (CMEK) to use to encrypt
	//  this container image.
	EncryptionKeyRef *kmsv1beta1.KMSCryptoKeyRef `json:"encryptionKeyRef,omitempty"`

	// Optional. Enables service mesh connectivity.
	ServiceMesh *ServiceMesh `json:"serviceMesh,omitempty"`

	// Optional. The action to take if the encryption key is revoked.
	EncryptionKeyRevocationAction *string `json:"encryptionKeyRevocationAction,omitempty"`

	// Optional. If encryption_key_revocation_action is SHUTDOWN, the duration
	//  before shutting down all instances.
	EncryptionKeyShutdownDuration *string `json:"encryptionKeyShutdownDuration,omitempty"`

	// Optional. The node selector for the revision template.
	NodeSelector *NodeSelector `json:"nodeSelector,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflrunworkerpool;hflrunworkerpools
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:metadata:annotations="gcp.hfl.dev/upstream-kind=run.cnrm.cloud.google.com/RunWorkerPool"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// RunWorkerPool is the Schema for the RunWorkerPool API
// +k8s:openapi-gen=true
type RunWorkerPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   RunWorkerPoolSpec   `json:"spec,omitempty"`
	Status RunWorkerPoolStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// RunWorkerPoolList contains a list of RunWorkerPool
type RunWorkerPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RunWorkerPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RunWorkerPool{}, &RunWorkerPoolList{})
}

// +kcc:proto=google.cloud.run.v2.VpcAccess

// +kcc:proto=google.cloud.run.v2.InstanceSplit
type InstanceSplit struct {
	// The allocation type for this instance split.
	// +kcc:proto:field=google.cloud.run.v2.InstanceSplit.type
	Type *string `json:"type,omitempty"`

	// Revision to which to assign this portion of instances, if split allocation
	//  is by revision.
	// +kcc:proto:field=google.cloud.run.v2.InstanceSplit.revision
	Revision *string `json:"revision,omitempty"`

	// Specifies percent of the instance split to this Revision.
	//  This defaults to zero if unspecified.
	// +kcc:proto:field=google.cloud.run.v2.InstanceSplit.percent
	Percent *int32 `json:"percent,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.InstanceSplitStatus
type InstanceSplitStatus struct {
	// The allocation type for this instance split.
	// +kcc:proto:field=google.cloud.run.v2.InstanceSplitStatus.type
	Type *string `json:"type,omitempty"`

	// Revision to which this instance split is assigned.
	// +kcc:proto:field=google.cloud.run.v2.InstanceSplitStatus.revision
	Revision *string `json:"revision,omitempty"`

	// Specifies percent of the instance split to this Revision.
	// +kcc:proto:field=google.cloud.run.v2.InstanceSplitStatus.percent
	Percent *int32 `json:"percent,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.WorkerPoolScaling
type WorkerPoolScaling struct {
	// Optional. The total number of instances in manual scaling mode.
	// +kcc:proto:field=google.cloud.run.v2.WorkerPoolScaling.manual_instance_count
	ManualInstanceCount *int32 `json:"manualInstanceCount,omitempty"`
}
