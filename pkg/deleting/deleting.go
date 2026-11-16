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

// Package deleting keeps a delete or an abandon from resolving references.
//
// When everything is deleted at once (a namespace, a Helm release, one of
// the catalog's charts), the objects a spec names (a Folder, a Project, a
// network) may be gone before the object that names them. directbase builds
// the adapter, and so resolves every reference, before it looks at the
// deletion: a reference that no longer resolves would hold the finalizer
// forever ("referenced Folder catalog-folder/mock not found").
//
// Wrap gives every kind two rules:
//
//   - Abandon (cnrm.cloud.google.com/deletion-policy: abandon) makes no
//     call, so it reads nothing: the adapter is Gone.
//   - An object being deleted whose status holds nothing but conditions
//     (no status.externalRef, no selfLink, no name) was never made or
//     acquired, so when its adapter cannot be built (its references do
//     not resolve) there is nothing of ours to delete: the adapter is
//     Gone. A kind whose status cannot show that it made something
//     (Options.NoRecord) keeps the error instead: a resource it made
//     would otherwise be left behind without a word.
//
// The third rule is each kind's own: an object being deleted that wrote
// status.externalRef takes its identity from there and resolves nothing
// (the parts of a name cannot change), as AssetFeed (#134), ServiceIdentity
// (#141) and the discovery kinds (pkg/discokind) do.
package deleting

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// Gone is the adapter for an object whose delete makes no call: Find
// reports not found, so directbase removes the finalizer without a call.
type Gone struct{}

var _ directbase.Adapter = Gone{}

func (Gone) Find(ctx context.Context) (bool, error) { return false, nil }
func (Gone) Create(ctx context.Context, op *directbase.CreateOperation) error {
	return fmt.Errorf("an object being deleted is never created")
}
func (Gone) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	return fmt.Errorf("an object being deleted is never updated")
}
func (Gone) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	return true, nil
}
func (Gone) Export(ctx context.Context) (*unstructured.Unstructured, error) { return nil, nil }

// Abandoning reports whether u is being deleted with the abandon policy.
func Abandoning(u *unstructured.Unstructured) bool {
	return !u.GetDeletionTimestamp().IsZero() && k8s.HasAbandonAnnotation(u)
}

// Options are a kind's exceptions to the rules.
type Options struct {
	// NoRecord is set for a kind whose status records nothing when it
	// makes its resource (no status.externalRef or other field): its
	// delete never takes the never-made rule.
	NoRecord bool
}

// Wrap is m with the abandon and never-made rules. It refuses a model it
// would hide an interface of, as the slot guard does.
func Wrap(m directbase.Model, opts Options) (directbase.Model, error) {
	if _, ok := m.(directbase.SensitiveFieldModel); ok {
		return nil, fmt.Errorf("deleting: the model watches Secrets (SensitiveFieldModel), which the wrapper does not pass on")
	}
	if _, ok := m.(directbase.IAMModel); ok {
		return nil, fmt.Errorf("deleting: the model is an IAMModel, which the wrapper does not pass on")
	}
	return &model{inner: m, opts: opts}, nil
}

type model struct {
	inner directbase.Model
	opts  Options
}

// made reports whether u's status records anything besides conditions:
// a name, a selfLink, observed state, written only once the resource was
// made or acquired.
func made(u *unstructured.Unstructured) bool {
	status, _, _ := unstructured.NestedMap(u.Object, "status")
	for k, v := range status {
		switch k {
		case "conditions", "observedGeneration":
			continue
		}
		if v != nil && v != "" {
			return true
		}
	}
	return false
}

func (m *model) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	if Abandoning(u) {
		return Gone{}, nil
	}
	a, err := m.inner.AdapterForObject(ctx, op)
	if err != nil && !u.GetDeletionTimestamp().IsZero() && !m.opts.NoRecord && !made(u) {
		klog.FromContext(ctx).Info("deleting without a call: never made (status holds only conditions) and the adapter cannot be built", "kind", u.GetKind(), "name", u.GetName(), "error", err.Error())
		return Gone{}, nil
	}
	return a, err
}

func (m *model) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return m.inner.AdapterForURL(ctx, url)
}
