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

package discokind

import (
	"context"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// laggingCache is a client whose reads of Secrets come from a cache that
// has not seen the client's own writes: each Secret read answers what was
// there before the last write, for the next blind reads after it (the
// manager's client, whose reads come from an informer).
type laggingCache struct {
	mu    sync.Mutex
	blind int
	// before is the Secret as it was before the last write (nil: none).
	before *corev1.Secret
	left   int
}

func (l *laggingCache) client(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	remember := func(ctx context.Context, c client.WithWatch, obj client.Object) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.before, l.left = nil, l.blind
		old := &corev1.Secret{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), old); err == nil {
			l.before = old
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			sec, ok := obj.(*corev1.Secret)
			if !ok {
				return c.Get(ctx, key, obj, opts...)
			}
			l.mu.Lock()
			lagging := l.left > 0
			before := l.before
			if lagging {
				l.left--
			}
			l.mu.Unlock()
			if !lagging {
				return c.Get(ctx, key, obj, opts...)
			}
			if before == nil {
				return apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, key.Name)
			}
			before.DeepCopyInto(sec)
			return nil
		},
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				remember(ctx, c, obj)
			}
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				remember(ctx, c, obj)
			}
			return c.Update(ctx, obj, opts...)
		},
	}).Build()
}

func scimTokenKind() *Kind {
	return &Kind{
		GVK:               schema.GroupVersionKind{Group: "iam.gcp.hfl.dev", Version: "v1alpha1", Kind: "IAMWorkforcePoolProviderScimToken"},
		SecretOutputs:     []string{"securityToken"},
		CreateOnlyOutputs: []string{"securityToken"},
	}
}

func scimTokenObject() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("iam.gcp.hfl.dev/v1alpha1")
	u.SetKind("IAMWorkforcePoolProviderScimToken")
	u.SetNamespace("ns")
	u.SetName("token")
	u.SetUID(types.UID("uid-1"))
	return u
}

func secretData(t *testing.T, kube client.Client) map[string][]byte {
	t.Helper()
	sec := &corev1.Secret{}
	if err := kube.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "token"}, sec); err != nil {
		t.Fatal(err)
	}
	return sec.Data
}

// The create path: reserve makes the Secret, the API answers the token,
// and write must put it in the Secret though the cache has not seen the
// Secret reserve made. Writing from a cache read alone would try to make
// the Secret again, fail as already there, and lose a token the API never
// returns again (the flake of TestIAMWorkforcePoolProviderScimToken's
// abandon case, 2026-11-23).
func TestSecretOutputWriteWithLaggingCache(t *testing.T) {
	for _, blind := range []int{0, 1, 3, 6} {
		l := &laggingCache{blind: blind}
		kube := l.client(t)
		s, err := newSecretOutput(scimTokenKind(), kube, scimTokenObject())
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		if err := s.reserve(ctx); err != nil {
			t.Fatalf("blind %d: reserve: %v", blind, err)
		}
		if err := s.write(ctx, map[string]string{"securityToken": "tok"}, false); err != nil {
			t.Fatalf("blind %d: write: %v", blind, err)
		}
		l.mu.Lock()
		l.left = 0
		l.mu.Unlock()
		if got := string(secretData(t, kube)["securityToken"]); got != "tok" {
			t.Errorf("blind %d: the Secret holds %q, want the token", blind, got)
		}
	}
}

// A later reconcile whose reserve finds, through a cache that has not
// caught up, no Secret where one is: it reads again rather than fail, and
// a Secret someone else owns still stops it.
func TestSecretOutputReserveWithLaggingCache(t *testing.T) {
	owner := scimTokenObject()
	ours := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "token", OwnerReferences: []metav1.OwnerReference{{UID: owner.GetUID()}}}}
	theirs := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "token"}}
	for name, tc := range map[string]struct {
		sec     *corev1.Secret
		wantErr string
	}{
		"ours":   {sec: ours},
		"theirs": {sec: theirs, wantErr: "is not this object's"},
	} {
		l := &laggingCache{}
		kube := l.client(t, tc.sec)
		l.mu.Lock()
		l.left = 2 // the cache has not seen the Secret yet
		l.mu.Unlock()
		s, err := newSecretOutput(scimTokenKind(), kube, owner)
		if err != nil {
			t.Fatal(err)
		}
		err = s.reserve(context.Background())
		if tc.wantErr == "" && err != nil {
			t.Errorf("%s: reserve: %v", name, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("%s: reserve: %v, want %q", name, err, tc.wantErr)
		}
	}
}

// When the read after a create fails, Create keeps what only the create's
// answer holds (write with some): the next reconcile's update path finds
// it in the Secret.
func TestSecretOutputWriteSome(t *testing.T) {
	k := scimTokenKind()
	k.SecretOutputs = append(k.SecretOutputs, "other")
	l := &laggingCache{}
	kube := l.client(t)
	s, err := newSecretOutput(k, kube, scimTokenObject())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.reserve(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.write(ctx, secretValues(k, nil, map[string]any{"securityToken": "tok"}), true); err != nil {
		t.Fatalf("write some: %v", err)
	}
	if got := string(secretData(t, kube)["securityToken"]); got != "tok" {
		t.Errorf("the Secret holds %q, want the token", got)
	}
	// The next reconcile: a read gives other, the Secret has the token.
	s2, _ := newSecretOutput(k, kube, scimTokenObject())
	if err := s2.write(ctx, secretValues(k, map[string]any{"other": "o"}, nil), false); err != nil {
		t.Fatalf("write after: %v", err)
	}
	d := secretData(t, kube)
	if string(d["securityToken"]) != "tok" || string(d["other"]) != "o" {
		t.Errorf("the Secret holds %q and %q", d["securityToken"], d["other"])
	}
	// Without the token, the next reconcile says to make the object again.
	k2 := scimTokenKind()
	s3, _ := newSecretOutput(k2, l.client(t), scimTokenObject())
	err = s3.write(ctx, map[string]string{}, false)
	if err == nil || !strings.Contains(err.Error(), "Delete this object") {
		t.Errorf("write without a create-only token: %v, want the delete-and-make-again error", err)
	}
}
