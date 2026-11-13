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
// apis/aiplatform/v1alpha1/aiplatformreasoningengine_types.go (UPSTREAM.md) as a stable
// kind (v1beta1, docs/design.md decision 26: Config Connector has it only as
// alpha). Changed: group, labels and short names are ours; the leaf types
// (environment variables, PSC interface, encryption) are upstream's, imported
// rather than copied, so an upstream fix reaches ours; location, project and
// resourceID are immutable by CEL (upstream's identity check refuses a
// change instead). The reasoning engine's ID is assigned by the API at
// create, so resourceID is only for acquiring an existing engine; the ID
// lives in status.externalRef.

package v1beta1

import (
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/aiplatform/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var AIPlatformReasoningEngineGVK = GroupVersion.WithKind("AIPlatformReasoningEngine")

// AIPlatformReasoningEngineSpec defines the desired state of AIPlatformReasoningEngine
// +kcc:spec:proto=google.cloud.aiplatform.v1.ReasoningEngine
type AIPlatformReasoningEngineSpec struct {
	// The project that this resource belongs to.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The location of this resource.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location *string `json:"location"`

	// The ID of an existing reasoning engine to acquire. The API assigns a
	// new engine's ID at create; leave this unset to create one. The ID is
	// then in status.externalRef.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// Required. The display name of the ReasoningEngine.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngine.display_name
	// +required
	DisplayName *string `json:"displayName"`

	// Optional. The description of the ReasoningEngine.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngine.description
	Description *string `json:"description,omitempty"`

	// Optional. Configurations of the ReasoningEngine
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngine.spec
	Spec *ReasoningEngineSpec `json:"spec,omitempty"`

	// Customer-managed encryption key spec for a ReasoningEngine. If set, this
	//  ReasoningEngine and all sub-resources of this ReasoningEngine will be
	//  secured by this key.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngine.encryption_spec
	EncryptionSpec *upstream.EncryptionSpec `json:"encryptionSpec,omitempty"`

	// Optional. The labels with user-defined metadata to organize your
	//  ReasoningEngine.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngine.labels
	Labels map[string]string `json:"labels,omitempty"`
}

// +kcc:proto=google.cloud.aiplatform.v1.ReasoningEngineSpec
type ReasoningEngineSpec struct {
	// Optional. The service account that the Reasoning Engine artifact runs as.
	//  It should have "roles/storage.objectViewer" for reading the user project's
	//  Cloud Storage and "roles/aiplatform.user" for using Vertex extensions. If
	//  not specified, the Vertex AI Reasoning Engine Service Agent in the project
	//  will be used.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.service_account
	ServiceAccountRef *refsv1beta1.IAMServiceAccountRef `json:"serviceAccountRef,omitempty"`

	// Optional. User provided package spec of the ReasoningEngine.
	//  Ignored when users directly specify a deployment image through
	//  `deployment_spec.first_party_image_override`, but keeping the
	//  field_behavior to avoid introducing breaking changes.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.package_spec
	PackageSpec *ReasoningEngineSpec_PackageSpec `json:"packageSpec,omitempty"`

	// Optional. The specification of a Reasoning Engine deployment.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.deployment_spec
	DeploymentSpec *ReasoningEngineSpec_DeploymentSpec `json:"deploymentSpec,omitempty"`

	// Optional. The OSS agent framework used to develop the agent.
	//  Currently supported values: "google-adk", "langchain", "langgraph", "ag2",
	//  "llama-index", "custom".
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.agent_framework
	AgentFramework *string `json:"agentFramework,omitempty"`
}

// +kcc:proto=google.cloud.aiplatform.v1.ReasoningEngineSpec.PackageSpec
type ReasoningEngineSpec_PackageSpec struct {
	// Optional. The Cloud Storage URI of the pickled python object.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.PackageSpec.pickle_object_gcs_uri
	PickleObjectGCSURI *string `json:"pickleObjectGCSURI,omitempty"`

	// Optional. The Cloud Storage URI of the dependency files in tar.gz format.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.PackageSpec.dependency_files_gcs_uri
	DependencyFilesGCSURI *string `json:"dependencyFilesGCSURI,omitempty"`

	// Optional. The Cloud Storage URI of the `requirements.txt` file
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.PackageSpec.requirements_gcs_uri
	RequirementsGCSURI *string `json:"requirementsGCSURI,omitempty"`

	// Optional. The Python version. Currently support 3.8, 3.9, 3.10, 3.11.
	//  If not specified, default value is 3.10.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.PackageSpec.python_version
	PythonVersion *string `json:"pythonVersion,omitempty"`
}

// +kcc:proto=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec
type ReasoningEngineSpec_DeploymentSpec struct {
	// Optional. Environment variables to be set with the Reasoning Engine
	//  deployment. The environment variables can be updated through the
	//  UpdateReasoningEngine API.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec.env
	Env []upstream.EnvVar `json:"env,omitempty"`

	// Optional. Environment variables where the value is a secret in Cloud
	//  Secret Manager.
	//  To use this feature, add 'Secret Manager Secret Accessor' role
	//  (roles/secretmanager.secretAccessor) to AI Platform Reasoning Engine
	//  Service Agent.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec.secret_env
	SecretEnv []upstream.SecretEnvVar `json:"secretEnv,omitempty"`

	// Optional. Configuration for PSC-I.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec.psc_interface_config
	PSCInterfaceConfig *upstream.PSCInterfaceConfig `json:"pscInterfaceConfig,omitempty"`

	// Optional. The minimum number of application instances that will be kept
	//  running at all times. Defaults to 1. Range: [0, 10].
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec.min_instances
	MinInstances *int32 `json:"minInstances,omitempty"`

	// Optional. The maximum number of application instances that can be
	//  launched to handle increased traffic. Defaults to 100. Range: [1, 1000].
	//
	//  If VPC-SC or PSC-I is enabled, the acceptable range is [1, 100].
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec.max_instances
	MaxInstances *int32 `json:"maxInstances,omitempty"`

	// Optional. Resource limits for each container. Only 'cpu' and 'memory'
	//  keys are supported. Defaults to {"cpu": "4", "memory": "4Gi"}.
	//
	//    * The only supported values for CPU are '1', '2', '4', '6' and '8'. For
	//    more information, go to
	//    https://cloud.google.com/run/docs/configuring/cpu.
	//    * The only supported values for memory are '1Gi', '2Gi', ... '32 Gi'.
	//    * For required cpu on different memory values, go to
	//    https://cloud.google.com/run/docs/configuring/memory-limits
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec.resource_limits
	ResourceLimits map[string]string `json:"resourceLimits,omitempty"`

	// Optional. Concurrency for each container and agent server. Recommended
	//  value: 2 * cpu + 1. Defaults to 9.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngineSpec.DeploymentSpec.container_concurrency
	ContainerConcurrency *int32 `json:"containerConcurrency,omitempty"`
}

// AIPlatformReasoningEngineStatus defines the config connector machine state of AIPlatformReasoningEngine
type AIPlatformReasoningEngineStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the AIPlatformReasoningEngine resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *AIPlatformReasoningEngineObservedState `json:"observedState,omitempty"`
}

// AIPlatformReasoningEngineObservedState is the state of the AIPlatformReasoningEngine resource as most recently observed in GCP.
// +kcc:observedstate:proto=google.cloud.aiplatform.v1.ReasoningEngine
type AIPlatformReasoningEngineObservedState struct {
	// Output only. Timestamp when this ReasoningEngine was created.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngine.create_time
	CreateTime *string `json:"createTime,omitempty"`

	// Output only. Timestamp when this ReasoningEngine was most recently updated.
	// +kcc:proto:field=google.cloud.aiplatform.v1.ReasoningEngine.update_time
	UpdateTime *string `json:"updateTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflaiplatformreasoningengine;hflaiplatformreasoningengines
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// AIPlatformReasoningEngine is a Vertex AI Agent Engine (reasoning engine).
// +k8s:openapi-gen=true
type AIPlatformReasoningEngine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   AIPlatformReasoningEngineSpec   `json:"spec,omitempty"`
	Status AIPlatformReasoningEngineStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// AIPlatformReasoningEngineList contains a list of AIPlatformReasoningEngine
type AIPlatformReasoningEngineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIPlatformReasoningEngine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIPlatformReasoningEngine{}, &AIPlatformReasoningEngineList{})
}
