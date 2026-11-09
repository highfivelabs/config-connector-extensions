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

package iam_test

// IAMWorkloadIdentityPoolNamespace end to end on the fake cloud
// (pkg/test/mockenv, patch 0006). The case: managed workload identities, a
// namespace in a workload identity pool for one team's workloads.

import (
	"net/url"
	"strings"
	"testing"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/iam/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	iamv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	pool      = "app-pool"
)

var gvk = iamv1alpha1.IAMWorkloadIdentityPoolNamespaceGVK

func cloudName(ns string) string {
	return "projects/" + mockenv.Project + "/locations/global/workloadIdentityPools/" + pool + "/namespaces/" + ns
}

func newNamespace(name, description string) *iamv1alpha1.IAMWorkloadIdentityPoolNamespace {
	return &iamv1alpha1.IAMWorkloadIdentityPoolNamespace{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: iamv1alpha1.IAMWorkloadIdentityPoolNamespaceSpec{
			ProjectRef:              &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:                mockenv.Ptr("global"),
			WorkloadIdentityPoolRef: &iamv1alpha1.IAMWorkloadIdentityPoolRef{Name: pool},
			Description:             mockenv.Ptr(description),
		},
	}
}

func requests(env *mockenv.Env, method, name string) []string {
	var out []string
	for _, r := range env.Requests() {
		if strings.HasPrefix(r, method+" ") && strings.Contains(r, "/v1/"+name+"?") {
			out = append(out, r)
		}
	}
	return out
}

func TestWorkloadIdentityPoolNamespace(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	get := func(t *testing.T, name string) *api.WorkloadIdentityPoolNamespace {
		t.Helper()
		ns, err := cloud.Projects.Locations.WorkloadIdentityPools.Namespaces.Get(cloudName(name)).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the namespace from the fake cloud: %v", err)
		}
		return ns
	}
	getObj := func(t *testing.T, name string) *iamv1alpha1.IAMWorkloadIdentityPoolNamespace {
		t.Helper()
		obj := &iamv1alpha1.IAMWorkloadIdentityPoolNamespace{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	// The pool, as Config Connector's IAMWorkloadIdentityPool (not reconciled
	// here; read to resolve workloadIdentityPoolRef by name).
	p := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"location":   "global",
		"projectRef": map[string]any{"external": mockenv.Project},
	}}}
	p.SetGroupVersionKind(iamv1alpha1.IAMWorkloadIdentityPoolGVK)
	p.SetNamespace(namespace)
	p.SetName(pool)
	if err := env.Kube.Create(env.Ctx, p); err != nil {
		t.Fatal(err)
	}

	const name = "team-a"
	obj := newNamespace(name, "team A's workloads")
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the namespace in the referenced pool", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != cloudName(name) {
			t.Errorf("status.externalRef = %q, want %q", ref, cloudName(name))
		}
		if state, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "state"); state != "ACTIVE" {
			t.Errorf("status.observedState.state = %q, want ACTIVE", state)
		}
		ns := get(t, name)
		if ns.Description != "team A's workloads" || ns.Disabled || ns.State != "ACTIVE" {
			t.Errorf("cloud namespace = %+v", ns)
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		gets, patches := len(requests(env, "GET", cloudName(name))), len(requests(env, "PATCH", cloudName(name)))
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for len(requests(env, "GET", cloudName(name))) == gets {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the namespace again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(gvk, namespace, name)
		if got := len(requests(env, "PATCH", cloudName(name))); got != patches {
			t.Errorf("PATCH requests after a resync with an unchanged spec: %d, want none", got-patches)
		}
	})

	t.Run("description and disabled change in place", func(t *testing.T) {
		before := len(requests(env, "PATCH", cloudName(name)))
		o := getObj(t, name)
		o.Spec.Description = mockenv.Ptr("team A, paused")
		o.Spec.Disabled = mockenv.Ptr(true)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, name)
		ns := get(t, name)
		if ns.Description != "team A, paused" || !ns.Disabled {
			t.Errorf("cloud namespace = %+v, want the new description and disabled", ns)
		}
		patches := requests(env, "PATCH", cloudName(name))[before:]
		if len(patches) != 1 {
			t.Fatalf("PATCH requests = %v, want one", patches)
		}
		u, err := url.Parse(strings.TrimPrefix(patches[0], "PATCH "))
		if err != nil {
			t.Fatal(err)
		}
		if mask := u.Query().Get("updateMask"); mask != "description,disabled" {
			t.Errorf("updateMask = %q, want description,disabled", mask)
		}
	})

	t.Run("enabling it again sends disabled false", func(t *testing.T) {
		o := getObj(t, name)
		o.Spec.Disabled = mockenv.Ptr(false)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, name)
		if ns := get(t, name); ns.Disabled {
			t.Errorf("cloud namespace still disabled: %+v", ns)
		}
	})

	t.Run("a pool in another location is refused", func(t *testing.T) {
		bad := newNamespace("elsewhere", "")
		bad.Spec.WorkloadIdentityPoolRef = &iamv1alpha1.IAMWorkloadIdentityPoolRef{
			External: "projects/" + mockenv.Project + "/locations/us-central1/workloadIdentityPools/" + pool,
		}
		if err := env.Kube.Create(env.Ctx, bad); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(gvk, namespace, "elsewhere")
		if !strings.Contains(msg, "workloadIdentityPoolRef is in") {
			t.Errorf("Ready message = %q, want the pool's location named", msg)
		}
		if got := requests(env, "POST", "projects/"+mockenv.Project+"/locations/global/workloadIdentityPools/"+pool+"/namespaces"); len(got) != 1 {
			t.Errorf("namespace create requests = %v, want only team-a's", got)
		}
	})

	t.Run("delete soft-deletes the namespace", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&iamv1alpha1.IAMWorkloadIdentityPoolNamespace{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		if ns := get(t, name); ns.State != "DELETED" || ns.ExpireTime == "" {
			t.Errorf("cloud namespace after delete = %+v, want state DELETED with an expireTime", ns)
		}
	})

	t.Run("a soft-deleted ID cannot be reused", func(t *testing.T) {
		again := newNamespace(name, "team A again")
		if err := env.Kube.Create(env.Ctx, again); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(gvk, namespace, name)
		if !strings.Contains(msg, "already exists") {
			t.Errorf("Ready message = %q, want the API's already-exists error", msg)
		}
		// Abandon it, so deleting the object does not wait on the cloud.
		o := getObj(t, name)
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
	})

	t.Run("abandon keeps the namespace", func(t *testing.T) {
		const kept = "team-b"
		k := newNamespace(kept, "team B's workloads")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, kept)
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if ns := get(t, kept); ns.State != "ACTIVE" {
			t.Errorf("abandoned namespace = %+v, want it ACTIVE", ns)
		}
	})
}
