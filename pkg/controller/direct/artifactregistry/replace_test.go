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

import (
	"testing"

	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
)

func dockerRemote(uri string) *pb.Repository {
	d := &pb.RemoteRepositoryConfig_DockerRepository{}
	if uri != "" {
		d.Upstream = &pb.RemoteRepositoryConfig_DockerRepository_CustomRepository_{
			CustomRepository: &pb.RemoteRepositoryConfig_DockerRepository_CustomRepository{Uri: uri},
		}
	} else {
		d.Upstream = &pb.RemoteRepositoryConfig_DockerRepository_PublicRepository_{
			PublicRepository: pb.RemoteRepositoryConfig_DockerRepository_DOCKER_HUB,
		}
	}
	return &pb.Repository{
		Mode:   pb.Repository_REMOTE_REPOSITORY,
		Format: pb.Repository_DOCKER,
		ModeConfig: &pb.Repository_RemoteRepositoryConfig{RemoteRepositoryConfig: &pb.RemoteRepositoryConfig{
			RemoteSource: &pb.RemoteRepositoryConfig_DockerRepository_{DockerRepository: d},
		}},
	}
}

func pythonRemote(uri string) *pb.Repository {
	return &pb.Repository{
		Mode:   pb.Repository_REMOTE_REPOSITORY,
		Format: pb.Repository_PYTHON,
		ModeConfig: &pb.Repository_RemoteRepositoryConfig{RemoteRepositoryConfig: &pb.RemoteRepositoryConfig{
			RemoteSource: &pb.RemoteRepositoryConfig_PythonRepository_{PythonRepository: &pb.RemoteRepositoryConfig_PythonRepository{
				Upstream: &pb.RemoteRepositoryConfig_PythonRepository_CustomRepository_{
					CustomRepository: &pb.RemoteRepositoryConfig_PythonRepository_CustomRepository{Uri: uri},
				},
			}},
		}},
	}
}

func TestCustomUpstreamURIChanged(t *testing.T) {
	withDescription := dockerRemote("https://a.example.com")
	withDescription.Description = "changed"
	withCredentials := dockerRemote("https://a.example.com")
	withCredentials.GetRemoteRepositoryConfig().UpstreamCredentials = &pb.RemoteRepositoryConfig_UpstreamCredentials{}
	standard := &pb.Repository{Mode: pb.Repository_STANDARD_REPOSITORY, Format: pb.Repository_DOCKER}

	for _, tc := range []struct {
		name            string
		desired, actual *pb.Repository
		want            bool
	}{
		{"same URI", dockerRemote("https://a.example.com"), dockerRemote("https://a.example.com"), false},
		{"changed URI", dockerRemote("https://b.example.com"), dockerRemote("https://a.example.com"), true},
		{"changed URI, another format", pythonRemote("https://b.example.com/simple"), pythonRemote("https://a.example.com/simple"), true},
		{"public upstream to custom", dockerRemote("https://a.example.com"), dockerRemote(""), true},
		{"custom upstream to public", dockerRemote(""), dockerRemote("https://a.example.com"), true},
		{"public upstreams only", dockerRemote(""), dockerRemote(""), false},
		{"another field changed", withDescription, dockerRemote("https://a.example.com"), false},
		{"credentials changed", withCredentials, dockerRemote("https://a.example.com"), false},
		{"a standard repository in the cloud", dockerRemote("https://a.example.com"), standard, false},
		{"a standard repository desired", standard, dockerRemote("https://a.example.com"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := customUpstreamURIChanged(tc.desired, tc.actual); got != tc.want {
				t.Errorf("customUpstreamURIChanged = %v, want %v", got, tc.want)
			}
		})
	}
}
