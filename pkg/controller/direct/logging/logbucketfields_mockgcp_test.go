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

package logging_test

// LoggingLogBucketFields end to end on the fake cloud (pkg/test/mockenv,
// patch 0055) or a real project (docs/live-runs.md). The bucket is the
// run's own (mockenv.Name), in us-central1, with no logs routed to it, so
// it costs nothing; the test deletes it (Cloud Logging keeps a deleted
// bucket's name for 7 days). The CMEK case runs live only with
// E2E_LOGGING_BUCKET_KMS_KEY, a us-central1 key the project's Log Router
// service account may use: a bucket's CMEK cannot be removed once set.

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/logging/apiv2"
	pb "cloud.google.com/go/logging/apiv2/loggingpb"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var configConnectorLogBucketGVK = schema.GroupVersionKind{Group: "logging.cnrm.cloud.google.com", Version: "v1beta1", Kind: "LoggingLogBucket"}

func TestLoggingLogBucketFields(t *testing.T) {
	const (
		namespace = "example"
		location  = "us-central1"
		name      = "fields"
	)
	gvk := krm.LoggingLogBucketFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := gcp.NewConfigRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	bucketID := mockenv.Name("lbf")
	parent := "projects/" + mockenv.Project + "/locations/" + location
	bucket, err := c.CreateBucket(env.Ctx, &pb.CreateBucketRequest{
		Parent: parent, BucketId: bucketID,
		Bucket: &pb.LogBucket{Description: "made by the test", RetentionDays: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	bucketName := bucket.GetName()
	t.Cleanup(func() {
		if err := c.DeleteBucket(env.Ctx, &pb.DeleteBucketRequest{Name: bucketName}); err != nil {
			t.Errorf("deleting bucket %s: %v", bucketName, err)
		}
	})

	// Config Connector's LoggingLogBucket for the bucket (not reconciled
	// here: read to resolve resourceRef by name, from its spec, as Config
	// Connector's reference does for a DCL object with no externalRef).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"projectRef": map[string]any{"external": "projects/" + mockenv.Project},
		"location":   location, "resourceID": bucketID,
	}}}
	cc.SetGroupVersionKind(configConnectorLogBucketGVK)
	cc.SetNamespace(namespace)
	cc.SetName("bucket")
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	getBucket := func(t *testing.T) *pb.LogBucket {
		t.Helper()
		b, err := c.GetBucket(env.Ctx, &pb.GetBucketRequest{Name: bucketName})
		if err != nil {
			t.Fatalf("reading the bucket: %v", err)
		}
		return b
	}
	indexes := func(b *pb.LogBucket) []string {
		var out []string
		for _, ic := range b.GetIndexConfigs() {
			out = append(out, ic.GetFieldPath()+"="+ic.GetType().String())
		}
		slices.Sort(out)
		return out
	}
	restricted := func(b *pb.LogBucket) []string {
		out := slices.Clone(b.GetRestrictedFields())
		slices.Sort(out)
		return out
	}
	patches := func() []string {
		var out []string
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, "/buckets/"+bucketID+"?") {
				out = append(out, req)
			}
		}
		return out
	}
	getObj := func(t *testing.T, name string) *krm.LoggingLogBucketFields {
		t.Helper()
		o := &krm.LoggingLogBucketFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
	}
	waitFor := func(t *testing.T, what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(time.Second)
		}
	}
	newFields := func(name string, ref *loggingv1beta1.LoggingLogBucketRef) *krm.LoggingLogBucketFields {
		return &krm.LoggingLogBucketFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec:       krm.LoggingLogBucketFieldsSpec{ResourceRef: ref},
		}
	}

	obj := newFields(name, &loggingv1beta1.LoggingLogBucketRef{Name: "bucket"})
	obj.Spec.IndexConfigs = []krm.IndexConfig{{FieldPath: "jsonPayload.request.status", Type: "INDEX_TYPE_INTEGER"}, {FieldPath: "jsonPayload.tenant", Type: "INDEX_TYPE_STRING"}}
	obj.Spec.RestrictedFields = []string{"jsonPayload.user.email"}
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the fields with a mask naming only them", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != bucketName {
			t.Errorf("status.externalRef = %q, want %q", ref, bucketName)
		}
		b := getBucket(t)
		if got, want := indexes(b), []string{"jsonPayload.request.status=INDEX_TYPE_INTEGER", "jsonPayload.tenant=INDEX_TYPE_STRING"}; !slices.Equal(got, want) {
			t.Errorf("index configs = %v, want %v", got, want)
		}
		if got := restricted(b); !slices.Equal(got, []string{"jsonPayload.user.email"}) {
			t.Errorf("restricted fields = %v", got)
		}
		if b.GetDescription() != "made by the test" || b.GetRetentionDays() != 1 {
			t.Errorf("the other fields changed: description %q, retention %d", b.GetDescription(), b.GetRetentionDays())
		}
		p := patches()
		if len(p) != 1 || !strings.Contains(p[0], "updateMask=indexConfigs%2CrestrictedFields") {
			t.Errorf("patches = %v, want one with updateMask=indexConfigs,restrictedFields", p)
		}
	})

	t.Run("an update of Config Connector's fields keeps them", func(t *testing.T) {
		// What Config Connector's update of a LoggingLogBucket sends when
		// its description changes: the bucket, with a mask of description.
		if _, err := c.UpdateBucket(env.Ctx, &pb.UpdateBucketRequest{
			Name: bucketName, Bucket: &pb.LogBucket{Description: "changed by config connector", RetentionDays: 1},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"description"}},
		}); err != nil {
			t.Fatal(err)
		}
		b := getBucket(t)
		if b.GetDescription() != "changed by config connector" {
			t.Errorf("description = %q, want the change", b.GetDescription())
		}
		if len(indexes(b)) != 2 || len(restricted(b)) != 1 {
			t.Errorf("after Config Connector's update: index configs %v, restricted fields %v", indexes(b), restricted(b))
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		resync(t, name)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the bucket %d times", n)
		}
	})

	t.Run("a changed list replaces the bucket's, and only it is sent", func(t *testing.T) {
		before := len(patches())
		o := getObj(t, name)
		o.Spec.IndexConfigs = []krm.IndexConfig{{FieldPath: "jsonPayload.tenant", Type: "INDEX_TYPE_STRING"}}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the index configs to change", func() bool { return len(indexes(getBucket(t))) == 1 })
		env.WaitReady(gvk, namespace, name)
		p := patches()[before:]
		if len(p) != 1 || !strings.Contains(p[0], "updateMask=indexConfigs") || strings.Contains(p[0], "restrictedFields") {
			t.Errorf("patches = %v, want one with updateMask=indexConfigs alone", p)
		}
	})

	t.Run("a field cleared outside is set again", func(t *testing.T) {
		if _, err := c.UpdateBucket(env.Ctx, &pb.UpdateBucketRequest{
			Name: bucketName, Bucket: &pb.LogBucket{},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"restricted_fields"}},
		}); err != nil {
			t.Fatal(err)
		}
		resync(t, name)
		if got := restricted(getBucket(t)); !slices.Equal(got, []string{"jsonPayload.user.email"}) {
			t.Errorf("after a resync the restricted fields = %v", got)
		}
	})

	t.Run("a CMEK key is set, and reported", func(t *testing.T) {
		key := "projects/" + mockenv.Project + "/locations/" + location + "/keyRings/logs/cryptoKeys/bucket"
		if mockenv.Live() {
			key = os.Getenv("E2E_LOGGING_BUCKET_KMS_KEY")
			if key == "" {
				t.Skip("E2E_LOGGING_BUCKET_KMS_KEY is not set: a live CMEK case needs a us-central1 key the project's Log Router may use (docs/kinds/LoggingLogBucketFields.md)")
			}
		}
		o := getObj(t, name)
		o.Spec.CmekSettings = &krm.CmekSettings{KmsKeyName: key}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the CMEK key", func() bool { return getBucket(t).GetCmekSettings().GetKmsKeyName() == key })
		waitFor(t, "status.cmekServiceAccountID", func() bool {
			return getObj(t, name).Status.CmekServiceAccountID != nil
		})
		env.WaitReady(gvk, namespace, name)
		if got := len(indexes(getBucket(t))); got != 1 {
			t.Errorf("setting CMEK changed the index configs: %d", got)
		}
	})

	t.Run("a second object for the bucket is refused", func(t *testing.T) {
		dup := newFields("dup", &loggingv1beta1.LoggingLogBucketRef{External: bucketName})
		dup.Spec.RestrictedFields = []string{"jsonPayload.other"}
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if got := restricted(getBucket(t)); !slices.Equal(got, []string{"jsonPayload.user.email"}) {
			t.Errorf("the refused object changed the bucket: %v", got)
		}
		dup.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing bucket is waited for, never made", func(t *testing.T) {
		missing := parent + "/buckets/" + mockenv.Name("never-made")
		w := newFields("waits", &loggingv1beta1.LoggingLogBucketRef{External: missing})
		w.Spec.RestrictedFields = []string{"jsonPayload.user.email"}
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing bucket waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete clears the lists and keeps the bucket", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.LoggingLogBucketFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		b := getBucket(t)
		if len(indexes(b)) != 0 || len(restricted(b)) != 0 {
			t.Errorf("after delete: index configs %v, restricted fields %v; want none", indexes(b), restricted(b))
		}
		if b.GetDescription() != "changed by config connector" {
			t.Errorf("delete changed the bucket's other fields: description %q", b.GetDescription())
		}
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &loggingv1beta1.LoggingLogBucketRef{Name: "bucket"})
		k.Spec.RestrictedFields = []string{"jsonPayload.kept"}
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := restricted(getBucket(t)); !slices.Equal(got, []string{"jsonPayload.kept"}) {
			t.Errorf("after an abandoning delete the restricted fields = %v, want [jsonPayload.kept]", got)
		}
	})
}
