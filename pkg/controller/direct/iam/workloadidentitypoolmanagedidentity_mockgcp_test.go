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

// IAMWorkloadIdentityPoolManagedIdentity end to end on the fake cloud
// (pkg/test/mockenv, patches 0006 and 0007). The case: a managed identity
// for one team's service, in that team's namespace, that two Compute Engine
// instances may receive.
//
// Against a real project (docs/live-runs.md) the test makes its own pool
// (usePool) and deletes it at the end, with everything in it.

import (
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/iam/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var miGVK = iamv1alpha1.IAMWorkloadIdentityPoolManagedIdentityGVK

const (
	vm1 = "//compute.googleapis.com/projects/123/uid/zones/us-central1-a/instances/1001"
	vm2 = "//compute.googleapis.com/projects/123/uid/zones/us-central1-a/instances/1002"
	vm3 = "//compute.googleapis.com/projects/123/uid/zones/us-central1-a/instances/1003"
)

func miCloudName(ns, id string) string {
	return cloudName(ns) + "/managedIdentities/" + id
}

func rules(resources ...string) []iamv1alpha1.AttestationRule {
	out := []iamv1alpha1.AttestationRule{}
	for _, r := range resources {
		out = append(out, iamv1alpha1.AttestationRule{GoogleCloudResource: r})
	}
	return out
}

func TestWorkloadIdentityPoolManagedIdentity(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	usePool(t, env, cloud, "mi-pool", "TRUST_DOMAIN")
	mis := cloud.Projects.Locations.WorkloadIdentityPools.Namespaces.ManagedIdentities

	// The pool (Config Connector's, read only) and the namespace (ours, Ready).
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
	const ns = "team-a"
	if err := env.Kube.Create(env.Ctx, newNamespace(ns, "team A's workloads")); err != nil {
		t.Fatal(err)
	}
	env.WaitReady(gvk, namespace, ns)

	const id = "inference"
	name := miCloudName(ns, id)
	mi := &iamv1alpha1.IAMWorkloadIdentityPoolManagedIdentity{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: id},
		Spec: iamv1alpha1.IAMWorkloadIdentityPoolManagedIdentitySpec{
			ProjectRef:                       &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:                         mockenv.Ptr("global"),
			WorkloadIdentityPoolNamespaceRef: &iamv1alpha1.IAMWorkloadIdentityPoolNamespaceRef{Name: ns},
			Description:                      mockenv.Ptr("the inference service"),
			AttestationRules:                 rules(vm1, vm2),
		},
	}
	if err := env.Kube.Create(env.Ctx, mi); err != nil {
		t.Fatal(err)
	}

	getObj := func(t *testing.T) *iamv1alpha1.IAMWorkloadIdentityPoolManagedIdentity {
		t.Helper()
		obj := &iamv1alpha1.IAMWorkloadIdentityPoolManagedIdentity{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: id}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	cloudRules := func(t *testing.T) []string {
		t.Helper()
		resp, err := mis.ListAttestationRules(name).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("listing attestation rules in the cloud: %v", err)
		}
		out := []string{}
		for _, r := range resp.AttestationRules {
			out = append(out, r.GoogleCloudResource)
		}
		slices.Sort(out)
		return out
	}
	sets := func() int { return len(requests(env, "POST", name+":setAttestationRules")) }

	t.Run("create makes the identity in the namespace, with its rules", func(t *testing.T) {
		u := env.WaitReady(miGVK, namespace, id)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != name {
			t.Errorf("status.externalRef = %q, want %q", ref, name)
		}
		got, err := mis.Get(name).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the managed identity from the cloud: %v", err)
		}
		if got.Description != "the inference service" || got.State != "ACTIVE" {
			t.Errorf("cloud managed identity = %+v", got)
		}
		if r := cloudRules(t); !slices.Equal(r, []string{vm1, vm2}) {
			t.Errorf("cloud attestation rules = %v, want %v", r, []string{vm1, vm2})
		}
	})

	t.Run("a resync changes nothing", func(t *testing.T) {
		gets, patches, set := len(requests(env, "GET", name)), len(requests(env, "PATCH", name)), sets()
		o := getObj(t)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for len(requests(env, "GET", name)) == gets {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the managed identity again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(miGVK, namespace, id)
		if len(requests(env, "PATCH", name)) != patches || sets() != set {
			t.Errorf("a resync with an unchanged spec sent %d PATCH and %d setAttestationRules", len(requests(env, "PATCH", name))-patches, sets()-set)
		}
	})

	t.Run("changed rules replace the set, alone", func(t *testing.T) {
		patches, set := len(requests(env, "PATCH", name)), sets()
		o := getObj(t)
		o.Spec.AttestationRules = rules(vm3, vm2)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(miGVK, namespace, id)
		if r := cloudRules(t); !slices.Equal(r, []string{vm2, vm3}) {
			t.Errorf("cloud attestation rules = %v, want %v", r, []string{vm2, vm3})
		}
		if sets()-set != 1 || len(requests(env, "PATCH", name)) != patches {
			t.Errorf("sent %d setAttestationRules and %d PATCH, want 1 and 0", sets()-set, len(requests(env, "PATCH", name))-patches)
		}
	})

	t.Run("description and disabled change in place", func(t *testing.T) {
		before := len(requests(env, "PATCH", name))
		o := getObj(t)
		o.Spec.Description = mockenv.Ptr("paused")
		o.Spec.Disabled = mockenv.Ptr(true)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(miGVK, namespace, id)
		got, err := mis.Get(name).Context(env.Ctx).Do()
		if err != nil {
			t.Fatal(err)
		}
		if got.Description != "paused" || !got.Disabled {
			t.Errorf("cloud managed identity = %+v", got)
		}
		patches := requests(env, "PATCH", name)[before:]
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

	t.Run("an empty list removes every rule", func(t *testing.T) {
		// A merge patch, as kubectl sends it: the typed client would drop an
		// empty list (omitempty), and an absent list leaves the rules alone.
		o := getObj(t)
		if err := env.Kube.Patch(env.Ctx, o, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"attestationRules":[]}}`))); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for len(cloudRules(t)) != 0 {
			if time.Now().After(deadline) {
				t.Fatalf("cloud attestation rules = %v, want none", cloudRules(t))
			}
			time.Sleep(200 * time.Millisecond)
		}
		env.WaitReady(miGVK, namespace, id)
	})

	t.Run("delete soft-deletes the identity", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, mi); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&iamv1alpha1.IAMWorkloadIdentityPoolManagedIdentity{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: id}})
		got, err := mis.Get(name).Context(env.Ctx).Do()
		if err != nil {
			t.Fatal(err)
		}
		if got.State != "DELETED" {
			t.Errorf("cloud managed identity after delete = %+v, want state DELETED", got)
		}
	})

	t.Run("abandon keeps the identity", func(t *testing.T) {
		const kept = "batch"
		k := mi.DeepCopy()
		k.ObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: kept, Annotations: map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}}
		k.Spec.AttestationRules = nil
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(miGVK, namespace, kept)
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		got, err := mis.Get(miCloudName(ns, kept)).Context(env.Ctx).Do()
		if err != nil || got.State != "ACTIVE" {
			t.Errorf("abandoned managed identity = %+v, %v; want it ACTIVE", got, err)
		}
	})
}
