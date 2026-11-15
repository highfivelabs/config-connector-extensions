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

package discokindtest

// A LookerInstance's OAuth client secret comes from a Kubernetes Secret
// (spec.oauthConfig.clientSecret.valueFrom.secretKeyRef, Config
// Connector's secret.Legacy; docs/design.md decision 33), so a chart never
// puts it in the manifest. The API never returns oauthConfig's client ID
// or secret, so they are never a difference: a resync writes nothing, and
// they are sent again only when status.inputVersion (a digest of the
// client ID and the Secret's name, key and resourceVersion) changes, as
// when the Secret is rotated.

import (
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// oauthSecret is the Secret the LookerInstance fixtures read the client
// secret from.
const oauthSecret = "looker-oauth"

func init() { kindtest.RegisterSetup("LookerInstance", makeOAuthSecret) }

// makeOAuthSecret makes the fixtures' Secret in the kindtest's namespace.
func makeOAuthSecret(t *testing.T, env *mockenv.Env) {
	createSecret(t, env, "discokind", "fixture-client-secret")
}

func createSecret(t *testing.T, env *mockenv.Env, namespace, value string) *corev1.Secret {
	t.Helper()
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: oauthSecret},
		Data:       map[string][]byte{"client-secret": []byte(value)},
	}
	if err := env.Kube.Create(env.Ctx, s); err != nil {
		t.Fatalf("creating the Secret %s/%s: %v", namespace, oauthSecret, err)
	}
	return s
}

func lookerInstance(name string, clientSecret map[string]any) *unstructured.Unstructured {
	k := discokind.ByKind("LookerInstance")
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": name, "namespace": secretNS},
		"spec": map[string]any{
			"projectRef":      map[string]any{"external": mockenv.Project},
			"location":        "us-central1",
			"platformEdition": "LOOKER_CORE_STANDARD_ANNUAL",
			"publicIpEnabled": true,
			"oauthConfig": map[string]any{
				"clientId":     name + ".apps.googleusercontent.com",
				"clientSecret": clientSecret,
			},
		},
	}}
	u.SetGroupVersionKind(k.GVK)
	return u
}

const secretNS = "looker-secret"

// TestLookerInstanceOAuthClientSecret: the client secret is read from the
// Secret, reaches the cloud, never shows in a read, is not a difference on
// a resync, and is sent again when the Secret changes. Fake only: a live
// Looker instance is billed by the year (the kind's live-skip).
func TestLookerInstanceOAuthClientSecret(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a Looker instance is billed by the year; see testdata/discokind/LookerInstance/live-skip")
	}
	k := discokind.ByKind("LookerInstance")
	env := mockenv.New(t, secretNS)
	ref := map[string]any{"valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": oauthSecret, "key": "client-secret"}}}
	obj := lookerInstance(mockenv.Name("lookerinstance-secret"), ref)
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}
	get := func() *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(k.GVK)
		if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(obj), u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	stored := func(name string) map[string]any {
		got := env.Disco.Stored(k, name)
		oc, _ := got["oauthConfig"].(map[string]any)
		return oc
	}

	t.Run("a missing Secret holds the create", func(t *testing.T) {
		msg := env.WaitNotReady(k.GVK, secretNS, obj.GetName())
		if !strings.Contains(msg, "Secret "+secretNS+"/"+oauthSecret+" not found") {
			t.Errorf("Ready's message is %q, want the missing Secret named", msg)
		}
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/instances?") {
				t.Errorf("a create went out without the Secret: %s", r)
			}
		}
	})

	sec := createSecret(t, env, secretNS, "s3cret-1")
	var name, version string
	t.Run("the Secret's value reaches the cloud", func(t *testing.T) {
		u := env.WaitReady(k.GVK, secretNS, obj.GetName())
		name, _, _ = unstructured.NestedString(u.Object, "status", "externalRef")
		version, _, _ = unstructured.NestedString(u.Object, "status", "inputVersion")
		if version == "" {
			t.Error("status.inputVersion is not set")
		}
		if strings.Contains(version, "s3cret") {
			t.Errorf("status.inputVersion %q holds the secret", version)
		}
		oc := stored(name)
		if oc["clientSecret"] != "s3cret-1" || oc["clientId"] != obj.GetName()+".apps.googleusercontent.com" {
			t.Errorf("the cloud holds oauthConfig %v, want the Secret's value and the client ID", oc)
		}
		api := &discokind.API{HTTP: env.HTTP, BaseURL: k.BaseURL}
		read, err := api.Get(env.Ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		readOC, _ := read["oauthConfig"].(map[string]any)
		if _, ok := readOC["clientSecret"]; ok {
			t.Errorf("a read shows the client secret: %v", read["oauthConfig"])
		}
	})
	if name == "" {
		t.FailNow()
	}

	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "PATCH ") || strings.HasPrefix(r, "POST ")) && strings.Contains(r, name) {
				n++
			}
		}
		return n
	}
	resync := func() {
		u := get()
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		ann[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
		u.SetAnnotations(ann)
		if err := env.Kube.Update(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		resync()
		time.Sleep(3 * time.Second)
		env.WaitReady(k.GVK, secretNS, obj.GetName())
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("a rotated Secret is sent again", func(t *testing.T) {
		sec.Data["client-secret"] = []byte("s3cret-2")
		if err := env.Kube.Update(env.Ctx, sec); err != nil {
			t.Fatal(err)
		}
		resync()
		deadline := time.Now().Add(mockenv.Timeout())
		for stored(name)["clientSecret"] != "s3cret-2" {
			if time.Now().After(deadline) {
				t.Fatalf("the cloud still holds oauthConfig %v", stored(name))
			}
			time.Sleep(200 * time.Millisecond)
		}
		deadline = time.Now().Add(mockenv.Timeout())
		for {
			v, _, _ := unstructured.NestedString(get().Object, "status", "inputVersion")
			if v != "" && v != version {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("status.inputVersion is still %q", v)
			}
			time.Sleep(200 * time.Millisecond)
		}
	})

	t.Run("a plain value works too", func(t *testing.T) {
		plain := lookerInstance(mockenv.Name("lookerinstance-plain"), map[string]any{"value": "plain-secret"})
		if err := env.Kube.Create(env.Ctx, plain); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(k.GVK, secretNS, plain.GetName())
		n, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		if got := stored(n)["clientSecret"]; got != "plain-secret" {
			t.Errorf("the cloud holds client secret %v, want the spec's value", got)
		}
	})

	t.Run("value and valueFrom together are refused", func(t *testing.T) {
		both := lookerInstance(mockenv.Name("lookerinstance-both"), map[string]any{"value": "x", "valueFrom": ref["valueFrom"]})
		if err := env.Kube.Create(env.Ctx, both); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(k.GVK, secretNS, both.GetName()); !strings.Contains(msg, "not both") {
			t.Errorf("Ready's message is %q, want value and valueFrom refused", msg)
		}
	})
}
