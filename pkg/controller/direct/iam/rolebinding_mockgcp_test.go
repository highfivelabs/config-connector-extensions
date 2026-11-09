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

// IAMRoleBinding end to end on the fake cloud (pkg/test/mockenv; patch
// 0018 serves Cloud Storage's policy paths) or a real project
// (docs/live-runs.md). The case: a module that says who has a role on a
// secret and on a bucket, and expects anyone else to be taken off it, with
// the other roles on the resource left alone. The secret and the bucket
// are the run's own (mockenv.Name) and are deleted at the end.

import (
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	secretmanager "google.golang.org/api/secretmanager/v1"
	storageapi "google.golang.org/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	iamv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var bindingGVK = iamv1alpha1.IAMRoleBindingGVK

const (
	accessor = "roles/secretmanager.secretAccessor"
	viewer   = "roles/secretmanager.viewer"
)

func TestIAMRoleBinding(t *testing.T) {
	env := mockenv.New(t, namespace)
	sm, err := secretmanager.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	gcs, err := storageapi.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	policies := &iampolicy.Client{HTTP: env.HTTP}
	secretType, _ := iampolicy.Lookup("SecretManagerSecret")
	bucketType, _ := iampolicy.Lookup("StorageBucket")

	// The members: the run's own service account is a real principal live;
	// the project viewers and editors are convenience members every
	// resource's policy accepts.
	saMember := "serviceAccount:" + mockenv.ServiceAccount(t)
	viewers := "projectViewer:" + mockenv.Project
	editors := "projectEditor:" + mockenv.Project
	owners := "projectOwner:" + mockenv.Project

	// The secret, in the cloud and as a Config Connector object (not
	// reconciled here; read to resolve resourceRef by name).
	secretID := mockenv.Name("role-binding")
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
	createSecretObject := func(name string) {
		o := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
			"resourceID":  secretID,
			"replication": map[string]any{"auto": map[string]any{}},
		}}}
		o.SetGroupVersionKind(secretManagerSecretGVK)
		o.SetNamespace(namespace)
		o.SetName(name)
		o.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	createSecretObject("db-password")

	// policyOf is the secret's policy; members of a role (no condition).
	policyOf := func(t *testing.T) *iampb.Policy {
		t.Helper()
		p, err := policies.Get(env.Ctx, secretType, secret)
		if err != nil {
			t.Fatalf("reading the secret's policy: %v", err)
		}
		return p
	}
	members := func(t *testing.T, role string) []string {
		t.Helper()
		m, _ := iampolicy.Members(policyOf(t), role, nil)
		return m
	}
	// outOfBand changes the policy as another writer would.
	outOfBand := func(t *testing.T, change func(*iampb.Policy) bool) {
		t.Helper()
		if _, err := policies.Modify(env.Ctx, secretType, secret, change); err != nil {
			t.Fatalf("changing the secret's policy out of band: %v", err)
		}
	}
	waitFor := func(t *testing.T, what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		o := &iamv1alpha1.IAMRoleBinding{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	newBinding := func(name, role string, members ...string) *iamv1alpha1.IAMRoleBinding {
		return &iamv1alpha1.IAMRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: iamv1alpha1.IAMRoleBindingSpec{
				ResourceRef: iamv1alpha1.IAMResourceReference{Kind: "SecretManagerSecret", APIVersion: "secretmanager.cnrm.cloud.google.com/v1beta1", Name: "db-password"},
				Role:        role,
				Members:     members,
			},
		}
	}
	update := func(t *testing.T, name string, change func(*iamv1alpha1.IAMRoleBinding)) {
		t.Helper()
		o := &iamv1alpha1.IAMRoleBinding{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	sorted := func(s ...string) []string { slices.Sort(s); return s }

	// Another role, bound by hand, that no binding below owns.
	outOfBand(t, func(p *iampb.Policy) bool {
		return iampolicy.SetMembers(p, viewer, nil, []string{owners})
	})

	const accessors = "accessors"
	if err := env.Kube.Create(env.Ctx, newBinding(accessors, accessor, saMember, viewers)); err != nil {
		t.Fatal(err)
	}

	t.Run("create gives the role to exactly these members", func(t *testing.T) {
		u := env.WaitReady(bindingGVK, namespace, accessors)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != secret+"#"+accessor {
			t.Errorf("status.externalRef = %q, want %q", ref, secret+"#"+accessor)
		}
		if got := members(t, accessor); !slices.Equal(got, sorted(saMember, viewers)) {
			t.Errorf("members of %s = %v, want %v", accessor, got, sorted(saMember, viewers))
		}
	})

	t.Run("another role on the secret is left alone", func(t *testing.T) {
		if got := members(t, viewer); !slices.Equal(got, []string{owners}) {
			t.Errorf("members of %s = %v, want [%s]", viewer, got, owners)
		}
	})

	t.Run("a member added out of band is removed", func(t *testing.T) {
		outOfBand(t, func(p *iampb.Policy) bool {
			return iampolicy.AddMembers(p, accessor, nil, []string{editors})
		})
		if !slices.Contains(members(t, accessor), editors) {
			t.Fatal("the out of band member was not added")
		}
		resync(t, accessors)
		waitFor(t, "the stray member to be removed", func() bool { return !slices.Contains(members(t, accessor), editors) })
		if got := members(t, accessor); !slices.Equal(got, sorted(saMember, viewers)) {
			t.Errorf("members of %s = %v, want %v", accessor, got, sorted(saMember, viewers))
		}
	})

	t.Run("a resync of a matching policy writes nothing", func(t *testing.T) {
		writes := func() int {
			n := 0
			for _, r := range env.Requests() {
				if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/secrets/"+secretID+":setIamPolicy") {
					n++
				}
			}
			return n
		}
		before := writes()
		resync(t, accessors)
		time.Sleep(2 * time.Second)
		env.WaitReady(bindingGVK, namespace, accessors)
		time.Sleep(time.Second)
		if got := writes(); got != before {
			t.Errorf("setIamPolicy calls after a resync with nothing to change: %d", got-before)
		}
	})

	t.Run("changing the members changes the role", func(t *testing.T) {
		update(t, accessors, func(o *iamv1alpha1.IAMRoleBinding) { o.Spec.Members = []string{saMember} })
		waitFor(t, "the member to be taken off", func() bool { return slices.Equal(members(t, accessor), []string{saMember}) })
		env.WaitReady(bindingGVK, namespace, accessors)
	})

	t.Run("the same role under a condition is a binding of its own", func(t *testing.T) {
		const timed = "timed"
		b := newBinding(timed, accessor, viewers)
		b.Spec.Condition = &iamv1alpha1.IAMCondition{
			Title:      "until 2099",
			Expression: `request.time < timestamp("2099-01-01T00:00:00Z")`,
		}
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(bindingGVK, namespace, timed)
		cond := &iampolicy.Condition{Title: b.Spec.Condition.Title, Expression: b.Spec.Condition.Expression}
		if got, _ := iampolicy.Members(policyOf(t), accessor, cond); !slices.Equal(got, []string{viewers}) {
			t.Errorf("conditional members = %v, want [%s]", got, viewers)
		}
		if got := members(t, accessor); !slices.Equal(got, []string{saMember}) {
			t.Errorf("unconditional members = %v, want [%s]", got, saMember)
		}
		// Deleting it removes its binding and no other.
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(b)
		if _, found := iampolicy.Members(policyOf(t), accessor, cond); found {
			t.Error("the conditional binding is still in the policy after its object was deleted")
		}
		if got := members(t, accessor); !slices.Equal(got, []string{saMember}) {
			t.Errorf("unconditional members after the delete = %v, want [%s]", got, saMember)
		}
	})

	t.Run("a binding waits for the object it names", func(t *testing.T) {
		b := newBinding("late", viewer, owners)
		b.Spec.ResourceRef.Name = "not-yet"
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(bindingGVK, namespace, "late")
		if !strings.Contains(msg, "not-yet") {
			t.Errorf("the message does not name the missing object: %q", msg)
		}
		createSecretObject("not-yet")
		// Config Connector would report the secret Ready; the reconciler
		// waits for that, then retries the binding at once.
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(secretManagerSecretGVK)
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "not-yet"}, o); err != nil {
			t.Fatal(err)
		}
		if err := unstructured.SetNestedSlice(o.Object, []any{map[string]any{
			"type": "Ready", "status": "True", "reason": "UpToDate", "lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
		}}, "status", "conditions"); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Status().Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(bindingGVK, namespace, "late")
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(b)
	})

	t.Run("a kind it does not know is refused", func(t *testing.T) {
		b := newBinding("unknown", viewer, owners)
		b.Spec.ResourceRef.Kind = "Spaceship"
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(bindingGVK, namespace, "unknown")
		if !strings.Contains(msg, "Spaceship") || !strings.Contains(msg, "SecretManagerSecret") {
			t.Errorf("the message should name the kind and list the supported ones: %q", msg)
		}
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(b)
	})

	t.Run("a bucket by its name, through Cloud Storage's own API", func(t *testing.T) {
		bucket := mockenv.Name("role-binding-bucket")
		if _, err := gcs.Buckets.Insert(mockenv.Project, &storageapi.Bucket{Name: bucket, Location: "US", Labels: mockenv.Labels()}).Context(env.Ctx).Do(); err != nil {
			t.Fatalf("creating the bucket: %v", err)
		}
		t.Cleanup(func() {
			if err := gcs.Buckets.Delete(bucket).Context(env.Ctx).Do(); err != nil && !iampolicy.IsNotFound(err) {
				t.Errorf("deleting the bucket: %v", err)
			}
		})
		const role = "roles/storage.objectViewer"
		b := newBinding("bucket-readers", role, saMember)
		b.Spec.ResourceRef = iamv1alpha1.IAMResourceReference{Kind: "StorageBucket", External: bucket}
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(bindingGVK, namespace, "bucket-readers")
		want := "projects/_/buckets/" + bucket + "#" + role
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		read := func() []string {
			p, err := policies.Get(env.Ctx, bucketType, "projects/_/buckets/"+bucket)
			if err != nil {
				t.Fatalf("reading the bucket's policy: %v", err)
			}
			m, _ := iampolicy.Members(p, role, nil)
			return m
		}
		if got := read(); !slices.Equal(got, []string{saMember}) {
			t.Errorf("bucket members = %v, want [%s]", got, saMember)
		}
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(b)
		if got := read(); len(got) != 0 {
			t.Errorf("bucket members after the delete = %v, want none", got)
		}
	})

	t.Run("delete removes the role's binding and no other", func(t *testing.T) {
		b := &iamv1alpha1.IAMRoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: accessors}}
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(b)
		if got := members(t, accessor); len(got) != 0 {
			t.Errorf("members of %s after the delete = %v, want none", accessor, got)
		}
		if got := members(t, viewer); !slices.Equal(got, []string{owners}) {
			t.Errorf("members of %s after the delete = %v, want [%s]", viewer, got, owners)
		}
	})

	t.Run("abandon leaves the binding in the cloud", func(t *testing.T) {
		const kept = "kept"
		b := newBinding(kept, accessor, viewers)
		b.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(bindingGVK, namespace, kept)
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(b)
		if got := members(t, accessor); !slices.Equal(got, []string{viewers}) {
			t.Errorf("members of %s after an abandoning delete = %v, want [%s]", accessor, got, viewers)
		}
	})
}
