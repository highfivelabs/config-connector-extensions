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

package biglake_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	biglakev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/biglake/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// TestBigLakeCatalogMissingIs403 is what BigLake answers for a catalog that
// does not exist: 403, not 404, even to an admin (the catalog's live run in
// hfl-dev-514205, where the controller's read before its create took the
// 403 for a denial and never made the catalog). TestBigLakeCatalog
// covers the create through that 403 (the fake answers it too). Here: once
// the object has made its catalog, a 403 is a denial again. The catalog
// deleted behind the controller's back reads 403; the object goes not
// Ready with it, and the controller does not make the catalog again.
func TestBigLakeCatalogMissingIs403(t *testing.T) {
	const ns = "biglake-403"
	env := mockenv.New(t, ns)
	k := discokind.ByKind("BigLakeCatalog")
	api := &discokind.API{HTTP: env.HTTP, BaseURL: k.BaseURL}
	gvk := biglakev1beta1.BigLakeCatalogGVK

	u := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"projectRef": map[string]any{"external": mockenv.Project},
			"location":   "us",
		},
	}}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(ns)
	u.SetName(mockenv.Name("biglakecatalog-403"))
	if err := env.Kube.Create(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
	got := env.WaitReady(gvk, ns, u.GetName())
	name, _, _ := unstructured.NestedString(got.Object, "status", "externalRef")
	if name == "" {
		t.Fatal("status.externalRef is not set")
	}
	t.Cleanup(func() {
		if _, err := api.Do(env.Ctx, http.MethodDelete, name, nil, nil); err != nil && !direct.IsNotFound(err) && !direct.HasHTTPCode(err, http.StatusForbidden) {
			t.Errorf("cleanup: deleting %s: %v", name, err)
		}
	})

	// The catalog goes behind the controller's back; BigLake then answers
	// 403 for it.
	if _, err := api.Do(env.Ctx, http.MethodDelete, name, nil, nil); err != nil {
		t.Fatalf("deleting %s: %v", name, err)
	}
	if _, err := api.Get(env.Ctx, name); !direct.HasHTTPCode(err, http.StatusForbidden) {
		t.Fatalf("reading the deleted %s: err = %v, want 403", name, err)
	}
	posts := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, http.MethodPost+" ") && strings.Contains(r, "catalogId="+u.GetName()) {
				n++
			}
		}
		return n
	}
	before := posts()

	// A resync: the 403 is a denial now, not a catalog to make again.
	if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(u), u); err != nil {
		t.Fatal(err)
	}
	ann := u.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
	u.SetAnnotations(ann)
	if err := env.Kube.Update(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
	msg := env.WaitNotReady(gvk, ns, u.GetName())
	if !strings.Contains(msg, "403") {
		t.Errorf("Ready's message = %q, want the 403", msg)
	}
	if n := posts() - before; n != 0 {
		t.Errorf("the controller sent %d creates for a catalog it had made, after a 403", n)
	}

	// Leave without a delete call (abandon reads nothing either): a delete
	// would read the same 403 and, rightly, keep the finalizer.
	if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(u), u); err != nil {
		t.Fatal(err)
	}
	ann = u.GetAnnotations()
	ann[k8s.DeletionPolicyAnnotation] = k8s.DeletionPolicyAbandon
	u.SetAnnotations(ann)
	if err := env.Kube.Update(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Delete(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
	env.WaitGone(u)
}
