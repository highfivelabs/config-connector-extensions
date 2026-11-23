// Copyright 2026 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/accesscontextmanager/accesscontextmanagergcpuseraccessbinding_controller.go
// (UPSTREAM.md), the direct controller of its alpha kind: the same calls
// (create under the organization with the name left out, the ID the
// server's, written back to spec.resourceID; get; patch with an update
// mask; delete; each a long-running operation) and the same refusal of a
// changed groupKey. Changed: our KRM types; google.golang.org/api's REST
// client instead of cloud.google.com/go/accesscontextmanager, whose pinned
// protos (v1.14.0) have none of the fields ours adds (dryRunAccessLevels,
// sessionSettings, restrictedClientApplications, scopedAccessSettings);
// the spec is mapped by its JSON, which has the API's names; the mask
// names the fields that differ, compared with unset, "", false and [] the
// same; a binding with no ID yet is looked for by its group key before a
// create (the API takes one binding per group, so a create retried after
// its ID was lost would otherwise fail); a delete of a binding that was
// made uses status.externalRef and resolves nothing.

package accesscontextmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	api "google.golang.org/api/accesscontextmanager/v1"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/accesscontextmanager/v1beta1"
)

func init() {
	registry.RegisterModel(krm.AccessContextManagerGCPUserAccessBindingGVK, NewGCPUserAccessBindingModel)
}

// NewGCPUserAccessBindingModel is the model of the kind.
func NewGCPUserAccessBindingModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &gcpUserAccessBindingModel{config: *config}, nil
}

var _ directbase.Model = &gcpUserAccessBindingModel{}

type gcpUserAccessBindingModel struct {
	config config.ControllerConfig
}

func (m *gcpUserAccessBindingModel) client(ctx context.Context) (*api.Service, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	svc, err := api.NewService(ctx, append([]option.ClientOption{}, opts...)...)
	if err != nil {
		return nil, fmt.Errorf("building accesscontextmanager client: %w", err)
	}
	return svc, nil
}

func (m *gcpUserAccessBindingModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.AccessContextManagerGCPUserAccessBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	id := &krm.AccessContextManagerGCPUserAccessBindingIdentity{}
	if ext := direct.ValueOf(obj.Status.ExternalRef); op.IsDeleting() && ext != "" {
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
	} else {
		resolved, err := obj.GetIdentity(ctx, op.Reader)
		if err != nil {
			return nil, err
		}
		id = resolved.(*krm.AccessContextManagerGCPUserAccessBindingIdentity)
	}
	svc, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &gcpUserAccessBindingAdapter{svc: svc, id: id, desired: obj}, nil
}

