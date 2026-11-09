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

// Written here after Terraform's google_project_default_service_accounts
// (read, not copied): what a project does to its default service
// accounts, the Compute Engine and App Engine ones Google creates when
// those APIs are enabled, which hold the Editor role by default. Config
// Connector has no kind or pull request for it. There is no proto message
// for it, so hack/scaffold-kind.sh does not apply.

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ProjectDefaultServiceAccountsGVK = GroupVersion.WithKind("ProjectDefaultServiceAccounts")

// ProjectDefaultServiceAccountsSpec says what to do to a project's default
// service accounts.
type ProjectDefaultServiceAccountsSpec struct {
	// The project whose default service accounts these are. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef"`

	// What to do to the default service accounts, the ones whose display
	// name is "Compute Engine default service account" or "App Engine
	// default service account": DELETE deletes them, DISABLE disables them,
	// DEPRIVILEGE removes them from every role binding in the project's IAM
	// policy. Kept true on every reconcile. Immutable.
	// +kubebuilder:validation:Enum=DELETE;DISABLE;DEPRIVILEGE
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="action is immutable"
	Action string `json:"action"`

	// What deleting this object does to the accounts it acted on: REVERT
	// (the default) undeletes or enables them and fails if it cannot;
	// REVERT_AND_IGNORE_FAILURE tries the same and deletes the object
	// anyway; NONE leaves them. DEPRIVILEGE is never reverted. Abandoning
	// the object (cnrm.cloud.google.com/deletion-policy: abandon) leaves
	// them too.
	// +kubebuilder:validation:Enum=NONE;REVERT;REVERT_AND_IGNORE_FAILURE
	// +kubebuilder:default=REVERT
	// +optional
	RestorePolicy *string `json:"restorePolicy,omitempty"`
}

// ProjectDefaultServiceAccountsStatus defines the config connector machine state of ProjectDefaultServiceAccounts
type ProjectDefaultServiceAccountsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The setting, as projects/{project}/defaultServiceAccounts. The API has
	//  no resource name for it; this one is ours.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The default service accounts acted on, unique ID to email: the ones
	//  deleting this object reverts (a deleted account can be undeleted only
	//  by its unique ID). Entries are added, never removed.
	ServiceAccounts map[string]string `json:"serviceAccounts,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflprojectdefaultserviceaccounts
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ProjectDefaultServiceAccounts is the Schema for the ProjectDefaultServiceAccounts API
// +k8s:openapi-gen=true
type ProjectDefaultServiceAccounts struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ProjectDefaultServiceAccountsSpec   `json:"spec,omitempty"`
	Status ProjectDefaultServiceAccountsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ProjectDefaultServiceAccountsList contains a list of ProjectDefaultServiceAccounts
type ProjectDefaultServiceAccountsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectDefaultServiceAccounts `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProjectDefaultServiceAccounts{}, &ProjectDefaultServiceAccountsList{})
}
