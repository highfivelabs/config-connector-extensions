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

// Written here, in the shape of upstream's generated mappers (which leave
// these fields as "MISSING"): a repository's vulnerability scanning config
// and a remote repository's Apt and Yum upstreams (application queue item
// 4). Only the scanning config's enablementConfig is mapped: its state,
// reason and last enable time are output only.

import (
	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1beta1"
)

func Repository_VulnerabilityScanningConfig_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.Repository_VulnerabilityScanningConfig) *krm.Repository_VulnerabilityScanningConfig {
	if in == nil {
		return nil
	}
	out := &krm.Repository_VulnerabilityScanningConfig{}
	out.EnablementConfig = direct.Enum_FromProto(mapCtx, in.GetEnablementConfig())
	return out
}

func Repository_VulnerabilityScanningConfig_v1beta1_ToProto(mapCtx *direct.MapContext, in *krm.Repository_VulnerabilityScanningConfig) *pb.Repository_VulnerabilityScanningConfig {
	if in == nil {
		return nil
	}
	out := &pb.Repository_VulnerabilityScanningConfig{}
	out.EnablementConfig = direct.Enum_ToProto[pb.Repository_VulnerabilityScanningConfig_EnablementConfig](mapCtx, in.EnablementConfig)
	return out
}

func ArtifactRegistryRepositoryAptRepository_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.RemoteRepositoryConfig_AptRepository) *krm.ArtifactRegistryRepositoryAptRepository {
	if in == nil {
		return nil
	}
	out := &krm.ArtifactRegistryRepositoryAptRepository{}
	if p := in.GetPublicRepository(); p != nil {
		out.PublicRepository = &krm.ArtifactRegistryRepositoryAptRepositoryPublicRepository{
			RepositoryBase: direct.Enum_FromProto(mapCtx, p.GetRepositoryBase()),
			RepositoryPath: direct.LazyPtr(p.GetRepositoryPath()),
		}
	}
	if c := in.GetCustomRepository(); c != nil {
		out.CustomRepository = &krm.ArtifactRegistryRepositoryAptRepositoryCustomRepository{URI: direct.LazyPtr(c.GetUri())}
	}
	return out
}

func ArtifactRegistryRepositoryAptRepository_v1beta1_ToProto(mapCtx *direct.MapContext, in *krm.ArtifactRegistryRepositoryAptRepository) *pb.RemoteRepositoryConfig_AptRepository {
	if in == nil {
		return nil
	}
	out := &pb.RemoteRepositoryConfig_AptRepository{}
	if p := in.PublicRepository; p != nil {
		out.Upstream = &pb.RemoteRepositoryConfig_AptRepository_PublicRepository_{PublicRepository: &pb.RemoteRepositoryConfig_AptRepository_PublicRepository{
			RepositoryBase: direct.Enum_ToProto[pb.RemoteRepositoryConfig_AptRepository_PublicRepository_RepositoryBase](mapCtx, p.RepositoryBase),
			RepositoryPath: direct.ValueOf(p.RepositoryPath),
		}}
	}
	if c := in.CustomRepository; c != nil {
		out.Upstream = &pb.RemoteRepositoryConfig_AptRepository_CustomRepository_{CustomRepository: &pb.RemoteRepositoryConfig_AptRepository_CustomRepository{Uri: direct.ValueOf(c.URI)}}
	}
	return out
}

func ArtifactRegistryRepositoryYumRepository_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.RemoteRepositoryConfig_YumRepository) *krm.ArtifactRegistryRepositoryYumRepository {
	if in == nil {
		return nil
	}
	out := &krm.ArtifactRegistryRepositoryYumRepository{}
	if p := in.GetPublicRepository(); p != nil {
		out.PublicRepository = &krm.ArtifactRegistryRepositoryYumRepositoryPublicRepository{
			RepositoryBase: direct.Enum_FromProto(mapCtx, p.GetRepositoryBase()),
			RepositoryPath: direct.LazyPtr(p.GetRepositoryPath()),
		}
	}
	if c := in.GetCustomRepository(); c != nil {
		out.CustomRepository = &krm.ArtifactRegistryRepositoryYumRepositoryCustomRepository{URI: direct.LazyPtr(c.GetUri())}
	}
	return out
}

func ArtifactRegistryRepositoryYumRepository_v1beta1_ToProto(mapCtx *direct.MapContext, in *krm.ArtifactRegistryRepositoryYumRepository) *pb.RemoteRepositoryConfig_YumRepository {
	if in == nil {
		return nil
	}
	out := &pb.RemoteRepositoryConfig_YumRepository{}
	if p := in.PublicRepository; p != nil {
		out.Upstream = &pb.RemoteRepositoryConfig_YumRepository_PublicRepository_{PublicRepository: &pb.RemoteRepositoryConfig_YumRepository_PublicRepository{
			RepositoryBase: direct.Enum_ToProto[pb.RemoteRepositoryConfig_YumRepository_PublicRepository_RepositoryBase](mapCtx, p.RepositoryBase),
			RepositoryPath: direct.ValueOf(p.RepositoryPath),
		}}
	}
	if c := in.CustomRepository; c != nil {
		out.Upstream = &pb.RemoteRepositoryConfig_YumRepository_CustomRepository_{CustomRepository: &pb.RemoteRepositoryConfig_YumRepository_CustomRepository{Uri: direct.ValueOf(c.URI)}}
	}
	return out
}

// keepUnownedRepositoryFields: a vulnerability scanning config or
// disallowUnspecifiedMode the spec leaves out is not owned, so the
// repository's own (the API defaults scanning to INHERITED) is never a
// difference.
func keepUnownedRepositoryFields(spec *krm.ArtifactRegistryRepositorySpec, desired, actual *pb.Repository) {
	if spec.VulnerabilityScanningConfig == nil {
		desired.VulnerabilityScanningConfig = actual.GetVulnerabilityScanningConfig()
	}
	if spec.DisallowUnspecifiedMode == nil {
		desired.DisallowUnspecifiedMode = actual.GetDisallowUnspecifiedMode()
	}
}
