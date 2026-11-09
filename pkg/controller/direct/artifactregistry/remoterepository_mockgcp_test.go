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

package artifactregistry_test

// ArtifactRegistryRepository with a custom remote upstream end to end on the
// fake cloud (pkg/test/mockenv), or a real project with E2E_GCP_TARGET=real
// (make test-live). The case: a remote repository that pulls
// through a private upstream registry (a company's own Docker registry,
// Maven repository, npm or Python index) instead of one of the public
// upstreams Artifact Registry knows by name.

import (
	"net/url"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/artifactregistry/apiv1"
	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	arv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	region    = "us-central1"
)

var gvk = arv1beta1.ArtifactRegistryRepositoryGVK

func newClient(t *testing.T, env *mockenv.Env) *gcp.Client {
	t.Helper()
	c, err := gcp.NewRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func cloudName(name string) string {
	return "projects/" + mockenv.Project + "/locations/" + region + "/repositories/" + name
}

func newRemoteRepository(name, format string, remote *arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig) *arv1beta1.ArtifactRegistryRepository {
	return &arv1beta1.ArtifactRegistryRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project},
			Labels:      mockenv.Labels(),
		},
		Spec: arv1beta1.ArtifactRegistryRepositorySpec{
			Location:               region,
			Format:                 mockenv.Ptr(format),
			Mode:                   mockenv.Ptr("REMOTE_REPOSITORY"),
			Description:            mockenv.Ptr("pull-through cache of a private upstream"),
			RemoteRepositoryConfig: remote,
		},
	}
}

// The upstreams here (example.com) do not exist. Real Artifact Registry
// checks a remote upstream when a repository is made, and refuses one it
// cannot reach ("Failed to validate remote upstream", seen live,
// docs/live-runs.md); mockgcp does not check. So every repository here
// sets disableUpstreamValidation.

func dockerUpstream(uri string) *arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig {
	return &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{
		Description:               mockenv.Ptr("private Docker registry"),
		DisableUpstreamValidation: mockenv.Ptr(true),
		DockerRepository: &arv1beta1.ArtifactRegistryRepositoryDockerRepository{
			CustomRepository: &arv1beta1.ArtifactRegistryRepositoryDockerRepositoryCustomRepository{URI: mockenv.Ptr(uri)},
		},
	}
}

// count counts the requests the fake cloud saw with this method for this
// repository (and not its operations).
func count(env *mockenv.Env, method, name string) int {
	n := 0
	for _, r := range env.Requests() {
		if strings.HasPrefix(r, method+" ") && strings.Contains(r, "/v1/"+name+"?") {
			n++
		}
	}
	return n
}

