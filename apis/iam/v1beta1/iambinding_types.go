// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// Written here. One role on one resource, with its members, authoritative
// for that role: members added out of band are removed. It is Terraform's
// google_*_iam_binding; Config Connector has no kind for it (IAMPolicyMember
// is additive, IAMPartialPolicy keeps what it lists, IAMPolicy owns the
// whole policy). Config Connector's open pull request #14133 adds an
// IAMPolicyBinding that is IAM v3's principal access boundary
// PolicyBinding, a different resource, which is why this kind is not named
// that (docs/design.md section 10, question 1).
//
// v1beta1 since 2026-11-10 (docs/design.md decision 24): the kind moved
// from v1alpha1 unchanged, and v1alpha1 is no longer served. Nothing had
// been released at v1alpha1.

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/iamrefs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var IAMBindingGVK = GroupVersion.WithKind("IAMBinding")

// IAMBindingSpec is the members of one role on one resource.
// +kubebuilder:validation:XValidation:rule="(has(self.members) && size(self.members) > 0) || (has(self.membersFrom) && size(self.membersFrom) > 0)",message="at least one of members and membersFrom is required"
type IAMBindingSpec struct {
	// The resource whose IAM policy holds the binding. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef IAMResourceReference `json:"resourceRef"`

	// The role. Immutable.
	// +kubebuilder:validation:Pattern=`^((projects|organizations)/[^/]+/)?roles/[\w_\.]+$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="role is immutable"
	Role string `json:"role"`

	// The condition under which the binding applies. Immutable. The role
	// with this condition is the binding this kind owns; the same role
	// without it, or with another condition, is left alone.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="condition is immutable"
	// +optional
	Condition *IAMCondition `json:"condition,omitempty"`

	// The IAM identities that have the role, as the API writes them
	// (user:ada@example.com, group:team@example.com,
	// serviceAccount:app@project.iam.gserviceaccount.com, domain:example.com,
	// allAuthenticatedUsers, ...). Any other member of this role and
	// condition on the resource is removed.
	// +optional
	Members []string `json:"members,omitempty"`

	// Members that are the identity of a Config Connector object, added to
	// members.
	// +optional
	MembersFrom []IAMBindingMemberFrom `json:"membersFrom,omitempty"`
}

// IAMBindingMemberFrom is a member taken from an object.
type IAMBindingMemberFrom struct {
	// The IAMServiceAccount (or an external service account email) to
	// bind: serviceAccount:{its email}.
	ServiceAccountRef iamrefs.IAMServiceAccountRef `json:"serviceAccountRef"`
}

// IAMBindingStatus defines the config connector machine state of IAMBinding
type IAMBindingStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The binding, as {resource}#{role}, or {resource}#{role}#{condition
	// title} with a condition, where the resource is its full name. The
	// API has no resource name for a binding; this one is ours.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The members the binding had when it was last applied, sorted.
	AppliedMembers []string `json:"appliedMembers,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfliambinding;hfliambindings
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// IAMBinding is the Schema for the IAMBinding API
// +k8s:openapi-gen=true
type IAMBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   IAMBindingSpec   `json:"spec,omitempty"`
	Status IAMBindingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// IAMBindingList contains a list of IAMBinding
type IAMBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMBinding{}, &IAMBindingList{})
}
