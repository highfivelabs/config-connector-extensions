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

package v1beta1

// Written here: a remote repository's upstream credentials are in no
// Config Connector code or pull request. The shape follows the Artifact
// Registry API (google.devtools.artifactregistry.v1.RemoteRepositoryConfig.
// UpstreamCredentials); the password is a Secret Manager secret version,
// referenced the way Config Connector's direct kinds reference one
// (refs/v1beta1.SecretManagerSecretVersionRef, as CloudBuildConnection and
// DataformRepository do).

import (
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
)

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.UpstreamCredentials
type ArtifactRegistryRepositoryUpstreamCredentials struct {
	// Use username and password to access the remote repository.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.UpstreamCredentials.username_password_credentials
	UsernamePasswordCredentials *ArtifactRegistryRepositoryUsernamePasswordCredentials `json:"usernamePasswordCredentials,omitempty"`
}

// +kcc:proto=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.UpstreamCredentials.UsernamePasswordCredentials
type ArtifactRegistryRepositoryUsernamePasswordCredentials struct {
	// The username to access the remote repository.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.UpstreamCredentials.UsernamePasswordCredentials.username
	Username *string `json:"username,omitempty"`

	// The Secret Manager secret version that holds the password to access
	//  the remote repository: a Config Connector SecretManagerSecretVersion
	//  by name, or external `projects/{project}/secrets/{secret}/versions/{version}`.
	// +kcc:proto:field=google.devtools.artifactregistry.v1.RemoteRepositoryConfig.UpstreamCredentials.UsernamePasswordCredentials.password_secret_version
	PasswordSecretVersionRef *refsv1beta1.SecretManagerSecretVersionRef `json:"passwordSecretVersionRef,omitempty"`
}
