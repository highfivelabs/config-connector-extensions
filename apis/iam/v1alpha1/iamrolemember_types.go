// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here. One member of one role on one resource, additive: it adds
// the member and leaves every other member of the role alone, and delete
// removes only that member. It is Terraform's google_*_iam_member and Config
// Connector's IAMPolicyMember for the resource types IAMPolicyMember cannot
// reference (docs/design.md 6.5, rank 2); IAMRoleBinding is its
// role-authoritative sibling. The name is the working one, not yet
// confirmed by the builder (docs/design.md section 10, question 3): the
// kind is named only by IAMRoleMemberGVK.

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var IAMRoleMemberGVK = GroupVersion.WithKind("IAMRoleMember")

// IAMRoleMemberSpec is one member of one role on one resource.
// +kubebuilder:validation:XValidation:rule="has(self.member) != has(self.memberFrom)",message="exactly one of member and memberFrom is required"
type IAMRoleMemberSpec struct {
	// The resource whose IAM policy holds the member. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef IAMResourceReference `json:"resourceRef"`

	// The role. Immutable.
	// +kubebuilder:validation:Pattern=`^((projects|organizations)/[^/]+/)?roles/[\w_\.]+$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="role is immutable"
	Role string `json:"role"`

	// The condition under which the member has the role. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="condition is immutable"
	// +optional
	Condition *IAMCondition `json:"condition,omitempty"`

	// The IAM identity, as the API writes it (user:ada@example.com,
	// group:team@example.com, serviceAccount:app@project.iam.gserviceaccount.com,
	// domain:example.com, allAuthenticatedUsers, ...). Immutable. Exactly
	// one of member and memberFrom is required.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="member is immutable"
	// +optional
	Member *string `json:"member,omitempty"`

	// The identity of a Config Connector object. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="memberFrom is immutable"
	// +optional
	MemberFrom *IAMRoleBindingMemberFrom `json:"memberFrom,omitempty"`
}

// IAMRoleMemberStatus defines the config connector machine state of IAMRoleMember
type IAMRoleMemberStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The member, as {resource}#{role}#{member}, or
	// {resource}#{role}#{member}#{condition title} with a condition, where
	// the resource is its full name. The API has no resource name for a
	// member; this one is ours.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfliamrolemember;hfliamrolemembers
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// IAMRoleMember is the Schema for the IAMRoleMember API
// +k8s:openapi-gen=true
type IAMRoleMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   IAMRoleMemberSpec   `json:"spec,omitempty"`
	Status IAMRoleMemberStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// IAMRoleMemberList contains a list of IAMRoleMember
type IAMRoleMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMRoleMember `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMRoleMember{}, &IAMRoleMemberList{})
}
