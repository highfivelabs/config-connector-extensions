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

package discokindtest

// What the BigLake kinds need in the cloud, and what BigLake answers for a
// resource that does not exist: 403 PERMISSION_DENIED, not 404, for a
// catalog, a database or a table, even to an admin (live runs
// cce-11141253-209c and, in the catalog repository, cat-11141340-9f1f).
// A database needs its catalog and a table its catalog and database: the
// generated kind tests' Setup makes them through the API (the fixtures'
// ${ID}cat and ${ID}db, for the test's object and its abandoned one), in
// the fake and live alike, and deletes them when the test ends, after the
// kinds' own objects. BigLake IDs take letters, digits and underscores
// only (run cce-11141216-dd17 was refused a catalog ID with dashes), so
// every ID is a name without its dashes; the sweeper reads a run's name
// back from it.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	baseURL  = "https://biglake.googleapis.com/v1/"
	location = "us"
)

func init() {
	for _, c := range []struct{ key, base string }{
		{"BigLakeDatabase", "biglakedatabase"},
		{"BigLakeDatabase/live", "biglakedatabase-live"},
	} {
		kindtest.RegisterSetup(c.key, func(t *testing.T, env *mockenv.Env) {
			for _, b := range []string{c.base, c.base + "-kept"} {
				makeCatalog(t, env, id(b)+"cat")
			}
		})
	}
	kindtest.RegisterSetup("BigLakeTable", func(t *testing.T, env *mockenv.Env) {
		for _, b := range []string{"biglaketable", "biglaketable-kept"} {
			cat := makeCatalog(t, env, id(b)+"cat")
			makeDatabase(t, env, cat, id(b)+"db")
		}
	})
}

// id is mockenv.Name(base) without its dashes: a BigLake ID.
func id(base string) string { return strings.ReplaceAll(mockenv.Name(base), "-", "") }

func apiClient(env *mockenv.Env) *discokind.API {
	return &discokind.API{HTTP: env.HTTP, BaseURL: baseURL}
}

// makeCatalog makes the catalog id and deletes it when the test ends; it
// returns the catalog's name.
func makeCatalog(t *testing.T, env *mockenv.Env, id string) string {
	t.Helper()
	coll := "projects/" + mockenv.Project + "/locations/" + location + "/catalogs"
	make1(t, env, coll, "catalogId", id, map[string]any{})
	return coll + "/" + id
}

// makeDatabase makes the database id in catalog and deletes it when the
// test ends (before its catalog: cleanups run last-made first).
func makeDatabase(t *testing.T, env *mockenv.Env, catalog, id string) string {
	t.Helper()
	coll := catalog + "/databases"
	make1(t, env, coll, "databaseId", id, map[string]any{
		"type":        "HIVE",
		"hiveOptions": map[string]any{"locationUri": "gs://" + id + "/db", "parameters": map[string]any{"owner": "cce"}},
	})
	return coll + "/" + id
}

// make1 creates coll/id (BigLake answers the resource, not an operation)
// and deletes it when the test ends.
func make1(t *testing.T, env *mockenv.Env, coll, idParam, id string, body map[string]any) {
	t.Helper()
	api := apiClient(env)
	name := coll + "/" + id
	if _, err := api.Do(env.Ctx, http.MethodPost, coll, map[string]string{idParam: id}, body); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	t.Logf("created %s", name)
	t.Cleanup(func() { deleteGone(t, api, name) })
}

// deleteGone deletes name; gone already (BigLake's 403 or a 404) is done.
func deleteGone(t *testing.T, api *discokind.API, name string) {
	t.Helper()
	_, err := api.Do(context.Background(), http.MethodDelete, name, nil, nil)
	if err != nil && !direct.IsNotFound(err) && !direct.HasHTTPCode(err, http.StatusForbidden) {
		t.Errorf("cleanup: deleting %s: %v", name, err)
	}
}

