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

// A secret the API returns (Kind.SecretOutputs) goes to a Kubernetes
// Secret, never to status (docs/design.md decision 38): in the object's
// namespace, owned by the object (so it goes when the object goes), named
// by spec.secretOutput.name or else the object's name, holding each value
// under the field's name. The values never reach a log, an event, an error
// or status: only the Secret's name and the fields' names do.

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// secretOutput is the Secret one object's SecretOutputs go to.
type secretOutput struct {
	kind  *Kind
	kube  client.Client
	key   types.NamespacedName
	owner metav1.OwnerReference
}

// SecretOutputName is the name of the Secret the object u writes its
// SecretOutputs to: spec.secretOutput.name, else the object's name.
func SecretOutputName(u *unstructured.Unstructured) string {
	if n, _, _ := unstructured.NestedString(u.Object, "spec", SecretOutputSpec, "name"); n != "" {
		return n
	}
	return u.GetName()
}

// newSecretOutput is the object u's Secret. reader is the client directbase
// hands the adapter, its manager's (directbase_controller.go:
// AdapterForObjectOperation{Reader: r.Reconciler.Client}), which writes too.
func newSecretOutput(k *Kind, reader client.Reader, u *unstructured.Unstructured) (*secretOutput, error) {
	kube, ok := reader.(client.Client)
	if !ok {
		return nil, fmt.Errorf("%s writes the API's secrets to a Secret, and its controller was given a client that cannot write (%T)", k.GVK.Kind, reader)
	}
	controller := true
	return &secretOutput{
		kind: k,
		kube: kube,
		key:  types.NamespacedName{Namespace: u.GetNamespace(), Name: SecretOutputName(u)},
		owner: metav1.OwnerReference{
			APIVersion: u.GetAPIVersion(),
			Kind:       u.GetKind(),
			Name:       u.GetName(),
			UID:        u.GetUID(),
			Controller: &controller,
		},
	}, nil
}

// secretValues are the SecretOutputs a read (got) and, when there was
// one, a create's answer (created) hold: the create's first, since only
// it has the CreateOnlyOutputs.
func secretValues(k *Kind, got, created map[string]any) map[string]string {
	out := map[string]string{}
	for _, f := range k.SecretOutputs {
		for _, src := range []map[string]any{created, got} {
			if v, ok := src[f].(string); ok && v != "" {
				out[f] = v
				break
			}
		}
	}
	return out
}

func (s *secretOutput) ours(sec *corev1.Secret) bool {
	for _, o := range sec.OwnerReferences {
		if o.UID == s.owner.UID {
			return true
		}
	}
	return false
}

func (s *secretOutput) notOurs() error {
	return fmt.Errorf("the Secret %s exists and is not this object's (no owner reference to it); it is left as it is: delete it, or name another with spec.%s.name", s.key, SecretOutputSpec)
}

// reserve makes the object's Secret, empty, unless it is there already,
// and fails if a Secret of that name is someone else's.
func (s *secretOutput) reserve(ctx context.Context) error {
	sec := &corev1.Secret{}
	err := s.kube.Get(ctx, s.key, sec)
	switch {
	case err == nil:
		if !s.ours(sec) {
			return s.notOurs()
		}
		return nil
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("reading the Secret %s: %w", s.key, err)
	}
	if err := s.kube.Create(ctx, s.newSecret(nil)); err != nil {
		return fmt.Errorf("making the Secret %s for %s: %w", s.key, strings.Join(s.kind.SecretOutputs, ", "), err)
	}
	return nil
}

func (s *secretOutput) newSecret(data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       s.key.Namespace,
			Name:            s.key.Name,
			OwnerReferences: []metav1.OwnerReference{s.owner},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}

// missing is why the Secret lacks field and the read did not give it.
func (s *secretOutput) missing(field string) error {
	if slices.Contains(s.kind.CreateOnlyOutputs, field) {
		return fmt.Errorf("the Secret %s has no %s, and the API returns it only when it makes the resource: it cannot be written again. Delete this object (which deletes the %s in the cloud) and make it again", s.key, field, s.kind.GVK.Kind)
	}
	return fmt.Errorf("the API did not return %s, and the Secret %s does not hold it", field, s.key)
}

// write makes the Secret hold values, each under its field's name. A field
// values lacks must be in the Secret already.
func (s *secretOutput) write(ctx context.Context, values map[string]string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sec := &corev1.Secret{}
		err := s.kube.Get(ctx, s.key, sec)
		if apierrors.IsNotFound(err) {
			data := map[string][]byte{}
			for _, f := range s.kind.SecretOutputs {
				v, ok := values[f]
				if !ok {
					return s.missing(f)
				}
				data[f] = []byte(v)
			}
			if err := s.kube.Create(ctx, s.newSecret(data)); err != nil {
				return fmt.Errorf("writing the Secret %s: %w", s.key, err)
			}
			klog.FromContext(ctx).Info("wrote the API's secrets to a Secret", "kind", s.kind.GVK.Kind, "secret", s.key.String(), "fields", s.kind.SecretOutputs)
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading the Secret %s: %w", s.key, err)
		}
		if !s.ours(sec) {
			return s.notOurs()
		}
		changed := false
		for _, f := range s.kind.SecretOutputs {
			v, ok := values[f]
			if !ok {
				if len(sec.Data[f]) == 0 {
					return s.missing(f)
				}
				continue
			}
			if string(sec.Data[f]) != v {
				if sec.Data == nil {
					sec.Data = map[string][]byte{}
				}
				sec.Data[f] = []byte(v)
				changed = true
			}
		}
		if !changed {
			return nil
		}
		if err := s.kube.Update(ctx, sec); err != nil {
			return fmt.Errorf("writing the Secret %s: %w", s.key, err)
		}
		klog.FromContext(ctx).Info("wrote the API's secrets to a Secret", "kind", s.kind.GVK.Kind, "secret", s.key.String(), "fields", s.kind.SecretOutputs)
		return nil
	})
}
