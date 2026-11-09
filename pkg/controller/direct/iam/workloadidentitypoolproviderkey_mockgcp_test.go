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

// IAMWorkloadIdentityPoolProviderKey end to end on the fake cloud
// (pkg/test/mockenv, patch 0010). The case: a SAML identity provider that
// encrypts its assertions, with the public key read from status.
//
// Against a real project (docs/live-runs.md) the test makes its own pool
// (usePool) and deletes it at the end, with everything in it.

import (
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

func TestWorkloadIdentityPoolProviderKey(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	usePool(t, env, cloud, "key-pool", "")
	useSAMLProvider(t, env, cloud, "saml-idp")
	keys := cloud.Projects.Locations.WorkloadIdentityPools.Providers.Keys
	keyGVK := iamv1alpha1.IAMWorkloadIdentityPoolProviderKeyGVK

	// The pool and provider, as Config Connector objects (not reconciled
	// here; read to resolve workloadIdentityPoolProviderRef by name).
	kcc := func(gvk metav1.GroupVersionKind, name string, spec map[string]any) {
		u := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
		u.SetAPIVersion(gvk.Group + "/" + gvk.Version)
		u.SetKind(gvk.Kind)
		u.SetNamespace(namespace)
		u.SetName(name)
		if err := env.Kube.Create(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	projectRef := map[string]any{"external": mockenv.Project}
	kcc(metav1.GroupVersionKind(iamv1alpha1.IAMWorkloadIdentityPoolGVK), pool, map[string]any{"location": "global", "projectRef": projectRef})
	kcc(metav1.GroupVersionKind(iamv1alpha1.IAMWorkloadIdentityPoolProviderGVK), "corp-saml", map[string]any{
		"location": "global", "projectRef": projectRef, "resourceID": "saml-idp",
		"workloadIdentityPoolRef": map[string]any{"name": pool},
	})

	const name = "saml-encryption"
	cloudKey := "projects/" + mockenv.Project + "/locations/global/workloadIdentityPools/" + pool + "/providers/saml-idp/keys/" + name
	key := &iamv1alpha1.IAMWorkloadIdentityPoolProviderKey{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: iamv1alpha1.IAMWorkloadIdentityPoolProviderKeySpec{
			ProjectRef:                      &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:                        mockenv.Ptr("global"),
			WorkloadIdentityPoolProviderRef: &iamv1alpha1.IAMWorkloadIdentityPoolProviderRef{Name: "corp-saml"},
			Use:                             "ENCRYPTION",
			KeyData:                         &iamv1alpha1.IAMWorkloadIdentityPoolProviderKeyKeyData{KeySpec: "RSA_2048"},
		},
	}
	if err := env.Kube.Create(env.Ctx, key); err != nil {
		t.Fatal(err)
	}
	getObj := func(t *testing.T) *iamv1alpha1.IAMWorkloadIdentityPoolProviderKey {
		t.Helper()
		obj := &iamv1alpha1.IAMWorkloadIdentityPoolProviderKey{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	creates := func() int {
		return len(requests(env, "POST", strings.TrimSuffix(cloudKey, "/"+name)))
	}

	t.Run("create makes the key and shows its public half", func(t *testing.T) {
		u := env.WaitReady(keyGVK, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != cloudKey {
			t.Errorf("status.externalRef = %q, want %q", ref, cloudKey)
		}
		pem, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "keyData", "key")
		format, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "keyData", "format")
		if !strings.Contains(pem, "BEGIN CERTIFICATE") || format != "RSA_X509_PEM" {
			t.Errorf("status.observedState.keyData = %q, %q; want the PEM and its format", pem, format)
		}
		got, err := keys.Get(cloudKey).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the key from the cloud: %v", err)
		}
		if got.State != "ACTIVE" || got.Use != "ENCRYPTION" || got.KeyData.KeySpec != "RSA_2048" {
			t.Errorf("cloud key = %+v", got)
		}
	})

	t.Run("a resync makes no second key", func(t *testing.T) {
		before := creates()
		gets := len(requests(env, "GET", cloudKey))
		o := getObj(t)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for len(requests(env, "GET", cloudKey)) == gets {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the key again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(keyGVK, namespace, name)
		if creates() != before {
			t.Errorf("a resync created %d more keys", creates()-before)
		}
	})

	t.Run("a key cannot be changed", func(t *testing.T) {
		o := getObj(t)
		o.Spec.KeyData.KeySpec = "RSA_4096"
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(keyGVK, namespace, name)
		if !strings.Contains(msg, "immutable") || !strings.Contains(msg, "keySpec") {
			t.Errorf("Ready message = %q, want the immutable keySpec named", msg)
		}
	})

	t.Run("delete soft-deletes the key", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, key); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&iamv1alpha1.IAMWorkloadIdentityPoolProviderKey{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		got, err := keys.Get(cloudKey).Context(env.Ctx).Do()
		if err != nil || got.State != "DELETED" {
			t.Errorf("cloud key after delete = %+v, %v; want state DELETED", got, err)
		}
	})

	t.Run("abandon keeps the key", func(t *testing.T) {
		k := key.DeepCopy()
		k.ObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: "kept", Annotations: map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(keyGVK, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		got, err := keys.Get(strings.TrimSuffix(cloudKey, name) + "kept").Context(env.Ctx).Do()
		if err != nil || got.State != "ACTIVE" {
			t.Errorf("abandoned key = %+v, %v; want it ACTIVE", got, err)
		}
	})
}
