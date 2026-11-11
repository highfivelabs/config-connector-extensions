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
// pkg/controller/direct/securitycenter/muteconfig/securitycentermuteconfig_controller.go
// (UPSTREAM.md). Changed: the package is securitycenter and the model is
// NewMuteConfigModel; the parent is the identity's organization, folder or
// project; on update the desired name is the one the API returned, so a
// name the API writes differently (a project's number for its ID) is no
// diff; export sets the parent reference it came from; delete of a
// missing mute config reports it deleted. Our KRM types.

package securitycenter

import (
	"context"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/securitycenter/apiv1"
	securitycenterpb "cloud.google.com/go/securitycenter/apiv1/securitycenterpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"

	krm "github.com/highfivelabs/config-connector-extensions/apis/securitycenter/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.SecurityCenterMuteConfigGVK, NewMuteConfigModel)
}

func NewMuteConfigModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &muteConfigModel{config: *config}, nil
}

var _ directbase.Model = &muteConfigModel{}

type muteConfigModel struct {
	config config.ControllerConfig
}

func (m *muteConfigModel) client(ctx context.Context) (*gcp.Client, error) {
	var opts []option.ClientOption
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp.NewRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building SecurityCenter MuteConfig client: %w", err)
	}
	return gcpClient, err
}

func (m *muteConfigModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.SecurityCenterMuteConfig{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	identity, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	return &securityCenterMuteConfigAdapter{
		id:        identity.(*krm.SecurityCenterMuteConfigIdentity),
		gcpClient: gcpClient,
		desired:   obj,
	}, nil
}

func (m *muteConfigModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	if !strings.HasPrefix(url, "//securitycenter.googleapis.com/") {
		return nil, nil
	}

	parsed := &krm.SecurityCenterMuteConfigIdentity{}
	if err := parsed.FromExternal(strings.TrimPrefix(url, "//")); err != nil {
		return nil, nil
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	return &securityCenterMuteConfigAdapter{
		id:        parsed,
		gcpClient: gcpClient,
	}, nil
}

type securityCenterMuteConfigAdapter struct {
	id        *krm.SecurityCenterMuteConfigIdentity
	gcpClient *gcp.Client
	desired   *krm.SecurityCenterMuteConfig
	actual    *securitycenterpb.MuteConfig
}

var _ directbase.Adapter = &securityCenterMuteConfigAdapter{}

func (a *securityCenterMuteConfigAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting SecurityCenterMuteConfig", "name", a.id.String())

	req := &securitycenterpb.GetMuteConfigRequest{
		Name: a.id.String(),
	}
	muteConfig, err := a.gcpClient.GetMuteConfig(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("SecurityCenterMuteConfig not found", "name", a.id.String())
			return false, nil
		}
		return false, fmt.Errorf("getting SecurityCenterMuteConfig %q: %w", a.id.String(), err)
	}

	a.actual = muteConfig
	return true, nil
}

func (a *securityCenterMuteConfigAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating SecurityCenterMuteConfig", "name", a.id.String())

	mapCtx := &direct.MapContext{}
	muteConfig := SecurityCenterMuteConfigSpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	req := &securitycenterpb.CreateMuteConfigRequest{
		Parent:       a.id.Parent.String(),
		MuteConfigId: a.id.MuteConfig,
		MuteConfig:   muteConfig,
	}

	created, err := a.gcpClient.CreateMuteConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("creating SecurityCenterMuteConfig %q: %w", a.id.String(), err)
	}

	log.V(2).Info("successfully created SecurityCenterMuteConfig", "name", a.id.String())

	status := &krm.SecurityCenterMuteConfigStatus{}
	status.ObservedState = SecurityCenterMuteConfigObservedState_FromProto(mapCtx, created)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.PtrTo(a.id.String())
	return createOp.UpdateStatus(ctx, status, nil)
}

func (a *securityCenterMuteConfigAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating SecurityCenterMuteConfig", "name", a.id.String())

	mapCtx := &direct.MapContext{}
	desiredProto := SecurityCenterMuteConfigSpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	desiredProto.Name = a.actual.GetName()

	if a.desired.Spec.Type != nil {
		desiredType := direct.Enum_ToProto[securitycenterpb.MuteConfig_MuteConfigType](mapCtx, a.desired.Spec.Type)
		if desiredType != a.actual.Type {
			return fmt.Errorf("type is immutable and cannot be updated from %s to %s", a.actual.Type, desiredType)
		}
	} else {
		desiredProto.Type = a.actual.Type
	}

	paths, report, err := common.CompareProtoMessageStructuredDiff(desiredProto, a.actual, common.BasicDiff)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		log.V(2).Info("no field needs update", "name", a.id.String())
		status := &krm.SecurityCenterMuteConfigStatus{}
		status.ObservedState = SecurityCenterMuteConfigObservedState_FromProto(mapCtx, a.actual)
		if mapCtx.Err() != nil {
			return mapCtx.Err()
		}
		status.ExternalRef = direct.PtrTo(a.id.String())
		return updateOp.UpdateStatus(ctx, status, nil)
	}

	report.Object = updateOp.GetUnstructured()
	structuredreporting.ReportDiff(ctx, report)

	req := &securitycenterpb.UpdateMuteConfigRequest{
		MuteConfig: desiredProto,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: sets.List(paths)},
	}

	updated, err := a.gcpClient.UpdateMuteConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("updating SecurityCenterMuteConfig %q: %w", a.id.String(), err)
	}

	status := &krm.SecurityCenterMuteConfigStatus{}
	status.ObservedState = SecurityCenterMuteConfigObservedState_FromProto(mapCtx, updated)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.PtrTo(a.id.String())
	return updateOp.UpdateStatus(ctx, status, nil)
}

func (a *securityCenterMuteConfigAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	mapCtx := &direct.MapContext{}
	spec := SecurityCenterMuteConfigSpec_FromProto(mapCtx, a.actual)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	obj := &krm.SecurityCenterMuteConfig{}
	obj.Spec = *spec
	switch {
	case a.id.Parent.OrganizationID != "":
		obj.Spec.OrganizationRef = &refs.OrganizationRef{External: a.id.Parent.OrganizationID}
	case a.id.Parent.FolderID != "":
		obj.Spec.FolderRef = &refs.FolderRefDeprecated{External: a.id.Parent.FolderID}
	default:
		obj.Spec.ProjectRef = &refs.ProjectRef{External: a.id.Parent.ProjectID}
	}

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u.SetName(a.id.MuteConfig)
	u.SetGroupVersionKind(krm.SecurityCenterMuteConfigGVK)
	u.Object = uObj
	return u, nil
}

func (a *securityCenterMuteConfigAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting SecurityCenterMuteConfig", "name", a.id.String())

	req := &securitycenterpb.DeleteMuteConfigRequest{
		Name: a.id.String(),
	}
	err := a.gcpClient.DeleteMuteConfig(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting SecurityCenterMuteConfig %q: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully deleted SecurityCenterMuteConfig", "name", a.id.String())
	return true, nil
}
