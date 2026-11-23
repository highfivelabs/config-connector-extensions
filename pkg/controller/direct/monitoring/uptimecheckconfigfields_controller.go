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

// Package monitoring holds config-connector-extensions' Cloud Monitoring
// controllers.
package monitoring

// Written here: MonitoringUptimeCheckConfigFields, a field kind
// (docs/design.md section 6.6, pattern 2; decision 37). Config Connector's
// MonitoringUptimeCheckConfig is DCL-based (no direct controller, on
// master either) and has no userLabels, and no HTTP check
// customContentType, acceptedResponseStatusCodes, pingConfig or
// serviceAgentAuthentication. The check is read with
// uptimeCheckConfigs.get and written with uptimeCheckConfigs.patch, whose
// update mask names only the fields this object owns that differ, the HTTP
// check's by their nested paths (httpCheck.pingConfig). DCL's update sends
// a mask of the paths of the fields it changed (dcl.UpdateMask: nested,
// httpCheck.path), never these, so its updates leave them alone; to be
// confirmed live, as for the other field kinds.

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	monitoringv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/monitoring/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	api "google.golang.org/api/monitoring/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/monitoring/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewUptimeCheckConfigFieldsModel is the MonitoringUptimeCheckConfigFields
// model.
func NewUptimeCheckConfigFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &uptimeFieldsModel{config: *config}, nil
}

type uptimeFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &uptimeFieldsModel{}

func (m *uptimeFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.MonitoringUptimeCheckConfigFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if fieldkind.Deleting(obj, prev) {
		return fieldkind.Gone{}, nil
	}
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the check is in status; Config Connector's object may be
		// gone already.
		ref = &monitoringv1beta1.MonitoringUptimeCheckConfigRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &monitoringv1beta1.MonitoringUptimeCheckConfigIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the uptime check of MonitoringUptimeCheckConfigFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.MonitoringUptimeCheckConfigFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building monitoring client: %w", err)
	}
	return &uptimeFieldsAdapter{id: id, gcpClient: c, desired: &obj.Spec}, nil
}

func (m *uptimeFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type uptimeFieldsAdapter struct {
	id        *monitoringv1beta1.MonitoringUptimeCheckConfigIdentity
	gcpClient *api.Service
	desired   *krm.MonitoringUptimeCheckConfigFieldsSpec
	actual    *api.UptimeCheckConfig
}

var _ directbase.Adapter = &uptimeFieldsAdapter{}

// Find reports whether the check exists. A missing one is "not found" so
// that a delete finishes when it is gone; a create then waits (Create).
func (a *uptimeFieldsAdapter) Find(ctx context.Context) (bool, error) {
	got, err := a.gcpClient.Projects.UptimeCheckConfigs.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting uptime check %s: %w", a.id, err)
	}
	a.actual = got
	return true, nil
}

// Create is called only while the check does not exist: this kind never
// makes it, so it waits for it.
func (a *uptimeFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("MonitoringUptimeCheckConfig", a.id.String())
}

// field is one owned field: its update mask path, and how to read it from
// and write it to the API's check.
type field struct {
	path string
	// spec is the spec's value, or nil when the spec leaves it out.
	spec func(*krm.MonitoringUptimeCheckConfigFieldsSpec) any
	get  func(*api.UptimeCheckConfig) any
	set  func(*api.UptimeCheckConfig, any) error
}

func httpCheck(c *api.UptimeCheckConfig) *api.HttpCheck {
	if c.HttpCheck == nil {
		c.HttpCheck = &api.HttpCheck{}
	}
	return c.HttpCheck
}

// into maps v (a spec value, nil to clear) to the API's type at out.
func into(v, out any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("mapping the spec to the API: %w", err)
	}
	return nil
}

func specHTTP(s *krm.MonitoringUptimeCheckConfigFieldsSpec) *krm.UptimeCheckConfig_HTTPCheckFields {
	if s.HTTPCheck == nil {
		return &krm.UptimeCheckConfig_HTTPCheckFields{}
	}
	return s.HTTPCheck
}

func readHTTP(c *api.UptimeCheckConfig) *api.HttpCheck {
	if c.HttpCheck == nil {
		return &api.HttpCheck{}
	}
	return c.HttpCheck
}

