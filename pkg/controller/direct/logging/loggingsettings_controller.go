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

// Package logging has the direct controller for this project's Cloud
// Logging kind.
//
// Written here: no Config Connector kind manages Log Router settings (its
// closed PR #3730 was a scaffold with no fields). This controller follows
// Terraform's google_logging_organization_settings and
// google_logging_folder_settings (read, not copied): the settings always
// exist, so create and update are both settings.patch with a mask of the
// fields the spec sets and the cloud does not match, and delete leaves the
// settings as they are. It uses the Logging REST client, as Config
// Connector's logging controllers do.
package logging

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	api "google.golang.org/api/logging/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.LoggingSettingsGVK, NewSettingsModel)
}

func NewSettingsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &settingsModel{config: *config}, nil
}

type settingsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &settingsModel{}

func (m *settingsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.LoggingSettings{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if err := common.NormalizeReferences(ctx, op.Reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}
	id, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	svc, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building logging client: %w", err)
	}
	return &settingsAdapter{id: id.(*krm.LoggingSettingsIdentity), svc: svc, desired: obj}, nil
}

func (m *settingsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type settingsAdapter struct {
	id      *krm.LoggingSettingsIdentity
	svc     *api.Service
	desired *krm.LoggingSettings
	actual  *api.Settings
}

var _ directbase.Adapter = &settingsAdapter{}

func (a *settingsAdapter) isFolder() bool {
	return strings.HasPrefix(a.id.Parent, "folders/")
}

func (a *settingsAdapter) get(ctx context.Context) (*api.Settings, error) {
	if a.isFolder() {
		return a.svc.Folders.GetSettings(a.id.Parent).Context(ctx).Do()
	}
	return a.svc.Organizations.GetSettings(a.id.Parent).Context(ctx).Do()
}

func (a *settingsAdapter) patch(ctx context.Context, s *api.Settings, mask string) (*api.Settings, error) {
	if a.isFolder() {
		return a.svc.Folders.UpdateSettings(a.id.Parent, s).UpdateMask(mask).Context(ctx).Do()
	}
	return a.svc.Organizations.UpdateSettings(a.id.Parent, s).UpdateMask(mask).Context(ctx).Do()
}

// Find reads the settings, which exist for every organization and folder.
func (a *settingsAdapter) Find(ctx context.Context) (bool, error) {
	actual, err := a.get(ctx)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting LoggingSettings %s: %w", a.id, err)
	}
	a.actual = actual
	return true, nil
}

// Create runs when the settings could not be read (for example, the
// organization or folder does not exist): the patch reports why.
func (a *settingsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	a.actual = &api.Settings{}
	return a.apply(ctx, createOp)
}

func (a *settingsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	return a.apply(ctx, updateOp)
}

// apply patches the fields the spec sets and the cloud does not match.
func (a *settingsAdapter) apply(ctx context.Context, op directbase.Operation) error {
	s := a.desired.Spec
	want := &api.Settings{}
	var paths []string
	if s.KMSKeyRef != nil && s.KMSKeyRef.External != a.actual.KmsKeyName {
		want.KmsKeyName = s.KMSKeyRef.External
		paths = append(paths, "kmsKeyName")
	}
	if s.StorageLocation != nil && *s.StorageLocation != a.actual.StorageLocation {
		want.StorageLocation = *s.StorageLocation
		want.ForceSendFields = append(want.ForceSendFields, "StorageLocation")
		paths = append(paths, "storageLocation")
	}
	if s.DisableDefaultSink != nil && *s.DisableDefaultSink != a.actual.DisableDefaultSink {
		want.DisableDefaultSink = *s.DisableDefaultSink
		want.ForceSendFields = append(want.ForceSendFields, "DisableDefaultSink")
		paths = append(paths, "disableDefaultSink")
	}
	latest := a.actual
	if len(paths) > 0 {
		sort.Strings(paths)
		klog.FromContext(ctx).V(2).Info("updating LoggingSettings", "name", a.id, "paths", paths)
		updated, err := a.patch(ctx, want, strings.Join(paths, ","))
		if err != nil {
			return fmt.Errorf("updating LoggingSettings %s: %w", a.id, err)
		}
		latest = updated
	}
	status := &krm.LoggingSettingsStatus{
		ExternalRef: direct.LazyPtr(a.id.String()),
		ObservedState: &krm.LoggingSettingsObservedState{
			KMSServiceAccountID:     direct.LazyPtr(latest.KmsServiceAccountId),
			LoggingServiceAccountID: direct.LazyPtr(latest.LoggingServiceAccountId),
		},
	}
	return op.UpdateStatus(ctx, status, nil)
}

// Delete leaves the settings as they are: they cannot be deleted, and
// Terraform's resources only forget them.
func (a *settingsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("forgetting LoggingSettings; the settings stay as they are", "name", a.id)
	return true, nil
}

func (a *settingsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}
