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

// Package fieldkind holds what the field kinds share: a kind of ours named
// `<Config Connector kind>Fields` that owns a few fields of a resource
// Config Connector's kind owns (docs/design.md section 6.6, pattern 2, and
// decision 37). Such a kind never creates or deletes the resource; it sets
// its own fields, and only those, on a resource that exists.
package fieldkind

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NotFoundError is what a field kind reports while the resource it names
// does not exist yet: the object waits (the reconciler tries again) and
// never makes the resource itself.
func NotFoundError(kind, resource string) error {
	return fmt.Errorf("%s %s does not exist yet; a %sFields object sets fields on an existing resource and never creates it", kind, resource, kind)
}

// CheckSoleOwner refuses a second object of the same field kind for the
// same resource in a namespace: two objects owning the same fields would
// write them in turn. The oldest object (by creation time, then name) owns
// the resource; any other is refused until the older one goes. external is
// the resource this object resolved to; another object counts from its
// status.externalRef, which it writes on its first successful reconcile, or
// from spec.resourceRef.external.
func CheckSoleOwner(ctx context.Context, reader client.Reader, gvk schema.GroupVersionKind, obj client.Object, external string) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := reader.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		return fmt.Errorf("listing %s objects: %w", gvk.Kind, err)
	}
	var owners []unstructured.Unstructured
	for _, u := range list.Items {
		if u.GetName() == obj.GetName() {
			continue
		}
		ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		if ref == "" {
			ref, _, _ = unstructured.NestedString(u.Object, "spec", "resourceRef", "external")
		}
		if sameResource(ref, external) && older(&u, obj) {
			owners = append(owners, u)
		}
	}
	if len(owners) == 0 {
		return nil
	}
	names := make([]string, 0, len(owners))
	for _, u := range owners {
		names = append(names, u.GetName())
	}
	slices.Sort(names)
	return fmt.Errorf("%s %s already sets the fields of %s; one %s object per resource", gvk.Kind, strings.Join(names, ", "), external, gvk.Kind)
}

func sameResource(a, b string) bool {
	trim := func(s string) string {
		s = strings.TrimPrefix(s, "//")
		if i := strings.Index(s, "projects/"); i > 0 {
			s = s[i:]
		}
		return strings.TrimPrefix(s, "/")
	}
	return a != "" && trim(a) == trim(b)
}

func older(a *unstructured.Unstructured, b client.Object) bool {
	ta, tb := a.GetCreationTimestamp(), b.GetCreationTimestamp()
	if !ta.Equal(&tb) {
		return ta.Before(&tb)
	}
	return a.GetName() < b.GetName()
}

// Gone is the adapter for an object being deleted that never wrote its
// resource (no status.externalRef): there is nothing to give back, and
// the Config Connector object its resourceRef names may be gone already,
// so the reference is not resolved. Find reports not found, so the
// delete finishes without a call.
type Gone struct{}

var _ directbase.Adapter = Gone{}

func (Gone) Find(ctx context.Context) (bool, error) { return false, nil }
func (Gone) Create(ctx context.Context, op *directbase.CreateOperation) error {
	return fmt.Errorf("a field object being deleted is never created")
}
func (Gone) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	return fmt.Errorf("a field object being deleted is never updated")
}
func (Gone) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	return true, nil
}
func (Gone) Export(ctx context.Context) (*unstructured.Unstructured, error) { return nil, nil }

// Deleting reports whether obj is being deleted without ever having
// written its resource: its adapter is then Gone.
func Deleting(obj client.Object, externalRef string) bool {
	return obj.GetDeletionTimestamp() != nil && externalRef == ""
}
