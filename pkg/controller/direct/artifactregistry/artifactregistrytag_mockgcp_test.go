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
// 0009). The case: a "prod" tag on an image, moved from one digest to the
// next when a release is promoted.

import (
	"net/url"
	"strings"
	"testing"
	"time"

	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	arv1beta1kcc "github.com/GoogleCloudPlatform/k8s-config-connector/apis/artifactregistry/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	arv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	digest1 = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	digest2 = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

func TestArtifactRegistryTag(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := newClient(t, env)
	tagGVK := arv1alpha1.ArtifactRegistryTagGVK

	// The repository, in the fake cloud and as a Config Connector object
	// (not reconciled here; read to resolve repositoryRef by name).
	const repo = "images"
	op, err := cloud.CreateRepository(env.Ctx, &pb.CreateRepositoryRequest{
		Parent:       "projects/" + mockenv.Project + "/locations/" + region,
		RepositoryId: repo,
		Repository:   &pb.Repository{Format: pb.Repository_DOCKER},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.Wait(env.Ctx); err != nil {
		t.Fatal(err)
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
			t.Fatalf("reading the tag from the fake cloud: %v", err)
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
			t.Errorf("the abandoned tag is gone from the fake cloud: %v", err)
		}
	})
}
