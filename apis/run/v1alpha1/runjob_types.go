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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// apis/run/v1beta1/runjob_types.go (UPSTREAM.md), the direct RunJob, as a
// Cloud Run job (google.cloud.run.v2.Job) that owns the whole job
// (docs/design.md section 6.7, application item 6):
//   - group run.gcp.hfl.dev at v1alpha1, own short names, no
//     cnrm.cloud.google.com labels, the gap label and upstream-kind
//     annotation; projectRef and location required, and they and resourceID
//     immutable by CEL (decision 20);
//   - the template's containers, volumes and VPC access are our RunService's
//     types (apis/run/v1beta1, themselves copied from upstream's worker
//     pool), which have the gRPC probe upstream's job types lack; a CEL rule
//     refuses the container fields the API takes only for a service
//     (baseImageURI, resources.cpuIdle, resources.startupCPUBoost);
//   - uncommented from upstream's unreachable fields: the execution
//     template's labels, the task template's nodeSelector and
//     gpuZonalRedundancyDisabled (GPUs), and binaryAuthorization.policy,
//     with a CEL rule that it and useDefault are not both set;
//   - the job's labels are metadata.labels, as upstream's direct RunJob
//     takes them;
//   - status as our RunService's: externalRef, conditions and an
//     observedState holding the job's output fields (one
//     latestCreatedExecution and one terminalCondition, as the API has
//     them, where upstream keeps Terraform's lists inline).

package v1alpha1

import (
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	k8sv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1beta1"
)

var RunJobGVK = GroupVersion.WithKind("RunJob")

// RunJobSpec defines the desired state of RunJob
// +kcc:spec:proto=google.cloud.run.v2.Job
type RunJobSpec struct {
	// The project that this resource belongs to.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refs.ProjectRef `json:"projectRef"`

	// The location of the cloud run job
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location *string `json:"location"`

	// The RunJob name. If not given, the metadata.name will be used.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// Optional. User-provided annotations, which are stored in GCP.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Optional. Settings for Binary Authorization feature.
	BinaryAuthorization *BinaryAuthorization `json:"binaryAuthorization,omitempty"`

	// Optional. Arbitrary identifier for the API client.
	Client *string `json:"client,omitempty"`

	// Optional. Arbitrary version identifier for the API client.
	ClientVersion *string `json:"clientVersion,omitempty"`

	// Optional. The launch stage of the job.
	// Possible values are `LAUNCH_STAGE_UNSPECIFIED`, `UNIMPLEMENTED`, `PRELAUNCH`, `EARLY_ACCESS`, `ALPHA`, `BETA`, `GA`, `DEPRECATED`.
	LaunchStage *string `json:"launchStage,omitempty"`

	// Required. The template used to create executions for this Job.
	Template *ExecutionTemplate `json:"template"`
}

