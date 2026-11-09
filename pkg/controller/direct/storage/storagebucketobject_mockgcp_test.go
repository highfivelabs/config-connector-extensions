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

package storage_test

// StorageBucketObject end to end on the fake cloud (pkg/test/mockenv, patch
// 0013) or a real project (docs/live-runs.md). The case: a function's
// configuration file and a startup script kept in a bucket, from the spec,
// a ConfigMap and a Secret. The bucket is the run's own (mockenv.Name, with
// mockenv.Labels) and the test deletes it at the end with every object in
// it, the abandoned one included.

import (
	"io"
	"strings"
	"testing"
	"time"

	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/storage/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	storagev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/storage/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

// bucket is the test's bucket; bucket names are global, so a live run's
// carries its run ID.
var bucket = mockenv.Name("app-config-bucket")

var gvk = storagev1alpha1.StorageBucketObjectGVK

func TestStorageBucketObject(t *testing.T) {
	env := mockenv.New(t, namespace)
	gcs, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	// The bucket, in the cloud and as a Config Connector object (not
	// reconciled here; read to resolve bucketRef by name).
	if _, err := gcs.Buckets.Insert(mockenv.Project, &api.Bucket{Name: bucket, Location: "US", Labels: mockenv.Labels()}).Context(env.Ctx).Do(); err != nil {
		t.Fatalf("creating the bucket: %v", err)
	}
	t.Cleanup(func() {
		objs, err := gcs.Objects.List(bucket).Context(env.Ctx).Do()
		if err != nil {
			t.Errorf("listing the bucket's objects: %v", err)
			return
		}
		for _, o := range objs.Items {
			if err := gcs.Objects.Delete(bucket, o.Name).Context(env.Ctx).Do(); err != nil && !strings.Contains(err.Error(), "404") {
				t.Errorf("deleting %s: %v", o.Name, err)
			}
		}
		if err := gcs.Buckets.Delete(bucket).Context(env.Ctx).Do(); err != nil && !strings.Contains(err.Error(), "404") {
			t.Errorf("deleting the bucket: %v", err)
		}
	})
	b := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"location": "US"}}}
	b.SetGroupVersionKind(storagev1beta1.StorageBucketGVK)
	b.SetNamespace(namespace)
	b.SetName(bucket)
	b.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Create(env.Ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "app"},
		Data:       map[string]string{"settings.json": `{"mode":"production"}`},
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Create(env.Ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "startup"},
		Data:       map[string][]byte{"script": []byte("#!/bin/sh\necho started\n")},
	}); err != nil {
		t.Fatal(err)
	}

	newObject := func(name, objectName string) *storagev1alpha1.StorageBucketObject {
		return &storagev1alpha1.StorageBucketObject{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: storagev1alpha1.StorageBucketObjectSpec{
				BucketRef:  &storagev1beta1.StorageBucketRef{Name: bucket},
				ResourceID: mockenv.Ptr(objectName),
			},
		}
	}
	read := func(t *testing.T, name string) (*api.Object, string) {
		t.Helper()
		o, err := gcs.Objects.Get(bucket, name).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading %s from the cloud: %v", name, err)
		}
		resp, err := gcs.Objects.Get(bucket, name).Context(env.Ctx).Download()
		if err != nil {
			t.Fatalf("downloading %s from the cloud: %v", name, err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return o, string(data)
	}
	uploads := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/upload/storage/v1/b/"+bucket+"/o") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *storagev1alpha1.StorageBucketObject {
		t.Helper()
		obj := &storagev1alpha1.StorageBucketObject{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	cfg := newObject("app-config", "config/app.yaml")
	cfg.Spec.Content = mockenv.Ptr("mode: production\n")
	cfg.Spec.ContentType = mockenv.Ptr("text/yaml")
	cfg.Spec.Metadata = map[string]string{"owner": "team-a"}
	if err := env.Kube.Create(env.Ctx, cfg); err != nil {
		t.Fatal(err)
	}

	var generation int64
	t.Run("create uploads the content", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "app-config")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != "projects/_/buckets/"+bucket+"/objects/config/app.yaml" {
			t.Errorf("status.externalRef = %q", ref)
		}
		o, data := read(t, "config/app.yaml")
		if data != "mode: production\n" || o.ContentType != "text/yaml" || o.Metadata["owner"] != "team-a" {
			t.Errorf("cloud object = %q, %q, %v", data, o.ContentType, o.Metadata)
		}
		generation = o.Generation
		if g, _, _ := unstructured.NestedInt64(u.Object, "status", "observedState", "generation"); g != generation {
			t.Errorf("status.observedState.generation = %d, want %d", g, generation)
		}
	})

	t.Run("a resync uploads nothing", func(t *testing.T) {
		before := uploads()
		o := getObj(t, "app-config")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "app-config")
		if uploads() != before {
			t.Errorf("a resync uploaded %d times", uploads()-before)
		}
	})

	t.Run("new content is uploaded as a new generation", func(t *testing.T) {
		before := uploads()
		o := getObj(t, "app-config")
		o.Spec.Content = mockenv.Ptr("mode: maintenance\n")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "app-config")
		got, data := read(t, "config/app.yaml")
		if data != "mode: maintenance\n" || got.Generation == generation {
			t.Errorf("cloud object after update = %q, generation %d (was %d)", data, got.Generation, generation)
		}
		if uploads()-before != 1 {
			t.Errorf("uploaded %d times, want once", uploads()-before)
		}
	})

	t.Run("content from a ConfigMap and a Secret", func(t *testing.T) {
		fromCM := newObject("settings", "settings.json")
		fromCM.Spec.ContentFrom = &storagev1alpha1.ContentSource{ConfigMapKeyRef: &storagev1alpha1.KeyRef{Name: "app", Key: "settings.json"}}
		fromSecret := newObject("startup", "scripts/startup.sh")
		fromSecret.Spec.ContentFrom = &storagev1alpha1.ContentSource{SecretKeyRef: &storagev1alpha1.KeyRef{Name: "startup", Key: "script"}}
		for _, o := range []*storagev1alpha1.StorageBucketObject{fromCM, fromSecret} {
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
		}
		env.WaitReady(gvk, namespace, "settings")
		env.WaitReady(gvk, namespace, "startup")
		if _, data := read(t, "settings.json"); data != `{"mode":"production"}` {
			t.Errorf("settings.json = %q", data)
		}
		if _, data := read(t, "scripts/startup.sh"); data != "#!/bin/sh\necho started\n" {
			t.Errorf("scripts/startup.sh = %q", data)
		}
	})

	t.Run("delete removes the object", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, cfg); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&storagev1alpha1.StorageBucketObject{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "app-config"}})
		if _, err := gcs.Objects.Get(bucket, "config/app.yaml").Context(env.Ctx).Do(); err == nil || !strings.Contains(err.Error(), "404") {
			t.Errorf("reading the deleted object: got %v, want 404", err)
		}
	})

	t.Run("abandon keeps the object", func(t *testing.T) {
		k := newObject("kept", "kept.txt")
		k.Spec.Content = mockenv.Ptr("kept")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if _, data := read(t, "kept.txt"); data != "kept" {
			t.Errorf("abandoned object = %q", data)
		}
	})
}
