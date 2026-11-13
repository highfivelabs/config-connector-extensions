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

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var EndpointsServiceGVK = GroupVersion.WithKind("EndpointsService")

// EndpointsServiceSpec is a Cloud Endpoints service: a Service Management
// managed service, its configuration and the rollout that serves it, as
// Terraform's google_endpoints_service makes them.
// +kubebuilder:validation:XValidation:rule="has(self.openapiConfig) != has(self.grpcConfig)",message="exactly one of openapiConfig and grpcConfig is required"
// +kubebuilder:validation:XValidation:rule="has(self.grpcConfig) == has(self.protocOutputBase64)",message="grpcConfig and protocOutputBase64 go together"
type EndpointsServiceSpec struct {
	// The service's name, a DNS name such as
	//  my-api.endpoints.my-project.cloud.goog. If not given, metadata.name
	//  is used. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// The project that produces the service. Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The service's OpenAPI document (YAML or JSON). Its host must be the
	//  service's name. A change submits a new configuration and rolls it out.
	// +optional
	OpenAPIConfig *string `json:"openapiConfig,omitempty"`

	// The service's gRPC service configuration (YAML). Goes with
	//  protocOutputBase64. A change submits a new configuration and rolls it out.
	// +optional
	GRPCConfig *string `json:"grpcConfig,omitempty"`

	// The protoc output (a FileDescriptorSet, protoc
	//  --include_imports --descriptor_set_out), base64-encoded. Goes with
	//  grpcConfig.
	// +optional
	ProtocOutputBase64 *string `json:"protocOutputBase64,omitempty"`
}

// EndpointsServiceStatus defines the config connector machine state of EndpointsService
type EndpointsServiceStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The service's name, services/{serviceName}.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The SHA-256 of the configuration files last submitted and rolled
	//  out, so a spec change is noticed.
	ConfigDigest *string `json:"configDigest,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *EndpointsServiceObservedState `json:"observedState,omitempty"`
}

// EndpointsServiceObservedState is the service's active configuration, as
// Terraform's computed attributes read it.
type EndpointsServiceObservedState struct {
	// The ID of the configuration the service serves (2026-11-13r0).
	ConfigID *string `json:"configID,omitempty"`

	// The address at which the service can be found: the configuration's
	//  name.
	DNSAddress *string `json:"dnsAddress,omitempty"`

	// The APIs the configuration serves.
	APIs []EndpointsServiceAPI `json:"apis,omitempty"`

	// The network endpoints the configuration declares.
	Endpoints []EndpointsServiceEndpoint `json:"endpoints,omitempty"`
}

// EndpointsServiceAPI is an API the service serves.
type EndpointsServiceAPI struct {
	// The API's fully qualified name.
	Name *string `json:"name,omitempty"`
	// SYNTAX_PROTO2 or SYNTAX_PROTO3.
	Syntax *string `json:"syntax,omitempty"`
	// The API's version.
	Version *string `json:"version,omitempty"`
	// The API's methods.
	Methods []EndpointsServiceMethod `json:"methods,omitempty"`
}

// EndpointsServiceMethod is a method of an API.
type EndpointsServiceMethod struct {
	// The method's simple name.
	Name *string `json:"name,omitempty"`
	// SYNTAX_PROTO2 or SYNTAX_PROTO3.
	Syntax *string `json:"syntax,omitempty"`
	// The request's type URL.
	RequestType *string `json:"requestType,omitempty"`
	// The response's type URL.
	ResponseType *string `json:"responseType,omitempty"`
}

// EndpointsServiceEndpoint is a network endpoint of the service.
type EndpointsServiceEndpoint struct {
	// The endpoint's canonical name.
	Name *string `json:"name,omitempty"`
	// The endpoint's target, an IP address or a host name.
	Address *string `json:"address,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflendpointsservice;hflendpointsservices
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// EndpointsService is the Schema for the EndpointsService API
// +k8s:openapi-gen=true
type EndpointsService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   EndpointsServiceSpec   `json:"spec,omitempty"`
	Status EndpointsServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// EndpointsServiceList contains a list of EndpointsService
type EndpointsServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EndpointsService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EndpointsService{}, &EndpointsServiceList{})
}
