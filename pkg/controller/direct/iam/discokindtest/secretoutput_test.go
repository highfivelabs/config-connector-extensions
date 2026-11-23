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

// The secrets the API returns, an OAuth client credential's clientSecret
// and a SCIM token's securityToken, go to a Kubernetes Secret owned by the
// object, never to status (docs/design.md decision 38, the discokind
// option secretOutputs). These tests prove the value reaches the Secret
// and nowhere else the cluster or the manager shows it: not the object
// (status included), not an event, not a log line at any verbosity.

import (
	"bytes"
	"flag"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"github.com/go-logr/logr/funcr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func init() {
	kindtest.RegisterSetup("IAMOauthClientCredential", func(t *testing.T, env *mockenv.Env) {
		makeOAuthClient(t, env, mockenv.Name("iamoauthclientcredential")+"-client")
	})
}

// makeOAuthClient makes the OAuth client a credential belongs to, in the
// project, and deletes it after the test.
func makeOAuthClient(t *testing.T, env *mockenv.Env, id string) {
	t.Helper()
	k := discokind.ByKind("IAMOauthClient")
	api := &discokind.API{HTTP: env.HTTP, BaseURL: k.BaseURL}
	coll := "projects/" + mockenv.Project + "/locations/global/oauthClients"
	body := map[string]any{
		"displayName":         "cce credential test",
		"clientType":          "CONFIDENTIAL_CLIENT",
		"allowedGrantTypes":   []any{"AUTHORIZATION_CODE_GRANT"},
		"allowedRedirectUris": []any{"https://www.example.com/callback"},
		"allowedScopes":       []any{"https://www.googleapis.com/auth/cloud-platform"},
	}
	err := kindtest.RetryQuota(func() error {
		_, err := api.Do(env.Ctx, http.MethodPost, coll, map[string]string{"oauthClientId": id}, body)
		return err
	})
	if err != nil {
		t.Fatalf("making the OAuth client %s: %v", id, err)
	}
	t.Cleanup(func() {
		if _, err := api.Do(env.Ctx, http.MethodDelete, coll+"/"+id, nil, nil); err != nil {
			t.Errorf("deleting the OAuth client %s: %v", id, err)
		}
	})
}

// logBuffer keeps every log line the manager writes.
type logBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuffer) write(prefix, args string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.WriteString(prefix + " " + args + "\n")
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// captureLogs sends every log line, klog's and controller-runtime's, at
// every verbosity, to a buffer until the test ends. Call it before
// mockenv.New, so the manager logs there from its start.
func captureLogs(t *testing.T) *logBuffer {
	buf := &logBuffer{}
	logger := funcr.New(buf.write, funcr.Options{Verbosity: 10})
	var fs flag.FlagSet
	klog.InitFlags(&fs)
	_ = fs.Set("v", "10")
	klog.SetLoggerWithOptions(logger, klog.ContextualLogger(true))
	ctrl.SetLogger(logger)
	t.Cleanup(func() {
		_ = fs.Set("v", "0")
		klog.ClearLogger()
	})
	return buf
}

const outputNS = "secret-outputs"

func newObject(kind, name string, spec map[string]any) *unstructured.Unstructured {
	k := discokind.ByKind(kind)
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": name, "namespace": outputNS},
		"spec":     spec,
	}}
	u.SetGroupVersionKind(k.GVK)
	return u
}

