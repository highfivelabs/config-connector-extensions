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

// Modified by High Five Labs for config-connector-extensions: adapted from
// our ComputePerInstanceConfig (computeperinstanceconfig_types.go,
// itself copied from Config Connector) for the zonal per-instance config,
// which Config Connector has only as a Terraform-based alpha kind
// (pkg/clients/generated/apis/compute/v1alpha1/computeperinstanceconfig_types.go,
// UPSTREAM.md), with that kind's field names: zone and
// instanceGroupManagerRef for region and regionInstanceGroupManagerRef. A
// stable kind (v1beta1, docs/design.md decision 26). The preserved-state
// types are the regional kind's (the same JSON). projectRef, zone,
// instanceGroupManagerRef and resourceID immutable by CEL (they make the
// config's name); group, labels and short names are ours.

package v1beta1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputePerInstanceConfigGVK = GroupVersion.WithKind("ComputePerInstanceConfig")

// ComputeInstanceGroupManagerRef refers to a zonal managed instance group:
// a ComputeInstanceGroupManager (Config Connector's) by name, or its
// external form projects/{project}/zones/{zone}/instanceGroupManagers/{name}
// or bare name.
type ComputeInstanceGroupManagerRef struct {
	/* The name of the referent. */
	// +optional
	Name string `json:"name,omitempty"`

	/* The namespace of the referent. */
	// +optional
	Namespace string `json:"namespace,omitempty"`

	/* The external representation of the referent. */
	// +optional
	External string `json:"external,omitempty"`
}

// ComputePerInstanceConfigSpec defines the desired state of ComputePerInstanceConfig
// +kcc:spec:proto=google.cloud.compute.v1.PerInstanceConfig
type ComputePerInstanceConfigSpec struct {
	/* The zonal instance group manager the instance belongs to. Immutable. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="instanceGroupManagerRef is immutable"
	InstanceGroupManagerRef ComputeInstanceGroupManagerRef `json:"instanceGroupManagerRef"`

	/* The minimal action to perform on the instance during an update.
	Default is 'NONE'. Possible values are:
	* REPLACE
	* RESTART
	* REFRESH
	* NONE. */
	// +optional
	MinimalAction *string `json:"minimalAction,omitempty"`

	/* The most disruptive action to perform on the instance during an update.
	Default is 'REPLACE'. Possible values are:
	* REPLACE
	* RESTART
	* REFRESH
	* NONE. */
	// +optional
	MostDisruptiveAllowedAction *string `json:"mostDisruptiveAllowedAction,omitempty"`

	/* The preserved state for this instance. */
	// +optional
	PreservedState *RegionperinstanceconfigPreservedState `json:"preservedState,omitempty"`

	/* The project that this resource belongs to. Immutable. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef refs.ProjectRef `json:"projectRef"`

	/* When true, deleting this config will immediately remove any specified state from the underlying instance.
	When false, deleting this config will *not* immediately remove any state from the underlying instance.
	State will be removed on the next instance recreation or update. */
	// +optional
	RemoveInstanceStateOnDestroy *bool `json:"removeInstanceStateOnDestroy,omitempty"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	/* Immutable. Zone where the containing instance group manager is located. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="zone is immutable"
	Zone string `json:"zone"`
}

// ComputePerInstanceConfigStatus defines the config connector machine state of ComputePerInstanceConfig
type ComputePerInstanceConfigStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputePerInstanceConfig's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeperinstanceconfig;hflcomputeperinstanceconfigs
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputePerInstanceConfig is the Schema for the ComputePerInstanceConfig API
// +k8s:openapi-gen=true
type ComputePerInstanceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputePerInstanceConfigSpec   `json:"spec,omitempty"`
	Status ComputePerInstanceConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputePerInstanceConfigList contains a list of ComputePerInstanceConfig
type ComputePerInstanceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputePerInstanceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputePerInstanceConfig{}, &ComputePerInstanceConfigList{})
}
