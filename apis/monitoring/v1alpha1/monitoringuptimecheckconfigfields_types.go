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

// Modified by High Five Labs for config-connector-extensions: a field kind
// (docs/design.md section 6.6, pattern 2; decision 37 names it), written
// here. The field types (UserLabels, and HttpCheck's
// CustomContentType, AcceptedResponseStatusCodes with ResponseStatusCode,
// PingConfig and ServiceAgentAuthentication) are copied from Config
// Connector apis/monitoring/v1beta1/types.generated.go (UPSTREAM.md), where
// they are generated but commented out (Config Connector's
// MonitoringUptimeCheckConfig is DCL-based and has none of them). Changed:
// enums listed, the lists atomic, the HTTP check's fields in a type of
// their own.

package v1alpha1

import (
	monitoringv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/monitoring/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var MonitoringUptimeCheckConfigFieldsGVK = GroupVersion.WithKind("MonitoringUptimeCheckConfigFields")

// MonitoringUptimeCheckConfigFieldsSpec is the fields of an uptime check
// this object owns. A field left unset is not owned: this object never
// sends it.
// +kubebuilder:validation:XValidation:rule="has(self.userLabels) || has(self.httpCheck)",message="set at least one of userLabels and httpCheck"
type MonitoringUptimeCheckConfigFieldsSpec struct {
	// Immutable. The uptime check: Config Connector's
	// MonitoringUptimeCheckConfig by name (monitoring.cnrm.cloud.google.com),
	// or any uptime check by external, as
	// "projects/{{projectID}}/uptimeCheckConfigs/{{uptimeCheckConfigID}}".
	// The check must exist; this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *monitoringv1beta1.MonitoringUptimeCheckConfigRef `json:"resourceRef"`

	// User-supplied key/value data to be used for organizing and
	//  identifying the `UptimeCheckConfig` objects.
	//
	//  The field can contain up to 64 entries. Each key and value is limited to
	//  63 Unicode characters or 128 bytes, whichever is smaller. Labels and
	//  values can contain only lowercase letters, numerals, underscores, and
	//  dashes. Keys must begin with a letter.
	// +optional
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.user_labels
	UserLabels map[string]string `json:"userLabels,omitempty"`

	// Fields of the HTTP check that Config Connector's
	// MonitoringUptimeCheckConfig sets (its spec.httpCheck): the check must
	// be an HTTP check.
	// +optional
	HTTPCheck *UptimeCheckConfig_HTTPCheckFields `json:"httpCheck,omitempty"`
}

// UptimeCheckConfig_HTTPCheckFields are fields of an HTTP check that
// Config Connector's MonitoringUptimeCheckConfig has no field for.
// +kcc:proto=google.monitoring.v3.UptimeCheckConfig.HttpCheck
// +kubebuilder:validation:XValidation:rule="has(self.customContentType) || has(self.acceptedResponseStatusCodes) || has(self.pingConfig) || has(self.serviceAgentAuthentication)",message="set at least one field of httpCheck"
type UptimeCheckConfig_HTTPCheckFields struct {
	// A user provided content type header to use for the check. The invalid
	//  configurations outlined in the `content_type` field apply to
	//  `custom_content_type`, as well as the following:
	//  1. `content_type` is `URL_ENCODED` and `custom_content_type` is set.
	//  2. `content_type` is `USER_PROVIDED` and `custom_content_type` is not
	//  set.
	// +optional
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.HttpCheck.custom_content_type
	CustomContentType *string `json:"customContentType,omitempty"`

	// If present, the check will only pass if the HTTP response status code is
	//  in this set of status codes. If empty, the HTTP status code will only
	//  pass if the HTTP status code is 200-299.
	// +optional
	// +listType=atomic
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.HttpCheck.accepted_response_status_codes
	AcceptedResponseStatusCodes []UptimeCheckConfig_HTTPCheck_ResponseStatusCode `json:"acceptedResponseStatusCodes,omitempty"`

	// Contains information needed to add pings to an HTTP check.
	// +optional
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.HttpCheck.ping_config
	PingConfig *UptimeCheckConfig_PingConfig `json:"pingConfig,omitempty"`

	// If specified, Uptime will generate and attach an OIDC JWT token for the
	//  Monitoring service agent service account as an `Authorization` header
	//  in the HTTP request when probing.
	// +optional
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.HttpCheck.service_agent_authentication
	ServiceAgentAuthentication *UptimeCheckConfig_HTTPCheck_ServiceAgentAuthentication `json:"serviceAgentAuthentication,omitempty"`
}

// +kcc:proto=google.monitoring.v3.UptimeCheckConfig.HttpCheck.ResponseStatusCode
// +kubebuilder:validation:XValidation:rule="has(self.statusValue) != has(self.statusClass)",message="set one of statusValue and statusClass"
type UptimeCheckConfig_HTTPCheck_ResponseStatusCode struct {
	// A status code to accept.
	// +optional
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.HttpCheck.ResponseStatusCode.status_value
	StatusValue *int32 `json:"statusValue,omitempty"`

	// A class of status codes to accept: STATUS_CLASS_1XX, STATUS_CLASS_2XX,
	// STATUS_CLASS_3XX, STATUS_CLASS_4XX, STATUS_CLASS_5XX or
	// STATUS_CLASS_ANY.
	// +optional
	// +kubebuilder:validation:Enum=STATUS_CLASS_1XX;STATUS_CLASS_2XX;STATUS_CLASS_3XX;STATUS_CLASS_4XX;STATUS_CLASS_5XX;STATUS_CLASS_ANY
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.HttpCheck.ResponseStatusCode.status_class
	StatusClass *string `json:"statusClass,omitempty"`
}

// +kcc:proto=google.monitoring.v3.UptimeCheckConfig.HttpCheck.ServiceAgentAuthentication
type UptimeCheckConfig_HTTPCheck_ServiceAgentAuthentication struct {
	// Type of authentication: OIDC_TOKEN.
	// +required
	// +kubebuilder:validation:Enum=OIDC_TOKEN
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.HttpCheck.ServiceAgentAuthentication.type
	Type *string `json:"type"`
}

// +kcc:proto=google.monitoring.v3.UptimeCheckConfig.PingConfig
type UptimeCheckConfig_PingConfig struct {
	// Number of ICMP pings. A maximum of 3 ICMP pings is currently supported.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	// +kcc:proto:field=google.monitoring.v3.UptimeCheckConfig.PingConfig.pings_count
	PingsCount *int32 `json:"pingsCount"`
}

// MonitoringUptimeCheckConfigFieldsStatus defines the observed state.
type MonitoringUptimeCheckConfigFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The uptime check whose fields this object sets, as
	// projects/{project}/uptimeCheckConfigs/{uptimeCheckConfig}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflmonitoringuptimecheckconfigfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// MonitoringUptimeCheckConfigFields sets fields of a Cloud Monitoring
// uptime check that Config Connector's MonitoringUptimeCheckConfig has no
// field for (userLabels, and an HTTP check's
// customContentType, acceptedResponseStatusCodes, pingConfig and
// serviceAgentAuthentication), and only those: it patches the check with
// an update mask naming them. Deleting it clears them.
// +k8s:openapi-gen=true
type MonitoringUptimeCheckConfigFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   MonitoringUptimeCheckConfigFieldsSpec   `json:"spec,omitempty"`
	Status MonitoringUptimeCheckConfigFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// MonitoringUptimeCheckConfigFieldsList contains a list of MonitoringUptimeCheckConfigFields
type MonitoringUptimeCheckConfigFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MonitoringUptimeCheckConfigFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MonitoringUptimeCheckConfigFields{}, &MonitoringUptimeCheckConfigFieldsList{})
}
