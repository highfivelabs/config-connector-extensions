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

// Package iam has the direct controllers for this project's IAM kinds.
//
// Written here: Config Connector has types for IAMWorkloadIdentityPoolNamespace
// only in pull request #14209, and no controller. This one is on Config
// Connector's direct reconciler, on google.golang.org/api/iam/v1, and
// follows Terraform's google_iam_workload_identity_pool_namespace (read, not
// copied): description and disabled change in place; a namespace in state
// DELETED (soft-deleted, kept for about 30 days) counts as absent; its ID
// cannot be reused until it is purged.
package iam

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	api "google.golang.org/api/iam/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.IAMWorkloadIdentityPoolNamespaceGVK, NewWorkloadIdentityPoolNamespaceModel)
}

func NewWorkloadIdentityPoolNamespaceModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &namespaceModel{config: *config}, nil
}

type namespaceModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &namespaceModel{}

func (m *namespaceModel) service(ctx context.Context) (*api.Service, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	svc, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building IAM client: %w", err)
	}
	return svc, nil
}

func (m *namespaceModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.IAMWorkloadIdentityPoolNamespace{}
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
	svc, err := m.service(ctx)
	if err != nil {
		return nil, err
	}
	return &namespaceAdapter{id: id.(*krm.IAMWorkloadIdentityPoolNamespaceIdentity), svc: svc, desired: obj}, nil
}

func (m *namespaceModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type namespaceAdapter struct {
	id      *krm.IAMWorkloadIdentityPoolNamespaceIdentity
	svc     *api.Service
	desired *krm.IAMWorkloadIdentityPoolNamespace
	actual  *api.WorkloadIdentityPoolNamespace
}

var _ directbase.Adapter = &namespaceAdapter{}

func (a *namespaceAdapter) namespaces() *api.ProjectsLocationsWorkloadIdentityPoolsNamespacesService {
	return a.svc.Projects.Locations.WorkloadIdentityPools.Namespaces
}

func (a *namespaceAdapter) Find(ctx context.Context) (bool, error) {
	actual, err := a.namespaces().Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	if actual.State == "DELETED" {
		// Soft-deleted: absent, as Terraform treats it. Creating it again
		// fails until the API purges it.
		return false, nil
	}
	a.actual = actual
	return true, nil
}

func (a *namespaceAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating IAMWorkloadIdentityPoolNamespace", "name", a.id)
	desired := &api.WorkloadIdentityPoolNamespace{
		Description: direct.ValueOf(a.desired.Spec.Description),
		Disabled:    direct.ValueOf(a.desired.Spec.Disabled),
	}
	op, err := a.namespaces().Create(a.id.ParentString(), desired).WorkloadIdentityPoolNamespaceId(a.id.WorkloadIdentityPoolNamespace).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return fmt.Errorf("creating IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	return a.readAndUpdateStatus(ctx, createOp)
}

func (a *namespaceAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	var mask []string
	if a.desired.Spec.Description != nil && direct.ValueOf(a.desired.Spec.Description) != a.actual.Description {
		mask = append(mask, "description")
	}
	if a.desired.Spec.Disabled != nil && direct.ValueOf(a.desired.Spec.Disabled) != a.actual.Disabled {
		mask = append(mask, "disabled")
	}
	if len(mask) == 0 {
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	klog.FromContext(ctx).V(2).Info("updating IAMWorkloadIdentityPoolNamespace", "name", a.id, "mask", mask)
	desired := &api.WorkloadIdentityPoolNamespace{
		Description:     direct.ValueOf(a.desired.Spec.Description),
		Disabled:        direct.ValueOf(a.desired.Spec.Disabled),
		ForceSendFields: []string{"Description", "Disabled"},
	}
	op, err := a.namespaces().Patch(a.id.String(), desired).UpdateMask(strings.Join(mask, ",")).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return fmt.Errorf("updating IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	return a.readAndUpdateStatus(ctx, updateOp)
}

func (a *namespaceAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting IAMWorkloadIdentityPoolNamespace", "name", a.id)
	op, err := a.namespaces().Delete(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return false, fmt.Errorf("deleting IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	return true, nil
}

func (a *namespaceAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	obj := &krm.IAMWorkloadIdentityPoolNamespace{}
	obj.Spec.Location = direct.LazyPtr(a.id.Location)
	obj.Spec.ResourceID = direct.LazyPtr(a.id.WorkloadIdentityPoolNamespace)
	obj.Spec.WorkloadIdentityPoolRef = &krm.IAMWorkloadIdentityPoolRef{External: a.id.ParentString()}
	obj.Spec.Description = direct.LazyPtr(a.actual.Description)
	obj.Spec.Disabled = direct.LazyPtr(a.actual.Disabled)
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	out := &unstructured.Unstructured{Object: u}
	out.SetName(a.id.WorkloadIdentityPoolNamespace)
	out.SetGroupVersionKind(krm.IAMWorkloadIdentityPoolNamespaceGVK)
	return out, nil
}

func (a *namespaceAdapter) wait(ctx context.Context, op *api.Operation) error {
	return waitOperation(ctx, op, func(name string) (*api.Operation, error) {
		return a.namespaces().Operations.Get(name).Context(ctx).Do()
	})
}

// waitOperation polls a long-running operation until it is done.
func waitOperation(ctx context.Context, op *api.Operation, get func(name string) (*api.Operation, error)) error {
	for !op.Done {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		next, err := get(op.Name)
		if err != nil {
			return fmt.Errorf("waiting for operation %s: %w", op.Name, err)
		}
		op = next
	}
	if op.Error != nil {
		return fmt.Errorf("operation %s failed: %s (code %d)", op.Name, op.Error.Message, op.Error.Code)
	}
	return nil
}

func (a *namespaceAdapter) readAndUpdateStatus(ctx context.Context, op directbase.Operation) error {
	actual, err := a.namespaces().Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("reading IAMWorkloadIdentityPoolNamespace %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, op, actual)
}

func (a *namespaceAdapter) updateStatus(ctx context.Context, op directbase.Operation, actual *api.WorkloadIdentityPoolNamespace) error {
	status := &krm.IAMWorkloadIdentityPoolNamespaceStatus{
		ExternalRef: direct.LazyPtr(a.id.String()),
		ObservedState: &krm.IAMWorkloadIdentityPoolNamespaceObservedState{
			State:      direct.LazyPtr(actual.State),
			ExpireTime: direct.LazyPtr(actual.ExpireTime),
		},
	}
	if actual.OwnerService != nil {
		status.ObservedState.OwnerService = &krm.OwnerServiceObservedState{PrincipalSubject: direct.LazyPtr(actual.OwnerService.PrincipalSubject)}
	}
	return op.UpdateStatus(ctx, status, nil)
}
