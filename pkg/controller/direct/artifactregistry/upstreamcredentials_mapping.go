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

// Written here: the mapping of a remote repository's upstream credentials
// and its update mask, in no Config Connector code or pull request. The
// behaviour follows the Artifact Registry API and Terraform's
// google_artifact_registry_repository (read, not copied): credentials can
// change in place, with an update mask that names their fields;
// disableUpstreamValidation is input only.

import (
	"slices"

	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	krm "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1beta1"
)

func ArtifactRegistryRepositoryUpstreamCredentials_FromProto(mapCtx *direct.MapContext, in *pb.RemoteRepositoryConfig_UpstreamCredentials) *krm.ArtifactRegistryRepositoryUpstreamCredentials {
	if in == nil {
		return nil
	}
	out := &krm.ArtifactRegistryRepositoryUpstreamCredentials{}
	if c := in.GetUsernamePasswordCredentials(); c != nil {
		out.UsernamePasswordCredentials = &krm.ArtifactRegistryRepositoryUsernamePasswordCredentials{
			Username: direct.LazyPtr(c.GetUsername()),
		}
		if v := c.GetPasswordSecretVersion(); v != "" {
			out.UsernamePasswordCredentials.PasswordSecretVersionRef = &refsv1beta1.SecretManagerSecretVersionRef{External: v}
		}
	}
	return out
}

func ArtifactRegistryRepositoryUpstreamCredentials_ToProto(mapCtx *direct.MapContext, in *krm.ArtifactRegistryRepositoryUpstreamCredentials) *pb.RemoteRepositoryConfig_UpstreamCredentials {
	if in == nil {
		return nil
	}
	out := &pb.RemoteRepositoryConfig_UpstreamCredentials{}
	if c := in.UsernamePasswordCredentials; c != nil {
		upc := &pb.RemoteRepositoryConfig_UpstreamCredentials_UsernamePasswordCredentials{
			Username: direct.ValueOf(c.Username),
		}
		if c.PasswordSecretVersionRef != nil {
			upc.PasswordSecretVersion = c.PasswordSecretVersionRef.External
		}
		out.Credentials = &pb.RemoteRepositoryConfig_UpstreamCredentials_UsernamePasswordCredentials_{UsernamePasswordCredentials: upc}
	}
	return out
}

const (
	maskRemoteRepositoryConfig = "remote_repository_config"
	maskUpstreamUsername       = "remote_repository_config.upstream_credentials.username_password_credentials.username"
	maskUpstreamPassword       = "remote_repository_config.upstream_credentials.username_password_credentials.password_secret_version"
)

// ignoreInputOnlyRemoteFields copies disableUpstreamValidation, which the
// API takes but never returns, from desired to actual before they are
// compared, so it is never a difference.
func ignoreInputOnlyRemoteFields(desired, actual *pb.Repository) {
	if rc := actual.GetRemoteRepositoryConfig(); rc != nil {
		rc.DisableUpstreamValidation = desired.GetRemoteRepositoryConfig().GetDisableUpstreamValidation()
	}
}

// narrowRemoteRepositoryConfigMask: when the remote repository config
// differs only in its upstream credentials, the whole-field mask path is
// replaced with the credential fields that changed, as Terraform does. Any
// other difference in the remote repository config keeps the whole-field
// path, as Config Connector's controller sends it.
func narrowRemoteRepositoryConfigMask(desired, actual *pb.Repository, mask *fieldmaskpb.FieldMask) {
	i := slices.Index(mask.GetPaths(), maskRemoteRepositoryConfig)
	if i < 0 {
		return
	}
	d, a := desired.GetRemoteRepositoryConfig(), actual.GetRemoteRepositoryConfig()
	if d == nil || a == nil {
		return
	}
	dRest, aRest := proto.Clone(d).(*pb.RemoteRepositoryConfig), proto.Clone(a).(*pb.RemoteRepositoryConfig)
	dRest.UpstreamCredentials, aRest.UpstreamCredentials = nil, nil
	dRest.DisableUpstreamValidation, aRest.DisableUpstreamValidation = false, false
	if !proto.Equal(dRest, aRest) {
		return
	}
	dc := d.GetUpstreamCredentials().GetUsernamePasswordCredentials()
	ac := a.GetUpstreamCredentials().GetUsernamePasswordCredentials()
	var paths []string
	if dc.GetUsername() != ac.GetUsername() {
		paths = append(paths, maskUpstreamUsername)
	}
	if dc.GetPasswordSecretVersion() != ac.GetPasswordSecretVersion() {
		paths = append(paths, maskUpstreamPassword)
	}
	mask.Paths = slices.Delete(mask.Paths, i, i+1)
	mask.Paths = append(mask.Paths, paths...)
}
