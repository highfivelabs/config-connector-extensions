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
// apis/compute/v1alpha1/computeregionperinstanceconfig_types.go (UPSTREAM.md) as a stable
// kind (v1beta1, docs/design.md decision 26: Config Connector has it only as
// alpha). Changed: projectRef, region, regionInstanceGroupManagerRef and
// resourceID immutable by CEL (they make the config's name); group, labels
// and short names are ours.

package v1beta1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionPerInstanceConfigGVK = GroupVersion.WithKind("ComputeRegionPerInstanceConfig")

type RegionperinstanceconfigDisk struct {
	/* A value that prescribes what should happen to the stateful disk when the VM instance is deleted.
	The available options are 'NEVER' and 'ON_PERMANENT_INSTANCE_DELETION'.
	'NEVER' - detach the disk when the VM is deleted, but do not delete the disk.
	'ON_PERMANENT_INSTANCE_DELETION' will delete the stateful disk when the VM is permanently
	deleted from the instance group. Default value: "NEVER" Possible values: ["NEVER", "ON_PERMANENT_INSTANCE_DELETION"]. */
	// +optional
	DeleteRule *string `json:"deleteRule,omitempty"`

	/* A unique device name that is reflected into the /dev/ tree of a Linux operating system running within the instance. */
	DeviceName string `json:"deviceName"`

	/* The mode of the disk. Default value: "READ_WRITE" Possible values: ["READ_ONLY", "READ_WRITE"]. */
	// +optional
	Mode *string `json:"mode,omitempty"`

	/* The URI of an existing persistent disk to attach under the specified device-name in the format
	'projects/project-id/zones/zone/disks/disk-name'. */
	Source string `json:"source"`
}

type RegionperinstanceconfigExternalIp struct {
	/* These stateful IPs will never be released during autohealing, update or VM instance recreate operations. This flag is used to configure if the IP reservation should be deleted after it is no longer used by the group, e.g. when the given instance or the whole group is deleted. Default value: "NEVER" Possible values: ["NEVER", "ON_PERMANENT_INSTANCE_DELETION"]. */
	// +optional
	AutoDelete *string `json:"autoDelete,omitempty"`

	InterfaceName string `json:"interfaceName"`

	/* Ip address representation. */
	// +optional
	IpAddress *RegionperinstanceconfigIpAddress `json:"ipAddress,omitempty"`
}

type RegionperinstanceconfigInternalIp struct {
	/* These stateful IPs will never be released during autohealing, update or VM instance recreate operations. This flag is used to configure if the IP reservation should be deleted after it is no longer used by the group, e.g. when the given instance or the whole group is deleted. Default value: "NEVER" Possible values: ["NEVER", "ON_PERMANENT_INSTANCE_DELETION"]. */
	// +optional
	AutoDelete *string `json:"autoDelete,omitempty"`

	InterfaceName string `json:"interfaceName"`

	/* Ip address representation. */
	// +optional
	IpAddress *RegionperinstanceconfigIpAddress `json:"ipAddress,omitempty"`
}

type RegionperinstanceconfigIpAddress struct {
	/* The URL of the reservation for this IP address. */
	// +optional
	Address *string `json:"address,omitempty"`
}

type RegionperinstanceconfigPreservedState struct {
	/* Stateful disks for the instance. */
	// +optional
	Disk []RegionperinstanceconfigDisk `json:"disk,omitempty"`

	/* Preserved external IPs defined for this instance. This map is keyed with the name of the network interface. */
	// +optional
	ExternalIp []RegionperinstanceconfigExternalIp `json:"externalIp,omitempty"`

	/* Preserved internal IPs defined for this instance. This map is keyed with the name of the network interface. */
	// +optional
	InternalIp []RegionperinstanceconfigInternalIp `json:"internalIp,omitempty"`

	/* Preserved metadata defined for this instance. This is a list of key->value pairs. */
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ComputeRegionInstanceGroupManagerRef struct {
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

// ComputeRegionPerInstanceConfigSpec defines the desired state of ComputeRegionPerInstanceConfig
// +kcc:spec:proto=google.cloud.compute.v1.PerInstanceConfig
type ComputeRegionPerInstanceConfigSpec struct {
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

	/* Immutable. Region where the containing instance group manager is located. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="region is immutable"
	Region string `json:"region"`

	/* The regional instance group manager the instance belongs to. Immutable. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="regionInstanceGroupManagerRef is immutable"
	RegionInstanceGroupManagerRef ComputeRegionInstanceGroupManagerRef `json:"regionInstanceGroupManagerRef"`

	/* When true, deleting this config will immediately remove any specified state from the underlying instance.
	When false, deleting this config will *not* immediately remove any state from the underlying instance.
	State will be removed on the next instance recreation or update. */
	// +optional
	RemoveInstanceStateOnDestroy *bool `json:"removeInstanceStateOnDestroy,omitempty"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`
}

// ComputeRegionPerInstanceConfigStatus defines the config connector machine state of ComputeRegionPerInstanceConfig
type ComputeRegionPerInstanceConfigStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputeRegionPerInstanceConfig's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregionperinstanceconfig;hflcomputeregionperinstanceconfigs
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionPerInstanceConfig is the Schema for the ComputeRegionPerInstanceConfig API
// +k8s:openapi-gen=true
type ComputeRegionPerInstanceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionPerInstanceConfigSpec   `json:"spec,omitempty"`
	Status ComputeRegionPerInstanceConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionPerInstanceConfigList contains a list of ComputeRegionPerInstanceConfig
type ComputeRegionPerInstanceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionPerInstanceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionPerInstanceConfig{}, &ComputeRegionPerInstanceConfigList{})
}
