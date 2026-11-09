// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here: what the IAM kinds that bind members to roles share. The
// resourceRef is shaped like Config Connector's IAMPolicyMember's, so a
// manifest moves between them with the kind and the members changed.

// IAMResourceReference is the resource whose IAM policy is changed.
// +kubebuilder:validation:XValidation:rule="has(self.name) != has(self.external)",message="exactly one of name and external is required"
type IAMResourceReference struct {
	// The Config Connector kind of the resource, for example Project,
	// Folder, StorageBucket or SecretManagerSecret. The kinds this version
	// accepts are listed in docs/design.md, section 6.5 (IAMBinding).
	Kind string `json:"kind"`

	// The API version of the referenced object, if the kind exists in more
	// than one group (a kind this project provides and Config Connector's).
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// The name of the Kubernetes object that manages the resource. Exactly
	// one of name and external is required.
	// +optional
	Name string `json:"name,omitempty"`

	// The namespace of that object; the binding's own if unset.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// The resource, when it is not managed by a Kubernetes object: its
	// full name (projects/{project}/locations/{location}/secrets/{secret}),
	// or for a project, folder, organization, bucket or service account its
	// ID, number, name or email.
	// +optional
	External string `json:"external,omitempty"`
}

// IAMCondition limits when a binding applies.
type IAMCondition struct {
	// The title of the condition.
	Title string `json:"title"`
	// The description of the condition.
	// +optional
	Description string `json:"description,omitempty"`
	// The Common Expression Language expression, for example
	// request.time < timestamp('2030-01-01T00:00:00Z').
	Expression string `json:"expression"`
}
