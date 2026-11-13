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

// IAMBinding and IAMMember deleted together with the object they name, in
// both orders. The case: a namespace or a release deleted all at once,
// where the parent (an APIGatewayAPI, a secret) is deleted first and sits
// with a deletion timestamp, not Ready, while Config Connector deletes it.
// The binding's delete must not wait on it: the policy goes with the
// resource, so it succeeds without a call. A parent kept with
// deletion-policy abandon keeps its resource, so a binding on it still
// removes itself. The parent is Config Connector's SecretManagerSecret,
// not reconciled here: a finalizer of the test's holds it while it is
// being deleted, as Config Connector's would.

import (
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	secretmanager "google.golang.org/api/secretmanager/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// holdFinalizer stands in for Config Connector's finalizer on the parent.
const holdFinalizer = "test.gcp.hfl.dev/hold"

func TestIAMBindingAndMemberWithTheirParent(t *testing.T) {
	env := mockenv.New(t, namespace)
	sm, err := secretmanager.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	policies := &iampolicy.Client{HTTP: env.HTTP}
	secretType, _ := iampolicy.Lookup("SecretManagerSecret")
	member := "serviceAccount:" + mockenv.ServiceAccount(t)

	secretID := mockenv.Name("iam-parent")
	secret := "projects/" + mockenv.Project + "/secrets/" + secretID
	if _, err := sm.Projects.Secrets.Create("projects/"+mockenv.Project, &secretmanager.Secret{
		Replication: &secretmanager.Replication{Automatic: &secretmanager.Automatic{}}, Labels: mockenv.Labels(),
	}).SecretId(secretID).Context(env.Ctx).Do(); err != nil {
		t.Fatalf("creating the secret: %v", err)
	}
	t.Cleanup(func() {
		if _, err := sm.Projects.Secrets.Delete(secret).Context(env.Ctx).Do(); err != nil && !iampolicy.IsNotFound(err) {
			t.Errorf("deleting the secret: %v", err)
		}
	})

	getObj := func(t *testing.T, gvk schema.GroupVersionKind, name string) *unstructured.Unstructured {
		t.Helper()
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	// parent makes a Config Connector object of the secret, Ready, held by
	// the test's finalizer when hold is set.
	parent := func(t *testing.T, name string, hold bool, annotations map[string]string) *unstructured.Unstructured {
		t.Helper()
		o := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
			"resourceID":  secretID,
			"replication": map[string]any{"auto": map[string]any{}},
		}}}
		o.SetGroupVersionKind(secretManagerSecretGVK)
		o.SetNamespace(namespace)
		o.SetName(name)
		ann := map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project}
		for k, v := range annotations {
			ann[k] = v
		}
		o.SetAnnotations(ann)
		if hold {
			o.SetFinalizers([]string{holdFinalizer})
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		setReady(t, env, getObj(t, secretManagerSecretGVK, name), "True", "UpToDate")
		return o
	}
	// deleteParent deletes the parent; when it is held, it then stays
	// with a deletion timestamp and Ready False, as Config Connector's
	// objects do while their resource is being deleted.
	deleteParent := func(t *testing.T, name string) {
		t.Helper()
		if err := env.Kube.Delete(env.Ctx, getObj(t, secretManagerSecretGVK, name)); err != nil {
			t.Fatal(err)
		}
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(secretManagerSecretGVK)
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err == nil {
			setReady(t, env, u, "False", "Deleting")
		}
	}
	// release takes the test's finalizer off the parent and waits for it
	// to go.
	release := func(t *testing.T, name string) {
		t.Helper()
		u := getObj(t, secretManagerSecretGVK, name)
		u.SetFinalizers(slices.DeleteFunc(u.GetFinalizers(), func(f string) bool { return f == holdFinalizer }))
		if err := env.Kube.Update(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(u)
	}
	holds := func(t *testing.T, role string) bool {
		t.Helper()
		p, err := policies.Get(env.Ctx, secretType, secret)
		if err != nil {
			t.Fatalf("reading the secret's policy: %v", err)
		}
		m, _ := iampolicy.Members(p, role, nil)
		return slices.Contains(m, member)
	}
	// iamCalls is how many requests reached the secret's policy so far.
	iamCalls := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.Contains(r, secretID+":getIamPolicy") || strings.Contains(r, secretID+":setIamPolicy") {
				n++
			}
		}
		return n
	}

	type kind struct {
		name string
		gvk  schema.GroupVersionKind
		// make is the object giving member role on the parent named parent.
		make func(name, parent, role string) *unstructured.Unstructured
	}
	ref := func(parent string) map[string]any {
		return map[string]any{"kind": "SecretManagerSecret", "apiVersion": "secretmanager.cnrm.cloud.google.com/v1beta1", "name": parent}
	}
	kinds := []kind{
		{"IAMBinding", bindingGVK, func(name, parent, role string) *unstructured.Unstructured {
			return object(bindingGVK, name, map[string]any{"resourceRef": ref(parent), "role": role, "members": []any{member}})
		}},
		{"IAMMember", memberGVK, func(name, parent, role string) *unstructured.Unstructured {
			return object(memberGVK, name, map[string]any{"resourceRef": ref(parent), "role": role, "member": member})
		}},
	}
	roles := map[string]string{"IAMBinding": accessor, "IAMMember": viewer}

	for _, k := range kinds {
		base := strings.ToLower(k.name)
		role := roles[k.name]
		// bound makes the parent and the binding, and waits for the role.
		bound := func(t *testing.T, name string, hold bool, annotations map[string]string) *unstructured.Unstructured {
			t.Helper()
			parent(t, name+"-parent", hold, annotations)
			b := k.make(name, name+"-parent", role)
			if err := env.Kube.Create(env.Ctx, b); err != nil {
				t.Fatal(err)
			}
			env.WaitReady(k.gvk, namespace, name)
			if !holds(t, role) {
				t.Fatalf("the secret's policy does not give %s %s", member, role)
			}
			return b
		}

		t.Run(k.name+": the parent being deleted first, the binding goes without a call", func(t *testing.T) {
			name := base + "-parent-first"
			b := bound(t, name, true, nil)
			deleteParent(t, name+"-parent")
			before := iamCalls()
			if err := env.Kube.Delete(env.Ctx, b); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(b)
			if n := iamCalls() - before; n != 0 {
				t.Errorf("deleting the binding made %d calls to the secret's policy, want none", n)
			}
			release(t, name+"-parent")
			// The real secret goes with its object; here it stays, so take
			// the role off for the cases below.
			if _, err := policies.Modify(env.Ctx, secretType, secret, func(p *iampb.Policy) bool {
				return iampolicy.Remove(p, role, nil)
			}); err != nil {
				t.Fatal(err)
			}
		})

		t.Run(k.name+": the parent already gone, the binding goes without a call", func(t *testing.T) {
			name := base + "-parent-gone"
			b := bound(t, name, false, nil)
			deleteParent(t, name+"-parent")
			env.WaitGone(object(secretManagerSecretGVK, name+"-parent", nil))
			before := iamCalls()
			if err := env.Kube.Delete(env.Ctx, b); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(b)
			if n := iamCalls() - before; n != 0 {
				t.Errorf("deleting the binding made %d calls to the secret's policy, want none", n)
			}
			if _, err := policies.Modify(env.Ctx, secretType, secret, func(p *iampb.Policy) bool {
				return iampolicy.Remove(p, role, nil)
			}); err != nil {
				t.Fatal(err)
			}
		})

		t.Run(k.name+": the binding deleted first, then the parent", func(t *testing.T) {
			name := base + "-binding-first"
			b := bound(t, name, true, nil)
			if err := env.Kube.Delete(env.Ctx, b); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(b)
			if holds(t, role) {
				t.Errorf("the secret's policy still gives %s %s after the binding was deleted", member, role)
			}
			deleteParent(t, name+"-parent")
			release(t, name+"-parent")
		})

		t.Run(k.name+": both deleted at once, both go", func(t *testing.T) {
			name := base + "-together"
			b := bound(t, name, true, nil)
			if err := env.Kube.Delete(env.Ctx, b); err != nil {
				t.Fatal(err)
			}
			deleteParent(t, name+"-parent")
			env.WaitGone(b)
			release(t, name+"-parent")
			// Whether the binding's delete ran before the parent's started
			// decides whether it removed the role; either is right.
			if _, err := policies.Modify(env.Ctx, secretType, secret, func(p *iampb.Policy) bool {
				return iampolicy.Remove(p, role, nil)
			}); err != nil {
				t.Fatal(err)
			}
		})

		t.Run(k.name+": a parent abandoned, the binding still removes itself", func(t *testing.T) {
			name := base + "-abandoned"
			b := bound(t, name, true, map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon})
			deleteParent(t, name+"-parent")
			if err := env.Kube.Delete(env.Ctx, b); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(b)
			if holds(t, role) {
				t.Errorf("the abandoned secret's policy still gives %s %s after the binding was deleted", member, role)
			}
			release(t, name+"-parent")
		})
	}
}

// object is an unstructured object of gvk in the test namespace.
func object(gvk schema.GroupVersionKind, name string, spec map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{}}
	if spec != nil {
		u.Object["spec"] = spec
	}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

// setReady writes the object's Ready condition, as Config Connector would.
func setReady(t *testing.T, env *mockenv.Env, u *unstructured.Unstructured, status, reason string) {
	t.Helper()
	if err := unstructured.SetNestedSlice(u.Object, []any{map[string]any{
		"type": "Ready", "status": status, "reason": reason, "lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
	}}, "status", "conditions"); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Status().Update(env.Ctx, u); err != nil {
		t.Fatal(err)
	}
}
