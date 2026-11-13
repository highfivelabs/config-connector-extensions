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
// apis/compute/v1alpha1/computeautoscaler_types.go (UPSTREAM.md) as a stable kind
// (v1beta1, docs/design.md decision 26: Config Connector has it only as
// alpha, with no controller). Changed: only the regional autoscaler's
// types (the file also holds the zonal one's, copied as
// computeautoscaler_types.go); projectRef, region, resourceID and target
// immutable by CEL (the name and the group it scales); group, labels and
// short names are ours.

package v1beta1

import (
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	v1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRegionAutoscalerGVK = GroupVersion.WithKind("ComputeRegionAutoscaler")

// +kcc:proto=google.cloud.compute.v1.AutoscalingPolicyCpuUtilization
type RegionautoscalerCpuUtilization struct {
	// Indicates whether predictive autoscaling based on CPU metric is enabled.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyCpuUtilization.predictive_method
	// +optional
	PredictiveMethod *string `json:"predictiveMethod,omitempty"`

	// The target CPU utilization that the autoscaler maintains.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyCpuUtilization.utilization_target
	Target float64 `json:"target"`
}

// +kcc:proto=google.cloud.compute.v1.AutoscalingPolicyLoadBalancingUtilization
type RegionautoscalerLoadBalancingUtilization struct {
	// Fraction of backend capacity utilization (set in HTTP(s) load balancing configuration) that the autoscaler maintains.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyLoadBalancingUtilization.utilization_target
	Target float64 `json:"target"`
}

// +kcc:proto=google.cloud.compute.v1.FixedOrPercent
type RegionautoscalerFixedOrPercent struct {
	// Specifies a fixed number of VM instances. This must be a positive integer.
	// +kcc:proto:field=google.cloud.compute.v1.FixedOrPercent.fixed
	// +optional
	Fixed *int32 `json:"fixed,omitempty"`

	// Specifies a percentage of instances between 0 to 100%, inclusive.
	// +kcc:proto:field=google.cloud.compute.v1.FixedOrPercent.percent
	// +optional
	Percent *int32 `json:"percent,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.AutoscalingPolicyScaleInControl
type RegionautoscalerScaleInControl struct {
	// Maximum allowed number (or %) of VMs that can be deducted from the peak recommendation during the window.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScaleInControl.max_scaled_in_replicas
	// +optional
	MaxScaledInReplicas *RegionautoscalerFixedOrPercent `json:"maxScaledInReplicas,omitempty"`

	// How far back autoscaling looks when computing recommendations to include directives regarding slower scale in.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScaleInControl.time_window_sec
	// +optional
	TimeWindowSec *int32 `json:"timeWindowSec,omitempty"`
}

type RegionautoscalerScaleDownControl struct {
	// +optional
	MaxScaledDownReplicas *RegionautoscalerFixedOrPercent `json:"maxScaledDownReplicas,omitempty"`

	// +optional
	TimeWindowSec *int32 `json:"timeWindowSec,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.AutoscalingPolicyScalingSchedule
type RegionautoscalerScalingSchedules struct {
	// A description of a scaling schedule.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScalingSchedule.description
	// +optional
	Description *string `json:"description,omitempty"`

	// A boolean value that specifies whether a scaling schedule can influence autoscaler recommendations.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScalingSchedule.disabled
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// The duration of time intervals, in seconds, for which this scaling schedule is to run.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScalingSchedule.duration_sec
	DurationSec int32 `json:"durationSec"`

	// The minimum number of VM instances that the autoscaler will recommend in time intervals starting according to schedule.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScalingSchedule.min_required_replicas
	MinRequiredReplicas int32 `json:"minRequiredReplicas"`

	Name string `json:"name"`

	// The start timestamps of time intervals when this scaling schedule is to provide a scaling signal.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScalingSchedule.schedule
	Schedule string `json:"schedule"`

	// The time zone to use when interpreting the schedule.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyScalingSchedule.time_zone
	// +optional
	TimeZone *string `json:"timeZone,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.AutoscalingPolicyCustomMetricUtilization
type RegionautoscalerMetric struct {
	// A filter string, compatible with a Stackdriver Monitoring filter string for TimeSeries.list API call.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyCustomMetricUtilization.filter
	// +optional
	Filter *string `json:"filter,omitempty"`

	// The identifier (type) of the Stackdriver Monitoring metric.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyCustomMetricUtilization.metric
	Name string `json:"name"`

	// If scaling is based on a per-group metric value that represents the total amount of work to be done or resource usage, set this value to an amount assigned for a single instance of the scaled group.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyCustomMetricUtilization.single_instance_assignment
	// +optional
	SingleInstanceAssignment *float64 `json:"singleInstanceAssignment,omitempty"`

	// The target value of the metric that autoscaler maintains.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyCustomMetricUtilization.utilization_target
	// +optional
	Target *float64 `json:"target,omitempty"`

	// Defines how target utilization value is expressed for a Stackdriver Monitoring metric.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicyCustomMetricUtilization.utilization_target_type
	// +optional
	Type *string `json:"type,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.AutoscalingPolicy
type RegionautoscalerAutoscalingPolicy struct {
	// The number of seconds that your application takes to initialize on a VM instance.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.cool_down_period_sec
	// +optional
	CooldownPeriod *int32 `json:"cooldownPeriod,omitempty"`

	// Defines the CPU utilization policy that allows the autoscaler to scale based on the average CPU utilization of a managed instance group.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.cpu_utilization
	// +optional
	CpuUtilization *RegionautoscalerCpuUtilization `json:"cpuUtilization,omitempty"`

	// Configuration parameters of autoscaling based on load balancer.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.load_balancing_utilization
	// +optional
	LoadBalancingUtilization *RegionautoscalerLoadBalancingUtilization `json:"loadBalancingUtilization,omitempty"`

	// The maximum number of instances that the autoscaler can scale out to.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.max_num_replicas
	MaxReplicas int32 `json:"maxReplicas"`

	// Configuration parameters of autoscaling based on a custom metric.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.custom_metric_utilizations
	// +optional
	Metric []RegionautoscalerMetric `json:"metric,omitempty"`

	// The minimum number of replicas that the autoscaler can scale in to.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.min_num_replicas
	MinReplicas int32 `json:"minReplicas"`

	// Defines the operating mode for this policy.
	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.mode
	// +optional
	Mode *string `json:"mode,omitempty"`

	// +optional
	ScaleDownControl *RegionautoscalerScaleDownControl `json:"scaleDownControl,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.scale_in_control
	// +optional
	ScaleInControl *RegionautoscalerScaleInControl `json:"scaleInControl,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.AutoscalingPolicy.scaling_schedules
	// +optional
	ScalingSchedules []RegionautoscalerScalingSchedules `json:"scalingSchedules,omitempty"`
}

// ComputeRegionAutoscalerSpec defines the desired state of ComputeRegionAutoscaler
// +kcc:spec:proto=google.cloud.compute.v1.Autoscaler
type ComputeRegionAutoscalerSpec struct {
	// The project that this resource belongs to. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef refs.ProjectRef `json:"projectRef"`

	// The configuration parameters for the autoscaling algorithm.
	// +kcc:proto:field=google.cloud.compute.v1.Autoscaler.autoscaling_policy
	AutoscalingPolicy RegionautoscalerAutoscalingPolicy `json:"autoscalingPolicy"`

	// An optional description of this resource.
	// +kcc:proto:field=google.cloud.compute.v1.Autoscaler.description
	// +optional
	Description *string `json:"description,omitempty"`

	// Immutable. URL of the region where the instance group resides.
	// +kcc:proto:field=google.cloud.compute.v1.Autoscaler.region
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="region is immutable"
	Region string `json:"region"`

	// Immutable. The ComputeRegionAutoscaler name. If not given, the metadata.name will be used.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// Immutable. URL of the regional managed instance group that this
	// autoscaler will scale.
	// +kcc:proto:field=google.cloud.compute.v1.Autoscaler.target
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="target is immutable"
	Target string `json:"target"`
}

// ComputeRegionAutoscalerStatus defines the config connector machine state of ComputeRegionAutoscaler
type ComputeRegionAutoscalerStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// Creation timestamp in RFC3339 text format.
	// +kcc:proto:field=google.cloud.compute.v1.Autoscaler.creation_timestamp
	// +optional
	CreationTimestamp *string `json:"creationTimestamp,omitempty"`

	// Server-defined URL for the resource.
	// +kcc:proto:field=google.cloud.compute.v1.Autoscaler.self_link
	// +optional
	SelfLink *string `json:"selfLink,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputeregionautoscaler;hflcomputeregionautoscalers
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRegionAutoscaler is the Schema for the ComputeRegionAutoscaler API
// +k8s:openapi-gen=true
type ComputeRegionAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRegionAutoscalerSpec   `json:"spec,omitempty"`
	Status ComputeRegionAutoscalerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRegionAutoscalerList contains a list of ComputeRegionAutoscaler
type ComputeRegionAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRegionAutoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRegionAutoscaler{}, &ComputeRegionAutoscalerList{})
}
