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

// Written here: a Cloud Router route policy is in no Config Connector code
// or pull request. The shape follows the Compute Engine API's RoutePolicy
// (and Terraform's google_compute_router_route_policy), in the layout of
// Config Connector's direct kinds.

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRouterRoutePolicyGVK = GroupVersion.WithKind("ComputeRouterRoutePolicy")

// ComputeRouterRoutePolicySpec is a route policy on a Cloud Router: terms,
// in priority order, that match BGP routes and act on them.
type ComputeRouterRoutePolicySpec struct {
	// The project that this resource belongs to.
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// The router's region. Immutable.
	Region string `json:"region"`

	// The Cloud Router the policy is on. Immutable.
	// +required
	RouterRef *ComputeRouterRef `json:"routerRef"`

	// The policy's name. If not given, the metadata.name will be used.
	//  Immutable.
	ResourceID *string `json:"resourceID,omitempty"`

	// The policy's type: ROUTE_POLICY_TYPE_IMPORT or ROUTE_POLICY_TYPE_EXPORT.
	Type *string `json:"type,omitempty"`

	// An optional description of the policy.
	Description *string `json:"description,omitempty"`

	// The policy's terms, applied in priority order.
	// +required
	Terms []RoutePolicyTerm `json:"terms"`
}

// RoutePolicyTerm is one term of a route policy.
type RoutePolicyTerm struct {
	// The evaluation order of this term; lower first. 0 to 2^31-1, unique
	//  within the policy.
	Priority int32 `json:"priority"`

	// The CEL expression a route must match for the actions to apply.
	// +required
	Match *RoutePolicyExpr `json:"match"`

	// CEL expressions run, in order, on a route that matches.
	Actions []RoutePolicyExpr `json:"actions,omitempty"`
}

// RoutePolicyExpr is a Common Expression Language expression.
type RoutePolicyExpr struct {
	// The expression.
	// +required
	Expression string `json:"expression"`

	// A title for the expression.
	Title *string `json:"title,omitempty"`

	// A description of the expression.
	Description *string `json:"description,omitempty"`

	// Where the expression came from, for reporting.
	Location *string `json:"location,omitempty"`
}

// ComputeRouterRoutePolicyStatus defines the observed state.
type ComputeRouterRoutePolicyStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The policy, as projects/{project}/regions/{region}/routers/{router}/routePolicies/{name}
	//  (not an API resource name: a route policy has none).
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the policy as most recently observed in GCP.
	ObservedState *ComputeRouterRoutePolicyObservedState `json:"observedState,omitempty"`
}

type ComputeRouterRoutePolicyObservedState struct {
	// The policy's fingerprint, which changes with each update.
	Fingerprint *string `json:"fingerprint,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputerouterroutepolicy;hflcomputerouterroutepolicies
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRouterRoutePolicy is a route policy on a Cloud Router, which
// filters or changes the BGP routes it imports or exports.
// +k8s:openapi-gen=true
type ComputeRouterRoutePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRouterRoutePolicySpec   `json:"spec,omitempty"`
	Status ComputeRouterRoutePolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeRouterRoutePolicyList contains a list of ComputeRouterRoutePolicy
type ComputeRouterRoutePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRouterRoutePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRouterRoutePolicy{}, &ComputeRouterRoutePolicyList{})
}
