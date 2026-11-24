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

package run_test

// RunWorkerPool (ours, the stable equivalent of Config Connector's alpha
// kind) end to end on the fake cloud (pkg/test/mockenv) or a real project
// (docs/live-runs.md). The case: a worker pool with labels on its revisions
// and a Binary Authorization policy by name, which Config Connector's
// RunWorkerPool cannot express, scaled to no instances (so it costs nothing
// live). Worker pool names are mockenv.Name(...), and the test deletes every
// worker pool it made at the end, the abandoned one included. Against a real
// project the image is Cloud Run's public sample job image (it need only
// exist: no instance runs), and the named policy only with
// E2E_RUN_BINAUTHZ_POLICY, as RunJob's test; without it the default policy.

import (
	"net/url"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/run/apiv2"
	pb "cloud.google.com/go/run/apiv2/runpb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	runv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var workerPoolGVK = runv1beta1.RunWorkerPoolGVK

func workerPoolCloudName(name string) string {
	return "projects/" + mockenv.Project + "/locations/" + region + "/workerPools/" + mockenv.Name(name)
}

func newWorkerPool(name string) *runv1beta1.RunWorkerPool {
	image, _ := jobImages()
	w := &runv1beta1.RunWorkerPool{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: runv1beta1.RunWorkerPoolSpec{
			ResourceID:  mockenv.Ptr(mockenv.Name(name)),
			Labels:      mockenv.Labels(),
			ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:    mockenv.Ptr(region),
			Description: mockenv.Ptr("queue consumers"),
			Template: &runv1beta1.WorkerPoolRevisionTemplate{
				Labels: map[string]string{"tier": "consumer"},
				Containers: []runv1beta1.Container{{
					Image: mockenv.Ptr(image),
				}},
			},
			Scaling: &runv1beta1.WorkerPoolScaling{ManualInstanceCount: mockenv.Ptr(int32(0))},
		},
	}
	if p := binauthzPolicy(); p != "" {
		w.Spec.BinaryAuthorization = &runv1beta1.BinaryAuthorization{Policy: mockenv.Ptr(p)}
	} else {
		w.Spec.BinaryAuthorization = &runv1beta1.BinaryAuthorization{UseDefault: mockenv.Ptr(true)}
	}
	return w
}

// workerPoolMasks returns the update masks of the worker pool's updates,
// in order.
func workerPoolMasks(t *testing.T, env *mockenv.Env, name string) []string {
	t.Helper()
	var out []string
	for _, r := range env.Requests() {
		if !strings.HasPrefix(r, "PATCH ") {
			continue
		}
		u, err := url.Parse(strings.TrimPrefix(r, "PATCH "))
		if err != nil {
			t.Fatal(err)
		}
		if u.Path == "/v2/"+workerPoolCloudName(name) {
			out = append(out, u.Query().Get("updateMask"))
		}
	}
	return out
}

func TestRunWorkerPool(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewWorkerPoolsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })
	// Delete every worker pool the test made, the abandoned one included.
	t.Cleanup(func() {
		for _, name := range []string{"consumers", "kept-pool"} {
			op, err := cloud.DeleteWorkerPool(env.Ctx, &pb.DeleteWorkerPoolRequest{Name: workerPoolCloudName(name)})
			if err == nil {
				_, err = op.Wait(env.Ctx)
			}
			if err != nil && status.Code(err) != codes.NotFound {
				t.Errorf("deleting worker pool %s: %v", workerPoolCloudName(name), err)
			}
		}
	})
	read := func(t *testing.T, name string) *pb.WorkerPool {
		t.Helper()
		wp, err := cloud.GetWorkerPool(env.Ctx, &pb.GetWorkerPoolRequest{Name: workerPoolCloudName(name)})
		if err != nil {
			t.Fatalf("reading the worker pool from the cloud: %v", err)
		}
		return wp
	}
	getObj := func(t *testing.T, name string) *runv1beta1.RunWorkerPool {
		t.Helper()
		obj := &runv1beta1.RunWorkerPool{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	// The stable equivalent of an alpha kind (docs/design.md decision 26)
	// is served at v1beta1 only.
	t.Run("v1alpha1 is not served", func(t *testing.T) {
		alpha := &unstructured.Unstructured{}
		alpha.SetGroupVersionKind(workerPoolGVK.GroupKind().WithVersion("v1alpha1"))
		alpha.SetNamespace(namespace)
		alpha.SetName("alpha")
		if err := env.Kube.Create(env.Ctx, alpha); !meta.IsNoMatchError(err) {
			t.Fatalf("creating a run.gcp.hfl.dev/v1alpha1 RunWorkerPool: err = %v, want no match for the kind", err)
		}
	})

	t.Run("a default and a named policy are refused", func(t *testing.T) {
		w := newWorkerPool("refused")
		w.Spec.BinaryAuthorization = &runv1beta1.BinaryAuthorization{UseDefault: mockenv.Ptr(true), Policy: mockenv.Ptr("projects/p/platforms/cloudRun/policies/x")}
		err := env.Kube.Create(env.Ctx, w)
		if err == nil || !strings.Contains(err.Error(), "one of useDefault and policy") {
			t.Errorf("creating a worker pool with both: err = %v, want one of useDefault and policy", err)
			if err == nil {
				_ = env.Kube.Delete(env.Ctx, w)
				env.WaitGone(w)
			}
		}
	})

	const name = "consumers"
	wp := newWorkerPool(name)
	if err := env.Kube.Create(env.Ctx, wp); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the worker pool with the fields Config Connector's lacks", func(t *testing.T) {
		u := env.WaitReady(workerPoolGVK, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != workerPoolCloudName(name) {
			t.Errorf("status.externalRef = %q, want %q", ref, workerPoolCloudName(name))
		}
		got := read(t, name)
		if got.GetTemplate().GetLabels()["tier"] != "consumer" {
			t.Errorf("cloud template labels = %v, want tier=consumer", got.GetTemplate().GetLabels())
		}
		if p := binauthzPolicy(); p != "" && got.GetBinaryAuthorization().GetPolicy() != p {
			t.Errorf("cloud Binary Authorization = %v, want policy %q", got.GetBinaryAuthorization(), p)
		}
		if got.GetScaling().GetManualInstanceCount() != 0 || got.GetDescription() != "queue consumers" {
			t.Errorf("cloud scaling / description = %v / %q", got.GetScaling(), got.GetDescription())
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		before := len(workerPoolMasks(t, env, name))
		gets := func() int {
			n := 0
			for _, r := range env.Requests() {
				if strings.HasPrefix(r, "GET ") && strings.Contains(r, "/v2/"+workerPoolCloudName(name)+"?") {
					n++
				}
			}
			return n
		}
		g := gets()
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for gets() == g {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the worker pool again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(workerPoolGVK, namespace, name)
		if m := workerPoolMasks(t, env, name)[before:]; len(m) != 0 {
			tmpl, _ := protojson.Marshal(read(t, name).GetTemplate())
			t.Errorf("updates after a resync with an unchanged spec: %q, want none; the cloud's template: %s", m, tmpl)
		}
	})

	t.Run("the template's labels change with one template update", func(t *testing.T) {
		before := len(workerPoolMasks(t, env, name))
		o := getObj(t, name)
		o.Spec.Template.Labels = map[string]string{"tier": "consumer", "queue": "orders"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(workerPoolGVK, namespace, name)
		got := read(t, name)
		if l := got.GetTemplate().GetLabels(); l["queue"] != "orders" || l["tier"] != "consumer" {
			t.Errorf("cloud template labels = %v", l)
		}
		if p := binauthzPolicy(); p != "" && got.GetBinaryAuthorization().GetPolicy() != p {
			t.Errorf("a template update changed Binary Authorization: %v", got.GetBinaryAuthorization())
		}
		if m := workerPoolMasks(t, env, name)[before:]; len(m) != 1 || m[0] != "template" {
			t.Errorf("update masks = %q, want one: template", m)
		}
	})

	t.Run("the policy changes alone", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("needs a second cloudRun platform policy; the fake's names are made up")
		}
		before := len(workerPoolMasks(t, env, name))
		o := getObj(t, name)
		o.Spec.BinaryAuthorization.Policy = mockenv.Ptr("projects/" + mockenv.Project + "/platforms/cloudRun/policies/consumers-relaxed")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(workerPoolGVK, namespace, name)
		got := read(t, name)
		if !strings.HasSuffix(got.GetBinaryAuthorization().GetPolicy(), "/consumers-relaxed") {
			t.Errorf("cloud Binary Authorization = %v, want the consumers-relaxed policy", got.GetBinaryAuthorization())
		}
		if got.GetTemplate().GetLabels()["queue"] != "orders" {
			t.Errorf("a policy update changed the template labels: %v", got.GetTemplate().GetLabels())
		}
		m := workerPoolMasks(t, env, name)[before:]
		if len(m) != 1 || (m[0] != "binary_authorization" && m[0] != "binaryAuthorization") {
			t.Errorf("update masks = %q, want one: binaryAuthorization", m)
		}
	})

	t.Run("location is immutable", func(t *testing.T) {
		o := getObj(t, name)
		o.Spec.Location = mockenv.Ptr("europe-west1")
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "location is immutable") {
			t.Errorf("changing the location: err = %v, want location is immutable", err)
		}
	})

	t.Run("delete removes the worker pool", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, wp); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&runv1beta1.RunWorkerPool{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := cloud.GetWorkerPool(env.Ctx, &pb.GetWorkerPoolRequest{Name: workerPoolCloudName(name)})
		if status.Code(err) != codes.NotFound {
			t.Errorf("reading the deleted worker pool: got %v, want NotFound", err)
		}
	})

	t.Run("abandon keeps the worker pool", func(t *testing.T) {
		const kept = "kept-pool"
		k := newWorkerPool(kept)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(workerPoolGVK, namespace, kept)
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		read(t, kept)
	})
}
