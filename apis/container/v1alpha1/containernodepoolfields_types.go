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
// decision 37 names it). The fields are the node pool's kubeletConfig
// fields Config Connector's ContainerNodePool lacks, with the API's names
// (NodeConfig.kubeletConfig in container.v1).

import (
	containerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/container/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ContainerNodePoolFieldsGVK = GroupVersion.WithKind("ContainerNodePoolFields")

// ContainerNodePoolFieldsSpec is the fields of a GKE node pool this object
// owns. A field left out is not owned: it is never written.
type ContainerNodePoolFieldsSpec struct {
	// Immutable. The node pool: Config Connector's ContainerNodePool by
	// name (container.cnrm.cloud.google.com), or any node pool by external,
	// as "projects/{{projectID}}/locations/{{location}}/clusters/{{cluster}}/nodePools/{{nodePool}}".
	// The node pool must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *containerv1beta1.ContainerNodePoolRef `json:"resourceRef"`

	// The node pool's kubelet settings this object owns.
	// +required
	Config *NodePoolFieldsConfig `json:"config"`
}

// NodePoolFieldsConfig is the part of GKE's NodeConfig this kind owns.
type NodePoolFieldsConfig struct {
	// Kubelet settings Config Connector's kubeletConfig lacks.
	// +required
	KubeletConfig *NodePoolKubeletConfig `json:"kubeletConfig"`
}

// NodePoolKubeletConfig is the part of GKE's NodeKubeletConfig this kind
// owns.
// +kubebuilder:validation:XValidation:rule="has(self.insecureKubeletReadonlyPortEnabled) || has(self.containerLogMaxSize) || has(self.containerLogMaxFiles) || has(self.allowedUnsafeSysctls) || has(self.maxParallelImagePulls)",message="set at least one field"
type NodePoolKubeletConfig struct {
	// Enable the kubelet's insecure read-only port (10255).
	// +optional
	InsecureKubeletReadonlyPortEnabled *bool `json:"insecureKubeletReadonlyPortEnabled,omitempty"`

	// The largest a container log file may grow before it is rotated, as
	// a quantity with a unit ("10Mi"); 10Mi to 500Mi.
	// +optional
	ContainerLogMaxSize *string `json:"containerLogMaxSize,omitempty"`

	// The most log files a container keeps; 2 to 10.
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`

	// Unsafe sysctls, or sysctl groups ending in "*", pods may set.
	// +optional
	AllowedUnsafeSysctls []string `json:"allowedUnsafeSysctls,omitempty"`

	// How many images the kubelet pulls at once; 2 to 5.
	// +optional
	MaxParallelImagePulls *int32 `json:"maxParallelImagePulls,omitempty"`
}

// ContainerNodePoolFieldsStatus defines the observed state.
type ContainerNodePoolFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The node pool whose fields this object sets, as
	// projects/{project}/locations/{location}/clusters/{cluster}/nodePools/{nodePool}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcontainernodepoolfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ContainerNodePoolFields sets kubelet settings of a GKE node pool that
// Config Connector's ContainerNodePool has no field for, and only those.
// The API takes a node pool's kubeletConfig whole, so each write reads the
// node pool and sends its kubeletConfig with only this object's fields
// changed. Deleting it clears them (the API's defaults apply again); with
// deletion-policy abandon they stay.
// +k8s:openapi-gen=true
type ContainerNodePoolFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ContainerNodePoolFieldsSpec   `json:"spec,omitempty"`
	Status ContainerNodePoolFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ContainerNodePoolFieldsList contains a list of ContainerNodePoolFields
type ContainerNodePoolFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerNodePoolFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerNodePoolFields{}, &ContainerNodePoolFieldsList{})
}
