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
// apis/accesscontextmanager/v1alpha1/accesscontextmanagergcpuseraccessbinding_types.go
// (UPSTREAM.md) as a stable kind (v1beta1, docs/design.md decision 26).
// Changed: accessLevels is optional (the API's v1 says so; a binding may
// set only scoped access settings) and holds at most one level, as the
// API takes; groupKey and organizationRef are immutable by CEL, and
// resourceID once set; status.externalRef. Written here, the fields
// Config Connector's alpha kind and its pinned Go client lack, with the
// API's names (the v1 REST Discovery document; Terraform's
// google_access_context_manager_gcp_user_access_binding has the same):
// dryRunAccessLevels, sessionSettings, restrictedClientApplications and
// scopedAccessSettings. Group, labels and short names are ours.

package v1beta1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var AccessContextManagerGCPUserAccessBindingGVK = GroupVersion.WithKind("AccessContextManagerGCPUserAccessBinding")

type OrganizationRef struct {
	/* The organization, as organizations/{id} or {id}. */
	// +required
	External string `json:"external"`
}

// +kubebuilder:validation:XValidation:rule="!has(oldSelf.resourceID) || (has(self.resourceID) && self.resourceID == oldSelf.resourceID)",message="resourceID is immutable once set"
// +kubebuilder:validation:XValidation:rule="!has(self.restrictedClientApplications) || !has(self.scopedAccessSettings)",message="restrictedClientApplications and scopedAccessSettings cannot both be set"
type AccessContextManagerGCPUserAccessBindingSpec struct {
	/* Access level that a user must have to be granted access. Only one access level is supported, not multiple. Example: "accessPolicies/9522/accessLevels/device_trusted". */
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=1
	AccessLevels []string `json:"accessLevels,omitempty"`

	/* Dry run access level that is evaluated but not enforced; a denial it would make is logged. Only one access level is supported. Example: "accessPolicies/9522/accessLevels/device_trusted". */
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=1
	DryRunAccessLevels []string `json:"dryRunAccessLevels,omitempty"`

	/* Immutable. Google Group id whose members are subject to this binding's restrictions. See "id" in the G Suite Directory API's Groups resource. If a group's email address/alias is changed, this resource will continue to point at the changed group. This field does not accept group email addresses or aliases. Example: "01d520gv4vjcrht". */
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="groupKey is immutable"
	GroupKey string `json:"groupKey"`

	/* Immutable. The organization that this resource belongs to. */
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="organizationRef is immutable"
	OrganizationRef *OrganizationRef `json:"organizationRef"`

	/* Immutable. Optional. The service-generated name of the resource (the last part of its name). Used for acquisition only. Leave unset to create a new resource; it is set after the create. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`

	/* A list of applications that are subject to this binding's restrictions. If the list is empty, the binding restrictions apply to all applications. */
	// +optional
	// +listType=atomic
	RestrictedClientApplications []GCPUserAccessBindingApplication `json:"restrictedClientApplications,omitempty"`

	/* A list of scoped access settings that set this binding's restrictions on a subset of applications. It cannot be set together with restrictedClientApplications. */
	// +optional
	// +listType=atomic
	ScopedAccessSettings []GCPUserAccessBindingScopedAccessSettings `json:"scopedAccessSettings,omitempty"`

	/* The Google Cloud session length (GCSL) policy for the group key. */
	// +optional
	SessionSettings *GCPUserAccessBindingSessionSettings `json:"sessionSettings,omitempty"`
}

// GCPUserAccessBindingApplication is an application a binding restricts.
type GCPUserAccessBindingApplication struct {
	/* The OAuth client ID of the application. */
	// +optional
	ClientID *string `json:"clientId,omitempty"`

	/* The name of the application. Example: "Cloud Console". */
	// +optional
	Name *string `json:"name,omitempty"`
}

