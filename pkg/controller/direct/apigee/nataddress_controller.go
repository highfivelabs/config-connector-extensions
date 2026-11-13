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
// pkg/controller/direct/apigee/apigeenataddress_controller.go, with the client
// (client.go) and operation wait (lro.go) it uses (UPSTREAM.md). Changed: our
// KRM types (stable, apigee.gcp.hfl.dev/v1beta1, docs/design.md decision 26);
// the model is returned to our manager, not registered with Config
// Connector's registry; status.externalRef written; Export is not supported
// (no kind here exports).

package apigee

import (
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	api "google.golang.org/api/apigee/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/apigee/v1beta1"
)

func NewApigeeNATAddressModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelApigeeNATAddress{config: config}, nil
}

var _ directbase.Model = &modelApigeeNATAddress{}

type modelApigeeNATAddress struct {
	config *config.ControllerConfig
}

func (m *modelApigeeNATAddress) service(ctx context.Context) (*api.Service, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	s, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building gcp service client: %w", err)
	}
	return s, nil
}

func (m *modelApigeeNATAddress) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.ApigeeNATAddress{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	i, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}
	id := i.(*krm.ApigeeNATAddressIdentity)

	s, err := m.service(ctx)
	if err != nil {
		return nil, err
	}
	return &ApigeeNATAddressAdapter{
		id:                 id,
		natAddressesClient: api.NewOrganizationsInstancesNatAddressesService(s),
		operationsClient:   api.NewOrganizationsOperationsService(s),
		desired:            obj,
	}, nil
}

// AdapterForURL is not supported: no kind here is exported.
func (m *modelApigeeNATAddress) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type ApigeeNATAddressAdapter struct {
	id                 *krm.ApigeeNATAddressIdentity
	natAddressesClient *api.OrganizationsInstancesNatAddressesService
	operationsClient   *api.OrganizationsOperationsService
	desired            *krm.ApigeeNATAddress
	actual             *api.GoogleCloudApigeeV1NatAddress
}

var _ directbase.Adapter = &ApigeeNATAddressAdapter{}

// Find retrieves the GCP resource.
// Return true means the object is found. This triggers Adapter `Update` call.
// Return false means the object is not found. This triggers Adapter `Create` call.
// Return a non-nil error requeues the requests.
func (a *ApigeeNATAddressAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ApigeeNATAddress", "name", a.id)

	resource, err := a.natAddressesClient.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ApigeeNATAddress %q: %w", a.id, err)
	}

	a.actual = resource
	return true, nil
}

// Create creates the resource in GCP based on `spec` and updates the Config Connector object `status` based on the GCP response.
func (a *ApigeeNATAddressAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ApigeeNATAddress", "name", a.id)
	mapCtx := &direct.MapContext{}

	desired := a.desired.DeepCopy()
	resource := ApigeeNATAddressSpec_ToAPI(mapCtx, &desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	resource.Name = a.id.NatAddress

	op, err := a.natAddressesClient.Create(a.id.ParentString(), resource).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating ApigeeNATAddress %s: %w", a.id, err)
	}
	if err := waitForApigeeOp(ctx, a.operationsClient, op); err != nil {
		return fmt.Errorf("ApigeeNATAddress %s waiting creation: %w", a.id, err)
	}

	created, err := a.natAddressesClient.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("getting created ApigeeNATAddress %s: %w", a.id, err)
	}

	log.V(2).Info("successfully created ApigeeNATAddress", "name", a.id)

	return a.updateStatus(ctx, createOp, created)
}

// Update updates the resource in GCP based on `spec` and updates the Config Connector object `status` based on the GCP response.
func (a *ApigeeNATAddressAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating ApigeeNATAddress", "id", a.id)

	// There are no mutable fields in the GCP ApigeeNATAddress API.
	// So, we will only update the KRM status.
	return a.updateStatus(ctx, updateOp, a.actual)
}

func (a *ApigeeNATAddressAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *api.GoogleCloudApigeeV1NatAddress) error {
	mapCtx := &direct.MapContext{}
	status := ApigeeNATAddressStatus_FromAPI(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

func (a *ApigeeNATAddressAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("export of ApigeeNATAddress is not supported")
}

// Delete deletes the resource from GCP service when the corresponding Config Connector resource is deleted.
func (a *ApigeeNATAddressAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ApigeeNATAddress", "id", a.id)

	op, err := a.natAddressesClient.Delete(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			// Return success if not found (assume it was already deleted).
			log.V(2).Info("skipping delete for non-existent ApigeeNATAddress, assuming it was already deleted", "id", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting ApigeeNATAddress %s: %w", a.id, err)
	}
	log.V(2).Info("successfully deleted ApigeeNATAddress", "id", a.id)

	if err := waitForApigeeOp(ctx, a.operationsClient, op); err != nil {
		return false, fmt.Errorf("waiting delete ApigeeNATAddress %s: %w", a.id, err)
	}
	return true, nil
}

func waitForApigeeOp(ctx context.Context, client *api.OrganizationsOperationsService, op *api.GoogleLongrunningOperation) error {
	return common.WaitForDoneOrTimeout(ctx, 2*time.Second, func() (bool, error) {
		current, err := client.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return false, fmt.Errorf("getting operation status of %q: %w", op.Name, err)
		}
		if current.Done {
			if current.Error != nil {
				return true, fmt.Errorf("operation %q completed with error: %v", op.Name, current.Error)
			} else {
				return true, nil
			}
		}
		return false, nil
	})
}
