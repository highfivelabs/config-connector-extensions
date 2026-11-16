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

package artifactregistry

// Written here, in no Config Connector code or pull request: a remote
// repository whose custom upstream URI changes is deleted and made again
// with the same name (docs/design.md decision 21, confirmed by the builder
// 2026-11-16, ARURI). The Artifact Registry API refuses the change in place
// ("Remote source configuartion cannot be updated", seen live), and
// Terraform's google_artifact_registry_repository marks custom_repository
// ForceNew (read, not copied). A remote repository holds only a cache of
// its upstream, so replacing it loses nothing that cannot be fetched again.
// Every other change, the upstream credentials and description included,
// is still an update in place.

import (
	"context"
	"fmt"

	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/klog/v2"
)

// customUpstreamURI is the custom upstream's URI of a remote repository
// config, whatever its format, or "" when it has none.
func customUpstreamURI(rc *pb.RemoteRepositoryConfig) string {
	switch {
	case rc.GetDockerRepository().GetCustomRepository() != nil:
		return rc.GetDockerRepository().GetCustomRepository().GetUri()
	case rc.GetMavenRepository().GetCustomRepository() != nil:
		return rc.GetMavenRepository().GetCustomRepository().GetUri()
	case rc.GetNpmRepository().GetCustomRepository() != nil:
		return rc.GetNpmRepository().GetCustomRepository().GetUri()
	case rc.GetPythonRepository().GetCustomRepository() != nil:
		return rc.GetPythonRepository().GetCustomRepository().GetUri()
	}
	return ""
}

// customUpstreamURIChanged reports whether the repository in the cloud is a
// remote repository and the desired one names a different custom upstream
// URI. A URI set on one side only (a public upstream swapped for a custom
// one, or back) is a change too.
func customUpstreamURIChanged(desired, actual *pb.Repository) bool {
	if actual.GetMode() != pb.Repository_REMOTE_REPOSITORY {
		return false
	}
	if desired.GetMode() != pb.Repository_REMOTE_REPOSITORY {
		return false
	}
	d, a := customUpstreamURI(desired.GetRemoteRepositoryConfig()), customUpstreamURI(actual.GetRemoteRepositoryConfig())
	if d == "" && a == "" {
		return false
	}
	return d != a
}

// replace deletes the repository and makes it again from the spec, with the
// same name. If the create fails after the delete, the next reconcile finds
// no repository and creates it.
func (a *ArtifactRegistryRepositoryAdapter) replace(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.Info("replacing ArtifactRegistryRepository: its custom upstream URI changed, which the API cannot update",
		"name", a.id.String(),
		"from", customUpstreamURI(a.actual.GetRemoteRepositoryConfig()))

	op, err := a.gcpClient.DeleteRepository(ctx, &pb.DeleteRepositoryRequest{Name: a.id.String()})
	if err != nil && !direct.IsNotFound(err) {
		return fmt.Errorf("replacing ArtifactRegistryRepository %s: deleting: %w", a.id, err)
	}
	if err == nil {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("replacing ArtifactRegistryRepository %s: waiting deletion: %w", a.id, err)
		}
	}

	created, err := a.createRepository(ctx)
	if err != nil {
		return fmt.Errorf("replacing ArtifactRegistryRepository %s: %w", a.id, err)
	}
	log.Info("replaced ArtifactRegistryRepository", "name", a.id.String())
	return a.updateStatus(ctx, updateOp, created)
}
