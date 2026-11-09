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

// ArtifactRegistryTag end to end on the fake cloud (pkg/test/mockenv, patch
// 0009), or a real project with E2E_GCP_TARGET=real (make test-live). The
// case: a "prod" tag on an image, moved from one digest to the next when a
// release is promoted.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	arv1beta1kcc "github.com/GoogleCloudPlatform/k8s-config-connector/apis/artifactregistry/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	arv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// pushImage uploads a minimal OCI image (a config with one label, no
// layers) as package "app" through the Docker registry API of a real
// repository, and returns its manifest digest: a real tag must point at a
// version that exists. The fake cloud holds no images, so there the tests
// use made-up digests.
func pushImage(t *testing.T, env *mockenv.Env, repo, label string) string {
	t.Helper()
	host := region + "-docker.pkg.dev"
	name := mockenv.Project + "/" + repo + "/app"
	ts, err := google.DefaultTokenSource(env.Ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := ts.Token()
	if err != nil {
		t.Fatal(err)
	}
	// The registry's token exchange, as docker login does it.
	req, _ := http.NewRequestWithContext(env.Ctx, "GET", "https://"+host+"/v2/token?service="+host+"&scope=repository:"+name+":push,pull", nil)
	req.SetBasicAuth("oauth2accesstoken", tok.AccessToken)
	var registryToken struct {
		Token string `json:"token"`
	}
	do := func(req *http.Request, want int) *http.Response {
		t.Helper()
		if registryToken.Token != "" {
			req.Header.Set("Authorization", "Bearer "+registryToken.Token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("%s %s: %s %s", req.Method, req.URL, resp.Status, body)
		}
		return resp
	}
	resp := do(req, http.StatusOK)
	if err := json.NewDecoder(resp.Body).Decode(&registryToken); err != nil || registryToken.Token == "" {
		t.Fatalf("registry token: %v", err)
	}
	resp.Body.Close()

	digest := func(b []byte) string { sum := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(sum[:]) }
	config := []byte(`{"architecture":"amd64","os":"linux","config":{"Labels":{"cce-e2e":"` + label + `"}},"rootfs":{"type":"layers","diff_ids":[]}}`)
	req, _ = http.NewRequestWithContext(env.Ctx, "POST", "https://"+host+"/v2/"+name+"/blobs/uploads/", nil)
	resp = do(req, http.StatusAccepted)
	resp.Body.Close()
	loc, err := resp.Request.URL.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	q.Set("digest", digest(config))
	loc.RawQuery = q.Encode()
	req, _ = http.NewRequestWithContext(env.Ctx, "PUT", loc.String(), bytes.NewReader(config))
	req.Header.Set("Content-Type", "application/octet-stream")
	do(req, http.StatusCreated).Body.Close()

	manifest := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json",`+
		`"config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"%s","size":%d},"layers":[]}`, digest(config), len(config)))
	req, _ = http.NewRequestWithContext(env.Ctx, "PUT", "https://"+host+"/v2/"+name+"/manifests/"+label, bytes.NewReader(manifest))
	req.Header.Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	do(req, http.StatusCreated).Body.Close()
	return digest(manifest)
}

