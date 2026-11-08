// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for gaps-gcp. Copied from Config Connector
// pkg/clients/generated/apis/sql/v1beta1/sqluser_types.go (the Go types of
// the Terraform-based SQLUser CRD, UPSTREAM.md), changing as little as
// possible:
//   - group sql.gcp.hfl.dev (groupversion_info.go), own short names, no
//     cnrm.cloud.google.com labels, so Config Connector never claims it;
//   - instanceRef and password use the shared reference types that
//     Config Connector's direct controllers use (refs/v1beta1.SQLInstanceRef,
//     as SQLSSLCert did when it moved to a direct controller, and
//     refs/v1beta1/secret.Legacy, as SQLInstance does); the schema is the
//     same as the Terraform-generated one;
//   - new: spec.databaseRoles and status.appliedDatabaseRoles.

package v1beta1

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	refsv1beta1secret "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1/secret"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var SQLUserGVK = GroupVersion.WithKind("SQLUser")

// SQLUserGK is the group and kind, for registration with the direct
// controller registry.
var SQLUserGK = schema.GroupKind{Group: GroupVersion.Group, Kind: "SQLUser"}

type UserPasswordPolicy struct {
	/* Number of failed attempts allowed before the user get locked. */
	// +optional
	AllowedFailedAttempts *int64 `json:"allowedFailedAttempts,omitempty"`

	/* If true, the check that will lock user after too many failed login attempts will be enabled. */
	// +optional
	EnableFailedAttemptsCheck *bool `json:"enableFailedAttemptsCheck,omitempty"`

	/* If true, the user must specify the current password before changing the password. This flag is supported only for MySQL. */
	// +optional
	EnablePasswordVerification *bool `json:"enablePasswordVerification,omitempty"`

	/* Password expiration duration with one week grace period. */
	// +optional
	PasswordExpirationDuration *string `json:"passwordExpirationDuration,omitempty"`

	// +optional
	Status []UserStatus `json:"status,omitempty"`
}

type UserStatus struct {
	/* If true, user does not have login privileges. */
	// +optional
	Locked *bool `json:"locked,omitempty"`

	/* Password expiration duration with one week grace period. */
	// +optional
	PasswordExpirationTime *string `json:"passwordExpirationTime,omitempty"`
}

type SQLUserSpec struct {
	/* A list of database roles to be assigned to the user. This option is only available for MySQL and PostgreSQL instances. The controller grants exactly these roles, revoking roles that were granted before and are not in the list (Terraform's google_sql_user.database_roles does the same). */
	// +optional
	// +listType=set
	DatabaseRoles []string `json:"databaseRoles,omitempty"`

	/* Immutable. The host the user can connect from. This is only supported for MySQL instances. Don't set this field for PostgreSQL instances. Can be an IP address. Changing this forces a new resource to be created. */
	// +optional
	Host *string `json:"host,omitempty"`

	InstanceRef refsv1beta1.SQLInstanceRef `json:"instanceRef"`

	/* The password for the user. Can be updated. For Postgres instances this is a Required field, unless type is set to
	either CLOUD_IAM_USER or CLOUD_IAM_SERVICE_ACCOUNT. */
	// +optional
	Password *refsv1beta1secret.Legacy `json:"password,omitempty"`

	// +optional
	PasswordPolicy *UserPasswordPolicy `json:"passwordPolicy,omitempty"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`

	/* Immutable. The user type. It determines the method to authenticate the user during login.
	The default is the database's built-in user type. Flags include "BUILT_IN", "CLOUD_IAM_USER", or "CLOUD_IAM_SERVICE_ACCOUNT". */
	// +optional
	Type *string `json:"type,omitempty"`
}

type UserSqlServerUserDetailsStatus struct {
	/* If the user has been disabled. */
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	/* The server roles for this user in the database. */
	// +optional
	ServerRoles []string `json:"serverRoles,omitempty"`
}

type SQLUserStatus struct {
	/* Conditions represent the latest available observations of the
	   SQLUser's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* The database roles the controller last granted, with revokeExistingRoles, from spec.databaseRoles. The Cloud SQL Admin API is not relied on to return a user's roles, so this is what the controller compares spec.databaseRoles against. */
	// +optional
	AppliedDatabaseRoles []string `json:"appliedDatabaseRoles,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// +optional
	SqlServerUserDetails []UserSqlServerUserDetailsStatus `json:"sqlServerUserDetails,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflsqluser;hflsqlusers
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:metadata:annotations="gcp.hfl.dev/upstream-kind=sql.cnrm.cloud.google.com/SQLUser"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// SQLUser is the Schema for the sql API
// +k8s:openapi-gen=true
type SQLUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SQLUserSpec   `json:"spec,omitempty"`
	Status SQLUserStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SQLUserList contains a list of SQLUser
type SQLUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SQLUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SQLUser{}, &SQLUserList{})
}