// +kcc:proto=google.cloud.run.v2.BinaryAuthorization
// +kubebuilder:validation:XValidation:rule="!(has(self.useDefault) && has(self.policy))",message="binaryAuthorization takes one of useDefault and policy"
type BinaryAuthorization struct {
	// Optional. If True, indicates to use the default project's binary
	//  authorization policy. If False, binary authorization will be disabled.
	// +kcc:proto:field=google.cloud.run.v2.BinaryAuthorization.use_default
	UseDefault *bool `json:"useDefault,omitempty"`

	// Optional. The path to a binary authorization policy.
	//  Format: `projects/{project}/platforms/cloudRun/{policy-name}`
	// +kcc:proto:field=google.cloud.run.v2.BinaryAuthorization.policy
	Policy *string `json:"policy,omitempty"`

	// Optional. If present, indicates to use Breakglass using this justification.
	//  If use_default is False, then it must be empty.
	//  For more information on breakglass, see
	//  https://cloud.google.com/binary-authorization/docs/using-breakglass
	// +kcc:proto:field=google.cloud.run.v2.BinaryAuthorization.breakglass_justification
	BreakglassJustification *string `json:"breakglassJustification,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.ExecutionTemplate
type ExecutionTemplate struct {
	// Unstructured key value map that can be used to organize and categorize
	//  objects.
	//  User-provided labels are shared with Google's billing system, so they can
	//  be used to filter, or break down billing charges by team, component,
	//  environment, state, etc. For more information, visit
	//  https://cloud.google.com/resource-manager/docs/creating-managing-labels or
	//  https://cloud.google.com/run/docs/configuring/labels.
	//
	//  <p>Cloud Run API v2 does not support labels with `run.googleapis.com`,
	//  `cloud.googleapis.com`, `serving.knative.dev`, or `autoscaling.knative.dev`
	//  namespaces, and they will be rejected. All system labels in v1 now have a
	//  corresponding field in v2 ExecutionTemplate.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionTemplate.labels
	Labels map[string]string `json:"labels,omitempty"`

	// Unstructured key value map that may be set by external tools to store and
	//  arbitrary metadata. They are not queryable and should be preserved
	//  when modifying objects.
	//
	//  <p>Cloud Run API v2 does not support annotations with `run.googleapis.com`,
	//  `cloud.googleapis.com`, `serving.knative.dev`, or `autoscaling.knative.dev`
	//  namespaces, and they will be rejected. All system annotations in v1 now
	//  have a corresponding field in v2 ExecutionTemplate.
	//
	//  <p>This field follows Kubernetes annotations' namespacing, limits, and
	//  rules.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionTemplate.annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// Optional. Specifies the maximum desired number of tasks the execution
	//  should run at given time. When the job is run, if this field is 0 or unset,
	//  the maximum possible value will be used for that execution. The actual
	//  number of tasks running in steady state will be less than this number when
	//  there are fewer tasks waiting to be completed remaining, i.e. when the work
	//  left to do is less than max parallelism.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionTemplate.parallelism
	Parallelism *int32 `json:"parallelism,omitempty"`

	// Specifies the desired number of tasks the execution should run.
	//  Setting to 1 means that parallelism is limited to 1 and the success of
	//  that task signals the success of the execution. Defaults to 1.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionTemplate.task_count
	TaskCount *int32 `json:"taskCount,omitempty"`

	// Required. Describes the task(s) that will be created when executing an
	//  execution.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionTemplate.template
	Template *TaskTemplate `json:"template,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.TaskTemplate
type TaskTemplate struct {
	// Holds the single container that defines the unit of execution for this
	//  task.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.containers
	// +kubebuilder:validation:MaxItems=10
	// +kubebuilder:validation:XValidation:rule="self.all(c, !has(c.baseImageURI) && (!has(c.resources) || (!has(c.resources.cpuIdle) && !has(c.resources.startupCPUBoost))))",message="baseImageURI, resources.cpuIdle and resources.startupCPUBoost are a service's; a job takes none of them"
	Containers []runv1beta1.Container `json:"containers,omitempty"`

	// Optional. A list of Volumes to make available to containers.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.volumes
	Volumes []runv1beta1.Volume `json:"volumes,omitempty"`

	// Number of retries allowed per Task, before marking this Task failed.
	//  Defaults to 3.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.max_retries
	MaxRetries *int32 `json:"maxRetries,omitempty"`

	// Optional. Max allowed time duration the Task may be active before the
	//  system will actively try to mark it failed and kill associated containers.
	//  This applies per attempt of a task, meaning each retry can run for the full
	//  timeout. Defaults to 600 seconds.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.timeout
	Timeout *string `json:"timeout,omitempty"`

	// Optional. Email address of the IAM service account associated with the Task
	//  of a Job. The service account represents the identity of the running task,
	//  and determines what permissions the task has. If not provided, the task
	//  will use the project's default service account.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.service_account
	ServiceAccountRef *refs.IAMServiceAccountRef `json:"serviceAccountRef,omitempty"`

	// Optional. The execution environment being used to host this Task.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.execution_environment
	ExecutionEnvironment *string `json:"executionEnvironment,omitempty"`

	// A reference to a customer managed encryption key (CMEK) to use to encrypt
	//  this container image. For more information, go to
	//  https://cloud.google.com/run/docs/securing/using-cmek
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.encryption_key
	EncryptionKeyRef *kmsv1beta1.KMSCryptoKeyRef `json:"encryptionKeyRef,omitempty"`

	// Optional. VPC Access configuration to use for this Task. For more
	//  information, visit
	//  https://cloud.google.com/run/docs/configuring/connecting-vpc.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.vpc_access
	VPCAccess *runv1beta1.VPCAccess `json:"vpcAccess,omitempty"`

	// Optional. The node selector for the task template.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.node_selector
	NodeSelector *runv1beta1.NodeSelector `json:"nodeSelector,omitempty"`

	// Optional. True if GPU zonal redundancy is disabled on this task template.
	// +kcc:proto:field=google.cloud.run.v2.TaskTemplate.gpu_zonal_redundancy_disabled
	GpuZonalRedundancyDisabled *bool `json:"gpuZonalRedundancyDisabled,omitempty"`
}

// RunJobStatus defines the config connector machine state of RunJob
type RunJobStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []k8sv1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the RunJob resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *RunJobObservedState `json:"observedState,omitempty"`
}