func TestArtifactRegistryTag(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := newClient(t, env)
	tagGVK := arv1alpha1.ArtifactRegistryTagGVK

	// The repository, in the cloud and as a Config Connector object (not
	// reconciled here; read to resolve repositoryRef by name). The test
	// deletes it, and the tags and images in it, at the end.
	repo := mockenv.Name("images")
	op, err := cloud.CreateRepository(env.Ctx, &pb.CreateRepositoryRequest{
		Parent:       "projects/" + mockenv.Project + "/locations/" + region,
		RepositoryId: repo,
		Repository:   &pb.Repository{Format: pb.Repository_DOCKER, Labels: mockenv.Labels()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.Wait(env.Ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		op, err := cloud.DeleteRepository(env.Ctx, &pb.DeleteRepositoryRequest{Name: cloudName(repo)})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && status.Code(err) != codes.NotFound {
			t.Errorf("deleting the repository: %v", err)
		}
	})
	digest1 := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	digest2 := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	if mockenv.Live() {
		digest1, digest2 = pushImage(t, env, repo, "v1"), pushImage(t, env, repo, "v2")
	}
	r := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"location": region, "format": "DOCKER"}}}
	r.SetGroupVersionKind(arv1beta1kcc.ArtifactRegistryRepositoryGVK)
	r.SetNamespace(namespace)
	r.SetName(repo)
	r.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, r); err != nil {
		t.Fatal(err)
	}

	pkg := cloudName(repo) + "/packages/app"
	tagName := pkg + "/tags/prod"
	newTag := func(name, version string) *arv1alpha1.ArtifactRegistryTag {
		return &arv1alpha1.ArtifactRegistryTag{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: arv1alpha1.ArtifactRegistryTagSpec{
				RepositoryRef: &arv1beta1kcc.ArtifactRegistryRepositoryRef{Name: repo},
				Package:       "app",
				Version:       version,
			},
		}
	}
	patches := func() []string {
		var out []string
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, "/v1/"+tagName+"?") {
				u, err := url.Parse(strings.TrimPrefix(req, "PATCH "))
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, u.Query().Get("updateMask"))
			}
		}
		return out
	}
	getObj := func(t *testing.T) *arv1alpha1.ArtifactRegistryTag {
		t.Helper()
		obj := &arv1alpha1.ArtifactRegistryTag{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "prod"}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	tag := newTag("prod", digest1)
	if err := env.Kube.Create(env.Ctx, tag); err != nil {
		t.Fatal(err)
	}

	t.Run("create points the tag at the version", func(t *testing.T) {
		u := env.WaitReady(tagGVK, namespace, "prod")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != tagName {
			t.Errorf("status.externalRef = %q, want %q", ref, tagName)
		}
		if v, _, _ := unstructured.NestedString(u.Object, "status", "observedVersion"); v != pkg+"/versions/"+digest1 {
			t.Errorf("status.observedVersion = %q", v)
		}
		got, err := cloud.GetTag(env.Ctx, &pb.GetTagRequest{Name: tagName})
		if err != nil {
			t.Fatalf("reading the tag from the cloud: %v", err)
		}
		if got.GetVersion() != pkg+"/versions/"+digest1 {
			t.Errorf("cloud tag version = %q, want %q", got.GetVersion(), pkg+"/versions/"+digest1)
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		before := len(patches())
		gets := func() int {
			n := 0
			for _, req := range env.Requests() {
				if strings.HasPrefix(req, "GET ") && strings.Contains(req, "/v1/"+tagName+"?") {
					n++
				}
			}
			return n
		}
		g := gets()
		o := getObj(t)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for gets() == g {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the tag again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(tagGVK, namespace, "prod")
		if p := patches()[before:]; len(p) != 0 {
			t.Errorf("updates after a resync: %q, want none", p)
		}
	})

	t.Run("a new version moves the tag", func(t *testing.T) {
		before := len(patches())
		o := getObj(t)
		o.Spec.Version = digest2
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(tagGVK, namespace, "prod")
		got, err := cloud.GetTag(env.Ctx, &pb.GetTagRequest{Name: tagName})
		if err != nil {
			t.Fatal(err)
		}
		if got.GetVersion() != pkg+"/versions/"+digest2 {
			t.Errorf("cloud tag version = %q, want the new digest", got.GetVersion())
		}
		if p := patches()[before:]; len(p) != 1 || p[0] != "version" {
			t.Errorf("update masks = %q, want one: version", p)
		}
	})

	t.Run("delete removes the tag", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, tag); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&arv1alpha1.ArtifactRegistryTag{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "prod"}})
		_, err := cloud.GetTag(env.Ctx, &pb.GetTagRequest{Name: tagName})
		if status.Code(err) != codes.NotFound {
			t.Errorf("reading the deleted tag: got %v, want NotFound", err)
		}
	})

	t.Run("abandon keeps the tag", func(t *testing.T) {
		k := newTag("stable", digest1)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(tagGVK, namespace, "stable")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if _, err := cloud.GetTag(env.Ctx, &pb.GetTagRequest{Name: pkg + "/tags/stable"}); err != nil {
			t.Errorf("the abandoned tag is gone from the cloud: %v", err)
		}
	})
}
