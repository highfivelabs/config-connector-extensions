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

// ChronicleRule (chronicle.gcp.hfl.dev/v1beta1): a Google SecOps rule in an
// instance, as Terraform's google_chronicle_rule. Config Connector's
// v1alpha1 kind has no instance field and so always fails with "cannot
// resolve instance"; this one carries it (decision 26). The rule's ID is
// the API's: create records it in status.externalRef, and spec.resourceID
// acquires a rule that exists. It speaks REST JSON to the location's host,
// as ChronicleRuleDeployment does.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// NewRuleModel is the direct reconciler's model for ChronicleRule.
func NewRuleModel(ctx context.Context, cfg *config.ControllerConfig) (directbase.Model, error) {
	return &ruleModel{config: *cfg}, nil
}

type ruleModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &ruleModel{}

func (m *ruleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.ChronicleRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ext := ""
	if obj.Status.ExternalRef != nil {
		ext = *obj.Status.ExternalRef
	}
	id := &kccchroniclev1alpha1.ChronicleRuleIdentity{}
	if !obj.GetDeletionTimestamp().IsZero() && ext != "" {
		// Deleting a rule that was made or acquired: its name is in status, so
		// a project object already gone does not hold the delete.
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
		*id = kccchroniclev1alpha1.ChronicleRuleIdentity{Project: project.ProjectID, Location: obj.Spec.Location, Instance: obj.Spec.Instance}
		if obj.Spec.ResourceID != nil {
			id.Rule = *obj.Spec.ResourceID
		}
		if ext != "" {
			old := &kccchroniclev1alpha1.ChronicleRuleIdentity{}
			if err := old.FromExternal(ext); err != nil {
				return nil, err
			}
			if id.Rule == "" {
				id.Rule = old.Rule
			}
			if old.String() != id.String() {
				return nil, fmt.Errorf("cannot change ChronicleRule identity (old=%q, new=%q)", ext, id.String())
			}
		}
	}
	httpClient, err := m.config.NewAuthenticatedHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	return &ruleAdapter{
		api:     &discokind.API{HTTP: httpClient, BaseURL: BaseURL(id.Location)},
		id:      id,
		desired: &obj.Spec,
	}, nil
}

func (m *ruleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type ruleAdapter struct {
	api     *discokind.API
	id      *kccchroniclev1alpha1.ChronicleRuleIdentity
	desired *krm.ChronicleRuleSpec
	actual  map[string]any
}

var _ directbase.Adapter = &ruleAdapter{}

// Find reads the rule when its ID is known (status.externalRef or
// spec.resourceID); without one there is nothing to find and Create makes
// it.
func (a *ruleAdapter) Find(ctx context.Context) (bool, error) {
	if a.id.Rule == "" {
		return false, nil
	}
	got, err := a.api.Do(ctx, http.MethodGet, a.id.String(), map[string]string{"view": "FULL"}, nil)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting %s: %w", a.id, err)
	}
	a.actual = got
	return true, nil
}

func (a *ruleAdapter) body() map[string]any {
	b := map[string]any{"text": deref(a.desired.Text)}
	if a.desired.Scope != nil {
		b["scope"] = *a.desired.Scope
	}
	return b
}

func (a *ruleAdapter) Create(ctx context.Context, op *directbase.CreateOperation) error {
	if a.id.Rule != "" {
		// spec.resourceID names a rule that is not there: the API assigns
		// IDs, so it cannot be made with that one.
		return fmt.Errorf("%s was not found: spec.resourceID acquires a rule that exists; leave it out to create one", a.id)
	}
	parent := a.id.ParentString()
	klog.FromContext(ctx).V(2).Info("creating a rule", "parent", parent)
	got, err := a.api.Do(ctx, http.MethodPost, parent+"/rules", nil, a.body())
	if err != nil {
		return fmt.Errorf("creating a rule in %s: %w", parent, err)
	}
	name, _ := got["name"].(string)
	if err := a.id.FromExternal(name); err != nil {
		return fmt.Errorf("the created rule's name: %w", err)
	}
	// The create's answer may leave out what a full read shows.
	if got, err = a.api.Do(ctx, http.MethodGet, name, map[string]string{"view": "FULL"}, nil); err != nil {
		return fmt.Errorf("reading %s after create: %w", name, err)
	}
	a.actual = got
	return op.UpdateStatus(ctx, a.status(), nil)
}

// changes are the update mask: the fields the spec sets that differ.
func (a *ruleAdapter) changes() []string {
	var mask []string
	if text, _ := a.actual["text"].(string); a.desired.Text != nil && text != *a.desired.Text {
		mask = append(mask, "text")
	}
	if scope, _ := a.actual["scope"].(string); a.desired.Scope != nil && scope != *a.desired.Scope {
		mask = append(mask, "scope")
	}
	return mask
}

func (a *ruleAdapter) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	if mask := a.changes(); len(mask) > 0 {
		klog.FromContext(ctx).V(2).Info("patching", "name", a.id.String(), "mask", mask)
		body := a.body()
		body["name"] = a.id.String()
		if _, err := a.api.Do(ctx, http.MethodPatch, a.id.String(), map[string]string{"updateMask": strings.Join(mask, ",")}, body); err != nil {
			return fmt.Errorf("patching %s (%s): %w", a.id, strings.Join(mask, ","), err)
		}
		got, err := a.api.Do(ctx, http.MethodGet, a.id.String(), map[string]string{"view": "FULL"}, nil)
		if err != nil {
			return fmt.Errorf("reading %s after update: %w", a.id, err)
		}
		a.actual = got
	}
	return op.UpdateStatus(ctx, a.status(), nil)
}

func (a *ruleAdapter) status() *krm.ChronicleRuleStatus {
	name := a.id.String()
	s := &krm.ChronicleRuleStatus{ExternalRef: &name}
	if etag, _ := a.actual["etag"].(string); etag != "" {
		s.Etag = &etag
	}
	s.ObservedState = ruleObservedState(a.actual)
	return s
}

// ruleObservedState is upstream's observed state from the API's JSON, whose
// field names match its JSON tags but for revisionId.
func ruleObservedState(rule map[string]any) *kccchroniclev1alpha1.ChronicleRuleObservedState {
	in := runtime.DeepCopyJSON(rule)
	if v, ok := in["revisionId"]; ok {
		in["revisionID"] = v
		delete(in, "revisionId")
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	out := &kccchroniclev1alpha1.ChronicleRuleObservedState{}
	if err := json.Unmarshal(b, out); err != nil {
		return nil
	}
	return out
}

func (a *ruleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("ChronicleRule does not support export")
}

// Delete deletes the rule; its deployment goes with it. FORCE deletes its
// retrohunts and detections too, without which the API refuses a rule
// that has them.
func (a *ruleAdapter) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	var query map[string]string
	if deref(a.desired.DeletionPolicy) == "FORCE" {
		query = map[string]string{"force": "true"}
	}
	if _, err := a.api.Do(ctx, http.MethodDelete, a.id.String(), query, nil); err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting %s: %w", a.id, err)
	}
	return true, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
