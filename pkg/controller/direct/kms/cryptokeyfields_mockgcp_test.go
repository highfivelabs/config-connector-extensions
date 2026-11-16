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

package kms_test

// KMSCryptoKeyFields end to end on the fake cloud (pkg/test/mockenv, patch
// 0052) or a real project (docs/live-runs.md). The key is the run's own
// (mockenv.Name), made with no key version, so it costs nothing; Cloud KMS
// never deletes a key or a key ring, so the key stays, its policy cleared,
// in the one key ring the runs share (cce-e2e-fields).

import (
	"slices"
	"strings"
	"testing"
	"time"

	api "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	hflkms "github.com/highfivelabs/config-connector-extensions/apis/kms/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var configConnectorKMSCryptoKeyGVK = schema.GroupVersionKind{Group: "kms.cnrm.cloud.google.com", Version: "v1beta1", Kind: "KMSCryptoKey"}

func TestKMSCryptoKeyFields(t *testing.T) {
	const (
		namespace = "example"
		location  = "us-central1"
		ring      = "cce-e2e-fields"
		name      = "kaj"
	)
	gvk := hflkms.KMSCryptoKeyFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := api.NewKeyManagementRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	// The key ring the runs share, made once (a key ring is never deleted),
	// and the run's key, with no version.
	ringName := "projects/" + mockenv.Project + "/locations/" + location + "/keyRings/" + ring
	if _, err := c.GetKeyRing(env.Ctx, &kmspb.GetKeyRingRequest{Name: ringName}); err != nil {
		if !direct.IsNotFound(err) {
			t.Fatal(err)
		}
		if _, err := c.CreateKeyRing(env.Ctx, &kmspb.CreateKeyRingRequest{
			Parent: "projects/" + mockenv.Project + "/locations/" + location, KeyRingId: ring, KeyRing: &kmspb.KeyRing{},
		}); err != nil {
			t.Fatal(err)
		}
	}
	keyID := mockenv.Name("kaj-key")
	key, err := c.CreateCryptoKey(env.Ctx, &kmspb.CreateCryptoKeyRequest{
		Parent: ringName, CryptoKeyId: keyID, SkipInitialVersionCreation: true,
		CryptoKey: &kmspb.CryptoKey{Purpose: kmspb.CryptoKey_ENCRYPT_DECRYPT, Labels: mockenv.Labels()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keyName := key.GetName()
	// Whatever a subtest left, the key ends with no policy.
	t.Cleanup(func() {
		if _, err := c.UpdateCryptoKey(env.Ctx, &kmspb.UpdateCryptoKeyRequest{
			CryptoKey:  &kmspb.CryptoKey{Name: keyName},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"key_access_justifications_policy"}},
		}); err != nil {
			t.Errorf("clearing the key's policy at the end: %v", err)
		}
	})

	// Config Connector's KMSCryptoKey for the key (not reconciled here: read
	// to resolve resourceRef by name, from its status.selfLink as the
	// Terraform-based controller writes it).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"keyRingRef": map[string]any{"external": ringName}, "resourceID": keyID, "purpose": "ENCRYPT_DECRYPT",
	}}}
	cc.SetGroupVersionKind(configConnectorKMSCryptoKeyGVK)
	cc.SetNamespace(namespace)
	cc.SetName("key")
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(cc.Object, keyName, "status", "selfLink"); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Status().Update(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	getKey := func(t *testing.T) *kmspb.CryptoKey {
		t.Helper()
		k, err := c.GetCryptoKey(env.Ctx, &kmspb.GetCryptoKeyRequest{Name: keyName})
		if err != nil {
			t.Fatalf("reading the key: %v", err)
		}
		return k
	}
	reasons := func(k *kmspb.CryptoKey) []string {
		if k.GetKeyAccessJustificationsPolicy() == nil {
			return nil
		}
		out := []string{}
		for _, r := range k.GetKeyAccessJustificationsPolicy().GetAllowedAccessReasons() {
			out = append(out, r.String())
		}
		slices.Sort(out)
		return out
	}
	patches := func() []string {
		var out []string
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, "/cryptoKeys/"+keyID+"?") {
				out = append(out, req)
			}
		}
		return out
	}
	getObj := func(t *testing.T, name string) *hflkms.KMSCryptoKeyFields {
		t.Helper()
		o := &hflkms.KMSCryptoKeyFields{}
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
	newFields := func(name string, ref *kmsv1beta1.KMSCryptoKeyRef, rs ...string) *hflkms.KMSCryptoKeyFields {
		return &hflkms.KMSCryptoKeyFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: hflkms.KMSCryptoKeyFieldsSpec{
				ResourceRef:                   ref,
				KeyAccessJustificationsPolicy: &hflkms.KeyAccessJustificationsPolicy{AllowedAccessReasons: rs},
			},
		}
	}

	obj := newFields(name, &kmsv1beta1.KMSCryptoKeyRef{Name: "key"}, "CUSTOMER_INITIATED_ACCESS", "GOOGLE_INITIATED_SYSTEM_OPERATION")
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the policy with a mask naming only it", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != keyName {
			t.Errorf("status.externalRef = %q, want %q", ref, keyName)
		}
		if got, want := reasons(getKey(t)), []string{"CUSTOMER_INITIATED_ACCESS", "GOOGLE_INITIATED_SYSTEM_OPERATION"}; !slices.Equal(got, want) {
			t.Errorf("the key's allowed reasons = %v, want %v", got, want)
		}
		p := patches()
		if len(p) != 1 || !strings.Contains(p[0], "updateMask=keyAccessJustificationsPolicy") || strings.Contains(p[0], "%2C") || strings.Contains(p[0], ",") {
			t.Errorf("patches = %v, want one with updateMask=keyAccessJustificationsPolicy alone", p)
		}
	})

	t.Run("an update of Config Connector's fields keeps the policy", func(t *testing.T) {
		// What Terraform's update of a KMSCryptoKey sends when its labels
		// change: the key's labels, with a mask of labels.
		labels := map[string]string{"changed-by": "config-connector"}
		for k, v := range mockenv.Labels() {
			labels[k] = v
		}
		if _, err := c.UpdateCryptoKey(env.Ctx, &kmspb.UpdateCryptoKeyRequest{
			CryptoKey:  &kmspb.CryptoKey{Name: keyName, Labels: labels},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"labels"}},
		}); err != nil {
			t.Fatal(err)
		}
		k := getKey(t)
		if k.GetLabels()["changed-by"] != "config-connector" {
			t.Errorf("labels = %v, want the change", k.GetLabels())
		}
		if got := reasons(k); len(got) != 2 {
			t.Errorf("after Config Connector's update the allowed reasons = %v, want the two set", got)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		resync(t, name)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the key %d times", n)
		}
	})

	t.Run("a changed list replaces the policy", func(t *testing.T) {
		o := getObj(t, name)
		o.Spec.KeyAccessJustificationsPolicy.AllowedAccessReasons = []string{"CUSTOMER_INITIATED_SUPPORT"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for !slices.Equal(reasons(getKey(t)), []string{"CUSTOMER_INITIATED_SUPPORT"}) {
			if time.Now().After(deadline) {
				t.Fatalf("the key's allowed reasons = %v, want [CUSTOMER_INITIATED_SUPPORT]", reasons(getKey(t)))
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, name)
	})

	t.Run("a policy cleared outside is set again", func(t *testing.T) {
		if _, err := c.UpdateCryptoKey(env.Ctx, &kmspb.UpdateCryptoKeyRequest{
			CryptoKey:  &kmspb.CryptoKey{Name: keyName},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"key_access_justifications_policy"}},
		}); err != nil {
			t.Fatal(err)
		}
		if got := reasons(getKey(t)); got != nil {
			t.Fatalf("clearing outside left %v", got)
		}
		resync(t, name)
		if got := reasons(getKey(t)); !slices.Equal(got, []string{"CUSTOMER_INITIATED_SUPPORT"}) {
			t.Errorf("after a resync the allowed reasons = %v, want [CUSTOMER_INITIATED_SUPPORT]", got)
		}
	})

	t.Run("a second object for the key is refused", func(t *testing.T) {
		dup := newFields("dup", &kmsv1beta1.KMSCryptoKeyRef{External: keyName}, "CUSTOMER_INITIATED_ACCESS")
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if got := reasons(getKey(t)); !slices.Equal(got, []string{"CUSTOMER_INITIATED_SUPPORT"}) {
			t.Errorf("the refused object changed the key: %v", got)
		}
		dup.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing key is waited for, never made", func(t *testing.T) {
		missing := ringName + "/cryptoKeys/" + mockenv.Name("never-made")
		w := newFields("waits", &kmsv1beta1.KMSCryptoKeyRef{External: missing}, "CUSTOMER_INITIATED_ACCESS")
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing key waited for", msg)
		}
		if _, err := c.GetCryptoKey(env.Ctx, &kmspb.GetCryptoKeyRequest{Name: missing}); !direct.IsNotFound(err) {
			t.Errorf("reading the missing key: %v, want not found", err)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete clears the policy and keeps the key", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&hflkms.KMSCryptoKeyFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		k := getKey(t)
		if got := reasons(k); got != nil {
			t.Errorf("after delete the allowed reasons = %v, want no policy", got)
		}
		if k.GetLabels()["changed-by"] != "config-connector" {
			t.Errorf("delete changed the key's other fields: labels %v", k.GetLabels())
		}
	})

	t.Run("abandon keeps the policy", func(t *testing.T) {
		k := newFields("kept", &kmsv1beta1.KMSCryptoKeyRef{Name: "key"}, "CUSTOMER_INITIATED_ACCESS")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := reasons(getKey(t)); !slices.Equal(got, []string{"CUSTOMER_INITIATED_ACCESS"}) {
			t.Errorf("after an abandoning delete the allowed reasons = %v, want [CUSTOMER_INITIATED_ACCESS]", got)
		}
	})
}
