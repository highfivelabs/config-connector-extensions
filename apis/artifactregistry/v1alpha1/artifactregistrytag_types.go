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

// Written here: a package tag is in no Config Connector code or pull
// request, nor in Terraform. The shape follows the Artifact Registry API's
// Tag (google.devtools.artifactregistry.v1.Tag), in the layout of Config
// Connector's direct kinds.

import (
	arv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/artifactregistry/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ArtifactRegistryTagGVK = GroupVersion.WithKind("ArtifactRegistryTag")

// ArtifactRegistryTagSpec defines the desired state of ArtifactRegistryTag.
// +kcc:spec:proto=google.devtools.artifactregistry.v1.Tag
type ArtifactRegistryTagSpec struct {
	// The repository the package is in: a Config Connector
	//  ArtifactRegistryRepository by name, or
	//  "projects/{{projectID}}/locations/{{location}}/repositories/{{repositoryID}}"
	//  as external.
	// +required
	RepositoryRef *arv1beta1.ArtifactRegistryRepositoryRef `json:"repositoryRef"`

	// The package the tag is on, for example a Docker image's name within
	//  the repository ("team/app"). Immutable.
	// +required
	Package string `json:"package"`

	// The tag, for example "prod". If not given, the metadata.name will be
	//  used. Immutable.
	ResourceID *string `json:"resourceID,omitempty"`

	// The version the tag points at: the version's ID, for a Docker image
	//  its digest ("sha256:..."). Moving the tag is changing this.
	// +required
	// +kcc:proto:field=google.devtools.artifactregistry.v1.Tag.version
	Version string `json:"version"`
}

// ArtifactRegistryTagStatus defines the observed state of ArtifactRegistryTag.
type ArtifactRegistryTagStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the ArtifactRegistryTag resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	// The full name of the version the tag points at, as the API returns it.
	ObservedVersion *string `json:"observedVersion,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflartifactregistrytag;hflartifactregistrytags
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ArtifactRegistryTag is a tag on a package in an Artifact Registry
// repository, pointing at one of its versions.
// +k8s:openapi-gen=true
type ArtifactRegistryTag struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ArtifactRegistryTagSpec   `json:"spec,omitempty"`
	Status ArtifactRegistryTagStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ArtifactRegistryTagList contains a list of ArtifactRegistryTag
type ArtifactRegistryTagList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArtifactRegistryTag `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ArtifactRegistryTag{}, &ArtifactRegistryTagList{})
}