var uptimeFields = []field{
	{
		path: "userLabels",
		spec: func(s *krm.MonitoringUptimeCheckConfigFieldsSpec) any {
			if s.UserLabels == nil {
				return nil
			}
			return s.UserLabels
		},
		get: func(c *api.UptimeCheckConfig) any { return c.UserLabels },
		set: func(c *api.UptimeCheckConfig, v any) error {
			c.UserLabels = map[string]string{}
			if v == nil {
				// Cleared: an empty map, sent (the mask names it).
				c.ForceSendFields = append(c.ForceSendFields, "UserLabels")
				return nil
			}
			return into(v, &c.UserLabels)
		},
	},
	{
		path: "httpCheck.customContentType",
		spec: func(s *krm.MonitoringUptimeCheckConfigFieldsSpec) any {
			if v := specHTTP(s).CustomContentType; v != nil {
				return *v
			}
			return nil
		},
		get: func(c *api.UptimeCheckConfig) any { return readHTTP(c).CustomContentType },
		set: func(c *api.UptimeCheckConfig, v any) error {
			s, _ := v.(string)
			httpCheck(c).CustomContentType = s
			return nil
		},
	},
	{
		path: "httpCheck.acceptedResponseStatusCodes",
		spec: func(s *krm.MonitoringUptimeCheckConfigFieldsSpec) any {
			if v := specHTTP(s).AcceptedResponseStatusCodes; v != nil {
				return v
			}
			return nil
		},
		get: func(c *api.UptimeCheckConfig) any { return readHTTP(c).AcceptedResponseStatusCodes },
		set: func(c *api.UptimeCheckConfig, v any) error {
			httpCheck(c).AcceptedResponseStatusCodes = nil
			if v == nil {
				return nil
			}
			return into(v, &httpCheck(c).AcceptedResponseStatusCodes)
		},
	},
	{
		path: "httpCheck.pingConfig",
		spec: func(s *krm.MonitoringUptimeCheckConfigFieldsSpec) any {
			if v := specHTTP(s).PingConfig; v != nil {
				return v
			}
			return nil
		},
		get: func(c *api.UptimeCheckConfig) any { return readHTTP(c).PingConfig },
		set: func(c *api.UptimeCheckConfig, v any) error {
			httpCheck(c).PingConfig = nil
			if v == nil {
				return nil
			}
			return into(v, &httpCheck(c).PingConfig)
		},
	},
	{
		path: "httpCheck.serviceAgentAuthentication",
		spec: func(s *krm.MonitoringUptimeCheckConfigFieldsSpec) any {
			if v := specHTTP(s).ServiceAgentAuthentication; v != nil {
				return v
			}
			return nil
		},
		get: func(c *api.UptimeCheckConfig) any { return readHTTP(c).ServiceAgentAuthentication },
		set: func(c *api.UptimeCheckConfig, v any) error {
			httpCheck(c).ServiceAgentAuthentication = nil
			if v == nil {
				return nil
			}
			return into(v, &httpCheck(c).ServiceAgentAuthentication)
		},
	},
}

// comparable is v as a JSON value with empty values pruned, so that unset,
// "", 0, {} and [] compare the same.
func comparable(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return pruneEmpty(out), nil
}

// changes is the patch body and update mask that make the check's owned
// fields what the spec says, or, clearing (Delete), unset.
func (a *uptimeFieldsAdapter) changes(clear bool) (*api.UptimeCheckConfig, []string, error) {
	body := &api.UptimeCheckConfig{}
	var mask []string
	for _, f := range uptimeFields {
		want := f.spec(a.desired)
		if want == nil {
			continue // not owned
		}
		if clear {
			want = nil
		}
		w, err := comparable(want)
		if err != nil {
			return nil, nil, err
		}
		h, err := comparable(f.get(a.actual))
		if err != nil {
			return nil, nil, err
		}
		if reflect.DeepEqual(w, h) {
			continue
		}
		if strings.HasPrefix(f.path, "httpCheck.") && a.actual.HttpCheck == nil {
			if clear {
				continue
			}
			return nil, nil, fmt.Errorf("uptime check %s is not an HTTP check: spec.httpCheck sets fields of the one Config Connector's MonitoringUptimeCheckConfig makes (its spec.httpCheck)", a.id)
		}
		if err := f.set(body, want); err != nil {
			return nil, nil, err
		}
		mask = append(mask, f.path)
	}
	return body, mask, nil
}

func (a *uptimeFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	body, mask, err := a.changes(false)
	if err != nil {
		return err
	}
	if err := a.patch(ctx, "setting", body, mask); err != nil {
		return err
	}
	return a.updateStatus(ctx, updateOp)
}

// patch sends the fields mask names, and nothing else.
func (a *uptimeFieldsAdapter) patch(ctx context.Context, verb string, body *api.UptimeCheckConfig, mask []string) error {
	if len(mask) == 0 {
		return nil
	}
	klog.FromContext(ctx).V(2).Info(verb+" fields of an uptime check", "name", a.id, "updateMask", mask)
	got, err := a.gcpClient.Projects.UptimeCheckConfigs.Patch(a.id.String(), body).UpdateMask(strings.Join(mask, ",")).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("%s %s of uptime check %s: %w", verb, strings.Join(mask, ", "), a.id, err)
	}
	a.actual = got
	return nil
}

// Delete clears the fields this object owns and never deletes the check.
// A check that is gone is not reached: Find said not found.
func (a *uptimeFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	body, mask, err := a.changes(true)
	if err != nil {
		return false, err
	}
	if err := a.patch(ctx, "clearing", body, mask); err != nil {
		return false, err
	}
	return true, nil
}

func (a *uptimeFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *uptimeFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.MonitoringUptimeCheckConfigFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	return op.UpdateStatus(ctx, status, nil)
}

// pruneEmpty drops "", false, 0, null, and lists and objects that are (or
// become) empty; an empty object in a list stays, as a placeholder.
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
