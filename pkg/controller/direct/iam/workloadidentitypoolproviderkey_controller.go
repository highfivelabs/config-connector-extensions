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

package iam

// Written here: no Config Connector code, pull request or Terraform
// resource has workload identity pool provider keys. This controller follows
// the IAM v1 API, and Terraform's workforce-pool analogue
// (google_iam_workforce_pool_provider_key, read, not copied) in treating
// every field as immutable: a change is refused, not applied. A key in state
// DELETED counts as absent, as for namespaces.

import (
	"context"
	"fmt"

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
	registry.RegisterModel(krm.IAMWorkloadIdentityPoolProviderKeyGVK, NewWorkloadIdentityPoolProviderKeyModel)
}

func NewWorkloadIdentityPoolProviderKeyModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &providerKeyModel{namespaceModel{config: *config}}, nil
}

type providerKeyModel struct {
	namespaceModel
}

var _ directbase.Model = &providerKeyModel{}

func (m *providerKeyModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.IAMWorkloadIdentityPoolProviderKey{}
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
	return &providerKeyAdapter{id: id.(*krm.IAMWorkloadIdentityPoolProviderKeyIdentity), svc: svc, desired: obj}, nil
}

func (m *providerKeyModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type providerKeyAdapter struct {
	id      *krm.IAMWorkloadIdentityPoolProviderKeyIdentity
	svc     *api.Service
	desired *krm.IAMWorkloadIdentityPoolProviderKey
	actual  *api.WorkloadIdentityPoolProviderKey
}

var _ directbase.Adapter = &providerKeyAdapter{}

func (a *providerKeyAdapter) keys() *api.ProjectsLocationsWorkloadIdentityPoolsProvidersKeysService {
	return a.svc.Projects.Locations.WorkloadIdentityPools.Providers.Keys
}

func (a *providerKeyAdapter) wait(ctx context.Context, op *api.Operation) error {
	return waitOperation(ctx, op, func(name string) (*api.Operation, error) {
		return a.keys().Operations.Get(name).Context(ctx).Do()
	})
}

func (a *providerKeyAdapter) keySpec() string {
	if a.desired.Spec.KeyData == nil {
		return ""
	}
	return a.desired.Spec.KeyData.KeySpec
}

func (a *providerKeyAdapter) Find(ctx context.Context) (bool, error) {
	actual, err := a.keys().Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting IAMWorkloadIdentityPoolProviderKey %s: %w", a.id, err)
	}
	if actual.State == "DELETED" {
		return false, nil
	}
	a.actual = actual
	return true, nil
}

func (a *providerKeyAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating IAMWorkloadIdentityPoolProviderKey", "name", a.id)
	desired := &api.WorkloadIdentityPoolProviderKey{
		Use:     a.desired.Spec.Use,
		KeyData: &api.KeyData{KeySpec: a.keySpec()},
	}
	op, err := a.keys().Create(a.id.ParentString(), desired).WorkloadIdentityPoolProviderKeyId(a.id.Key).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating IAMWorkloadIdentityPoolProviderKey %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return fmt.Errorf("creating IAMWorkloadIdentityPoolProviderKey %s: %w", a.id, err)
	}
	actual, err := a.keys().Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("reading IAMWorkloadIdentityPoolProviderKey %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, createOp, actual)
}

func (a *providerKeyAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if a.actual.Use != a.desired.Spec.Use {
		return fmt.Errorf("IAMWorkloadIdentityPoolProviderKey %s: use is immutable (is %q, spec asks %q); make a new key instead", a.id, a.actual.Use, a.desired.Spec.Use)
	}
	if a.actual.KeyData != nil && a.actual.KeyData.KeySpec != a.keySpec() {
		return fmt.Errorf("IAMWorkloadIdentityPoolProviderKey %s: keyData.keySpec is immutable (is %q, spec asks %q); make a new key instead", a.id, a.actual.KeyData.KeySpec, a.keySpec())
	}
	return a.updateStatus(ctx, updateOp, a.actual)
}

func (a *providerKeyAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting IAMWorkloadIdentityPoolProviderKey", "name", a.id)
	op, err := a.keys().Delete(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting IAMWorkloadIdentityPoolProviderKey %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return false, fmt.Errorf("deleting IAMWorkloadIdentityPoolProviderKey %s: %w", a.id, err)
	}
	return true, nil
}

func (a *providerKeyAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *providerKeyAdapter) updateStatus(ctx context.Context, op directbase.Operation, actual *api.WorkloadIdentityPoolProviderKey) error {
	status := &krm.IAMWorkloadIdentityPoolProviderKeyStatus{
		ExternalRef: direct.LazyPtr(a.id.String()),
		ObservedState: &krm.IAMWorkloadIdentityPoolProviderKeyObservedState{
			State:      direct.LazyPtr(actual.State),
			ExpireTime: direct.LazyPtr(actual.ExpireTime),
		},
	}
	if kd := actual.KeyData; kd != nil {
		status.ObservedState.KeyData = &krm.IAMWorkloadIdentityPoolProviderKeyKeyDataObservedState{
			Format:        direct.LazyPtr(kd.Format),
			Key:           direct.LazyPtr(kd.Key),
			NotBeforeTime: direct.LazyPtr(kd.NotBeforeTime),
			NotAfterTime:  direct.LazyPtr(kd.NotAfterTime),
		}
	}
	return op.UpdateStatus(ctx, status, nil)
}
