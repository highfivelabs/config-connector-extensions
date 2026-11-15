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

package asset_test

// AssetFeed end to end on the fake cloud (pkg/test/mockenv) or a real
// project (docs/live-runs.md). The case: a landing zone's project sends its
// IAM policy changes to a Pub/Sub topic, as the catalog's project chart
// renders fabric's asset_feeds. Google Cloud names the feed
// projects/<number>/feeds/<id> while the projectRef names the project by
// its ID (or a Project object); Config Connector's AssetFeed refuses every
// reconcile after the create for that (catalog PR #215) and its feed can
// never be deleted. Ours treats the number and the ID as the same project.
//
// A live run needs cloudasset.feeds.* for the live-test service account
// (roles/cloudasset.owner; it has roles/cloudasset.viewer and
// roles/pubsub.admin, and both APIs are on); it skips the test unless
// E2E_CLOUD_ASSET=true (docs/live-runs.md). The folder's and the
// organization's feeds run in the fake cloud only.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/asset/apiv1"
	pb "cloud.google.com/go/asset/apiv1/assetpb"
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"google.golang.org/api/cloudresourcemanager/v1"
	pubsub "google.golang.org/api/pubsub/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	assetv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/asset/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

func TestAssetFeed(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_CLOUD_ASSET") != "true" {
		t.Skip("a live AssetFeed needs cloudasset.feeds.* (roles/cloudasset.owner) for the live-test service account; set E2E_CLOUD_ASSET=true where it has it (docs/live-runs.md)")
	}
	feedGVK := assetv1beta1.AssetFeedGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })
	crm, err := cloudresourcemanager.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	p, err := crm.Projects.Get(mockenv.Project).Context(env.Ctx).Do()
	if err != nil {
		t.Fatal(err)
	}
	number := fmt.Sprintf("%d", p.ProjectNumber)
	ps, err := pubsub.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	topic := "projects/" + mockenv.Project + "/topics/" + mockenv.Name("asset-feed")
	if _, err := ps.Projects.Topics.Create(topic, &pubsub.Topic{Labels: mockenv.Labels()}).Context(env.Ctx).Do(); err != nil {
		t.Fatalf("creating the topic: %v", err)
	}

	byNumber := func(base string) string { return "projects/" + number + "/feeds/" + mockenv.Name(base) }
	get := func(n string) (*pb.Feed, error) { return cloud.GetFeed(env.Ctx, &pb.GetFeedRequest{Name: n}) }
	mustGet := func(t *testing.T, n string) *pb.Feed {
		t.Helper()
		f, err := get(n)
		if err != nil {
			t.Fatalf("reading %s: %v", n, err)
		}
		return f
	}
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "PATCH ") || strings.HasPrefix(r, "DELETE ")) && strings.Contains(r, "/feeds") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, n string) *assetv1beta1.AssetFeed {
		t.Helper()
		obj := &assetv1beta1.AssetFeed{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: n}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
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
	// Delete what the test made in the project, the abandoned feed
	// included, and what a failed step left; then the topic.
	t.Cleanup(func() {
		for _, base := range []string{"iam-changes", "by-object", "kept"} {
			err := cloud.DeleteFeed(env.Ctx, &pb.DeleteFeedRequest{Name: byNumber(base)})
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up %s: %v", byNumber(base), err)
			}
		}
		if _, err := ps.Projects.Topics.Delete(topic).Context(env.Ctx).Do(); err != nil {
			t.Errorf("deleting the topic: %v", err)
		}
	})

	newFeed := func(n string) *assetv1beta1.AssetFeed {
		return &assetv1beta1.AssetFeed{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n, Labels: mockenv.Labels()},
			Spec: assetv1beta1.AssetFeedSpec{
				ResourceID:  mockenv.Ptr(mockenv.Name(n)),
				ContentType: mockenv.Ptr("IAM_POLICY"),
				AssetTypes:  []string{"cloudresourcemanager.googleapis.com/Project"},
				FeedOutputConfig: &assetv1beta1.FeedOutputConfig{
					PubsubDestination: &assetv1beta1.PubsubDestination{TopicRef: &pubsubv1beta1.PubSubTopicRef{External: topic}},
				},
			},
		}
	}

	feed := newFeed("iam-changes")
	feed.Spec.AssetFeedParent.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
	if err := env.Kube.Create(env.Ctx, feed); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the project's feed, named by the project's number", func(t *testing.T) {
		u := env.WaitReady(feedGVK, namespace, "iam-changes")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != byNumber("iam-changes") {
			t.Errorf("status.externalRef = %q, want %q", ref, byNumber("iam-changes"))
		}
		got := mustGet(t, byNumber("iam-changes"))
		if got.ContentType != pb.ContentType_IAM_POLICY || got.GetFeedOutputConfig().GetPubsubDestination().GetTopic() != topic {
			t.Errorf("feed = %v", got)
		}
	})

	// The bug: upstream's identity compares status.externalRef's
	// projects/<number> with the projectRef's ID and refuses this reconcile
	// (and every later one, the delete included).
	t.Run("a changed spec after the create is one update of it", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "iam-changes")
		o.Spec.AssetTypes = []string{"cloudresourcemanager.googleapis.com/Project", "storage.googleapis.com/Bucket"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the asset types to change", func() bool {
			f, err := get(byNumber("iam-changes"))
			return err == nil && len(f.AssetTypes) == 2
		})
		env.WaitReady(feedGVK, namespace, "iam-changes")
		w := writes()[before:]
		if len(w) != 1 || !strings.HasPrefix(w[0], "PATCH ") {
			t.Errorf("writes = %v, want one PATCH of asset_types", w)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "iam-changes")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/reconcile-interval-in-seconds": "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(4 * time.Second)
		env.WaitReady(feedGVK, namespace, "iam-changes")
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("the CRD refuses a change of parent", func(t *testing.T) {
		o := getObj(t, "iam-changes")
		o.Spec.AssetFeedParent.ProjectRef = &refsv1beta1.ProjectRef{External: "another-project"}
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "projectRef is immutable") {
			t.Errorf("changing projectRef: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("the CRD refuses none or two parents", func(t *testing.T) {
		none := newFeed("no-parent")
		two := newFeed("two-parents")
		two.Spec.AssetFeedParent.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
		two.Spec.AssetFeedParent.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/123456"}
		for _, o := range []*assetv1beta1.AssetFeed{none, two} {
			err := env.Kube.Create(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef, folderRef and projectRef") {
				t.Errorf("creating %s: got %v, want the API server to refuse it", o.Name, err)
			}
		}
	})

	t.Run("delete deletes the feed", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "iam-changes")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&assetv1beta1.AssetFeed{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "iam-changes"}})
		if _, err := get(byNumber("iam-changes")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the feed: %v, want not found", err)
		}
	})

	// The catalog's other failure: the projectRef names a Project object,
	// and everything is deleted at once, the Project object first.
	t.Run("a feed whose Project object is deleted first", func(t *testing.T) {
		projectGVK := schema.GroupVersionKind{Group: "resourcemanager.cnrm.cloud.google.com", Version: "v1beta1", Kind: "Project"}
		proj := &unstructured.Unstructured{}
		proj.SetGroupVersionKind(projectGVK)
		proj.SetNamespace(namespace)
		proj.SetName("landing")
		_ = unstructured.SetNestedField(proj.Object, mockenv.Project, "spec", "resourceID")
		_ = unstructured.SetNestedField(proj.Object, "landing zone", "spec", "name")
		_ = unstructured.SetNestedField(proj.Object, "123456789", "spec", "organizationRef", "external")
		if err := env.Kube.Create(env.Ctx, proj); err != nil {
			t.Fatal(err)
		}
		o := newFeed("by-object")
		o.Spec.AssetFeedParent.ProjectRef = &refsv1beta1.ProjectRef{Name: "landing"}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(feedGVK, namespace, "by-object")
		mustGet(t, byNumber("by-object"))
		// No Project controller runs here, so the Project object goes at once.
		if err := env.Kube.Delete(env.Ctx, proj); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, getObj(t, "by-object")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&assetv1beta1.AssetFeed{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "by-object"}})
		if _, err := get(byNumber("by-object")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the feed: %v, want not found", err)
		}
	})

	// Never made: its Project object never existed, so it never became
	// Ready, and its delete makes no call.
	t.Run("a feed whose Project object never existed is deleted without a call", func(t *testing.T) {
		o := newFeed("orphan")
		o.Spec.AssetFeedParent.ProjectRef = &refsv1beta1.ProjectRef{Name: "no-such-project"}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(feedGVK, namespace, "orphan"); !strings.Contains(msg, "not found") {
			t.Errorf("not-Ready message = %q, want the missing Project", msg)
		}
		before := len(writes())
		if err := env.Kube.Delete(env.Ctx, getObj(t, "orphan")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&assetv1beta1.AssetFeed{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "orphan"}})
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("the delete wrote %v", w)
		}
	})

	t.Run("a folder's and an organization's feeds", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never makes a folder's or an organization's feed (docs/live-runs.md)")
		}
		folder := newFeed("sandbox-folder")
		folder.Spec.AssetFeedParent.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/123456"}
		org := newFeed("org-wide")
		org.Spec.AssetFeedParent.OrganizationRef = &refsv1beta1.OrganizationRef{External: "organizations/987654"}
		for _, o := range []*assetv1beta1.AssetFeed{folder, org} {
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
		}
		for n, parent := range map[string]string{"sandbox-folder": "folders/123456", "org-wide": "organizations/987654"} {
			want := parent + "/feeds/" + n
			u := env.WaitReady(feedGVK, namespace, n)
			if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
				t.Errorf("%s: status.externalRef = %q, want %q", n, ref, want)
			}
			mustGet(t, want)
			if err := env.Kube.Delete(env.Ctx, getObj(t, n)); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(&assetv1beta1.AssetFeed{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n}})
			if _, err := get(want); !direct.IsNotFound(err) {
				t.Errorf("after delete, reading %s: %v, want not found", want, err)
			}
		}
	})

	t.Run("abandon keeps the feed", func(t *testing.T) {
		kept := newFeed("kept")
		kept.Spec.AssetFeedParent.ProjectRef = &refsv1beta1.ProjectRef{External: "projects/" + mockenv.Project}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(feedGVK, namespace, "kept")
		kept = getObj(t, "kept")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&assetv1beta1.AssetFeed{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(byNumber("kept")); err != nil {
			t.Errorf("after abandon, reading the feed: %v, want it kept", err)
		}
	})
}
