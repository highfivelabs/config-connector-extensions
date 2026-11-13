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

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector pull request #14133 (head dcb10c53, open, unmerged),
// apis/iam/v1alpha1/iampolicybinding_types.go (UPSTREAM.md). Changed: the
// parent is one of organizationRef, folderRef and projectRef (decision 18,
// as Terraform's google_iam_{organizations,folders,projects}_policy_binding),
// not an organization only; the policy is policyRef, which takes our
// IAMPrincipalAccessBoundaryPolicy by name or a policy's name as external;
// etag is in status.observedState, not the spec; location defaults to
// global; the parent, location, resourceID, target, policyKind and policy
// are immutable (CEL, decision 20); group, labels and short names are ours.

// API sources for IAMPolicyBinding, recorded by generate-types:
// +kcc:source:proto=https://github.com/googleapis/googleapis/blob/e7513c828b8db536fa12fc5493d541f323aedf90/google/iam/v3/policy_binding_resources.proto
// +kcc:source:service-docs=https://cloud.google.com/iam/
// +kcc:source:resource-docs=https://docs.cloud.google.com/iam/docs/reference/rest/v3/projects.locations.policyBindings

package v1alpha1

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var IAMPolicyBindingGVK = GroupVersion.WithKind("IAMPolicyBinding")

// IAMPolicyBindingSpec defines the desired state of IAMPolicyBinding
// +kcc:spec:proto=google.iam.v3.PolicyBinding
// +kubebuilder:validation:XValidation:rule="(has(self.organizationRef) ? 1 : 0) + (has(self.folderRef) ? 1 : 0) + (has(self.projectRef) ? 1 : 0) == 1",message="exactly one of organizationRef, folderRef and projectRef is required"
type IAMPolicyBindingSpec struct {
	// The organization the binding belongs to: exactly one of
	// organizationRef, folderRef and projectRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="organizationRef is immutable"
	OrganizationRef *refsv1beta1.OrganizationRef `json:"organizationRef,omitempty"`

	// The folder the binding belongs to: exactly one of organizationRef,
	// folderRef and projectRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="folderRef is immutable"
	FolderRef *refsv1beta1.FolderRefDeprecated `json:"folderRef,omitempty"`

	// The project the binding belongs to: exactly one of organizationRef,
	// folderRef and projectRef. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef,omitempty"`

	// The location of the binding (global, the default). Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location *string `json:"location,omitempty"`

	// The IAMPolicyBinding name. If not given, the metadata.name will be used. Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	// Optional. The description of the policy binding. Must be less than or equal
	//  to 63 characters.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.display_name
	DisplayName *string `json:"displayName,omitempty"`

	// Optional. User-defined annotations. See
	//  https://google.aip.dev/148#annotations for more details such as format and
	//  size limitations
	// +kcc:proto:field=google.iam.v3.PolicyBinding.annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// Required. Immutable. The full resource name of the resource to which the
	//  policy will be bound. Immutable once set.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.target
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="target is immutable"
	Target *PolicyBinding_Target `json:"target"`

	// Immutable. The kind of the policy to attach in this binding:
	//  PRINCIPAL_ACCESS_BOUNDARY or ACCESS. Left out, the API sets it to the
	//  policy's kind.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.policy_kind
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="policyKind is immutable"
	PolicyKind *string `json:"policyKind,omitempty"`

	// Required. Immutable. The policy to be bound: an IAMPrincipalAccessBoundaryPolicy
	//  (iam.gcp.hfl.dev) by name, or a policy's resource name as external
	//  (organizations/{organization}/locations/global/principalAccessBoundaryPolicies/{id}).
	//  The binding parent and the policy must belong to the same organization.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.policy
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="policyRef is immutable"
	PolicyRef *PolicyBindingPolicyRef `json:"policyRef"`

	// Optional. The condition to apply to the policy binding. When set, the
	//  `expression` field in the `Expr` must include from 1 to 10 subexpressions,
	//  joined by the
	//  "||"(Logical OR), "&&"(Logical AND) or "!"(Logical NOT) operators and
	//  cannot contain more than 250 characters.
	//
	//  The condition is currently only supported when bound to policies of kind
	//  principal access boundary.
	//
	//  When the bound policy is a principal access boundary policy, the only
	//  supported attributes in any subexpression are `principal.type` and
	//  `principal.subject`. An example expression is: "principal.type ==
	//  'iam.googleapis.com/ServiceAccount'" or "principal.subject ==
	//  'bob@example.com'".
	// +kcc:proto:field=google.iam.v3.PolicyBinding.condition
	Condition *PolicyBindingExpr `json:"condition,omitempty"`
}