func (m *gcpUserAccessBindingModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type gcpUserAccessBindingAdapter struct {
	svc     *api.Service
	id      *krm.AccessContextManagerGCPUserAccessBindingIdentity
	desired *krm.AccessContextManagerGCPUserAccessBinding
	actual  *api.GcpUserAccessBinding
}

var _ directbase.Adapter = &gcpUserAccessBindingAdapter{}

func (a *gcpUserAccessBindingAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	if a.id.GcpUserAccessBinding == "" {
		// No ID yet: a binding of the group may have been made by an
		// earlier create whose ID was not written back.
		if a.desired == nil {
			return false, nil
		}
		found, err := a.findByGroupKey(ctx)
		if err != nil || found == nil {
			return false, err
		}
		if err := a.id.FromExternal(found.Name); err != nil {
			return false, err
		}
		// Its name goes to status.externalRef, from which the identity
		// takes the ID from then on.
		log.V(2).Info("found AccessContextManagerGCPUserAccessBinding by its group key", "name", found.Name)
		a.actual = found
		return true, nil
	}
	log.V(2).Info("finding AccessContextManagerGCPUserAccessBinding", "name", a.id.String())
	actual, err := a.svc.Organizations.GcpUserAccessBindings.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting AccessContextManagerGCPUserAccessBinding %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *gcpUserAccessBindingAdapter) findByGroupKey(ctx context.Context) (*api.GcpUserAccessBinding, error) {
	var found *api.GcpUserAccessBinding
	err := a.svc.Organizations.GcpUserAccessBindings.List(a.id.ParentString()).Context(ctx).Pages(ctx, func(page *api.ListGcpUserAccessBindingsResponse) error {
		for _, b := range page.GcpUserAccessBindings {
			if b.GroupKey == a.desired.Spec.GroupKey && found == nil {
				found = b
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing the AccessContextManagerGCPUserAccessBindings of %s: %w", a.id.ParentString(), err)
	}
	return found, nil
}

func (a *gcpUserAccessBindingAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating AccessContextManagerGCPUserAccessBinding", "parent", a.id.ParentString())

	desired, err := gcpUserAccessBindingSpecToAPI(&a.desired.Spec)
	if err != nil {
		return err
	}
	// As upstream: the name is never set in a create.
	desired.Name = ""
	op, err := a.svc.Organizations.GcpUserAccessBindings.Create(a.id.ParentString(), desired).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating AccessContextManagerGCPUserAccessBinding: %w", err)
	}
	created := &api.GcpUserAccessBinding{}
	if err := a.wait(ctx, op, created); err != nil {
		return fmt.Errorf("waiting for AccessContextManagerGCPUserAccessBinding creation: %w", err)
	}
	log.V(2).Info("successfully created AccessContextManagerGCPUserAccessBinding", "name", created.Name)

	createdID := &krm.AccessContextManagerGCPUserAccessBindingIdentity{}
	if err := createdID.FromExternal(created.Name); err != nil {
		return fmt.Errorf("parsing created resource name %q: %w", created.Name, err)
	}
	if direct.ValueOf(a.desired.Spec.ResourceID) == "" {
		if err := createOp.SetSpecResourceID(ctx, createdID.GcpUserAccessBinding); err != nil {
			return err
		}
	}
	a.id.GcpUserAccessBinding = createdID.GcpUserAccessBinding
	return a.updateStatus(ctx, createOp, created)
}

func (a *gcpUserAccessBindingAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	if a.actual.GroupKey != "" && a.desired.Spec.GroupKey != a.actual.GroupKey {
		return fmt.Errorf("groupKey is immutable and cannot be updated")
	}
	mask := gcpUserAccessBindingUpdateMask(&a.desired.Spec, a.actual)
	if len(mask) == 0 {
		log.V(2).Info("no diff detected for AccessContextManagerGCPUserAccessBinding", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	log.V(2).Info("updating AccessContextManagerGCPUserAccessBinding", "name", a.id.String(), "updateMask", mask)
	desired, err := gcpUserAccessBindingSpecToAPI(&a.desired.Spec)
	if err != nil {
		return err
	}
	desired.Name = a.id.String()
	op, err := a.svc.Organizations.GcpUserAccessBindings.Patch(a.id.String(), desired).UpdateMask(strings.Join(mask, ",")).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating AccessContextManagerGCPUserAccessBinding %s: %w", a.id.String(), err)
	}
	latest := &api.GcpUserAccessBinding{}
	if err := a.wait(ctx, op, latest); err != nil {
		return fmt.Errorf("waiting for AccessContextManagerGCPUserAccessBinding %s update: %w", a.id.String(), err)
	}
	if latest.Name == "" {
		// An operation without the binding in its response: read it.
		if latest, err = a.svc.Organizations.GcpUserAccessBindings.Get(a.id.String()).Context(ctx).Do(); err != nil {
			return fmt.Errorf("getting AccessContextManagerGCPUserAccessBinding %s: %w", a.id.String(), err)
		}
	}
	log.V(2).Info("successfully updated AccessContextManagerGCPUserAccessBinding", "name", a.id.String())
	return a.updateStatus(ctx, updateOp, latest)
}

func (a *gcpUserAccessBindingAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *gcpUserAccessBindingAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	if a.id.GcpUserAccessBinding == "" {
		return true, nil
	}
	log.V(2).Info("deleting AccessContextManagerGCPUserAccessBinding", "name", a.id.String())
	op, err := a.svc.Organizations.GcpUserAccessBindings.Delete(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting AccessContextManagerGCPUserAccessBinding %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, nil); err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("waiting for AccessContextManagerGCPUserAccessBinding %s deletion: %w", a.id.String(), err)
	}
	return true, nil
}

// wait polls the operation until it is done and decodes its response into
// into (when into is not nil).
func (a *gcpUserAccessBindingAdapter) wait(ctx context.Context, op *api.Operation, into any) error {
	for !op.Done {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		var err error
		op, err = a.svc.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return err
		}
	}
	if op.Error != nil {
		return fmt.Errorf("operation %s failed: %d: %s", op.Name, op.Error.Code, op.Error.Message)
	}
	if into != nil && len(op.Response) > 0 {
		if err := json.Unmarshal(op.Response, into); err != nil {
			return fmt.Errorf("reading the response of operation %s: %w", op.Name, err)
		}
	}
	return nil
}

func (a *gcpUserAccessBindingAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *api.GcpUserAccessBinding) error {
	name := latest.Name
	if name == "" {
		name = a.id.String()
	}
	status := &krm.AccessContextManagerGCPUserAccessBindingStatus{
		Name:        direct.LazyPtr(name),
		ExternalRef: direct.LazyPtr(name),
	}
	return op.UpdateStatus(ctx, status, nil)
}

// gcpUserAccessBindingFields is the spec without the organization and the
// ID, in the API's JSON names.
func gcpUserAccessBindingFields(spec *krm.AccessContextManagerGCPUserAccessBindingSpec) *krm.AccessContextManagerGCPUserAccessBindingSpec {
	s := spec.DeepCopy()
	s.OrganizationRef = nil
	s.ResourceID = nil
	return s
}

// gcpUserAccessBindingSpecToAPI is the spec as the API's binding: the JSON
// names are the API's.
func gcpUserAccessBindingSpecToAPI(spec *krm.AccessContextManagerGCPUserAccessBindingSpec) (*api.GcpUserAccessBinding, error) {
	b, err := json.Marshal(gcpUserAccessBindingFields(spec))
	if err != nil {
		return nil, err
	}
	out := &api.GcpUserAccessBinding{}
	if err := json.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("mapping the spec to the API: %w", err)
	}
	return out, nil
}

// gcpUserAccessBindingPaths are the fields an update mask can name, by
// their JSON names, in the API's mask spelling.
var gcpUserAccessBindingPaths = []struct{ field, path string }{
	{"accessLevels", "access_levels"},
	{"dryRunAccessLevels", "dry_run_access_levels"},
	{"restrictedClientApplications", "restricted_client_applications"},
	{"scopedAccessSettings", "scoped_access_settings"},
	{"sessionSettings", "session_settings"},
}

// gcpUserAccessBindingUpdateMask names the fields of the spec that differ
// from the binding as read; unset, "", false, 0 and [] are the same.
func gcpUserAccessBindingUpdateMask(spec *krm.AccessContextManagerGCPUserAccessBindingSpec, actual *api.GcpUserAccessBinding) []string {
	want := gcpUserAccessBindingComparable(gcpUserAccessBindingFields(spec))
	have := gcpUserAccessBindingComparable(actual)
	var mask []string
	for _, p := range gcpUserAccessBindingPaths {
		if !reflect.DeepEqual(want[p.field], have[p.field]) {
			mask = append(mask, p.path)
		}
	}
	return mask
}

func gcpUserAccessBindingComparable(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	out, _ := pruneEmpty(m).(map[string]any)
	return out
}

// pruneEmpty drops "", false, 0, null, and lists and objects that are (or
// become) empty.
func pruneEmpty(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range t {
			if p := pruneEmpty(e); p != nil {
				out[k] = p
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		var out []any
		for _, e := range t {
			p := pruneEmpty(e)
			if p == nil {
				p = map[string]any{}
			}
			out = append(out, p)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case string:
		if t == "" {
			return nil
		}
	case bool:
		if !t {
			return nil
		}
	case float64:
		if t == 0 {
			return nil
		}
	case nil:
		return nil
	}
	return v
}
