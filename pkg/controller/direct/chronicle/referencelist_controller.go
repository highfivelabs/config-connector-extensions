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

package chronicle

// ChronicleReferenceList (chronicle.gcp.hfl.dev/v1beta1): a Google SecOps
// reference list in an instance, as Terraform's
// google_chronicle_reference_list. Config Connector's v1alpha1 kind has no
// instance field and so always fails with "cannot resolve instance"; this
// one carries it (decision 26). The API has no delete for a reference list,
// so deleting the object leaves the list, as Terraform does.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/chronicle/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
)

// NewReferenceListModel is the direct reconciler's model for
// ChronicleReferenceList.
func NewReferenceListModel(ctx context.Context, cfg *config.ControllerConfig) (directbase.Model, error) {
	return &referenceListModel{config: *cfg}, nil
}

type referenceListModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &referenceListModel{}

func (m *referenceListModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.ChronicleReferenceList{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ext := ""
	if obj.Status.ExternalRef != nil {
		ext = *obj.Status.ExternalRef
	}
	id := &kccchroniclev1alpha1.ChronicleReferenceListIdentity{}
	if !obj.GetDeletionTimestamp().IsZero() && ext != "" {
		// Deleting: the name is in status, so a project object already gone
		// does not hold the delete (which leaves the list anyway).
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
	} else {
		project, err := refsv1beta1.ResolveProject(ctx, op.Reader, obj.GetNamespace(), obj.Spec.ProjectRef)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, fmt.Errorf("spec.projectRef is required")
		}
		listID := obj.GetName()
		if obj.Spec.ResourceID != nil && *obj.Spec.ResourceID != "" {
			listID = *obj.Spec.ResourceID
		}
		*id = kccchroniclev1alpha1.ChronicleReferenceListIdentity{Project: project.ProjectID, Location: obj.Spec.Location, Instance: obj.Spec.Instance, ReferenceList: listID}
		if ext != "" && ext != id.String() {
			return nil, fmt.Errorf("cannot change ChronicleReferenceList identity (old=%q, new=%q)", ext, id.String())
		}
	}
	httpClient, err := m.config.NewAuthenticatedHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	return &referenceListAdapter{
		api:     &discokind.API{HTTP: httpClient, BaseURL: BaseURL(id.Location)},
		id:      id,
		desired: &obj.Spec,
	}, nil
}

func (m *referenceListModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type referenceListAdapter struct {
	api     *discokind.API
	id      *kccchroniclev1alpha1.ChronicleReferenceListIdentity
	desired *krm.ChronicleReferenceListSpec
	actual  map[string]any
}

var _ directbase.Adapter = &referenceListAdapter{}

// fullView reads the entries too, which the default view leaves out.
var fullView = map[string]string{"view": "REFERENCE_LIST_VIEW_FULL"}

func (a *referenceListAdapter) Find(ctx context.Context) (bool, error) {
	got, err := a.api.Do(ctx, http.MethodGet, a.id.String(), fullView, nil)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting %s: %w", a.id, err)
	}
	a.actual = got
	return true, nil
}

// body is the request body for the fields the spec sets.
func (a *referenceListAdapter) body() map[string]any {
	b := map[string]any{"description": deref(a.desired.Description), "syntaxType": deref(a.desired.SyntaxType)}
	b["entries"] = toJSON(a.desired.Entries)
	if a.desired.ScopeInfo != nil {
		b["scopeInfo"] = toJSON(a.desired.ScopeInfo)
	}
	return b
}

func (a *referenceListAdapter) Create(ctx context.Context, op *directbase.CreateOperation) error {
	parent := "projects/" + a.id.Project + "/locations/" + a.id.Location + "/instances/" + a.id.Instance
	klog.FromContext(ctx).V(2).Info("creating a reference list", "name", a.id.String())
	if _, err := a.api.Do(ctx, http.MethodPost, parent+"/referenceLists", map[string]string{"referenceListId": a.id.ReferenceList}, a.body()); err != nil {
		return fmt.Errorf("creating %s: %w", a.id, err)
	}
	got, err := a.api.Do(ctx, http.MethodGet, a.id.String(), fullView, nil)
	if err != nil {
		return fmt.Errorf("reading %s after create: %w", a.id, err)
	}
	a.actual = got
	return op.UpdateStatus(ctx, a.status(), nil)
}

// changes are the update mask: the fields the spec sets that differ. An
// entry list the API returns without entries is empty, and the spec's
// order counts (the API keeps it).
func (a *referenceListAdapter) changes() []string {
	var mask []string
	want := a.body()
	for _, f := range []string{"description", "entries", "syntaxType", "scopeInfo"} {
		w, ok := want[f]
		if !ok {
			continue
		}
		if !sameJSON(w, a.actual[f]) {
			mask = append(mask, f)
		}
	}
	return mask
}

func (a *referenceListAdapter) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	if mask := a.changes(); len(mask) > 0 {
		klog.FromContext(ctx).V(2).Info("patching", "name", a.id.String(), "mask", mask)
		body := map[string]any{"name": a.id.String()}
		all := a.body()
		for _, f := range mask {
			body[f] = all[f]
		}
		if _, err := a.api.Do(ctx, http.MethodPatch, a.id.String(), map[string]string{"updateMask": strings.Join(mask, ",")}, body); err != nil {
			return fmt.Errorf("patching %s (%s): %w", a.id, strings.Join(mask, ","), err)
		}
		got, err := a.api.Do(ctx, http.MethodGet, a.id.String(), fullView, nil)
		if err != nil {
			return fmt.Errorf("reading %s after update: %w", a.id, err)
		}
		a.actual = got
	}
	return op.UpdateStatus(ctx, a.status(), nil)
}

func (a *referenceListAdapter) status() *krm.ChronicleReferenceListStatus {
	name := a.id.String()
	s := &krm.ChronicleReferenceListStatus{ExternalRef: &name}
	if b, err := json.Marshal(a.actual); err == nil {
		observed := &kccchroniclev1alpha1.ChronicleReferenceListObservedState{}
		if json.Unmarshal(b, observed) == nil {
			s.ObservedState = observed
		}
	}
	return s
}

func (a *referenceListAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("ChronicleReferenceList does not support export")
}

// Delete leaves the list: the API has no delete for a reference list
// (Terraform's google_chronicle_reference_list leaves it too).
func (a *referenceListAdapter) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).Info("leaving the reference list: the API has no delete", "name", a.id.String())
	return true, nil
}

// toJSON is v as the API's JSON (maps, lists, strings).
func toJSON(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

// sameJSON compares a wanted value with the API's, an empty list or object
// equal to one the API leaves out.
func sameJSON(want, got any) bool {
	empty := func(v any) bool {
		switch x := v.(type) {
		case nil:
			return true
		case []any:
			return len(x) == 0
		case map[string]any:
			return len(x) == 0
		}
		return false
	}
	if empty(want) && empty(got) {
		return true
	}
	return reflect.DeepEqual(want, got)
}
