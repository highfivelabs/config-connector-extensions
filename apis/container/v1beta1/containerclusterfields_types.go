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

package v1beta1

// Written here: a field kind (docs/design.md section 6.6, pattern 2;
// decision 37 names it). Each field is one the GKE API changes with one
// clusters.update `desired*` field and Config Connector's ContainerCluster
// has no field for; the names are the API's (Cluster in container.v1).

import (
	containerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/container/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ContainerClusterFieldsGVK = GroupVersion.WithKind("ContainerClusterFields")

// ContainerClusterFieldsSpec is the fields of a GKE cluster this object
// owns. A field left out is not owned: it is never read or written.
// +kubebuilder:validation:XValidation:rule="has(self.secretManagerConfig) || has(self.rbacBindingConfig) || has(self.anonymousAuthenticationConfig) || has(self.addonsConfig) || has(self.nodePoolAutoConfig)",message="set at least one field"
type ContainerClusterFieldsSpec struct {
	// Immutable. The cluster: Config Connector's ContainerCluster by name
	// (container.cnrm.cloud.google.com), or any cluster by external, as
	// "projects/{{projectID}}/locations/{{location}}/clusters/{{cluster}}".
	// The cluster must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *containerv1beta1.ContainerClusterRef `json:"resourceRef"`

	// Secret Manager add-on (clusters.update desiredSecretManagerConfig).
	// +optional
	SecretManagerConfig *SecretManagerConfig `json:"secretManagerConfig,omitempty"`

	// The RBAC bindings to system:anonymous and the system:unauthenticated
	// and system:authenticated groups (desiredRbacBindingConfig).
	// +optional
	RBACBindingConfig *RBACBindingConfig `json:"rbacBindingConfig,omitempty"`

	// Anonymous authentication to the API server
	// (desiredAnonymousAuthenticationConfig).
	// +optional
	AnonymousAuthenticationConfig *AnonymousAuthenticationConfig `json:"anonymousAuthenticationConfig,omitempty"`

	// Add-ons Config Connector's addonsConfig lacks (desiredAddonsConfig,
	// sent with only these add-ons set).
	// +optional
	AddonsConfig *AddonsConfig `json:"addonsConfig,omitempty"`

	// Settings of auto-provisioned node pools Config Connector's
	// nodePoolAutoConfig lacks (desiredNodePoolAutoConfigKubeletConfig).
	// +optional
	NodePoolAutoConfig *NodePoolAutoConfig `json:"nodePoolAutoConfig,omitempty"`
}

// SecretManagerConfig is GKE's SecretManagerConfig.
type SecretManagerConfig struct {
	// Enable the Secret Manager CSI driver add-on.
	// +required
	Enabled *bool `json:"enabled"`

	// Rotation of the secrets the add-on mounts.
	// +optional
	RotationConfig *SecretManagerRotationConfig `json:"rotationConfig,omitempty"`
}

// SecretManagerRotationConfig is GKE's SecretManagerConfig.RotationConfig.
type SecretManagerRotationConfig struct {
	// Enable rotation.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// How often secrets are rotated, as a duration ("120s").
	// +optional
	RotationInterval *string `json:"rotationInterval,omitempty"`
}

// RBACBindingConfig is GKE's RBACBindingConfig.
type RBACBindingConfig struct {
	// Allow bindings to system:anonymous or system:unauthenticated.
	// +optional
	EnableInsecureBindingSystemUnauthenticated *bool `json:"enableInsecureBindingSystemUnauthenticated,omitempty"`

	// Allow bindings to system:authenticated, other than the default ones.
	// +optional
	EnableInsecureBindingSystemAuthenticated *bool `json:"enableInsecureBindingSystemAuthenticated,omitempty"`
}

// AnonymousAuthenticationConfig is GKE's AnonymousAuthenticationConfig.
type AnonymousAuthenticationConfig struct {
	// ENABLED, or LIMITED (anonymous requests reach only the health check
	// endpoints).
	// +required
	// +kubebuilder:validation:Enum=ENABLED;LIMITED
	Mode string `json:"mode"`
}

// AddonsConfig is the part of GKE's AddonsConfig this kind owns.
type AddonsConfig struct {
	// The Stateful HA add-on.
	// +optional
	StatefulHAConfig *StatefulHAConfig `json:"statefulHaConfig,omitempty"`
}

// StatefulHAConfig is GKE's StatefulHAConfig.
type StatefulHAConfig struct {
	// Enable the add-on.
	// +required
	Enabled bool `json:"enabled"`
}

// NodePoolAutoConfig is the part of GKE's NodePoolAutoConfig this kind owns.
type NodePoolAutoConfig struct {
	// Kubelet settings of auto-provisioned node pools.
	// +optional
	NodeKubeletConfig *AutoConfigKubeletConfig `json:"nodeKubeletConfig,omitempty"`
}

// AutoConfigKubeletConfig is the part of GKE's NodeKubeletConfig that
// applies to auto-provisioned node pools.
type AutoConfigKubeletConfig struct {
	// Enable the kubelet's insecure read-only port (10255).
	// +optional
	InsecureKubeletReadonlyPortEnabled *bool `json:"insecureKubeletReadonlyPortEnabled,omitempty"`
}

// ContainerClusterFieldsStatus defines the observed state.
type ContainerClusterFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The cluster whose fields this object sets, as
	// projects/{project}/locations/{location}/clusters/{cluster}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcontainerclusterfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ContainerClusterFields sets fields of a GKE cluster that Config
// Connector's ContainerCluster has no field for, and only those: one
// clusters.update per changed field, each carrying a single desired*
// field. Deleting it leaves the fields as they are (the API has no "unset"
// for them; set the value wanted before deleting).
// +k8s:openapi-gen=true
type ContainerClusterFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ContainerClusterFieldsSpec   `json:"spec,omitempty"`
	Status ContainerClusterFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ContainerClusterFieldsList contains a list of ContainerClusterFields
type ContainerClusterFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerClusterFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerClusterFields{}, &ContainerClusterFieldsList{})
}
