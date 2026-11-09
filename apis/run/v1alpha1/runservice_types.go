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
// apis/run/v1alpha1/runworkerpool_types.go (UPSTREAM.md), the direct RunService, as
// a narrow direct Cloud Run service that owns the whole service:
//   - RunService becomes RunService (google.cloud.run.v2.Service), its
//     template RevisionTemplate becomes RevisionTemplate
//     (google.cloud.run.v2.RevisionTemplate); group run.gcp.hfl.dev, own
//     short names, no cnrm.cloud.google.com labels, the gap label and
//     upstream-kind annotation;
//   - instance splits (worker pools only) are left out; traffic is not
//     modelled, so a service sends all traffic to its latest revision;
//   - written here, from the Cloud Run v2 API: the service's ingress,
//     invokerIamDisabled, defaultUriDisabled and scaling (ServiceScaling),
//     and the template's scaling, timeout, maxInstanceRequestConcurrency,
//     executionEnvironment, sessionAffinity, healthCheckDisabled and
//     gpuZonalRedundancyDisabled (with nodeSelector, already in the
//     template, for GPUs); and in status, the service's URIs.
//   - secret references are typed SecretManagerSecretRef and
//     SecretManagerSecretVersionRef, not their aliases SecretRef and
//     SecretVersionRef (controller-gen v0.16.5 panics on aliases under Go
//     1.27; same schema).

package v1alpha1

import (
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	networkservicesv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/networkservices/v1alpha1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	secretmanagerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/secretmanager/v1beta1"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	vpcaccessv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/vpcaccess/v1beta1"
	k8sv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var RunServiceGVK = GroupVersion.WithKind("RunService")

// RunServiceSpec defines the desired state of RunService
// +kcc:spec:proto=google.cloud.run.v2.Service
type RunServiceSpec struct {
	// The project that this resource belongs to.
	ProjectRef *refs.ProjectRef `json:"projectRef"`

	// The location of this resource.
	Location *string `json:"location"`

	// The RunService name. If not given, the metadata.name will be used.
	ResourceID *string `json:"resourceID,omitempty"`

	// Optional. Unstructured key value map that can be used to organize and
	//  categorize objects. User-provided labels are shared with Google's billing
	//  system, so they can be used to filter, or break down billing charges by
	//  team, component, environment, state, etc. For more information, visit
	//  https://cloud.google.com/resource-manager/docs/creating-managing-labels or
	//  https://cloud.google.com/run/docs/configuring/labels.
	Labels map[string]string `json:"labels,omitempty"`

	// User-provided description of the Service. This field currently has a
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

	// Required. The template used to create revisions for this Service.
	Template *RevisionTemplate `json:"template,omitempty"`

	// Written here (not in Config Connector): the Service's own fields.

	// Optional. Specifies service-level scaling settings.
	Scaling *ServiceScaling `json:"scaling,omitempty"`

	// Optional. Provides the ingress settings for this Service: one of
	//  INGRESS_TRAFFIC_ALL, INGRESS_TRAFFIC_INTERNAL_ONLY,
	//  INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER, INGRESS_TRAFFIC_NONE.
	Ingress *string `json:"ingress,omitempty"`

	// Optional. Disables IAM permission check for run.routes.invoke for
	//  callers of this service: anyone who can reach it may invoke it.
	InvokerIamDisabled *bool `json:"invokerIamDisabled,omitempty"`

	// Optional. Disables the public, default run.app URI of the service.
	DefaultUriDisabled *bool `json:"defaultUriDisabled,omitempty"`

	// Optional. One or more custom audiences that you want this service to support.
	//  Specify each custom audience as the full URL in a string.
	CustomAudiences []string `json:"customAudiences,omitempty"`
}

// RunServiceStatus defines the config connector machine state of RunService
type RunServiceStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []k8sv1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the RunService resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *RunServiceObservedState `json:"observedState,omitempty"`
}