// GCPUserAccessBindingScopedAccessSettings are the access settings of one
// scope (an application).
type GCPUserAccessBindingScopedAccessSettings struct {
	/* Access settings for this scope. May be empty if dryRunSettings is set. */
	// +optional
	ActiveSettings *GCPUserAccessBindingAccessSettings `json:"activeSettings,omitempty"`

	/* Dry-run access settings for this scope. May be empty if activeSettings is set. */
	// +optional
	DryRunSettings *GCPUserAccessBindingAccessSettings `json:"dryRunSettings,omitempty"`

	/* The application the settings apply to: the scoped access settings' key, unique and non-empty. */
	// +required
	Scope *GCPUserAccessBindingAccessScope `json:"scope"`
}

// GCPUserAccessBindingAccessScope is what scoped access settings apply to.
type GCPUserAccessBindingAccessScope struct {
	/* Client scope for this access scope. */
	// +optional
	ClientScope *GCPUserAccessBindingClientScope `json:"clientScope,omitempty"`
}

// GCPUserAccessBindingClientScope is an application scope.
type GCPUserAccessBindingClientScope struct {
	/* The application that is subject to this binding's scope. */
	// +optional
	RestrictedClientApplication *GCPUserAccessBindingApplication `json:"restrictedClientApplication,omitempty"`
}

// GCPUserAccessBindingAccessSettings are a scope's access level and
// session settings.
type GCPUserAccessBindingAccessSettings struct {
	/* Access level that a user must have to be granted access. Only one access level is supported. */
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=1
	AccessLevels []string `json:"accessLevels,omitempty"`

	/* Session settings applied to user access on the scope. */
	// +optional
	SessionSettings *GCPUserAccessBindingSessionSettings `json:"sessionSettings,omitempty"`
}

// GCPUserAccessBindingSessionSettings is a Google Cloud session length
// policy.
type GCPUserAccessBindingSessionSettings struct {
	/* How long a user is allowed to take between actions before a new access token must be issued, as a duration ("3600s"). Only set for Google Cloud apps. */
	// +optional
	MaxInactivity *string `json:"maxInactivity,omitempty"`

	/* The session length, as a duration ("43200s"). Zero disables the session; sessionLengthEnabled false makes it infinite. */
	// +optional
	SessionLength *string `json:"sessionLength,omitempty"`

	/* Enables Google Cloud session length. When false, the other fields are disregarded and the session length is infinite. */
	// +optional
	SessionLengthEnabled *bool `json:"sessionLengthEnabled,omitempty"`

	/* How the user signs in again when the session is up: LOGIN (the default), SECURITY_KEY or PASSWORD. */
	// +optional
	// +kubebuilder:validation:Enum=SESSION_REAUTH_METHOD_UNSPECIFIED;LOGIN;SECURITY_KEY;PASSWORD
	SessionReauthMethod *string `json:"sessionReauthMethod,omitempty"`

	/* Only for OIDC apps: when true, the re-auth period is the minimum of sessionLength and the OIDC max_age parameter. */
	// +optional
	UseOidcMaxAge *bool `json:"useOidcMaxAge,omitempty"`
}

type AccessContextManagerGCPUserAccessBindingStatus struct {
	/* Conditions represent the latest available observation of the resource's current state. */
	// +optional
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* Immutable. Assigned by the server during creation. The last segment has an arbitrary length and has only URI unreserved characters (as defined by RFC 3986 Section 2.3). Should not be specified by the client during creation. Example: "organizations/256/gcpUserAccessBindings/b3-BhcX_Ud5N". */
	// +optional
	Name *string `json:"name,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	/* The binding's name, as status.name; a delete uses it and resolves nothing. */
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflaccesscontextmanagergcpuseraccessbinding;hflaccesscontextmanagergcpuseraccessbindings
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// AccessContextManagerGCPUserAccessBinding binds a Google Group's members
// to an organization's access levels and session policy (context-aware
// access to Google Cloud).
// +k8s:openapi-gen=true
type AccessContextManagerGCPUserAccessBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   AccessContextManagerGCPUserAccessBindingSpec   `json:"spec,omitempty"`
	Status AccessContextManagerGCPUserAccessBindingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// AccessContextManagerGCPUserAccessBindingList contains a list of AccessContextManagerGCPUserAccessBinding
type AccessContextManagerGCPUserAccessBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessContextManagerGCPUserAccessBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccessContextManagerGCPUserAccessBinding{}, &AccessContextManagerGCPUserAccessBindingList{})
}