// PolicyBindingPolicyRef is the policy a binding binds.
// +kubebuilder:validation:XValidation:rule="has(self.external) != has(self.name)",message="exactly one of external and name is required"
type PolicyBindingPolicyRef struct {
	// A policy's resource name, organizations/{organization}/locations/global/principalAccessBoundaryPolicies/{id}.
	External string `json:"external,omitempty"`

	// The name of an IAMPrincipalAccessBoundaryPolicy (iam.gcp.hfl.dev).
	Name string `json:"name,omitempty"`

	// The namespace of the IAMPrincipalAccessBoundaryPolicy; the binding's when left out.
	Namespace string `json:"namespace,omitempty"`
}

// +kcc:proto=google.iam.v3.PolicyBinding.Target
type PolicyBinding_Target struct {
	// Immutable. The full resource name that's used for principal access
	//  boundary policy bindings. The principal set must be directly parented
	//  by the policy binding's parent or same as the parent if the target is a
	//  project/folder/organization. Examples:
	//  //cloudresourcemanager.googleapis.com/projects/PROJECT_NUMBER,
	//  //cloudresourcemanager.googleapis.com/folders/FOLDER_ID,
	//  //cloudresourcemanager.googleapis.com/organizations/ORGANIZATION_ID,
	//  //iam.googleapis.com/locations/global/workforcePools/POOL_ID.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.Target.principal_set
	PrincipalSet *string `json:"principalSet,omitempty"`
}

// +kcc:proto=google.type.Expr
type PolicyBindingExpr struct {
	// Textual representation of an expression in Common Expression Language
	//  syntax.
	// +kcc:proto:field=google.type.Expr.expression
	Expression *string `json:"expression,omitempty"`

	// Optional. Title for the expression.
	// +kcc:proto:field=google.type.Expr.title
	Title *string `json:"title,omitempty"`

	// Optional. Description of the expression.
	// +kcc:proto:field=google.type.Expr.description
	Description *string `json:"description,omitempty"`

	// Optional. String indicating the location of the expression for error
	//  reporting, e.g. a file name and a position in the file.
	// +kcc:proto:field=google.type.Expr.location
	Location *string `json:"location,omitempty"`
}

// IAMPolicyBindingStatus defines the config connector machine state of IAMPolicyBinding
type IAMPolicyBindingStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the IAMPolicyBinding resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *IAMPolicyBindingObservedState `json:"observedState,omitempty"`
}

// IAMPolicyBindingObservedState is the state of the IAMPolicyBinding resource as most recently observed in GCP.
// +kcc:observedstate:proto=google.iam.v3.PolicyBinding
type IAMPolicyBindingObservedState struct {
	// Output only. The globally unique ID of the policy binding. Assigned when
	//  the policy binding is created.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.uid
	Uid *string `json:"uid,omitempty"`

	// The etag of the policy binding.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.etag
	Etag *string `json:"etag,omitempty"`

	// The kind of the bound policy, as the API set it.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.policy_kind
	PolicyKind *string `json:"policyKind,omitempty"`

	// Output only. The globally unique ID of the policy to be bound.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.policy_uid
	PolicyUid *string `json:"policyUid,omitempty"`

	// Output only. The time when the policy binding was created.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.create_time
	CreateTime *string `json:"createTime,omitempty"`

	// Output only. The time when the policy binding was most recently updated.
	// +kcc:proto:field=google.iam.v3.PolicyBinding.update_time
	UpdateTime *string `json:"updateTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfliampolicybinding;hfliampolicybindings
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// IAMPolicyBinding is the Schema for the IAMPolicyBinding API: an IAM v3
// policy binding (a principal access boundary policy bound to the
// principals of an organization, folder or project).
// +k8s:openapi-gen=true
type IAMPolicyBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   IAMPolicyBindingSpec   `json:"spec,omitempty"`
	Status IAMPolicyBindingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// IAMPolicyBindingList contains a list of IAMPolicyBinding
type IAMPolicyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMPolicyBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMPolicyBinding{}, &IAMPolicyBindingList{})
}
