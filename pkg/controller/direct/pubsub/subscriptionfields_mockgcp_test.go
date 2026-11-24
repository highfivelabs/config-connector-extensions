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

package pubsub_test

// PubSubSubscriptionFields end to end on the fake cloud (pkg/test/mockenv,
// patch 0065) or a real project (docs/live-runs.md). The topic and pull
// subscription are the run's own (mockenv.Name), cost nothing with no
// messages, and are deleted at the end. Live, the export cases skip: a
// Bigtable export needs an instance (which costs money), and the BigQuery
// and Cloud Storage exports a table or bucket the project's Pub/Sub
// service agent can write to.

import (
	"strings"
	"testing"
	"time"

	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/pubsub/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/pubsub/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestPubSubSubscriptionFields(t *testing.T) {
	gvk := krm.PubSubSubscriptionFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	project := "projects/" + mockenv.Project
	topicName := project + "/topics/" + mockenv.Name("sf")
	if _, err := c.Projects.Topics.Create(topicName, &api.Topic{}).Context(env.Ctx).Do(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := c.Projects.Topics.Delete(topicName).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting topic %s: %v", topicName, err)
		}
	})
	get := func(t *testing.T, name string) *api.Subscription {
		t.Helper()
		got, err := c.Projects.Subscriptions.Get(name).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the subscription %s: %v", name, err)
		}
		return got
	}
	// makeSubscription makes a subscription of the topic and waits until
	// it reads back (creation is eventually consistent).
	makeSubscription := func(t *testing.T, id string, s *api.Subscription) string {
		t.Helper()
		name := project + "/subscriptions/" + id
		s.Topic = topicName
		if _, err := c.Projects.Subscriptions.Create(name, s).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := c.Projects.Subscriptions.Delete(name).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting subscription %s: %v", name, err)
			}
		})
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			_, err := c.Projects.Subscriptions.Get(name).Context(env.Ctx).Do()
			if err == nil {
				return name
			}
			if !direct.IsNotFound(err) || time.Now().After(deadline) {
				t.Fatalf("reading the new subscription %s: %v", name, err)
			}
			time.Sleep(time.Second)
		}
	}
	patches := func(name string) int {
		n := 0
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, name) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *krm.PubSubSubscriptionFields {
		t.Helper()
		o := &krm.PubSubSubscriptionFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	update := func(t *testing.T, name string, change func(*krm.PubSubSubscriptionFields)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *krm.PubSubSubscriptionFields) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		})
	}
	resyncWritesNothing := func(t *testing.T, name, sub string) {
		t.Helper()
		before := patches(sub)
		resync(t, name)
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		if n := patches(sub) - before; n != 0 {
			t.Errorf("a resync patched the subscription %d times", n)
		}
	}
	settle := func(t *testing.T, name, sub string, ok func(*api.Subscription) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !ok(get(t, sub)) {
			if time.Now().After(deadline) {
				t.Fatalf("the subscription did not change: %+v", get(t, sub))
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, name)
	}
	newFields := func(name string, ref *pubsubv1beta1.PubSubSubscriptionRef) *krm.PubSubSubscriptionFields {
		return &krm.PubSubSubscriptionFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.PubSubSubscriptionFieldsSpec{ResourceRef: ref, MessageTransforms: []krm.MessageTransform{{
				JavascriptUdf: &krm.JavaScriptUdf{FunctionName: mockenv.Ptr("redact"), Code: mockenv.Ptr(udf)},
			}}},
		}
	}

	subID := mockenv.Name("sf-pull")
	sub := makeSubscription(t, subID, &api.Subscription{Labels: map[string]string{"made-by": "test"}, AckDeadlineSeconds: 20})
	// Config Connector's PubSubSubscription for it (not reconciled here:
	// read to resolve resourceRef by name, from its spec and project
	// annotation).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"resourceID": subID,
		"topicRef":   map[string]any{"external": topicName},
	}}}
	cc.SetGroupVersionKind(schema.GroupVersionKind{Group: "pubsub.cnrm.cloud.google.com", Version: "v1beta1", Kind: "PubSubSubscription"})
	cc.SetNamespace(namespace)
	cc.SetName("subscription")
	cc.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	obj := newFields("fields", &pubsubv1beta1.PubSubSubscriptionRef{Name: "subscription"})
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the transform, and nothing else", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != sub {
			t.Errorf("status.externalRef = %q, want %q", ref, sub)
		}
		got := get(t, sub)
		if len(got.MessageTransforms) != 1 || got.MessageTransforms[0].JavascriptUdf == nil || got.MessageTransforms[0].JavascriptUdf.FunctionName != "redact" {
			t.Errorf("messageTransforms = %+v", got.MessageTransforms)
		}
		if got.Labels["made-by"] != "test" || got.AckDeadlineSeconds != 20 {
			t.Errorf("labels %v, ackDeadlineSeconds %d: changed", got.Labels, got.AckDeadlineSeconds)
		}
		if got.BigtableConfig != nil {
			t.Errorf("bigtableConfig = %+v, set though the spec leaves it out", got.BigtableConfig)
		}
	})

	t.Run("Terraform's patch keeps it", func(t *testing.T) {
		// What Config Connector's PubSubSubscription (Terraform) sends when
		// its fields change: a mask of those top-level fields, never these.
		if _, err := c.Projects.Subscriptions.Patch(sub, &api.UpdateSubscriptionRequest{
			Subscription: &api.Subscription{Labels: map[string]string{"made-by": "config-connector"}, AckDeadlineSeconds: 30},
			UpdateMask:   "labels,ackDeadlineSeconds",
		}).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		got := get(t, sub)
		if got.Labels["made-by"] != "config-connector" || len(got.MessageTransforms) != 1 {
			t.Errorf("after Terraform's patch: labels %v, transforms %+v", got.Labels, got.MessageTransforms)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		resyncWritesNothing(t, "fields", sub)
	})

	t.Run("a change is patched, and one outside is set again", func(t *testing.T) {
		update(t, "fields", func(o *krm.PubSubSubscriptionFields) {
			o.Spec.MessageTransforms = append(o.Spec.MessageTransforms, krm.MessageTransform{
				JavascriptUdf: &krm.JavaScriptUdf{FunctionName: mockenv.Ptr("passthrough"), Code: mockenv.Ptr("function passthrough(message, metadata) { return message; }")},
				Disabled:      mockenv.Ptr(true),
			})
		})
		settle(t, "fields", sub, func(got *api.Subscription) bool {
			return len(got.MessageTransforms) == 2 && got.MessageTransforms[1].Disabled
		})
		if _, err := c.Projects.Subscriptions.Patch(sub, &api.UpdateSubscriptionRequest{Subscription: &api.Subscription{}, UpdateMask: "messageTransforms"}).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		resync(t, "fields")
		settle(t, "fields", sub, func(got *api.Subscription) bool { return len(got.MessageTransforms) == 2 })
		resyncWritesNothing(t, "fields", sub)
	})

	t.Run("a Bigtable export is set on a pull subscription", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live Bigtable export needs a Bigtable instance, which costs money (docs/kinds/PubSubSubscriptionFields.md)")
		}
		table := project + "/instances/events/tables/messages"
		update(t, "fields", func(o *krm.PubSubSubscriptionFields) {
			o.Spec.BigtableConfig = &krm.BigtableConfig{Table: mockenv.Ptr(table), AppProfileID: mockenv.Ptr("pubsub"), WriteMetadata: mockenv.Ptr(true)}
		})
		settle(t, "fields", sub, func(got *api.Subscription) bool {
			b := got.BigtableConfig
			return b != nil && b.Table == table && b.AppProfileId == "pubsub" && b.WriteMetadata
		})
		if got := get(t, sub); len(got.MessageTransforms) != 2 {
			t.Errorf("setting the export changed the transforms: %+v", got.MessageTransforms)
		}
		u := env.WaitReady(gvk, namespace, "fields")
		if st, _, _ := unstructured.NestedString(u.Object, "status", "bigtableConfigState"); st != "ACTIVE" {
			t.Errorf("status.bigtableConfigState = %q, want ACTIVE", st)
		}
		resyncWritesNothing(t, "fields", sub)
	})

	t.Run("the CRD refuses no field and two exports", func(t *testing.T) {
		bad := newFields("bad", &pubsubv1beta1.PubSubSubscriptionRef{External: sub})
		bad.Spec.MessageTransforms = nil
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "at least one of") {
			t.Errorf("creating no field: %v, want refused", err)
		}
		bad = newFields("bad", &pubsubv1beta1.PubSubSubscriptionRef{External: sub})
		bad.Spec.BigtableConfig = &krm.BigtableConfig{Table: mockenv.Ptr("t")}
		bad.Spec.BigqueryConfig = &krm.BigQueryConfigFields{ServiceAccountEmail: mockenv.Ptr("writer@example.com")}
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "exports to one place") {
			t.Errorf("creating a Bigtable and a BigQuery export: %v, want refused", err)
		}
	})

	t.Run("a second object for the subscription is refused", func(t *testing.T) {
		dup := newFields("dup", &pubsubv1beta1.PubSubSubscriptionRef{External: sub})
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing subscription is waited for, never made", func(t *testing.T) {
		missing := project + "/subscriptions/" + mockenv.Name("never-made")
		w := newFields("waits", &pubsubv1beta1.PubSubSubscriptionRef{External: missing})
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing subscription waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete clears the fields and keeps the subscription", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		got := get(t, sub)
		if len(got.MessageTransforms) != 0 || got.BigtableConfig != nil {
			t.Errorf("after delete: transforms %+v, bigtableConfig %+v", got.MessageTransforms, got.BigtableConfig)
		}
		if got.Labels["made-by"] != "config-connector" || got.AckDeadlineSeconds != 30 {
			t.Errorf("delete changed the subscription's other fields: labels %v, ackDeadlineSeconds %d", got.Labels, got.AckDeadlineSeconds)
		}
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &pubsubv1beta1.PubSubSubscriptionRef{Name: "subscription"})
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := get(t, sub); len(got.MessageTransforms) != 1 {
			t.Errorf("after an abandoning delete the transforms are %+v", got.MessageTransforms)
		}
	})

	t.Run("a BigQuery export's service account", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live BigQuery export needs a table the Pub/Sub service agent can write to (docs/kinds/PubSubSubscriptionFields.md)")
		}
		table := mockenv.Project + ".events.messages"
		bq := makeSubscription(t, mockenv.Name("sf-bq"), &api.Subscription{BigqueryConfig: &api.BigQueryConfig{Table: table, WriteMetadata: true}})
		writer := "writer@" + mockenv.Project + ".iam.gserviceaccount.com"
		o := &krm.PubSubSubscriptionFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "bq", Labels: mockenv.Labels()},
			Spec: krm.PubSubSubscriptionFieldsSpec{
				ResourceRef:    &pubsubv1beta1.PubSubSubscriptionRef{External: bq},
				BigqueryConfig: &krm.BigQueryConfigFields{ServiceAccountEmail: mockenv.Ptr(writer)},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		settle(t, "bq", bq, func(got *api.Subscription) bool {
			return got.BigqueryConfig != nil && got.BigqueryConfig.ServiceAccountEmail == writer
		})
		if got := get(t, bq).BigqueryConfig; got.Table != table || !got.WriteMetadata {
			t.Errorf("setting the service account changed the export: %+v", got)
		}
		resyncWritesNothing(t, "bq", bq)

		// Config Connector changes the export: Terraform sends it whole,
		// without the service account (the partial case); the next
		// reconcile puts it back.
		if _, err := c.Projects.Subscriptions.Patch(bq, &api.UpdateSubscriptionRequest{
			Subscription: &api.Subscription{BigqueryConfig: &api.BigQueryConfig{Table: table, WriteMetadata: true, DropUnknownFields: true}},
			UpdateMask:   "bigqueryConfig",
		}).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		resync(t, "bq")
		settle(t, "bq", bq, func(got *api.Subscription) bool {
			return got.BigqueryConfig.ServiceAccountEmail == writer && got.BigqueryConfig.DropUnknownFields
		})

		// A second object for it is refused, whatever fields it sets.
		bt := &krm.PubSubSubscriptionFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "bq-bigtable", Labels: mockenv.Labels()},
			Spec: krm.PubSubSubscriptionFieldsSpec{
				ResourceRef:    &pubsubv1beta1.PubSubSubscriptionRef{External: bq},
				BigtableConfig: &krm.BigtableConfig{Table: mockenv.Ptr(project + "/instances/i/tables/t")},
			},
		}
		if err := env.Kube.Create(env.Ctx, bt); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "bq-bigtable"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if err := env.Kube.Delete(env.Ctx, bt); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(bt)

		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
		if got := get(t, bq).BigqueryConfig; got == nil || got.ServiceAccountEmail != "" || got.Table != table {
			t.Errorf("after delete the export is %+v, want its table and no service account", got)
		}
	})

	t.Run("a Bigtable export is refused where the subscription exports elsewhere", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live BigQuery export needs a table the Pub/Sub service agent can write to")
		}
		bq := makeSubscription(t, mockenv.Name("sf-bq2"), &api.Subscription{BigqueryConfig: &api.BigQueryConfig{Table: mockenv.Project + ".events.messages"}})
		o := &krm.PubSubSubscriptionFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "bigtable-on-bq", Labels: mockenv.Labels()},
			Spec: krm.PubSubSubscriptionFieldsSpec{
				ResourceRef:    &pubsubv1beta1.PubSubSubscriptionRef{External: bq},
				BigtableConfig: &krm.BigtableConfig{Table: mockenv.Ptr(project + "/instances/i/tables/t")},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "bigtable-on-bq"); !strings.Contains(msg, "delivers to one place") {
			t.Errorf("Ready message = %q, want the Bigtable export refused", msg)
		}
		if got := get(t, bq); got.BigtableConfig != nil || got.BigqueryConfig == nil {
			t.Errorf("the subscription changed: bigtable %+v, bigquery %+v", got.BigtableConfig, got.BigqueryConfig)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)

		// And export fields where there is no such export.
		o = &krm.PubSubSubscriptionFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "gcs-on-bq", Labels: mockenv.Labels()},
			Spec: krm.PubSubSubscriptionFieldsSpec{
				ResourceRef:        &pubsubv1beta1.PubSubSubscriptionRef{External: bq},
				CloudStorageConfig: &krm.CloudStorageConfigFields{MaxMessages: mockenv.Ptr(int64(5000))},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "gcs-on-bq"); !strings.Contains(msg, "has no Cloud Storage export") {
			t.Errorf("Ready message = %q, want the missing export said", msg)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
	})

	t.Run("a Cloud Storage export's fields", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live Cloud Storage export needs a bucket the Pub/Sub service agent can write to (docs/kinds/PubSubSubscriptionFields.md)")
		}
		gcs := makeSubscription(t, mockenv.Name("sf-gcs"), &api.Subscription{CloudStorageConfig: &api.CloudStorageConfig{
			Bucket: "export-bucket", FilenamePrefix: "msgs-", AvroConfig: &api.AvroConfig{WriteMetadata: true},
		}})
		writer := "writer@" + mockenv.Project + ".iam.gserviceaccount.com"
		o := &krm.PubSubSubscriptionFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "gcs", Labels: mockenv.Labels()},
			Spec: krm.PubSubSubscriptionFieldsSpec{
				ResourceRef: &pubsubv1beta1.PubSubSubscriptionRef{External: gcs},
				CloudStorageConfig: &krm.CloudStorageConfigFields{
					ServiceAccountEmail:    mockenv.Ptr(writer),
					FilenameDatetimeFormat: mockenv.Ptr("YYYY-MM-DD/hh_mm_ssZ"),
					MaxMessages:            mockenv.Ptr(int64(5000)),
					AvroConfig:             &krm.CloudStorageConfigAvroConfigFields{UseTopicSchema: mockenv.Ptr(true)},
				},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		settle(t, "gcs", gcs, func(got *api.Subscription) bool {
			s := got.CloudStorageConfig
			return s != nil && s.ServiceAccountEmail == writer && s.MaxMessages == 5000 && s.FilenameDatetimeFormat == "YYYY-MM-DD/hh_mm_ssZ" && s.AvroConfig != nil && s.AvroConfig.UseTopicSchema
		})
		if s := get(t, gcs).CloudStorageConfig; s.Bucket != "export-bucket" || s.FilenamePrefix != "msgs-" || !s.AvroConfig.WriteMetadata || s.TextConfig != nil {
			t.Errorf("setting the fields changed the export's others: %+v (avro %+v)", s, s.AvroConfig)
		}
		resyncWritesNothing(t, "gcs", gcs)

		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
		s := get(t, gcs).CloudStorageConfig
		if s.ServiceAccountEmail != "" || s.MaxMessages != 0 || s.FilenameDatetimeFormat != "" || s.AvroConfig == nil || s.AvroConfig.UseTopicSchema {
			t.Errorf("after delete the export is %+v (avro %+v), want the fields cleared", s, s.AvroConfig)
		}
		if s.Bucket != "export-bucket" || !s.AvroConfig.WriteMetadata {
			t.Errorf("delete changed the export's other fields: %+v (avro %+v)", s, s.AvroConfig)
		}
	})
}
