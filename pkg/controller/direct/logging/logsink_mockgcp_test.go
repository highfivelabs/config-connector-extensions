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

// LoggingLogSink end to end on the fake cloud (pkg/test/mockenv, patches
// 0029 and 0049) or a real project (docs/live-runs.md). The cases: a
// project keeps its audit logs in its own _Default log bucket through a
// sink of its own, leaving out a noisy log and later pausing it; a
// landing zone's folder intercepts its children's logs into a central
// project (interceptChildren); an organization exports everything to a
// Cloud Storage bucket.
//
// A live run makes the project's sink in the run's project, routing to its
// own _Default bucket with a filter that matches nothing. The folder's
// sink runs live only with the logging test folder (E2E_LOGGING_FOLDER, as
// LoggingSettings), on which the live-test service account administers
// logging (roles/logging.configWriter has logging.sinks.*);
// it also matches nothing, so it intercepts no real logs. A live run never
// uses an organization.

import (
	"os"
	"strings"
	"testing"
	"time"

	loggingv1beta1kcc "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/logging/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	loggingv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/logging/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var sinkGVK = loggingv1beta1.LoggingLogSinkGVK

// sinkEnv is what the LoggingLogSink tests share.
type sinkEnv struct {
	env   *mockenv.Env
	cloud *api.Service
}

func newSinkEnv(t *testing.T) *sinkEnv {
	t.Helper()
	env := mockenv.New(t, namespace)
	cloud, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	return &sinkEnv{env: env, cloud: cloud}
}

func (e *sinkEnv) get(name string) (*api.LogSink, error) {
	return e.cloud.Sinks.Get(name).Context(e.env.Ctx).Do()
}

func (e *sinkEnv) mustGet(t *testing.T, name string) *api.LogSink {
	t.Helper()
	s, err := e.get(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return s
}

// writes are the requests that change a sink of parent.
func (e *sinkEnv) writes(parent string) []string {
	var out []string
	for _, r := range e.env.Requests() {
		if strings.HasPrefix(r, "GET ") || strings.HasPrefix(r, "DELETE ") {
			continue
		}
		if strings.Contains(r, "/v2/"+parent+"/sinks") {
			out = append(out, r)
		}
	}
	return out
}

func (e *sinkEnv) getObj(t *testing.T, name string) *loggingv1beta1.LoggingLogSink {
	t.Helper()
	obj := &loggingv1beta1.LoggingLogSink{}
	if err := e.env.Kube.Get(e.env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func (e *sinkEnv) waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// resync forces a reconcile of name and says what it wrote under parent.
func (e *sinkEnv) resync(t *testing.T, name, parent string) []string {
	t.Helper()
	before := len(e.writes(parent))
	o := e.getObj(t, name)
	o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
	if err := e.env.Kube.Update(e.env.Ctx, o); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Second)
	e.env.WaitReady(sinkGVK, namespace, name)
	return e.writes(parent)[before:]
}

// cleanup deletes the sinks named, whatever a failed step left.
func (e *sinkEnv) cleanup(t *testing.T, names ...string) {
	t.Cleanup(func() {
		for _, n := range names {
			_, err := e.cloud.Sinks.Delete(n).Context(e.env.Ctx).Do()
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up sink %s: %v", n, err)
			}
		}
	})
}

// refused creates obj and wants the API server to refuse it with msg.
func (e *sinkEnv) refused(t *testing.T, obj *loggingv1beta1.LoggingLogSink, msg string) {
	t.Helper()
	err := e.env.Kube.Create(e.env.Ctx, obj)
	if err == nil {
		_ = e.env.Kube.Delete(e.env.Ctx, obj)
		t.Errorf("creating %s: accepted, want the API server to refuse it (%s)", obj.Name, msg)
		return
	}
	if !strings.Contains(err.Error(), msg) {
		t.Errorf("creating %s: %v, want %q", obj.Name, err, msg)
	}
}

func TestLoggingLogSink(t *testing.T) {
	e := newSinkEnv(t)
	parent := "projects/" + mockenv.Project
	sinkName := func(base string) string { return parent + "/sinks/" + mockenv.Name(base) }
	bucket := parent + "/locations/global/buckets/_Default"
	// A filter that matches no log: a live sink routes nothing.
	filter := `logName="` + parent + `/logs/` + mockenv.Name("none") + `"`
	e.cleanup(t, sinkName("audit"), sinkName("kept"))

	newSink := func(name string) *loggingv1beta1.LoggingLogSink {
		return &loggingv1beta1.LoggingLogSink{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: loggingv1beta1.LoggingLogSinkSpec{
				ProjectRef: &refs.ProjectRef{External: mockenv.Project},
				ResourceID: mockenv.Ptr(mockenv.Name(name)),
				Destination: loggingv1beta1.LoggingLogSinkDestination{
					LoggingLogBucketRef: &loggingv1beta1kcc.LoggingLogBucketRef{External: bucket},
				},
				Filter:               mockenv.Ptr(filter),
				Description:          mockenv.Ptr("the project's audit logs, kept in its own bucket"),
				UniqueWriterIdentity: mockenv.Ptr(true),
			},
		}
	}

	sink := newSink("audit")
	sink.Spec.Exclusions = []loggingv1beta1kcc.LogSinkExclusions{{
		Name:   "no-reads",
		Filter: `protoPayload.methodName:"Get"`,
	}}
	if err := e.env.Kube.Create(e.env.Ctx, sink); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the sink in the project", func(t *testing.T) {
		u := e.env.WaitReady(sinkGVK, namespace, "audit")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != sinkName("audit") {
			t.Errorf("status.externalRef = %q, want %q", ref, sinkName("audit"))
		}
		got := e.mustGet(t, sinkName("audit"))
		if got.Destination != "logging.googleapis.com/"+bucket {
			t.Errorf("destination = %q", got.Destination)
		}
		if got.Filter != filter || got.Disabled || got.InterceptChildren {
			t.Errorf("filter %q, disabled %v, interceptChildren %v", got.Filter, got.Disabled, got.InterceptChildren)
		}
		if len(got.Exclusions) != 1 || got.Exclusions[0].Name != "no-reads" {
			t.Errorf("exclusions = %v", got.Exclusions)
		}
		wi, _, _ := unstructured.NestedString(u.Object, "status", "writerIdentity")
		if wi != got.WriterIdentity {
			t.Errorf("status.writerIdentity = %q, cloud %q", wi, got.WriterIdentity)
		}
		var create string
		for _, r := range e.writes(parent) {
			if strings.HasPrefix(r, "POST ") {
				create = r
			}
		}
		if !strings.Contains(create, "uniqueWriterIdentity=true") {
			t.Errorf("create request %q, want uniqueWriterIdentity=true", create)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		if w := e.resync(t, "audit", parent); len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("pausing and a second exclusion are one update of those fields", func(t *testing.T) {
		before := len(e.writes(parent))
		o := e.getObj(t, "audit")
		o.Spec.Disabled = mockenv.Ptr(true)
		o.Spec.Exclusions = append(o.Spec.Exclusions, loggingv1beta1kcc.LogSinkExclusions{
			Name:        "no-lists",
			Description: mockenv.Ptr("list calls are noise"),
			Filter:      `protoPayload.methodName:"List"`,
		})
		if err := e.env.Kube.Update(e.env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		e.waitFor(t, "the sink to be disabled", func() bool {
			s, err := e.get(sinkName("audit"))
			return err == nil && s.Disabled && len(s.Exclusions) == 2
		})
		e.env.WaitReady(sinkGVK, namespace, "audit")
		w := e.writes(parent)[before:]
		if len(w) != 1 {
			t.Fatalf("writes = %v, want one update", w)
		}
		if !strings.Contains(w[0], "updateMask=disabled%2Cexclusions") {
			t.Errorf("update %q, want a mask of disabled and exclusions only", w[0])
		}
		got := e.mustGet(t, sinkName("audit"))
		if got.Destination != "logging.googleapis.com/"+bucket || got.Exclusions[1].Description != "list calls are noise" {
			t.Errorf("sink after update = %+v", got)
		}
		if w := e.resync(t, "audit", parent); len(w) != 0 {
			t.Errorf("a resync after the update wrote %v", w)
		}
	})

	t.Run("the CRD refuses a change of parent", func(t *testing.T) {
		o := e.getObj(t, "audit")
		o.Spec.ProjectRef = &refs.ProjectRef{External: "other-project"}
		err := e.env.Kube.Update(e.env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "projectRef is immutable") {
			t.Errorf("changing projectRef: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("the CRD refuses no parent or two", func(t *testing.T) {
		none := newSink("no-parent")
		none.Spec.ProjectRef = nil
		e.refused(t, none, "exactly one of projectRef, folderRef and organizationRef is required")
		two := newSink("two-parents")
		two.Spec.FolderRef = &refs.FolderRef{External: "folders/123456"}
		e.refused(t, two, "exactly one of projectRef, folderRef and organizationRef is required")
	})

	t.Run("the CRD refuses two destinations", func(t *testing.T) {
		o := newSink("two-destinations")
		o.Spec.Destination.ProjectRef = &refs.ProjectRef{External: mockenv.Project}
		e.refused(t, o, "exactly one of bigQueryDatasetRef, loggingLogBucketRef, pubSubTopicRef, storageBucketRef and projectRef is required")
	})

	t.Run("the CRD refuses a project's intercepting sink", func(t *testing.T) {
		o := newSink("intercepting")
		o.Spec.IncludeChildren = mockenv.Ptr(true)
		o.Spec.InterceptChildren = mockenv.Ptr(true)
		o.Spec.Destination = loggingv1beta1.LoggingLogSinkDestination{ProjectRef: &refs.ProjectRef{External: mockenv.Project}}
		e.refused(t, o, "interceptChildren needs a folderRef or organizationRef parent")
	})

	t.Run("delete deletes the sink", func(t *testing.T) {
		if err := e.env.Kube.Delete(e.env.Ctx, e.getObj(t, "audit")); err != nil {
			t.Fatal(err)
		}
		e.env.WaitGone(&loggingv1beta1.LoggingLogSink{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "audit"}})
		if _, err := e.get(sinkName("audit")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the sink: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the sink", func(t *testing.T) {
		kept := newSink("kept")
		if err := e.env.Kube.Create(e.env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		e.env.WaitReady(sinkGVK, namespace, "kept")
		kept = e.getObj(t, "kept")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := e.env.Kube.Update(e.env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		if err := e.env.Kube.Delete(e.env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		e.env.WaitGone(&loggingv1beta1.LoggingLogSink{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := e.get(sinkName("kept")); err != nil {
			t.Errorf("after abandon, reading the sink: %v, want it kept", err)
		}
	})
}

func TestLoggingLogSinkFolderIntercepts(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_LOGGING_FOLDER") == "" {
		t.Skip("a live folder sink needs the logging test folder, E2E_LOGGING_FOLDER, on which the live-test service account administers logging (docs/live-runs.md)")
	}
	e := newSinkEnv(t)
	folder := mockenv.LoggingFolder(t)
	sinkName := func(base string) string { return folder + "/sinks/" + mockenv.Name(base) }
	filter := `logName="projects/` + mockenv.Project + `/logs/` + mockenv.Name("none") + `"`
	e.cleanup(t, sinkName("landing-zone"))

	newSink := func(name string) *loggingv1beta1.LoggingLogSink {
		return &loggingv1beta1.LoggingLogSink{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: loggingv1beta1.LoggingLogSinkSpec{
				// The bare ID, as Config Connector's LoggingLogSink takes it.
				FolderRef:  &refs.FolderRef{External: strings.TrimPrefix(folder, "folders/")},
				ResourceID: mockenv.Ptr(mockenv.Name(name)),
				Destination: loggingv1beta1.LoggingLogSinkDestination{
					ProjectRef: &refs.ProjectRef{External: mockenv.Project},
				},
				Filter:            mockenv.Ptr(filter),
				IncludeChildren:   mockenv.Ptr(true),
				InterceptChildren: mockenv.Ptr(true),
			},
		}
	}

	if err := e.env.Kube.Create(e.env.Ctx, newSink("landing-zone")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes an intercepting sink in the folder", func(t *testing.T) {
		u := e.env.WaitReady(sinkGVK, namespace, "landing-zone")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != sinkName("landing-zone") {
			t.Errorf("status.externalRef = %q, want %q", ref, sinkName("landing-zone"))
		}
		got := e.mustGet(t, sinkName("landing-zone"))
		if got.Destination != "logging.googleapis.com/projects/"+mockenv.Project {
			t.Errorf("destination = %q, want the project", got.Destination)
		}
		if !got.IncludeChildren || !got.InterceptChildren {
			t.Errorf("includeChildren %v, interceptChildren %v, want both", got.IncludeChildren, got.InterceptChildren)
		}
		wi, _, _ := unstructured.NestedString(u.Object, "status", "writerIdentity")
		if wi == "" || wi != got.WriterIdentity {
			t.Errorf("status.writerIdentity = %q, cloud %q", wi, got.WriterIdentity)
		}
		var create string
		for _, r := range e.writes(folder) {
			if strings.HasPrefix(r, "POST ") {
				create = r
			}
		}
		if !strings.Contains(create, "uniqueWriterIdentity=true") {
			t.Errorf("create request %q, want uniqueWriterIdentity=true (a folder's sink)", create)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		if w := e.resync(t, "landing-zone", folder); len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("the CRD refuses to stop intercepting", func(t *testing.T) {
		o := e.getObj(t, "landing-zone")
		o.Spec.InterceptChildren = mockenv.Ptr(false)
		err := e.env.Kube.Update(e.env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "interceptChildren is immutable") {
			t.Errorf("changing interceptChildren: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("the CRD refuses an intercepting sink without its children or a project", func(t *testing.T) {
		noChildren := newSink("no-children")
		noChildren.Spec.IncludeChildren = nil
		e.refused(t, noChildren, "interceptChildren needs includeChildren true")
		bucket := newSink("to-a-bucket")
		bucket.Spec.Destination = loggingv1beta1.LoggingLogSinkDestination{
			LoggingLogBucketRef: &loggingv1beta1kcc.LoggingLogBucketRef{External: "projects/" + mockenv.Project + "/locations/global/buckets/_Default"},
		}
		e.refused(t, bucket, "interceptChildren needs a Cloud project destination")
	})

	t.Run("delete deletes the sink", func(t *testing.T) {
		if err := e.env.Kube.Delete(e.env.Ctx, e.getObj(t, "landing-zone")); err != nil {
			t.Fatal(err)
		}
		e.env.WaitGone(&loggingv1beta1.LoggingLogSink{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "landing-zone"}})
		if _, err := e.get(sinkName("landing-zone")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the sink: %v, want not found", err)
		}
	})
}

func TestLoggingLogSinkOrganization(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a live run never uses an organization")
	}
	e := newSinkEnv(t)
	name := org + "/sinks/everything"
	e.cleanup(t, name)

	sink := &loggingv1beta1.LoggingLogSink{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "everything"},
		Spec: loggingv1beta1.LoggingLogSinkSpec{
			OrganizationRef: &refs.OrganizationRef{External: strings.TrimPrefix(org, "organizations/")},
			Destination: loggingv1beta1.LoggingLogSinkDestination{
				// fabric's form of a bucket destination
				StorageBucketRef: &storagev1beta1.StorageBucketRef{External: "storage.googleapis.com/org-audit-archive"},
			},
			IncludeChildren: mockenv.Ptr(true),
		},
	}
	if err := e.env.Kube.Create(e.env.Ctx, sink); err != nil {
		t.Fatal(err)
	}
	u := e.env.WaitReady(sinkGVK, namespace, "everything")
	if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != name {
		t.Errorf("status.externalRef = %q, want %q", ref, name)
	}
	got := e.mustGet(t, name)
	if got.Destination != "storage.googleapis.com/org-audit-archive" || !got.IncludeChildren || got.InterceptChildren {
		t.Errorf("sink = %+v", got)
	}
	if w := e.resync(t, "everything", org); len(w) != 0 {
		t.Errorf("a resync wrote %v", w)
	}

	// The fake refuses what the API refuses, even past the CRD: an
	// intercepting sink must route to a project.
	bad := &api.LogSink{Name: "bad", Destination: "storage.googleapis.com/b", IncludeChildren: true, InterceptChildren: true}
	if _, err := e.cloud.Sinks.Create(org, bad).Context(e.env.Ctx).Do(); err == nil || !strings.Contains(err.Error(), "must be a Cloud project") {
		t.Errorf("an intercepting sink to a bucket: %v, want refused", err)
	}

	if err := e.env.Kube.Delete(e.env.Ctx, e.getObj(t, "everything")); err != nil {
		t.Fatal(err)
	}
	e.env.WaitGone(&loggingv1beta1.LoggingLogSink{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "everything"}})
	if _, err := e.get(name); !direct.IsNotFound(err) {
		t.Errorf("after delete, reading the sink: %v, want not found", err)
	}
}
