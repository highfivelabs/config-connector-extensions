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
// pkg/controller/direct/serviceusage/serviceidentity_controller.go
// (UPSTREAM.md). Upstream's parent is a project; ours is a folder or an
// organization (services.generateServiceIdentity takes either, as
// Terraform's google_folder_service_identity and
// google_organization_service_identity call it), and the status carries
// the identity's name in externalRef so a changed parent is refused rather
// than reported with the old email, and observedState through the generated
// mapper (mapper.generated.go). Export is left out (no exporter here).

// Package serviceusage has the direct controller for this project's Service
// Usage kind.
package serviceusage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apiscommon "github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/api/option"
	gcp "google.golang.org/api/serviceusage/v1beta1"
	pb "google.golang.org/genproto/googleapis/api/serviceusage/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	krm "github.com/highfivelabs/config-connector-extensions/apis/serviceusage/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ServiceIdentityGVK, NewServiceIdentityModel)
}

func NewServiceIdentityModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &serviceIdentityModel{config: *config}, nil
}

var _ directbase.Model = &serviceIdentityModel{}

type serviceIdentityModel struct {
	config config.ControllerConfig
}

func (m *serviceIdentityModel) client(ctx context.Context) (*gcp.APIService, error) {
	var opts []option.ClientOption

	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}

	gcpClient, err := gcp.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building serviceusage v1beta1 client: %w", err)
	}

	return gcpClient, nil
}

func (m *serviceIdentityModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	kube := op.Reader
	obj := &krm.ServiceIdentity{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	if err := common.NormalizeReferences(ctx, kube, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	id, err := obj.GetIdentity(ctx, kube)
	if err != nil {
		return nil, err
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	return &serviceIdentityAdapter{
		gcpClient:   gcpClient,
		id:          id.(*krm.ServiceIdentityIdentity),
		desiredKube: obj,
	}, nil
}

func (m *serviceIdentityModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type serviceIdentityAdapter struct {
	gcpClient   *gcp.APIService
	id          *krm.ServiceIdentityIdentity
	desiredKube *krm.ServiceIdentity
	actual      *gcp.ServiceIdentity
}

var _ directbase.Adapter = &serviceIdentityAdapter{}

func (a *serviceIdentityAdapter) Find(ctx context.Context) (bool, error) {
	// The GCP Service Usage API only provides a ':generateServiceIdentity' mutation endpoint and does
	// not support a standard GET/Read endpoint for Service Identities.
	// Therefore, we must rely solely on the status of our Kubernetes resource. If we already have the email
	// stored in status, we assume the Service Identity exists and is reconciled. Otherwise, we return false
	// to trigger the Create() flow where we safely invoke ':generateServiceIdentity'.
	if a.desiredKube != nil && a.desiredKube.Status.Email != nil && *a.desiredKube.Status.Email != "" {
		a.actual = &gcp.ServiceIdentity{
			Email: *a.desiredKube.Status.Email,
		}
		if observed := a.desiredKube.Status.ObservedState; observed != nil {
			a.actual.UniqueId = apiscommon.ValueOf(observed.UniqueID)
		}
		return true, nil
	}

	return false, nil
}

func (a *serviceIdentityAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	parent := a.id.String()
	op, err := a.gcpClient.Services.GenerateServiceIdentity(parent).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("generating service identity for %s: %w", parent, err)
	}

	completedOp, err := a.waitForOp(ctx, op)
	if err != nil {
		return fmt.Errorf("waiting for service identity generation: %w", err)
	}

	var identity gcp.ServiceIdentity
	if err := json.Unmarshal(completedOp.Response, &identity); err != nil {
		return fmt.Errorf("unmarshalling service identity response: %w", err)
	}
	if identity.Email == "" {
		return fmt.Errorf("generating service identity for %s: the service returned no email (it may have no service agent at this level)", parent)
	}

	a.actual = &identity
	return a.updateStatus(ctx, createOp)
}

func (a *serviceIdentityAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	// ServiceIdentity does not support actual updates in the Service Usage GCP API.
	// We simply run updateStatus to keep the Kubernetes resource status synchronized with GCP.
	if a.actual == nil {
		return fmt.Errorf("actual service identity not set during update")
	}
	return a.updateStatus(ctx, updateOp)
}

func (a *serviceIdentityAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	// ServiceIdentity cannot be deleted from the GCP Service Usage API, as it is dynamically
	// generated and managed by GCP once initialized via ':generateServiceIdentity'.
	// Therefore, Delete is a no-op on the GCP side, and we return true to successfully delete the Kube resource.
	return true, nil
}

func (a *serviceIdentityAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *serviceIdentityAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	mapCtx := &direct.MapContext{}
	status := &krm.ServiceIdentityStatus{}
	status.Email = apiscommon.LazyPtr(a.actual.Email)
	status.ExternalRef = apiscommon.LazyPtr(a.id.String())
	status.ObservedState = ServiceIdentityObservedState_FromProto(mapCtx, &pb.ServiceIdentity{
		Email:    a.actual.Email,
		UniqueId: a.actual.UniqueId,
	})
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	return op.UpdateStatus(ctx, status, nil)
}

func (a *serviceIdentityAdapter) waitForOp(ctx context.Context, op *gcp.Operation) (*gcp.Operation, error) {
	return common.WaitForOperation(ctx, 2*time.Second, func(current *gcp.Operation) (bool, error) {
		if current.Done {
			if current.Error != nil {
				return true, fmt.Errorf("operation %q completed with error: %s", op.Name, current.Error.Message)
			}
			return true, nil
		}
		return false, nil
	}, func() (*gcp.Operation, error) {
		current, err := a.gcpClient.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("getting operation status of %q: %w", op.Name, err)
		}
		return current, nil
	})
}
