// Copyright 2025 Google LLC
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
// pkg/controller/direct/spanner/spannerinstanceconfig_controller.go and the client
// helper in client.go (UPSTREAM.md). Changed: our KRM types (stable,
// spanner.gcp.hfl.dev/v1beta1, docs/design.md decision 26); the model is
// returned to our manager, not registered with Config Connector's registry;
// an update waits for its operation, reads the configuration again and
// writes status.externalRef (upstream's did neither, so its status was the
// state before the update and lost the external reference); labels are
// compared as maps, an empty map equal to none (upstream's reflect.DeepEqual
// told a nil map from an empty one); CloudLabels for the slot guard; Export
// is not supported (no kind here exports).

package spanner

import (
	"context"
	"fmt"
	"maps"

	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	instancepb "cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	krm "github.com/highfivelabs/config-connector-extensions/apis/spanner/v1beta1"
)

func NewInstanceConfigModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &instanceConfigModel{config: config}, nil
}

var _ directbase.Model = &instanceConfigModel{}

type instanceConfigModel struct {
	config *config.ControllerConfig
}

func (m *instanceConfigModel) client(ctx context.Context) (*instance.InstanceAdminClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	client, err := instance.NewInstanceAdminRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building spanner instance client: %w", err)
	}
	return client, nil
}

func (m *instanceConfigModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.SpannerInstanceConfig{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := krm.NewInstanceConfigIdentity(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	instanceConfigClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	return &instanceConfigAdapter{
		gcpClient: instanceConfigClient,
		id:        id,
		desired:   obj,
	}, nil
}

// AdapterForURL is not supported: no kind here is exported.
func (m *instanceConfigModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type instanceConfigAdapter struct {
	gcpClient *instance.InstanceAdminClient
	id        *krm.InstanceConfigIdentity
	desired   *krm.SpannerInstanceConfig
	actual    *instancepb.InstanceConfig
}

var _ directbase.Adapter = &instanceConfigAdapter{}

func (a *instanceConfigAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting spanner instance config", "name", a.id)

	req := &instancepb.GetInstanceConfigRequest{Name: a.id.String()}
	actual, err := a.gcpClient.GetInstanceConfig(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting spanner instance config %q from gcp: %w", a.id.String(), err)
	}

	a.actual = actual
	return true, nil
}

// CloudLabels are the configuration's labels as read by Find, for the slot
// guard (pkg/slotguard).
func (a *instanceConfigAdapter) CloudLabels() map[string]string {
	return a.actual.GetLabels()
}

func (a *instanceConfigAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating spanner instance config", "name", a.id)

	mapCtx := &direct.MapContext{}
	desired := a.desired.DeepCopy()
	resource := SpannerInstanceConfigSpec_ToProto(mapCtx, &desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	resource.Name = a.id.String()

	req := &instancepb.CreateInstanceConfigRequest{
		Parent:           a.id.Parent().String(),
		InstanceConfigId: a.id.ID(),
		InstanceConfig:   resource,
		ValidateOnly:     false,
	}
	op, err := a.gcpClient.CreateInstanceConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("creating spanner instance config %s: %w", a.id.String(), err)
	}
	created, err := op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("spanner instance config %s waiting creation: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully created spanner instance config in gcp", "name", a.id)

	return a.updateStatus(ctx, createOp, created)
}

// sameLabels reports whether two label sets are equal, an empty set equal
// to none.
func sameLabels(a, b map[string]string) bool {
	return maps.Equal(a, b)
}

// SpannerInstanceConfig supports update.
func (a *instanceConfigAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating spanner instance config", "name", a.id)

	mapCtx := &direct.MapContext{}
	desired := a.desired.DeepCopy()
	resource := SpannerInstanceConfigSpec_ToProto(mapCtx, &desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	resource.Name = a.id.String()

	// Check the spec for changes
	// Only display_name and labels can be updated.
	report := &structuredreporting.Diff{Object: updateOp.GetUnstructured()}
	updateMask := &fieldmaskpb.FieldMask{}
	if resource.DisplayName != a.actual.DisplayName {
		report.AddField("display_name", a.actual.DisplayName, resource.DisplayName)
		updateMask.Paths = append(updateMask.Paths, "display_name")
	}
	if !sameLabels(resource.Labels, a.actual.Labels) {
		report.AddField("labels", a.actual.Labels, resource.Labels)
		updateMask.Paths = append(updateMask.Paths, "labels")
	}
	if len(updateMask.Paths) == 0 {
		log.V(2).Info("no field to update", "name", a.id)
		return a.updateStatus(ctx, updateOp, a.actual)
	}

	structuredreporting.ReportDiff(ctx, report)

	req := &instancepb.UpdateInstanceConfigRequest{
		InstanceConfig: resource,
		UpdateMask:     updateMask,
		ValidateOnly:   false,
	}
	op, err := a.gcpClient.UpdateInstanceConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("updating spanner instance config %s: %w", a.id.String(), err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("spanner instance config %s waiting update: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully updated spanner instance config in gcp", "name", a.id)

	latest, err := a.gcpClient.GetInstanceConfig(ctx, &instancepb.GetInstanceConfigRequest{Name: a.id.String()})
	if err != nil {
		return fmt.Errorf("reading updated spanner instance config %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, latest)
}

func (a *instanceConfigAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *instancepb.InstanceConfig) error {
	mapCtx := &direct.MapContext{}
	status := &krm.SpannerInstanceConfigStatus{}
	status.ObservedState = SpannerInstanceConfigObservedState_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

func (a *instanceConfigAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting spanner instance config", "name", a.id)

	req := &instancepb.DeleteInstanceConfigRequest{
		Name:         a.id.String(),
		ValidateOnly: false,
	}
	err := a.gcpClient.DeleteInstanceConfig(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			// Return success if not found (assume it was already deleted).
			log.V(2).Info("skipping delete for non-existent spanner instance config, assuming it was already deleted", "name", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting spanner instance config %s: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully deleted spanner instance config", "name", a.id)

	return true, nil
}

func (a *instanceConfigAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("export of SpannerInstanceConfig is not supported")
}
