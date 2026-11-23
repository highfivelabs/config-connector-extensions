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

package monitoring

// Written here: MonitoringAlertPolicyFields, a field kind (docs/design.md
// section 6.6, pattern 2; decision 37). Config Connector's
// MonitoringAlertPolicy has no documentation.subject, documentation.links
// or alertStrategy.notificationPrompts. The policy is read with
// alertPolicies.get and written with alertPolicies.patch.
//
// The partial case, as a node pool's kubeletConfig is
// (ContainerNodePoolFields): Config Connector's MonitoringAlertPolicy runs
// Terraform by default, whose google_monitoring_alert_policy update (read
// at the pin's fork) names documentation and alertStrategy whole in its
// update mask when it changes them, and sends them without these fields;
// its opt-in direct controller does the same (a mask of the top-level
// fields that differ). So ours sends documentation and alertStrategy as
// read with its fields set (the API takes them whole, under the masks
// documentation and alertStrategy), and a change Config Connector makes to
// the same object drops ours until our next reconcile puts them back.

import (
	"context"
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

// NewAlertPolicyFieldsModel is the MonitoringAlertPolicyFields model.
func NewAlertPolicyFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &alertFieldsModel{config: *config}, nil
}

type alertFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &alertFieldsModel{}

func (m *alertFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.MonitoringAlertPolicyFields{}
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
		// Deleting: the policy is in status; Config Connector's object may
		// be gone already.
		ref = &monitoringv1beta1.MonitoringAlertPolicyRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &monitoringv1beta1.MonitoringAlertPolicyIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the alert policy of MonitoringAlertPolicyFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.MonitoringAlertPolicyFieldsGVK, obj, id.String()); err != nil {
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
	return &alertFieldsAdapter{id: id, gcpClient: c, desired: &obj.Spec}, nil
}

func (m *alertFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type alertFieldsAdapter struct {
	id        *monitoringv1beta1.MonitoringAlertPolicyIdentity
	gcpClient *api.Service
	desired   *krm.MonitoringAlertPolicyFieldsSpec
	actual    *api.AlertPolicy
}

var _ directbase.Adapter = &alertFieldsAdapter{}

// Find reports whether the policy exists. A missing one is "not found" so
// that a delete finishes when it is gone; a create then waits (Create).
func (a *alertFieldsAdapter) Find(ctx context.Context) (bool, error) {
	got, err := a.gcpClient.Projects.AlertPolicies.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting alert policy %s: %w", a.id, err)
	}
	a.actual = got
	return true, nil
}

// Create is called only while the policy does not exist: this kind never
// makes it, so it waits for it.
func (a *alertFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("MonitoringAlertPolicy", a.id.String())
}

// changes is the patch body and update mask that make the policy's owned
// fields what the spec says, or, clearing (Delete), unset: each owned
// field's parent (documentation, alertStrategy) as read, with the owned
// fields set.
func (a *alertFieldsAdapter) changes(clear bool) (*api.AlertPolicy, []string, error) {
	body := &api.AlertPolicy{}
	var mask []string

	if d := a.desired.Documentation; d != nil {
		doc := &api.Documentation{}
		if a.actual.Documentation != nil {
			c := *a.actual.Documentation
			doc = &c
		}
		differs := false
		if d.Subject != nil {
			want := ""
			if !clear {
				want = *d.Subject
			}
			differs = differs || doc.Subject != want
			doc.Subject = want
		}
		if d.Links != nil {
			var want []*api.Link
			if !clear {
				for _, l := range d.Links {
					want = append(want, &api.Link{DisplayName: direct.ValueOf(l.DisplayName), Url: direct.ValueOf(l.URL)})
				}
			}
			if !sameLinks(doc.Links, want) {
				differs = true
			}
			doc.Links = want
		}
		if differs {
			body.Documentation = doc
			mask = append(mask, "documentation")
		}
	}

	if s := a.desired.AlertStrategy; s != nil && s.NotificationPrompts != nil {
		strategy := &api.AlertStrategy{}
		if a.actual.AlertStrategy != nil {
			c := *a.actual.AlertStrategy
			strategy = &c
		}
		var want []string
		if !clear {
			want = s.NotificationPrompts
		}
		if !sameStrings(strategy.NotificationPrompts, want) {
			strategy.NotificationPrompts = want
			body.AlertStrategy = strategy
			mask = append(mask, "alertStrategy")
		}
	}
	return body, mask, nil
}

func sameLinks(a, b []*api.Link) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].DisplayName != b[i].DisplayName || a[i].Url != b[i].Url {
			return false
		}
	}
	return true
}

func sameStrings(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func (a *alertFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
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
func (a *alertFieldsAdapter) patch(ctx context.Context, verb string, body *api.AlertPolicy, mask []string) error {
	if len(mask) == 0 {
		return nil
	}
	klog.FromContext(ctx).V(2).Info(verb+" fields of an alert policy", "name", a.id, "updateMask", mask)
	got, err := a.gcpClient.Projects.AlertPolicies.Patch(a.id.String(), body).UpdateMask(strings.Join(mask, ",")).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("%s %s of alert policy %s: %w", verb, strings.Join(mask, ", "), a.id, err)
	}
	a.actual = got
	return nil
}

// Delete clears the fields this object owns and never deletes the policy.
// A policy that is gone is not reached: Find said not found.
func (a *alertFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	body, mask, err := a.changes(true)
	if err != nil {
		return false, err
	}
	if err := a.patch(ctx, "clearing", body, mask); err != nil {
		return false, err
	}
	return true, nil
}

func (a *alertFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *alertFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.MonitoringAlertPolicyFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	return op.UpdateStatus(ctx, status, nil)
}
