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

package iampolicy

import (
	"context"
	"fmt"
	"strings"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ref is a resourceRef: the resource whose IAM policy is bound, by the
// name of a Kubernetes object that manages it, or externally.
type Ref struct {
	APIVersion, Kind, Name, Namespace, External string
}

// ErrGone is returned by Resolve when the object a reference names does
// not exist: a binding being deleted has nothing left to remove.
type ErrGone struct{ Err error }

func (e *ErrGone) Error() string { return e.Err.Error() }
func (e *ErrGone) Unwrap() error { return e.Err }

// Resolve finds the resource type and the full name of the resource a
// reference names. An object that is missing or not Ready yet is a
// reference error, which the reconciler waits on and retries (wrapped in
// ErrGone when it is missing).
func Resolve(ctx context.Context, reader client.Reader, defaultNamespace string, ref Ref) (*ResourceType, string, error) {
	t, ok := Lookup(ref.Kind)
	if !ok {
		return nil, "", fmt.Errorf("resourceRef.kind %q is not supported (use one of %s)", ref.Kind, strings.Join(Kinds(), ", "))
	}
	if (ref.Name == "") == (ref.External == "") {
		return nil, "", fmt.Errorf("resourceRef needs exactly one of name and external")
	}
	if ref.External != "" {
		name, err := t.ResolveExternal(ref.External)
		return t, name, err
	}
	candidates := t.GVKs
	if ref.APIVersion != "" {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, "", fmt.Errorf("resourceRef.apiVersion %q: %w", ref.APIVersion, err)
		}
		candidates = []schema.GroupVersionKind{gv.WithKind(ref.Kind)}
	}
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("a %s cannot be named by name; use resourceRef.external (%s)", t.Kind, t.Name)
	}
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if key.Namespace == "" {
		key.Namespace = defaultNamespace
	}
	var obj *unstructured.Unstructured
	for _, gvk := range candidates {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		err := reader.Get(ctx, key, u)
		if err == nil {
			obj = u
			break
		}
		if !apierrors.IsNotFound(err) && !isNoMatch(err) {
			return nil, "", fmt.Errorf("reading referenced %s %s: %w", gvk.Kind, key, err)
		}
	}
	if obj == nil {
		return nil, "", &ErrGone{k8s.NewReferenceNotFoundError(candidates[0], key)}
	}
	if !ready(obj) {
		return nil, "", k8s.NewReferenceNotReadyError(obj.GroupVersionKind(), key)
	}
	parts := ObjectParts{Location: nestedString(obj, "spec", "location")}
	parts.ID = nestedString(obj, "spec", "resourceID")
	if parts.ID == "" {
		parts.ID = obj.GetName()
	}
	parts.ExternalRef = nestedString(obj, "status", "externalRef")
	parts.Status, _, _ = unstructured.NestedMap(obj.Object, "status")
	var projErr error
	parts.Project, projErr = refs.ResolveProjectID(ctx, reader, obj)
	name, err := t.ResolveObject(parts)
	if err != nil {
		if projErr != nil {
			return nil, "", fmt.Errorf("%w (%v)", err, projErr)
		}
		return nil, "", err
	}
	return t, name, nil
}

func nestedString(u *unstructured.Unstructured, path ...string) string {
	s, _, _ := unstructured.NestedString(u.Object, path...)
	return s
}

// ready is false when the object reports a Ready condition that is not True.
// An object that reports none (a kind that does not, or one not reconciled
// by anything in a test) is taken as it is.
func ready(u *unstructured.Unstructured) bool {
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conds {
		m, _ := c.(map[string]any)
		if m["type"] == "Ready" {
			return m["status"] == "True"
		}
	}
	return true
}

func isNoMatch(err error) bool { return meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err) }
