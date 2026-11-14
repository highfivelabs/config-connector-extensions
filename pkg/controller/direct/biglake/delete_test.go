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

// Deleting BigLake objects whose resources BigLake has already deleted:
// a catalog's delete takes its databases and tables along, and they then
// answer 403, not 404 (the catalog repository's live run
// cat-11141549-4cb0 deleted its whole biglake-catalog chart at once, and
// the database's and table's finalizers held for 30 minutes). A delete
// takes such a 403 for gone only when a listing proves it
// (discokind.GoneAfter403); a real denial keeps the finalizer.

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// newObject makes an object of the BigLake kind with spec (projectRef and
// location added) in ns.
func newObject(t *testing.T, env *mockenv.Env, kind, ns, name string, spec map[string]any) *unstructured.Unstructured {
	t.Helper()
	k := discokind.ByKind(kind)
	if k == nil {
		t.Fatalf("no discokind kind %s", kind)
	}
	spec["projectRef"] = map[string]any{"external": mockenv.Project}
	spec["location"] = location
	u := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	u.SetGroupVersionKind(k.GVK)
	u.SetNamespace(ns)
	u.SetName(name)
	u.SetLabels(mockenv.Labels())
	if err := env.Kube.Create(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
	return u
}

// readyName waits for u to be Ready and returns its resource's name.
func readyName(t *testing.T, env *mockenv.Env, u *unstructured.Unstructured) string {
	t.Helper()
	got := env.WaitReady(u.GroupVersionKind(), u.GetNamespace(), u.GetName())
	name, _, _ := unstructured.NestedString(got.Object, "status", "externalRef")
	if name == "" {
		t.Fatalf("%s %s: status.externalRef is not set", u.GetKind(), u.GetName())
	}
	return name
}

func tableSpec(cat, db, loc string) map[string]any {
	return map[string]any{
		"catalog":  cat,
		"database": db,
		"type":     "HIVE",
		"hiveOptions": map[string]any{
			"tableType": "EXTERNAL_TABLE",
			"storageDescriptor": map[string]any{
				"locationUri":  loc,
				"inputFormat":  "org.apache.hadoop.mapred.TextInputFormat",
				"outputFormat": "org.apache.hadoop.hive.ql.io.HiveIgnoreKeyTextOutputFormat",
			},
		},
	}
}

// TestBigLakeDeleteAllAtOnce: a BigLakeCatalog, a BigLakeDatabase in it and
// a BigLakeTable in that, all three deleted at once, as a chart's delete
// does. The catalog's object is deleted first, and the others as soon as
// BigLake has deleted the catalog, which took the database and table
// along: their reads and deletes then answer 403, and their finalizers
// must still go (listing the project's catalogs proves the catalog gone).
// Without the fix (go test -tags cce_delete403_off) they hold, and the
// test fails. Fake and live.
func TestBigLakeDeleteAllAtOnce(t *testing.T) {
	const base = "biglake-all"
	ns := base
	env := mockenv.New(t, ns)
	api := apiClient(env)
	rid := id(base)

	cat := newObject(t, env, "BigLakeCatalog", ns, mockenv.Name(base+"-cat"), map[string]any{"resourceID": rid + "cat"})
	catName := readyName(t, env, cat)
	t.Cleanup(func() { deleteGone(t, api, catName) })
	db := newObject(t, env, "BigLakeDatabase", ns, mockenv.Name(base+"-db"), map[string]any{
		"resourceID":  rid + "db",
		"catalog":     rid + "cat",
		"type":        "HIVE",
		"hiveOptions": map[string]any{"locationUri": "gs://" + rid + "/db"},
	})
	dbName := readyName(t, env, db)
	t.Cleanup(func() { deleteGone(t, api, dbName) })
	tbl := newObject(t, env, "BigLakeTable", ns, mockenv.Name(base+"-table"), func() map[string]any {
		s := tableSpec(rid+"cat", rid+"db", "gs://"+rid+"/table")
		s["resourceID"] = rid + "table"
		return s
	}())
	tblName := readyName(t, env, tbl)
	t.Cleanup(func() { deleteGone(t, api, tblName) })

	if err := env.Kube.Delete(env.Ctx, cat); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(mockenv.Timeout())
	for {
		_, err := api.Get(env.Ctx, catName)
		if direct.HasHTTPCode(err, http.StatusForbidden) || direct.IsNotFound(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the catalog %s was not deleted: last read err = %v", catName, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	// BigLake deleted the database and table with their catalog.
	for _, n := range []string{dbName, tblName} {
		if _, err := api.Get(env.Ctx, n); !direct.HasHTTPCode(err, http.StatusForbidden) {
			t.Errorf("reading %s after its catalog's delete: err = %v, want 403", n, err)
		}
	}
	for _, u := range []*unstructured.Unstructured{db, tbl} {
		if err := env.Kube.Delete(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []*unstructured.Unstructured{cat, db, tbl} {
		env.WaitGone(u)
	}

	// Nothing is left: the project's catalogs in the location list without
	// the catalog.
	out, err := api.Do(env.Ctx, http.MethodGet, discokind.CollectionOf(catName), map[string]string{"pageSize": "1000"}, nil)
	if err != nil {
		t.Fatalf("listing catalogs: %v", err)
	}
	items, _ := out["catalogs"].([]any)
	for _, it := range items {
		if m, _ := it.(map[string]any); m["name"] == catName {
			t.Errorf("the catalog %s is still listed: %v", catName, m)
		}
	}
}

// waitMessage waits until u's Ready condition's message contains want,
// and returns it.
func waitMessage(t *testing.T, env *mockenv.Env, u *unstructured.Unstructured, want string) string {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	last := ""
	for time.Now().Before(deadline) {
		got := &unstructured.Unstructured{}
		got.SetGroupVersionKind(u.GroupVersionKind())
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: u.GetNamespace(), Name: u.GetName()}, got); err != nil {
			t.Fatal(err)
		}
		_, last = mockenv.Ready(got)
		if strings.Contains(last, want) {
			return last
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("%s %s: Ready's message never had %q; last %q", u.GetKind(), u.GetName(), want, last)
	return ""
}

// TestBigLakeDeleteDenied: a 403 while deleting that a listing does not
// prove gone is a denial, and the finalizer stays. The fake denies one
// table (SetDeny) whose database still lists it: the delete fails with
// the 403 and the object stays until the denial is lifted. A second table,
// deleted behind the controller's back while its database stays, is
// proven gone by its own collection's listing. Fake only: a live run
// cannot take a permission away from one table.
func TestBigLakeDeleteDenied(t *testing.T) {
	if mockenv.Live() {
		t.Skip("needs the fake's SetDeny")
	}
	const base = "biglake-denied"
	ns := base
	env := mockenv.New(t, ns)
	api := apiClient(env)
	rid := id(base)
	catName := makeCatalog(t, env, rid+"cat")
	makeDatabase(t, env, catName, rid+"db")

	denied := newObject(t, env, "BigLakeTable", ns, mockenv.Name(base+"-a"), func() map[string]any {
		s := tableSpec(rid+"cat", rid+"db", "gs://"+rid+"/a")
		s["resourceID"] = rid + "a"
		return s
	}())
	deniedName := readyName(t, env, denied)
	t.Cleanup(func() { deleteGone(t, api, deniedName) })
	gone := newObject(t, env, "BigLakeTable", ns, mockenv.Name(base+"-b"), func() map[string]any {
		s := tableSpec(rid+"cat", rid+"db", "gs://"+rid+"/b")
		s["resourceID"] = rid + "b"
		return s
	}())
	goneName := readyName(t, env, gone)

	k := discokind.ByKind("BigLakeTable")
	env.Disco.SetDeny(k, deniedName, true)
	if err := env.Kube.Delete(env.Ctx, denied); err != nil {
		t.Fatal(err)
	}
	msg := waitMessage(t, env, denied, "not taken for gone")
	if !strings.Contains(msg, "403") || !strings.Contains(msg, "shows") {
		t.Errorf("Ready's message = %q, want the 403 and the listing that shows the table", msg)
	}
	// Still held: the finalizer did not go.
	time.Sleep(2 * time.Second)
	if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: ns, Name: denied.GetName()}, denied); err != nil {
		t.Fatalf("the denied table's object went: %v", err)
	}
	env.Disco.SetDeny(k, deniedName, false)
	env.WaitGone(denied)

	// Deleted behind the controller's back, its database still there.
	if _, err := api.Do(env.Ctx, http.MethodDelete, goneName, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Delete(env.Ctx, gone); err != nil {
		t.Fatal(err)
	}
	env.WaitGone(gone)
}