// RunJobObservedState is the state of the RunJob resource as most recently observed in GCP.
// +kcc:observedstate:proto=google.cloud.run.v2.Job
type RunJobObservedState struct {
	// Output only. Server assigned unique identifier for the Execution. The value
	// is a UUID4 string and guaranteed to remain unchanged until the resource is
	// deleted.
	Uid *string `json:"uid,omitempty"`

	// Output only. A number that monotonically increases every time the user
	//  modifies the desired state.
	Generation *int64 `json:"generation,omitempty"`

	// Output only. The creation time.
	CreateTime *string `json:"createTime,omitempty"`

	// Output only. The last-modified time.
	UpdateTime *string `json:"updateTime,omitempty"`

	// Output only. The deletion time. It is only populated as a response to a
	// Delete request.
	DeleteTime *string `json:"deleteTime,omitempty"`

	// Output only. For a deleted resource, the time after which it will be
	// permanently deleted.
	ExpireTime *string `json:"expireTime,omitempty"`

	// Output only. Email address of the authenticated creator.
	Creator *string `json:"creator,omitempty"`

	// Output only. Email address of the last authenticated modifier.
	LastModifier *string `json:"lastModifier,omitempty"`

	// Output only. The generation of this Job. See comments in `reconciling`
	//  for additional information on reconciliation process in Cloud Run.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// Output only. The Condition of this Job, containing its readiness status,
	// and detailed error information in case it did not reach the desired state.
	TerminalCondition *runv1beta1.RunCondition `json:"terminalCondition,omitempty"`

	// Output only. The Conditions of all other associated sub-resources. They
	//  contain additional diagnostics information in case the Job does not
	//  reach its desired state.
	ObservedConditions []runv1beta1.RunCondition `json:"observedConditions,omitempty"`

	// Output only. Number of executions created for this job.
	ExecutionCount *int32 `json:"executionCount,omitempty"`

	// Output only. Name of the last created execution.
	LatestCreatedExecution *ExecutionReference `json:"latestCreatedExecution,omitempty"`

	// Output only. Returns true if the Job is currently being acted upon by the
	// system to bring it into the desired state.
	Reconciling *bool `json:"reconciling,omitempty"`

	// Output only. A system-generated fingerprint for this version of the
	// resource. May be used to detect modification conflict during updates.
	Etag *string `json:"etag,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.ExecutionReference
type ExecutionReference struct {
	// Name of the execution.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionReference.name
	Name *string `json:"name,omitempty"`

	// Creation timestamp of the execution.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionReference.create_time
	CreateTime *string `json:"createTime,omitempty"`

	// Creation timestamp of the execution.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionReference.completion_time
	CompletionTime *string `json:"completionTime,omitempty"`

	// The deletion time of the execution. It is only
	//  populated as a response to a Delete request.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionReference.delete_time
	DeleteTime *string `json:"deleteTime,omitempty"`

	// Status for the execution completion.
	// +kcc:proto:field=google.cloud.run.v2.ExecutionReference.completion_status
	CompletionStatus *string `json:"completionStatus,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflrunjob;hflrunjobs
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:metadata:annotations="gcp.hfl.dev/upstream-kind=run.cnrm.cloud.google.com/RunJob"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// RunJob is the Schema for the RunJob API
// +k8s:openapi-gen=true
type RunJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   RunJobSpec   `json:"spec,omitempty"`
	Status RunJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// RunJobList contains a list of RunJob
type RunJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RunJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RunJob{}, &RunJobList{})
}
