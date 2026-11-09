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

// A remote repository that logs in to its private upstream, end to end on
// the fake cloud (pkg/test/mockenv). Written here: upstreamCredentials is
// in no Config Connector code or pull request.

import (
	"net/url"
	"strings"
	"testing"
	"time"

	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	arv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// kccObject creates a Config Connector object (not reconciled here; only
// read to resolve a reference by name).
func kccObject(t *testing.T, env *mockenv.Env, kind, name string, spec map[string]any) {
	t.Helper()
	u := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	u.SetAPIVersion("secretmanager.cnrm.cloud.google.com/v1beta1")
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
}

// updateMasks returns the updateMask of every update of the repository so far.
func updateMasks(t *testing.T, env *mockenv.Env, name string) []string {
	t.Helper()
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
	return masks
}

func readCredentials(t *testing.T, env *mockenv.Env, name string) *pb.RemoteRepositoryConfig {
	t.Helper()
	got, err := newClient(t, env).GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
	if err != nil {
		t.Fatalf("reading the repository from the fake cloud: %v", err)
	}
	return got.GetRemoteRepositoryConfig()
}

func TestArtifactRegistryUpstreamCredentials(t *testing.T) {
	env := mockenv.New(t, namespace)
	const (
		name     = "private-upstream"
		version1 = "projects/" + mockenv.Project + "/secrets/registry-password/versions/1"
		version2 = "projects/" + mockenv.Project + "/secrets/registry-password/versions/2"
	)

	// The password's secret version, as Config Connector objects.
	kccObject(t, env, "SecretManagerSecret", "registry-password", map[string]any{
		"replication": map[string]any{"automatic": true},
	})
	kccObject(t, env, "SecretManagerSecretVersion", "registry-password-1", map[string]any{
		"resourceID": "1",
		"secretRef":  map[string]any{"name": "registry-password"},
		"secretData": map[string]any{"value": "not-a-real-password"},
	})

	remote := dockerUpstream("https://registry.example.com")
	remote.UpstreamCredentials = &arv1beta1.ArtifactRegistryRepositoryUpstreamCredentials{
		UsernamePasswordCredentials: &arv1beta1.ArtifactRegistryRepositoryUsernamePasswordCredentials{
			Username:                 mockenv.Ptr("puller"),
			PasswordSecretVersionRef: &refsv1beta1.SecretManagerSecretVersionRef{Name: "registry-password-1"},
		},
	}
	remote.DisableUpstreamValidation = mockenv.Ptr(true)
	repo := newRemoteRepository(name, "DOCKER", remote)
	if err := env.Kube.Create(env.Ctx, repo); err != nil {
		t.Fatal(err)
	}

	get := func(t *testing.T) *arv1beta1.ArtifactRegistryRepository {
		t.Helper()
		obj := &arv1beta1.ArtifactRegistryRepository{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	t.Run("create sends the credentials, the password by its secret version", func(t *testing.T) {
		env.WaitReady(gvk, namespace, name)
		rc := readCredentials(t, env, name)
		upc := rc.GetUpstreamCredentials().GetUsernamePasswordCredentials()
		if upc.GetUsername() != "puller" || upc.GetPasswordSecretVersion() != version1 {
			t.Errorf("cloud credentials = %v, want username puller and %s (resolved from the SecretManagerSecretVersion's name)", upc, version1)
		}
		if rc.GetDockerRepository().GetCustomRepository().GetUri() != "https://registry.example.com" {
			t.Errorf("cloud custom upstream = %v", rc.GetDockerRepository())
		}
		if rc.GetDisableUpstreamValidation() {
			t.Errorf("the fake cloud returned disableUpstreamValidation; the API takes it as input only")
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		// disableUpstreamValidation is true in the spec and never comes back
		// from the API; the controller must not see that as a difference.
		gets, patches := count(env, "GET", cloudName(name)), count(env, "PATCH", cloudName(name))
		obj := get(t)
		if obj.Annotations == nil {
			obj.Annotations = map[string]string{}
		}
		obj.Annotations[k8s.InternalForceReconcileAnnotation] = "1"
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for count(env, "GET", cloudName(name)) == gets {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the repository again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(gvk, namespace, name)
		if got := count(env, "PATCH", cloudName(name)); got != patches {
			t.Errorf("PATCH requests after a resync with an unchanged spec: %d, want none", got-patches)
		}
	})

	t.Run("a new password version is sent alone", func(t *testing.T) {
		before := len(updateMasks(t, env, name))
		obj := get(t)
		obj.Spec.RemoteRepositoryConfig.UpstreamCredentials.UsernamePasswordCredentials.PasswordSecretVersionRef =
			&refsv1beta1.SecretManagerSecretVersionRef{External: version2}
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, name)
		upc := readCredentials(t, env, name).GetUpstreamCredentials().GetUsernamePasswordCredentials()
		if upc.GetPasswordSecretVersion() != version2 || upc.GetUsername() != "puller" {
			t.Errorf("cloud credentials = %v, want puller and %s", upc, version2)
		}
		masks := updateMasks(t, env, name)[before:]
		want := "remoteRepositoryConfig.upstreamCredentials.usernamePasswordCredentials.passwordSecretVersion"
		if len(masks) != 1 || masks[0] != want {
			t.Errorf("update masks = %q, want one: %s", masks, want)
		}
	})

	t.Run("a new username is sent alone", func(t *testing.T) {
		before := len(updateMasks(t, env, name))
		obj := get(t)
		obj.Spec.RemoteRepositoryConfig.UpstreamCredentials.UsernamePasswordCredentials.Username = mockenv.Ptr("puller-2")
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, name)
		upc := readCredentials(t, env, name).GetUpstreamCredentials().GetUsernamePasswordCredentials()
		if upc.GetUsername() != "puller-2" || upc.GetPasswordSecretVersion() != version2 {
			t.Errorf("cloud credentials = %v, want puller-2 and %s", upc, version2)
		}
		masks := updateMasks(t, env, name)[before:]
		want := "remoteRepositoryConfig.upstreamCredentials.usernamePasswordCredentials.username"
		if len(masks) != 1 || masks[0] != want {
			t.Errorf("update masks = %q, want one: %s", masks, want)
		}
	})

	t.Run("delete removes the repository", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, repo); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&arv1beta1.ArtifactRegistryRepository{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := newClient(t, env).GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
		if status.Code(err) != codes.NotFound {
			t.Errorf("reading the deleted repository: got %v, want NotFound", err)
		}
	})
}
