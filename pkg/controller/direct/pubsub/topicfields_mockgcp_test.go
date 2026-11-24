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

// PubSubTopicFields end to end on the fake cloud (pkg/test/mockenv, patch
// 0064) or a real project (docs/live-runs.md). The topic is the run's own
// (mockenv.Name), with no subscription, costs nothing and is deleted at the
// end. Live, the Cloud Storage import case skips: the topic's service agent
// would need to read a bucket.

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

const namespace = "example"

const udf = `function redact(message, metadata) {
  const data = JSON.parse(message.data);
  delete data.email;
  return {data: JSON.stringify(data), attributes: message.attributes};
}`

func TestPubSubTopicFields(t *testing.T) {
	gvk := krm.PubSubTopicFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	topic := mockenv.Name("tf")
	topicName := "projects/" + mockenv.Project + "/topics/" + topic
	if _, err := c.Projects.Topics.Create(topicName, &api.Topic{Labels: map[string]string{"made-by": "test"}}).Context(env.Ctx).Do(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := c.Projects.Topics.Delete(topicName).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting topic %s: %v", topicName, err)
		}
	})
	get := func(t *testing.T) *api.Topic {
		t.Helper()
		got, err := c.Projects.Topics.Get(topicName).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the topic: %v", err)
		}
		return got
	}
	patches := func() int {
		n := 0
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, "/topics/"+topic) {
				n++
			}
		}
		return n
	}

	// Config Connector's PubSubTopic for it (not reconciled here: read to
	// resolve resourceRef by name, from its spec and project annotation).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"resourceID": topic}}}
	cc.SetGroupVersionKind(schema.GroupVersionKind{Group: "pubsub.cnrm.cloud.google.com", Version: "v1beta1", Kind: "PubSubTopic"})
	cc.SetNamespace(namespace)
	cc.SetName("topic")
	cc.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	getObj := func(t *testing.T, name string) *krm.PubSubTopicFields {
		t.Helper()
		o := &krm.PubSubTopicFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	update := func(t *testing.T, name string, change func(*krm.PubSubTopicFields)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resyncWritesNothing := func(t *testing.T, name string) {
		t.Helper()
		before := patches()
		update(t, name, func(o *krm.PubSubTopicFields) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		})
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		if n := patches() - before; n != 0 {
			t.Errorf("a resync patched the topic %d times", n)
		}
	}
	settle := func(t *testing.T, name string, ok func(*api.Topic) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !ok(get(t)) {
			if time.Now().After(deadline) {
				t.Fatalf("the topic did not change: %+v", get(t))
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, name)
	}
	newFields := func(name string, ref *pubsubv1beta1.PubSubTopicRef) *krm.PubSubTopicFields {
		return &krm.PubSubTopicFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.PubSubTopicFieldsSpec{ResourceRef: ref, MessageTransforms: []krm.MessageTransform{{
				JavascriptUdf: &krm.JavaScriptUdf{FunctionName: mockenv.Ptr("redact"), Code: mockenv.Ptr(udf)},
			}}},
		}
	}

	obj := newFields("fields", &pubsubv1beta1.PubSubTopicRef{Name: "topic"})
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the transform, and nothing else", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != topicName {
			t.Errorf("status.externalRef = %q, want %q", ref, topicName)
		}
		got := get(t)
		if len(got.MessageTransforms) != 1 || got.MessageTransforms[0].JavascriptUdf == nil || got.MessageTransforms[0].JavascriptUdf.FunctionName != "redact" {
			t.Errorf("messageTransforms = %+v", got.MessageTransforms)
		}
		if got.Labels["made-by"] != "test" {
			t.Errorf("labels = %v, changed", got.Labels)
		}
		if got.IngestionDataSourceSettings != nil {
			t.Errorf("ingestionDataSourceSettings = %+v, set though the spec leaves it out", got.IngestionDataSourceSettings)
		}
	})

	t.Run("Terraform's patch keeps it", func(t *testing.T) {
		// What Config Connector's PubSubTopic (Terraform) sends when its
		// fields change: a mask of those fields, never these.
		if _, err := c.Projects.Topics.Patch(topicName, &api.UpdateTopicRequest{
			Topic:      &api.Topic{Labels: map[string]string{"made-by": "config-connector"}, MessageRetentionDuration: "86400s"},
			UpdateMask: "labels,messageRetentionDuration",
		}).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		got := get(t)
		if got.Labels["made-by"] != "config-connector" || len(got.MessageTransforms) != 1 {
			t.Errorf("after Terraform's patch: labels %v, transforms %+v", got.Labels, got.MessageTransforms)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		resyncWritesNothing(t, "fields")
	})

	t.Run("a change is patched, and one outside is set again", func(t *testing.T) {
		update(t, "fields", func(o *krm.PubSubTopicFields) {
			o.Spec.MessageTransforms = append(o.Spec.MessageTransforms, krm.MessageTransform{
				JavascriptUdf: &krm.JavaScriptUdf{FunctionName: mockenv.Ptr("passthrough"), Code: mockenv.Ptr("function passthrough(message, metadata) { return message; }")},
				Disabled:      mockenv.Ptr(true),
			})
		})
		settle(t, "fields", func(got *api.Topic) bool {
			return len(got.MessageTransforms) == 2 && got.MessageTransforms[1].Disabled
		})
		if _, err := c.Projects.Topics.Patch(topicName, &api.UpdateTopicRequest{Topic: &api.Topic{}, UpdateMask: "messageTransforms"}).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		update(t, "fields", func(o *krm.PubSubTopicFields) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		})
		settle(t, "fields", func(got *api.Topic) bool { return len(got.MessageTransforms) == 2 })
		resyncWritesNothing(t, "fields")
	})

	t.Run("an import from Cloud Storage is set", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live import topic's service agent needs to read a bucket, which the live-test project does not set up (docs/kinds/PubSubTopicFields.md)")
		}
		update(t, "fields", func(o *krm.PubSubTopicFields) {
			o.Spec.IngestionDataSourceSettings = &krm.IngestionDataSourceSettings{
				CloudStorage: &krm.IngestionDataSourceSettings_CloudStorage{
					Bucket:     mockenv.Ptr("import-bucket"),
					TextFormat: &krm.IngestionDataSourceSettings_CloudStorage_TextFormat{Delimiter: mockenv.Ptr("\n")},
					MatchGlob:  mockenv.Ptr("**.jsonl"),
				},
				PlatformLogsSettings: &krm.PlatformLogsSettings{Severity: mockenv.Ptr("WARNING")},
			}
		})
		settle(t, "fields", func(got *api.Topic) bool {
			s := got.IngestionDataSourceSettings
			return s != nil && s.CloudStorage != nil && s.CloudStorage.Bucket == "import-bucket" && s.PlatformLogsSettings != nil
		})
		if got := get(t); len(got.MessageTransforms) != 2 {
			t.Errorf("setting the import changed the transforms: %+v", got.MessageTransforms)
		}
		resyncWritesNothing(t, "fields")
	})

	t.Run("the CRD refuses two sources and no field", func(t *testing.T) {
		bad := newFields("bad", &pubsubv1beta1.PubSubTopicRef{External: topicName})
		bad.Spec.IngestionDataSourceSettings = &krm.IngestionDataSourceSettings{
			CloudStorage: &krm.IngestionDataSourceSettings_CloudStorage{Bucket: mockenv.Ptr("b")},
			AwsKinesis:   &krm.IngestionDataSourceSettings_AwsKinesis{StreamArn: mockenv.Ptr("arn:aws:kinesis:us-east-1:000000000000:stream/s")},
		}
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "at most one ingestion source") {
			t.Errorf("creating two sources: %v, want refused", err)
		}
		bad = newFields("bad", &pubsubv1beta1.PubSubTopicRef{External: topicName})
		bad.Spec.MessageTransforms = nil
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "at least one of") {
			t.Errorf("creating no field: %v, want refused", err)
		}
	})

	t.Run("a second object for the topic is refused", func(t *testing.T) {
		dup := newFields("dup", &pubsubv1beta1.PubSubTopicRef{External: topicName})
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

	t.Run("a missing topic is waited for, never made", func(t *testing.T) {
		missing := "projects/" + mockenv.Project + "/topics/" + mockenv.Name("never-made")
		w := newFields("waits", &pubsubv1beta1.PubSubTopicRef{External: missing})
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing topic waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete clears the fields and keeps the topic", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		got := get(t)
		if len(got.MessageTransforms) != 0 || got.IngestionDataSourceSettings != nil {
			t.Errorf("after delete: transforms %+v, ingestion %+v", got.MessageTransforms, got.IngestionDataSourceSettings)
		}
		if got.Labels["made-by"] != "config-connector" {
			t.Errorf("delete changed the topic's other fields: labels %v", got.Labels)
		}
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &pubsubv1beta1.PubSubTopicRef{Name: "topic"})
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := get(t); len(got.MessageTransforms) != 1 {
			t.Errorf("after an abandoning delete the transforms are %+v", got.MessageTransforms)
		}
	})
}