func TestArtifactRegistryRemoteRepository(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := newClient(t, env)

	const (
		uri       = "https://registry.example.com"
		uriChange = "https://mirror.example.com"
	)
	name := mockenv.Name("docker-upstream")
	repo := newRemoteRepository(name, "DOCKER", dockerUpstream(uri))
	if err := env.Kube.Create(env.Ctx, repo); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the custom upstream", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if got, _, _ := unstructured.NestedString(u.Object, "status", "name"); got != name {
			t.Errorf("status.name = %q, want %q", got, name)
		}
		got, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
		if err != nil {
			t.Fatalf("reading the repository from the cloud: %v", err)
		}
		if got.GetMode() != pb.Repository_REMOTE_REPOSITORY || got.GetFormat() != pb.Repository_DOCKER {
			t.Errorf("cloud mode/format = %v/%v, want REMOTE_REPOSITORY/DOCKER", got.GetMode(), got.GetFormat())
		}
		docker := got.GetRemoteRepositoryConfig().GetDockerRepository()
		if docker.GetCustomRepository().GetUri() != uri {
			t.Errorf("cloud dockerRepository = %v, want customRepository.uri %q", docker, uri)
		}
		if docker.GetPublicRepository() != pb.RemoteRepositoryConfig_DockerRepository_PUBLIC_REPOSITORY_UNSPECIFIED {
			t.Errorf("cloud publicRepository = %v, want none beside a custom repository", docker.GetPublicRepository())
		}
		if got.GetRemoteRepositoryConfig().GetDescription() != "private Docker registry" {
			t.Errorf("cloud remoteRepositoryConfig.description = %q", got.GetRemoteRepositoryConfig().GetDescription())
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		// The controller reads the repository back and compares it with the
		// spec; a mapper that dropped customRepository would see a
		// difference and send remote_repository_config in an update.
		gets, patches := count(env, "GET", cloudName(name)), count(env, "PATCH", cloudName(name))
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
			t.Fatal(err)
		}
		// Config Connector's force-reconcile annotation: the reconciler
		// skips other annotation changes (pkg/controller/predicate).
		ann := u.GetAnnotations()
		ann[k8s.InternalForceReconcileAnnotation] = "1"
		u.SetAnnotations(ann)
		if err := env.Kube.Update(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for count(env, "GET", cloudName(name)) == gets {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the repository again after the annotation changed")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(gvk, namespace, name)
		if got := count(env, "PATCH", cloudName(name)); got != patches {
			t.Errorf("PATCH requests after a resync with an unchanged spec: %d, want none", got-patches)
		}
	})

	t.Run("a description change keeps the custom upstream", func(t *testing.T) {
		obj := &arv1beta1.ArtifactRegistryRepository{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Spec.Description = mockenv.Ptr("pull-through cache, changed")
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, name)
		got, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
		if err != nil {
			t.Fatal(err)
		}
		if got.GetDescription() != "pull-through cache, changed" {
			t.Errorf("cloud description = %q, want the changed one", got.GetDescription())
		}
		if got.GetRemoteRepositoryConfig().GetDockerRepository().GetCustomRepository().GetUri() != uri {
			t.Errorf("cloud custom upstream after a description change = %v, want %q", got.GetRemoteRepositoryConfig(), uri)
		}
		var masks []string
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/v1/"+cloudName(name)+"?") {
				u, err := url.Parse(strings.TrimPrefix(r, "PATCH "))
				if err != nil {
					t.Fatal(err)
				}
				masks = append(masks, u.Query().Get("updateMask"))
			}
		}
		if len(masks) != 1 || masks[0] != "description" {
			t.Errorf("update masks = %q, want one update of description only", masks)
		}
	})

	t.Run("a changed upstream URI is refused", func(t *testing.T) {
		// Terraform marks custom_repository ForceNew; Config Connector's
		// direct controller sends remoteRepositoryConfig in an update mask,
		// which the real API refuses (seen live, docs/live-runs.md), and
		// mockgcp with it (patch 0016).
		obj := &arv1beta1.ArtifactRegistryRepository{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Spec.RemoteRepositoryConfig.DockerRepository.CustomRepository.URI = mockenv.Ptr(uriChange)
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(gvk, namespace, name)
		if !strings.Contains(msg, "Remote source configuartion cannot be updated") {
			t.Errorf("Ready message = %q, want the API's refusal (\"Remote source configuartion cannot be updated\")", msg)
		}
		got, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
		if err != nil {
			t.Fatal(err)
		}
		if got.GetRemoteRepositoryConfig().GetDockerRepository().GetCustomRepository().GetUri() != uri {
			t.Errorf("cloud custom upstream = %v, want it unchanged (%q)", got.GetRemoteRepositoryConfig(), uri)
		}
	})

	t.Run("delete removes the repository", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, repo); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&arv1beta1.ArtifactRegistryRepository{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
		if status.Code(err) != codes.NotFound {
			t.Errorf("reading the deleted repository: got %v, want NotFound", err)
		}
	})

	t.Run("abandon keeps the repository", func(t *testing.T) {
		kept := mockenv.Name("kept-upstream")
		r := newRemoteRepository(kept, "DOCKER", dockerUpstream(uri))
		r.Annotations["cnrm.cloud.google.com/deletion-policy"] = "abandon"
		if err := env.Kube.Create(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, kept)
		// The test deletes what it abandoned (a live run must leave nothing).
		t.Cleanup(func() {
			op, err := cloud.DeleteRepository(env.Ctx, &pb.DeleteRepositoryRequest{Name: cloudName(kept)})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && status.Code(err) != codes.NotFound {
				t.Errorf("deleting the abandoned repository: %v", err)
			}
		})
		if err := env.Kube.Delete(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&arv1beta1.ArtifactRegistryRepository{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: kept}})
		got, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(kept)})
		if err != nil {
			t.Fatalf("the abandoned repository is gone from the cloud: %v", err)
		}
		if got.GetRemoteRepositoryConfig().GetDockerRepository().GetCustomRepository().GetUri() != uri {
			t.Errorf("abandoned repository's custom upstream = %v, want %q", got.GetRemoteRepositoryConfig(), uri)
		}
	})
}