// RunServiceObservedState is the state of the RunService resource as most recently observed in GCP.
// +kcc:observedstate:proto=google.cloud.run.v2.Service
type RunServiceObservedState struct {
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

	// Required. The template used to create revisions for this Service.
	Template *RevisionTemplateObservedState `json:"template,omitempty"`

	// Output only. The generation of this Service currently serving traffic.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// Output only. The Condition of this Service, containing its readiness
	//  status, and detailed error information in case it did not reach a serving
	//  state.
	TerminalCondition *RunCondition `json:"terminalCondition,omitempty"`

	// Output only. The Conditions of all other associated sub-resources. They
	//  contain additional diagnostics information in case the Service does not
	//  reach its Serving state.
	ObservedConditions []RunCondition `json:"observedConditions,omitempty"`

	// Output only. Name of the latest revision that is serving traffic.
	LatestReadyRevision *string `json:"latestReadyRevision,omitempty"`

	// Output only. Name of the last created revision.
	LatestCreatedRevision *string `json:"latestCreatedRevision,omitempty"`

	// Written here: the main URI in which the Service is serving traffic.
	URI *string `json:"uri,omitempty"`

	// Written here: all URLs serving traffic for this Service.
	URLs []string `json:"urls,omitempty"`

	// Output only. One or more custom audiences that you want this service to support.
	CustomAudiences []string `json:"customAudiences,omitempty"`

	// Output only. Reserved for future use.
	SatisfiesPzs *bool `json:"satisfiesPzs,omitempty"`

	// Output only. Returns true if the Service is currently being acted upon
	//  by the system to bring it into the desired state.
	Reconciling *bool `json:"reconciling,omitempty"`

	// Output only. A system-generated fingerprint for this version of the
	//  resource. May be used to detect modification conflict during updates.
	Etag *string `json:"etag,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.RevisionTemplate
type RevisionTemplate struct {
	// Optional. The unique name for the revision. If this field is omitted, it
	//  will be automatically generated based on the Service name.
	Revision *string `json:"revision,omitempty"`

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

	// Written here (not in RunService's template): the RevisionTemplate
	// fields a service has.

	// Optional. Scaling settings for this Revision.
	Scaling *RevisionScaling `json:"scaling,omitempty"`

	// Optional. Max allowed time for an instance to respond to a request,
	//  as a duration such as "300s".
	Timeout *string `json:"timeout,omitempty"`

	// Optional. Sets the maximum number of requests that each serving instance
	//  can receive.
	MaxInstanceRequestConcurrency *int32 `json:"maxInstanceRequestConcurrency,omitempty"`

	// Optional. The sandbox environment to host this Revision:
	//  EXECUTION_ENVIRONMENT_GEN1 or EXECUTION_ENVIRONMENT_GEN2.
	ExecutionEnvironment *string `json:"executionEnvironment,omitempty"`

	// Optional. Enable session affinity.
	SessionAffinity *bool `json:"sessionAffinity,omitempty"`

	// Optional. Disables health checking containers during deployment.
	HealthCheckDisabled *bool `json:"healthCheckDisabled,omitempty"`

	// Optional. True if GPU zonal redundancy is disabled on this revision.
	GpuZonalRedundancyDisabled *bool `json:"gpuZonalRedundancyDisabled,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflrunservice;hflrunservices
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:metadata:annotations="gcp.hfl.dev/upstream-kind=run.cnrm.cloud.google.com/RunService"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// RunService is the Schema for the RunService API
// +k8s:openapi-gen=true
type RunService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   RunServiceSpec   `json:"spec,omitempty"`
	Status RunServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// RunServiceList contains a list of RunService
type RunServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RunService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RunService{}, &RunServiceList{})
}