// TestSecretOutputs: each kind's secret reaches its Secret and nothing
// else; a Secret that is not the object's stops the create before the API
// is called; a lost clientSecret is written again from a read; a lost
// securityToken, which only the create returns, makes the object not
// Ready with what to do, and no second token is made. Fake only: it
// reads what the fake holds (Stored); the kinds' own tests run live.
func TestSecretOutputs(t *testing.T) {
	if mockenv.Live() {
		t.Skip("reads the fake's store; TestIAMOauthClientCredential checks the Secret live")
	}
	logs := captureLogs(t)
	env := mockenv.New(t, outputNS)
	cred := discokind.ByKind("IAMOauthClientCredential")
	tok := discokind.ByKind("IAMWorkforcePoolProviderScimToken")
	makeOAuthClient(t, env, "secret-test-client")
	var values []string

	get := func(u *unstructured.Unstructured) *unstructured.Unstructured {
		t.Helper()
		got := &unstructured.Unstructured{}
		got.SetGroupVersionKind(u.GroupVersionKind())
		if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(u), got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	resync := func(u *unstructured.Unstructured) {
		t.Helper()
		got := get(u)
		ann := got.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		ann[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
		got.SetAnnotations(ann)
		if err := env.Kube.Update(env.Ctx, got); err != nil {
			t.Fatal(err)
		}
	}
	secret := func(name string) *corev1.Secret {
		t.Helper()
		s := &corev1.Secret{}
		if err := env.Kube.Get(env.Ctx, client.ObjectKey{Namespace: outputNS, Name: name}, s); err != nil {
			t.Fatalf("the Secret %s: %v", name, err)
		}
		return s
	}
	deleteSecret := func(name string) {
		t.Helper()
		if err := env.Kube.Delete(env.Ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: outputNS, Name: name}}); err != nil {
			t.Fatal(err)
		}
	}
	count := func(method, part string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, method+" ") && strings.Contains(r, part) {
				n++
			}
		}
		return n
	}
	credSpec := func(id string) map[string]any {
		return map[string]any{
			"projectRef":  map[string]any{"external": mockenv.Project},
			"location":    "global",
			"oauthClient": "secret-test-client",
			"resourceID":  id,
			"displayName": "cce secret test",
		}
	}

	t.Run("a Secret that is not the object's stops the create", func(t *testing.T) {
		taken := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: outputNS, Name: "taken"}, Data: map[string][]byte{"other": []byte("x")}}
		if err := env.Kube.Create(env.Ctx, taken); err != nil {
			t.Fatal(err)
		}
		spec := credSpec("cred-taken")
		spec["secretOutput"] = map[string]any{"name": "taken"}
		obj := newObject(cred.GVK.Kind, "cred-taken", spec)
		if err := env.Kube.Create(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(cred.GVK, outputNS, obj.GetName())
		if !strings.Contains(msg, "secret-outputs/taken exists and is not this object's") {
			t.Errorf("Ready's message is %q, want the Secret named as not the object's", msg)
		}
		if n := count(http.MethodPost, "/credentials?"); n != 0 {
			t.Errorf("%d creates went out with the Secret taken", n)
		}
		if s := secret("taken"); string(s.Data["other"]) != "x" || len(s.Data) != 1 || len(s.OwnerReferences) != 0 {
			t.Errorf("the Secret that was not the object's changed: %v, owners %v", s.Data, s.OwnerReferences)
		}
		if err := env.Kube.Delete(env.Ctx, get(obj)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
	})

	t.Run("a credential's client secret is in its Secret and not in status", func(t *testing.T) {
		spec := credSpec("cred-secret")
		spec["secretOutput"] = map[string]any{"name": "cred-out"}
		obj := newObject(cred.GVK.Kind, "cred", spec)
		if err := env.Kube.Create(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(cred.GVK, outputNS, obj.GetName())
		name, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		want, _ := env.Disco.Stored(cred, name)["clientSecret"].(string)
		if want == "" {
			t.Fatalf("the fake holds no clientSecret for %s", name)
		}
		values = append(values, want)
		got := kindtest.CheckSecretOutputs(t, env, cred, u)
		if got["clientSecret"] != want {
			t.Errorf("the Secret cred-out holds another clientSecret than the cloud's")
		}
		if err := env.Kube.Get(env.Ctx, client.ObjectKey{Namespace: outputNS, Name: "cred"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
			t.Errorf("a Secret named after the object exists beside spec.secretOutput.name's (err %v)", err)
		}

		t.Run("a lost Secret is written again from a read", func(t *testing.T) {
			deleteSecret("cred-out")
			resync(obj)
			deadline := time.Now().Add(mockenv.Timeout())
			for {
				s := &corev1.Secret{}
				err := env.Kube.Get(env.Ctx, client.ObjectKey{Namespace: outputNS, Name: "cred-out"}, s)
				if err == nil && string(s.Data["clientSecret"]) == want {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("the Secret cred-out was not written again (err %v)", err)
				}
				time.Sleep(200 * time.Millisecond)
			}
			kindtest.CheckSecretOutputs(t, env, cred, env.WaitReady(cred.GVK, outputNS, obj.GetName()))
		})

		t.Run("delete disables the credential first", func(t *testing.T) {
			if err := env.Kube.Delete(env.Ctx, get(obj)); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(obj)
			if env.Disco.Stored(cred, name) != nil {
				t.Errorf("%s is still in the cloud", name)
			}
			patched := false
			for _, r := range env.Requests() {
				if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, name+"?") && strings.Contains(r, "updateMask=disabled") {
					patched = true
				}
				if strings.HasPrefix(r, "DELETE ") && strings.HasSuffix(r, name) && !patched {
					t.Errorf("the delete went out before the credential was disabled")
				}
			}
		})
	})

	t.Run("a SCIM token is in its Secret, and only the create returns it", func(t *testing.T) {
		obj := newObject(tok.GVK.Kind, "tok", map[string]any{
			"location":      "global",
			"workforcePool": "secret-test-pool",
			"provider":      "secret-test-provider",
			"scimTenant":    "secret-test-tenant",
			"displayName":   "cce secret test",
		})
		if err := env.Kube.Create(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(tok.GVK, outputNS, obj.GetName())
		name, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		want, _ := env.Disco.Stored(tok, name)["securityToken"].(string)
		if want == "" {
			t.Fatalf("the fake holds no securityToken for %s", name)
		}
		values = append(values, want)
		if got := kindtest.CheckSecretOutputs(t, env, tok, u); got["securityToken"] != want {
			t.Errorf("the Secret tok holds another securityToken than the cloud's")
		}
		api := &discokind.API{HTTP: env.HTTP, BaseURL: tok.BaseURL}
		read, err := api.Get(env.Ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := read["securityToken"]; ok {
			t.Errorf("a read of %s returns securityToken; the API returns it only at create", name)
		}

		t.Run("a lost Secret cannot be written again", func(t *testing.T) {
			creates := count(http.MethodPost, "/tokens?")
			deleteSecret("tok")
			resync(obj)
			msg := env.WaitNotReady(tok.GVK, outputNS, obj.GetName())
			if !strings.Contains(msg, "returns it only when it makes the resource") || !strings.Contains(msg, "make it again") {
				t.Errorf("Ready's message is %q, want it to say the token is gone and the object must be made again", msg)
			}
			if n := count(http.MethodPost, "/tokens?") - creates; n != 0 {
				t.Errorf("%d more tokens were made", n)
			}
			if err := env.Kube.Get(env.Ctx, client.ObjectKey{Namespace: outputNS, Name: "tok"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
				t.Errorf("the Secret tok came back without its token (err %v)", err)
			}
		})

		t.Run("delete removes the token", func(t *testing.T) {
			if err := env.Kube.Delete(env.Ctx, get(obj)); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(obj)
			if st := env.Disco.Stored(tok, name)["state"]; st != "DELETED" {
				t.Errorf("%s is %v in the cloud, want DELETED", name, st)
			}
		})
	})

	t.Run("no secret is in a log line or an event", func(t *testing.T) {
		if len(values) == 0 {
			t.Skip("no secret was made")
		}
		out := logs.String()
		if !strings.Contains(out, "wrote the API's secrets to a Secret") {
			t.Errorf("the manager's logs were not captured (%d bytes)", len(out))
		}
		events := &corev1.EventList{}
		if err := env.Kube.List(env.Ctx, events, client.InNamespace(outputNS)); err != nil {
			t.Fatal(err)
		}
		for _, v := range values {
			if strings.Contains(out, v) {
				t.Errorf("a secret is in the manager's logs")
			}
			for _, ev := range events.Items {
				if strings.Contains(ev.Message, v) {
					t.Errorf("a secret is in the event %s on %s", ev.Reason, ev.InvolvedObject.Name)
				}
			}
		}
	})
}