// testMissingIs403 is what a kind with missingIs403 must do with BigLake's
// 403 for a resource that does not exist. setup makes the object's parents
// (live too) and returns its spec without resourceID. The object is made
// through the 403 its read before the create gets (in the fake as live;
// with the kind's missingIs403 off it never becomes Ready). Once made, a
// 403 is a denial again: the resource deleted behind the controller's back
// reads 403, the object goes not Ready with it, and the controller does
// not make the resource again.
func testMissingIs403(t *testing.T, kindName, base string, setup func(env *mockenv.Env) map[string]any) {
	ns := base
	env := mockenv.New(t, ns)
	k := discokind.ByKind(kindName)
	if k == nil {
		t.Fatalf("no discokind kind %s", kindName)
	}
	if !k.MissingIs403 {
		t.Errorf("%s does not have missingIs403 (hack/discokind/kinds.yaml)", kindName)
	}
	api := apiClient(env)
	gvk := k.GVK

	rid := id(base)
	spec := setup(env)
	spec["projectRef"] = map[string]any{"external": mockenv.Project}
	spec["location"] = location
	spec["resourceID"] = rid
	u := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(ns)
	u.SetName(mockenv.Name(base))
	if err := env.Kube.Create(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
	got := env.WaitReady(gvk, ns, u.GetName())
	name, _, _ := unstructured.NestedString(got.Object, "status", "externalRef")
	if name == "" {
		t.Fatal("status.externalRef is not set")
	}
	t.Cleanup(func() { deleteGone(t, api, name) })

	// The resource goes behind the controller's back; BigLake then answers
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
			if strings.HasPrefix(r, http.MethodPost+" ") && strings.Contains(r, k.IDParam+"="+rid) {
				n++
			}
		}
		return n
	}
	before := posts()

	// A resync: the 403 is a denial now, not a resource to make again.
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
		t.Errorf("the controller sent %d creates for a %s it had made, after a 403", n, kindName)
	}

	// Leave without a delete call (abandon reads nothing either). A delete
	// would read the same 403 and go only once its collection's listing
	// proves the resource gone (TestBigLakeDeleteDenied,
	// TestBigLakeDeleteAllAtOnce).
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

// TestBigLakeDatabaseLive is TestBigLakeDatabase on the fixtures in
// testdata/discokind/BigLakeDatabase/live/, with the catalogs the Setup
// makes: the kind test that runs live (TestBigLakeDatabase's fixtures keep
// their live-skip, which once said the test lacked its catalog).
func TestBigLakeDatabaseLive(t *testing.T) { kindtest.RunCase(t, "BigLakeDatabase", "live") }

// TestBigLakeDatabaseMissingIs403: a catalog, then a database in it
// (catalog repository run cat-11141340-9f1f: "Permission
// 'biglake.databases.get' denied" before the database's create).
func TestBigLakeDatabaseMissingIs403(t *testing.T) {
	const base = "biglakedatabase-403"
	testMissingIs403(t, "BigLakeDatabase", base, func(env *mockenv.Env) map[string]any {
		makeCatalog(t, env, id(base)+"cat")
		return map[string]any{
			"catalog":     id(base) + "cat",
			"type":        "HIVE",
			"hiveOptions": map[string]any{"locationUri": "gs://" + id(base) + "/db"},
		}
	})
}

// TestBigLakeTableMissingIs403: a catalog, a database, then a table in it
// (the same run: "biglake.tables.get denied" before the table's create,
// with Config Connector's own BigLakeTable).
func TestBigLakeTableMissingIs403(t *testing.T) {
	const base = "biglaketable-403"
	testMissingIs403(t, "BigLakeTable", base, func(env *mockenv.Env) map[string]any {
		cat := makeCatalog(t, env, id(base)+"cat")
		makeDatabase(t, env, cat, id(base)+"db")
		return map[string]any{
			"catalog":  id(base) + "cat",
			"database": id(base) + "db",
			"type":     "HIVE",
			"hiveOptions": map[string]any{
				"tableType": "EXTERNAL_TABLE",
				"storageDescriptor": map[string]any{
					"locationUri":  "gs://" + id(base) + "/table",
					"inputFormat":  "org.apache.hadoop.mapred.TextInputFormat",
					"outputFormat": "org.apache.hadoop.hive.ql.io.HiveIgnoreKeyTextOutputFormat",
				},
			},
		}
	})
}