// +kcc:proto=google.cloud.run.v2.VpcAccess
type VPCAccess struct {
	// VPC Access connector name.
	ConnectorRef *vpcaccessv1beta1.VPCAccessConnectorRef `json:"connectorRef,omitempty"`

	// Optional. Traffic VPC egress settings.
	Egress *string `json:"egress,omitempty"`

	// Optional. Direct VPC egress settings. Currently only single network
	//  interface is supported.
	NetworkInterfaces []VPCAccess_NetworkInterface `json:"networkInterfaces,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.VpcAccess.NetworkInterface
type VPCAccess_NetworkInterface struct {
	// Optional. The VPC network that the Cloud Run resource will be able to
	//  send traffic to.
	NetworkRef *computerefs.ComputeNetworkRef `json:"networkRef,omitempty"`

	// Optional. The VPC subnetwork that the Cloud Run resource will get IPs
	//  from.
	SubnetworkRef *computev1beta1.ComputeSubnetworkRef `json:"subnetworkRef,omitempty"`

	// Optional. Network tags applied to this Cloud Run resource.
	Tags []string `json:"tags,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.Container
type Container struct {
	// Name of the container specified as a DNS_LABEL (RFC 1123).
	Name *string `json:"name,omitempty"`

	// Required. Name of the container image in Dockerhub, Google Artifact
	//  Registry, or Google Container Registry.
	Image *string `json:"image,omitempty"`

	// Entrypoint array. Not executed within a shell.
	Command []string `json:"command,omitempty"`

	// Arguments to the entrypoint.
	Args []string `json:"args,omitempty"`

	// List of environment variables to set in the container.
	Env []EnvVar `json:"env,omitempty"`

	// Compute Resource requirements by this container.
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// List of ports to expose from the container.
	Ports []ContainerPort `json:"ports,omitempty"`

	// Volume to mount into the container's filesystem.
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`

	// Container's working directory.
	WorkingDir *string `json:"workingDir,omitempty"`

	// Periodic probe of container liveness.
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`

	// Startup probe of application within the container.
	StartupProbe *Probe `json:"startupProbe,omitempty"`

	// Names of the containers that must start before this container.
	DependsOn []string `json:"dependsOn,omitempty"`

	// Base image for this container. Only supported for services.
	BaseImageURI *string `json:"baseImageURI,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.EnvVar
type EnvVar struct {
	// Required. Name of the environment variable. Must not exceed 32768
	//  characters.
	Name *string `json:"name,omitempty"`

	// Literal value of the environment variable.
	Value *string `json:"value,omitempty"`

	// Source for the environment variable's value.
	ValueSource *EnvVarSource `json:"valueSource,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.EnvVarSource
type EnvVarSource struct {
	// Selects a secret and a specific version from Cloud Secret Manager.
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.SecretKeySelector
type SecretKeySelector struct {
	// Required. The name of the secret in Cloud Secret Manager.
	SecretRef *secretmanagerv1beta1.SecretManagerSecretRef `json:"secretRef,omitempty"`

	// The Cloud Secret Manager secret version.
	VersionRef *secretmanagerv1beta1.SecretManagerSecretVersionRef `json:"versionRef,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.VersionToPath
type VersionToPath struct {
	// Required. The relative path of the secret in the container.
	Path *string `json:"path,omitempty"`

	// The Cloud Secret Manager secret version.
	//  Can be 'latest' for the latest value, or an integer or a secret alias for a
	//  specific version.
	VersionRef *secretmanagerv1beta1.SecretManagerSecretVersionRef `json:"versionRef,omitempty"`

	// Integer octal mode bits to use on this file, must be a value between
	//  01 and 0777 (octal). If 0 or not set, the Volume's default mode will be
	//  used.
	Mode *int32 `json:"mode,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.Volume
type Volume struct {
	// Required. Volume's name.
	Name *string `json:"name,omitempty"`

	// Secret represents a secret that should populate this volume.
	Secret *SecretVolumeSource `json:"secret,omitempty"`

	// For Cloud SQL volumes, contains the specific instances that should be
	//  mounted.
	CloudSQLInstance *CloudSQLInstance `json:"cloudSQLInstance,omitempty"`

	// Ephemeral storage used as a shared volume.
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`

	// For NFS Voumes, contains the path to the nfs Volume
	Nfs *NfsVolumeSource `json:"nfs,omitempty"`

	// Persistent storage backed by a Google Cloud Storage bucket.
	GCS *GCSVolumeSource `json:"gcs,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.SecretVolumeSource
type SecretVolumeSource struct {
	// Required. The name of the secret in Cloud Secret Manager.
	SecretRef *secretmanagerv1beta1.SecretManagerSecretRef `json:"secretRef,omitempty"`

	// If unspecified, the volume will expose a file whose name is the
	//  secret, relative to VolumeMount.mount_path.
	Items []VersionToPath `json:"items,omitempty"`

	// Integer representation of mode bits to use on created files by default.
	DefaultMode *int32 `json:"defaultMode,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.GCSVolumeSource
type GCSVolumeSource struct {
	// Cloud Storage Bucket name.
	BucketRef *storagev1beta1.StorageBucketRef `json:"bucketRef,omitempty"`

	// If true, the volume will be mounted as read only for all mounts.
	ReadOnly *bool `json:"readOnly,omitempty"`

	// A list of additional flags to pass to the gcsfuse CLI.
	MountOptions []string `json:"mountOptions,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.CloudSqlInstance
type CloudSQLInstance struct {
	// The Cloud SQL instance connection names.
	InstanceRefs []*refs.SQLInstanceRef `json:"instanceRefs,omitempty"`
}

// +kcc:observedstate:proto=google.cloud.run.v2.RevisionTemplate
type RevisionTemplateObservedState struct {
	// Output only. The unique name for the revision.
	Revision *string `json:"revision,omitempty"`

	// Holds list of the containers that defines the unit of execution for this
	//  Revision.
	Containers []ContainerObservedState `json:"containers,omitempty"`
}

// +kcc:observedstate:proto=google.cloud.run.v2.Container
type ContainerObservedState struct {
	// Output only. The build info of the container image.
	BuildInfo *BuildInfoObservedState `json:"buildInfo,omitempty"`
}

// +kcc:observedstate:proto=google.cloud.run.v2.BuildInfo
type BuildInfoObservedState struct {
	// Output only. Entry point of the function when the image is a Cloud Run
	//  function.
	FunctionTarget *string `json:"functionTarget,omitempty"`

	// Output only. Source code location of the image.
	SourceLocation *string `json:"sourceLocation,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.BinaryAuthorization
type BinaryAuthorization struct {
	// Optional. If True, indicates to use the default project's binary
	//  authorization policy. If False, binary authorization will be disabled.
	UseDefault *bool `json:"useDefault,omitempty"`

	// Optional. The path to a binary authorization policy.
	//  Format: `projects/{project}/platforms/cloudRun/{policy-name}`
	//  Commented out because this field refers to Binary Authorization Platform Policies,
	//  which are not yet supported as a KRM resource in KCC.
	// Policy *string `json:"policy,omitempty"`

	// Optional. If present, indicates to use Breakglass using this justification.
	//  If use_default is False, then it must be empty.
	//  For more information on breakglass, see
	//  https://cloud.google.com/binary-authorization/docs/using-breakglass
	BreakglassJustification *string `json:"breakglassJustification,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.ServiceMesh
// +k8s:deepcopy-gen=true
type ServiceMesh struct {
	// The Mesh resource name.
	MeshRef *networkservicesv1alpha1.NetworkServicesMeshRef `json:"meshRef,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.Condition
// +k8s:deepcopy-gen=true
type RunCondition struct {
	// type is used to communicate the status of the reconciliation process.
	//  See also:
	//  https://github.com/knative/serving/blob/main/docs/spec/errors.md#error-conditions-and-reporting
	//  Types common to all resources include:
	//  * "Ready": True when the Resource is ready.
	Type *string `json:"type,omitempty"`

	// State of the condition.
	State *string `json:"state,omitempty"`

	// Human readable message indicating details about the current status.
	Message *string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime *string `json:"lastTransitionTime,omitempty"`

	// How to interpret failures of this condition, one of Error, Warning, Info
	Severity *string `json:"severity,omitempty"`

	// A common (service-level) reason for this condition.
	Reason *string `json:"reason,omitempty"`

	// A reason for the revision condition.
	RevisionReason *string `json:"revisionReason,omitempty"`

	// A reason for the execution condition.
	ExecutionReason *string `json:"executionReason,omitempty"`
}
