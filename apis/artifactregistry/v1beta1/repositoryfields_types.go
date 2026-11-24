// Copyright 2025 Google LLC
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

// Modified by High Five Labs for config-connector-extensions. Copied from
// Config Connector apis/artifactregistry/v1beta1/types.generated.go
// (UPSTREAM.md), where these types are generated but commented out as
// unreachable: a repository's vulnerability scanning config and a remote
// repository's Apt and Yum upstreams. Changed: named after this kind's
// other remote types, enums listed, and a CEL rule that an upstream is
// public or custom, not both.

package v1beta1

// +kcc:proto=google.devtools.artifactregistry.v1.Repository.VulnerabilityScanningConfig
type Repository_VulnerabilityScanningConfig struct {
	// Optional. Config for whether this repository has vulnerability scanning
	//  disabled: INHERITED (scanning follows the project's Container Scanning
	//  API) or DISABLED.
	// +kubebuilder:validation:Enum=INHERITED;DISABLED
	// +kcc:proto:field=google.devtools.artifactregistry.v1.Repository.VulnerabilityScanningConfig.enablement_config
	EnablementConfig *string `json:"enablementConfig,omitempty"`
}

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository
// +kubebuilder:validation:XValidation:rule="has(self.publicRepository) != has(self.customRepository)",message="set one of publicRepository and customRepository"
type ArtifactRegistryRepositoryAptRepository struct {
	// One of the publicly available Apt repositories supported by Artifact
	//  Registry.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository.public_repository
	PublicRepository *ArtifactRegistryRepositoryAptRepositoryPublicRepository `json:"publicRepository,omitempty"`

	// Customer-specified remote repository.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository.custom_repository
	CustomRepository *ArtifactRegistryRepositoryAptRepositoryCustomRepository `json:"customRepository,omitempty"`
}

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository.CustomRepository
type ArtifactRegistryRepositoryAptRepositoryCustomRepository struct {
	// An http/https uri reference to the upstream remote repository, for ex:
	//  "https://my.apt.registry/".
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository.CustomRepository.uri
	URI *string `json:"uri,omitempty"`
}

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository.PublicRepository
type ArtifactRegistryRepositoryAptRepositoryPublicRepository struct {
	// A common public repository base for Apt: DEBIAN, UBUNTU or
	// DEBIAN_SNAPSHOT.
	// +kubebuilder:validation:Enum=DEBIAN;UBUNTU;DEBIAN_SNAPSHOT
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository.PublicRepository.repository_base
	RepositoryBase *string `json:"repositoryBase,omitempty"`

	// A custom field to define a path to a specific repository from the base.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.AptRepository.PublicRepository.repository_path
	RepositoryPath *string `json:"repositoryPath,omitempty"`
}

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository
// +kubebuilder:validation:XValidation:rule="has(self.publicRepository) != has(self.customRepository)",message="set one of publicRepository and customRepository"
type ArtifactRegistryRepositoryYumRepository struct {
	// One of the publicly available Yum repositories supported by Artifact
	//  Registry.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository.public_repository
	PublicRepository *ArtifactRegistryRepositoryYumRepositoryPublicRepository `json:"publicRepository,omitempty"`

	// Customer-specified remote repository.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository.custom_repository
	CustomRepository *ArtifactRegistryRepositoryYumRepositoryCustomRepository `json:"customRepository,omitempty"`
}

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository.CustomRepository
type ArtifactRegistryRepositoryYumRepositoryCustomRepository struct {
	// An http/https uri reference to the upstream remote repository, for ex:
	//  "https://my.yum.registry/".
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository.CustomRepository.uri
	URI *string `json:"uri,omitempty"`
}

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository.PublicRepository
type ArtifactRegistryRepositoryYumRepositoryPublicRepository struct {
	// A common public repository base for Yum: CENTOS, CENTOS_DEBUG,
	// CENTOS_VAULT, CENTOS_STREAM, ROCKY or EPEL.
	// +kubebuilder:validation:Enum=CENTOS;CENTOS_DEBUG;CENTOS_VAULT;CENTOS_STREAM;ROCKY;EPEL
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository.PublicRepository.repository_base
	RepositoryBase *string `json:"repositoryBase,omitempty"`

	// A custom field to define a path to a specific repository from the base.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.YumRepository.PublicRepository.repository_path
	RepositoryPath *string `json:"repositoryPath,omitempty"`
}