// TestArtifactRegistryCustomUpstreamFormats creates a remote repository with
// a custom upstream for each format the PR covers, and reads the upstream
// back from the fake cloud.
func TestArtifactRegistryCustomUpstreamFormats(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := newClient(t, env)

	for _, tc := range []struct {
		name, format, uri string
		remote            *arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig
		read              func(*pb.RemoteRepositoryConfig) string
	}{
		{
			name: "maven-upstream", format: "MAVEN", uri: "https://maven.example.com/releases",
			remote: &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{DisableUpstreamValidation: mockenv.Ptr(true), MavenRepository: &arv1beta1.ArtifactRegistryRepositoryMavenRepository{
				CustomRepository: &arv1beta1.ArtifactRegistryRepositoryMavenRepositoryCustomRepository{URI: mockenv.Ptr("https://maven.example.com/releases")},
			}},
			read: func(c *pb.RemoteRepositoryConfig) string {
				return c.GetMavenRepository().GetCustomRepository().GetUri()
			},
		},
		{
			name: "npm-upstream", format: "NPM", uri: "https://npm.example.com",
			remote: &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{DisableUpstreamValidation: mockenv.Ptr(true), NpmRepository: &arv1beta1.ArtifactRegistryRepositoryNpmRepository{
				CustomRepository: &arv1beta1.ArtifactRegistryRepositoryNpmRepositoryCustomRepository{URI: mockenv.Ptr("https://npm.example.com")},
			}},
			read: func(c *pb.RemoteRepositoryConfig) string { return c.GetNpmRepository().GetCustomRepository().GetUri() },
		},
		{
			name: "python-upstream", format: "PYTHON", uri: "https://pypi.example.com/simple",
			remote: &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{DisableUpstreamValidation: mockenv.Ptr(true), PythonRepository: &arv1beta1.ArtifactRegistryRepositoryPythonRepository{
				CustomRepository: &arv1beta1.ArtifactRegistryRepositoryPythonRepositoryCustomRepository{URI: mockenv.Ptr("https://pypi.example.com/simple")},
			}},
			read: func(c *pb.RemoteRepositoryConfig) string {
				return c.GetPythonRepository().GetCustomRepository().GetUri()
			},
		},
	} {
		t.Run(tc.format, func(t *testing.T) {
			name := mockenv.Name(tc.name)
			repo := newRemoteRepository(name, tc.format, tc.remote)
			if err := env.Kube.Create(env.Ctx, repo); err != nil {
				t.Fatal(err)
			}
			// The test deletes what it made (a live run must leave nothing).
			t.Cleanup(func() {
				if err := env.Kube.Delete(env.Ctx, repo); err != nil {
					t.Errorf("deleting %s: %v", name, err)
					return
				}
				env.WaitGone(&arv1beta1.ArtifactRegistryRepository{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
			})
			env.WaitReady(gvk, namespace, name)
			got, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
			if err != nil {
				t.Fatalf("reading the repository from the cloud: %v", err)
			}
			if u := tc.read(got.GetRemoteRepositoryConfig()); u != tc.uri {
				t.Errorf("cloud customRepository.uri = %q, want %q", u, tc.uri)
			}
		})
	}
}
