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

package datacatalog_test

// DataCatalogTag end to end on the fake cloud (pkg/test/mockenv) or a real
// project (docs/live-runs.md). The case: a data platform tags a dataset's
// catalog entry with its owner, as the catalog's data-catalog-tag module
// does. The test makes the tag template, the entry group and the entry
// through the API, then the tag as an object. The tag's ID is the API's,
// so the test checks that the object finds it again by status.externalRef,
// and that a second object acquires a tag by resourceID.
//
// A live run needs datacatalog.googleapis.com on and roles/datacatalog.admin
// for the live-test service account; it skips the test unless
// E2E_DATA_CATALOG=true.

import (
	"os"
	"strings"
	"testing"
	"time"

	api "cloud.google.com/go/datacatalog/apiv1"
	pb "cloud.google.com/go/datacatalog/apiv1/datacatalogpb"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/datacatalog/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/iterator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	datacatalogv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/datacatalog/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	location  = "us-central1"
)

func TestDataCatalogTag(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_DATA_CATALOG") != "true" {
		t.Skip("a live DataCatalogTag needs datacatalog.googleapis.com on and roles/datacatalog.admin for the live-test service account; set E2E_DATA_CATALOG=true where it has them (docs/live-runs.md)")
	}
	gvk := datacatalogv1beta1.DataCatalogTagGVK
	env := mockenv.New(t, namespace)
	cloud, err := api.NewRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	parent := "projects/" + mockenv.Project + "/locations/" + location
	// A tag template ID takes letters, digits and underscores; an entry
	// group's and an entry's take no dashes either.
	id := func(base string) string { return strings.ReplaceAll(mockenv.Name(base), "-", "_") }
	template := parent + "/tagTemplates/" + id("owners")
	group := parent + "/entryGroups/" + id("lake")
	entry := group + "/entries/" + id("orders")

	waitFor := func(t *testing.T, what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	listTags := func(t *testing.T) []*pb.Tag {
		t.Helper()
		var out []*pb.Tag
		it := cloud.ListTags(env.Ctx, &pb.ListTagsRequest{Parent: entry})
		for {
			tag, err := it.Next()
			if err == iterator.Done {
				return out
			}
			if err != nil {
				t.Fatalf("listing tags of %s: %v", entry, err)
			}
			out = append(out, tag)
		}
	}
	findTag := func(t *testing.T, name string) *pb.Tag {
		t.Helper()
		for _, tag := range listTags(t) {
			if tag.Name == name {
				return tag
			}
		}
		return nil
	}
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "PATCH ")) && strings.Contains(r, "/tags") && !strings.Contains(r, "/tagTemplates") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, n string) *datacatalogv1beta1.DataCatalogTag {
		t.Helper()
		obj := &datacatalogv1beta1.DataCatalogTag{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: n}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	externalRef := func(t *testing.T, u *unstructured.Unstructured) string {
		t.Helper()
		ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		if !strings.HasPrefix(ref, entry+"/tags/") || len(ref) == len(entry+"/tags/") {
			t.Fatalf("status.externalRef = %q, want %s/tags/<the API's ID>", ref, entry)
		}
		return ref
	}

	// The template, the entry group and the entry, deleted last (cleanups
	// run last first), after any tag a failed step left.
	if _, err := cloud.CreateTagTemplate(env.Ctx, &pb.CreateTagTemplateRequest{
		Parent:        parent,
		TagTemplateId: id("owners"),
		TagTemplate: &pb.TagTemplate{
			DisplayName: "Owners",
			Fields: map[string]*pb.TagTemplateField{
				"owner": {DisplayName: "Owner", Type: &pb.FieldType{TypeDecl: &pb.FieldType_PrimitiveType_{PrimitiveType: pb.FieldType_STRING}}},
			},
		},
	}); err != nil {
		t.Fatalf("making tag template: %v", err)
	}
	t.Cleanup(func() {
		if err := cloud.DeleteTagTemplate(env.Ctx, &pb.DeleteTagTemplateRequest{Name: template, Force: true}); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting %s: %v", template, err)
		}
	})
	if _, err := cloud.CreateEntryGroup(env.Ctx, &pb.CreateEntryGroupRequest{
		Parent: parent, EntryGroupId: id("lake"), EntryGroup: &pb.EntryGroup{DisplayName: "lake"},
	}); err != nil {
		t.Fatalf("making entry group: %v", err)
	}
	t.Cleanup(func() {
		if err := cloud.DeleteEntryGroup(env.Ctx, &pb.DeleteEntryGroupRequest{Name: group, Force: true}); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting %s: %v", group, err)
		}
	})
	if _, err := cloud.CreateEntry(env.Ctx, &pb.CreateEntryRequest{
		Parent: group, EntryId: id("orders"),
		Entry: &pb.Entry{
			DisplayName:    "orders",
			EntryType:      &pb.Entry_UserSpecifiedType{UserSpecifiedType: "dataset"},
			System:         &pb.Entry_UserSpecifiedSystem{UserSpecifiedSystem: "cce_tests"},
			LinkedResource: "//example.com/" + id("orders"),
		},
	}); err != nil {
		t.Fatalf("making entry: %v", err)
	}
	t.Cleanup(func() {
		if err := cloud.DeleteEntry(env.Ctx, &pb.DeleteEntryRequest{Name: entry}); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting %s: %v", entry, err)
		}
	})
	t.Cleanup(func() {
		for _, tag := range listTags(t) {
			if err := cloud.DeleteTag(env.Ctx, &pb.DeleteTagRequest{Name: tag.Name}); err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up %s: %v", tag.Name, err)
			}
		}
	})

	newTag := func(n, owner string) *datacatalogv1beta1.DataCatalogTag {
		return &datacatalogv1beta1.DataCatalogTag{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n, Labels: mockenv.Labels()},
			Spec: datacatalogv1beta1.DataCatalogTagSpec{
				DataCatalogTagParent: datacatalogv1beta1.DataCatalogTagParent{EntryRef: &upstream.EntryRef{External: entry}},
				TemplateRef:          &upstream.TagTemplateRef{External: template},
				Fields:               map[string]upstream.TagField{"owner": {StringValue: mockenv.Ptr(owner)}},
			},
		}
	}

	if err := env.Kube.Create(env.Ctx, newTag("orders-owner", "data-platform")); err != nil {
		t.Fatal(err)
	}
	var tagName string

	t.Run("create tags the entry, with the API's ID", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "orders-owner")
		tagName = externalRef(t, u)
		got := findTag(t, tagName)
		if got == nil {
			t.Fatalf("no tag %s on %s", tagName, entry)
		}
		if got.Template != template || got.Fields["owner"].GetStringValue() != "data-platform" {
			t.Errorf("tag = %v", got)
		}
		if w := writes(); len(w) != 1 {
			t.Errorf("writes = %v, want one create", w)
		}
	})

	t.Run("a resync finds the tag by its ID and writes nothing", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "orders-owner")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		u := env.WaitReady(gvk, namespace, "orders-owner")
		if got := externalRef(t, u); got != tagName {
			t.Errorf("status.externalRef = %q, want %q (the same tag)", got, tagName)
		}
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("a changed field is one update of the fields, and keeps the ID", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "orders-owner")
		o.Spec.Fields = map[string]upstream.TagField{"owner": {StringValue: mockenv.Ptr("analytics")}}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the owner to change", func() bool {
			got := findTag(t, tagName)
			return got != nil && got.Fields["owner"].GetStringValue() == "analytics"
		})
		u := env.WaitReady(gvk, namespace, "orders-owner")
		if got := externalRef(t, u); got != tagName {
			t.Errorf("after the update, status.externalRef = %q, want %q", got, tagName)
		}
		w := writes()[before:]
		if len(w) != 1 || !strings.HasPrefix(w[0], "PATCH ") || !strings.Contains(w[0], "updateMask=fields") {
			t.Errorf("writes = %v, want one PATCH with updateMask=fields", w)
		}
		if n := len(listTags(t)); n != 1 {
			t.Errorf("%d tags on the entry, want 1", n)
		}
	})

	t.Run("the CRD refuses a change of template", func(t *testing.T) {
		o := getObj(t, "orders-owner")
		o.Spec.TemplateRef = &upstream.TagTemplateRef{External: parent + "/tagTemplates/another"}
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "templateRef is immutable") {
			t.Errorf("changing templateRef: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("abandon keeps the tag", func(t *testing.T) {
		env.WaitReady(gvk, namespace, "orders-owner")
		o := getObj(t, "orders-owner")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&datacatalogv1beta1.DataCatalogTag{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "orders-owner"}})
		if findTag(t, tagName) == nil {
			t.Errorf("after abandon, the tag %s is gone, want it kept", tagName)
		}
	})

	t.Run("resourceID acquires the abandoned tag, and delete deletes it", func(t *testing.T) {
		before := len(writes())
		o := newTag("acquired", "analytics")
		o.Spec.ResourceID = mockenv.Ptr(tagName[strings.LastIndex(tagName, "/")+1:])
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "acquired")
		if got := externalRef(t, u); got != tagName {
			t.Errorf("status.externalRef = %q, want %q (the abandoned tag)", got, tagName)
		}
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("acquiring an unchanged tag wrote %v", w)
		}
		if err := env.Kube.Delete(env.Ctx, getObj(t, "acquired")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&datacatalogv1beta1.DataCatalogTag{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "acquired"}})
		if findTag(t, tagName) != nil {
			t.Errorf("after delete, the tag %s is still there", tagName)
		}
	})
}
