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

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/compute/v1beta1/types.generated.go (UPSTREAM.md), where these two
// types are generated but commented out as unreachable (Config Connector's
// ComputeSecurityPolicy has no threshold configs). Name is required here,
// as the API requires it; the lists are atomic; the granularity type's
// values are named.

package v1alpha1

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig
type SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig struct {
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.auto_deploy_confidence_threshold
	// +optional
	AutoDeployConfidenceThreshold *float32 `json:"autoDeployConfidenceThreshold,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.auto_deploy_expiration_sec
	// +optional
	AutoDeployExpirationSec *int32 `json:"autoDeployExpirationSec,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.auto_deploy_impacted_baseline_threshold
	// +optional
	AutoDeployImpactedBaselineThreshold *float32 `json:"autoDeployImpactedBaselineThreshold,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.auto_deploy_load_threshold
	// +optional
	AutoDeployLoadThreshold *float32 `json:"autoDeployLoadThreshold,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.detection_absolute_qps
	// +optional
	DetectionAbsoluteQps *float32 `json:"detectionAbsoluteQps,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.detection_load_threshold
	// +optional
	DetectionLoadThreshold *float32 `json:"detectionLoadThreshold,omitempty"`

	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.detection_relative_to_baseline_qps
	// +optional
	DetectionRelativeToBaselineQps *float32 `json:"detectionRelativeToBaselineQps,omitempty"`

	// The name must be 1-63 characters long, and comply with RFC1035. The name must be unique within the security policy.
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.name
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Configuration options for enabling Adaptive Protection to operate on specified granular traffic units.
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig.traffic_granularity_configs
	// +optional
	// +listType=atomic
	TrafficGranularityConfigs []SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig `json:"trafficGranularityConfigs,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig
type SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig struct {
	// If enabled, traffic matching each unique value for the specified type constitutes a separate traffic unit. It can only be set to true if `value` is empty.
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig.enable_each_unique_value
	// +optional
	EnableEachUniqueValue *bool `json:"enableEachUniqueValue,omitempty"`

	// Type of this configuration: HTTP_HEADER_HOST or HTTP_PATH.
	// +kubebuilder:validation:Enum=HTTP_HEADER_HOST;HTTP_PATH
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig.type
	// +optional
	Type *string `json:"type,omitempty"`

	// Requests that match this value constitute a granular traffic unit.
	// +kcc:proto:field=google.cloud.compute.v1.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig.value
	// +optional
	Value *string `json:"value,omitempty"`
}
