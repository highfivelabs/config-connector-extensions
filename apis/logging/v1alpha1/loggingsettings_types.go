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

// Written here from the Cloud Logging API's Settings
// (organizations/{id}/settings, folders/{id}/settings) and Terraform's
// google_logging_organization_settings and google_logging_folder_settings
// (read, not copied). Config Connector's closed PR #3730 was a scaffold
// with no fields; nothing is taken from it.

import (
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var LoggingSettingsGVK = GroupVersion.WithKind("LoggingSettings")

// LoggingSettingsSpec is the Log Router settings of one organization or
// folder. The settings always exist; a field left unset is not managed.
// +kubebuilder:validation:XValidation:rule="has(self.organizationRef) != has(self.folderRef)",message="exactly one of organizationRef and folderRef is required"
type LoggingSettingsSpec struct {
	// The organization whose settings these are. Immutable.
	// +optional
	OrganizationRef *refsv1beta1.OrganizationRef `json:"organizationRef,omitempty"`

	// The folder whose settings these are. Immutable.
	// +optional
	FolderRef *refsv1beta1.FolderRefDeprecated `json:"folderRef,omitempty"`

	// The Cloud KMS key the Log Router uses for CMEK. Grant
	//  roles/cloudkms.cryptoKeyEncrypterDecrypter on it to
	//  status.observedState.kmsServiceAccountID first.
	// +optional
	KMSKeyRef *kmsv1beta1.KMSCryptoKeyRef `json:"kmsKeyRef,omitempty"`

	// The region for the _Default and _Required log buckets of projects and
	//  folders created after this is set, for example europe-west1.
	// +optional
	StorageLocation *string `json:"storageLocation,omitempty"`

	// If true, the _Default sink of projects and folders created after this
	//  is set starts disabled.
	// +optional
	DisableDefaultSink *bool `json:"disableDefaultSink,omitempty"`
}

// LoggingSettingsObservedState is what the API reports about the settings.
type LoggingSettingsObservedState struct {
	// The service account the Log Router uses to reach the Cloud KMS key.
	KMSServiceAccountID *string `json:"kmsServiceAccountID,omitempty"`

	// The service account the Log Router uses to write to log buckets and
	//  sinks.
	LoggingServiceAccountID *string `json:"loggingServiceAccountID,omitempty"`
}

// LoggingSettingsStatus defines the config connector machine state of LoggingSettings
type LoggingSettingsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The settings' resource name, organizations/{id}/settings or
	//  folders/{id}/settings.
	ExternalRef *string `json:"externalRef,omitempty"`

	// ObservedState is the state of the resource as most recently observed in GCP.
	ObservedState *LoggingSettingsObservedState `json:"observedState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflloggingsettings
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// LoggingSettings is the Schema for the LoggingSettings API
// +k8s:openapi-gen=true
type LoggingSettings struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   LoggingSettingsSpec   `json:"spec,omitempty"`
	Status LoggingSettingsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// LoggingSettingsList contains a list of LoggingSettings
type LoggingSettingsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoggingSettings `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoggingSettings{}, &LoggingSettingsList{})
}
