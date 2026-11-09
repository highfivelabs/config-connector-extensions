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

// Written here: Config Connector has types for
// IAMWorkloadIdentityPoolManagedIdentity only in pull request #14418, and no
// controller. This one follows Terraform's
// google_iam_workload_identity_pool_managed_identity (read, not copied):
// description and disabled change in place; attestation rules are set with
// setAttestationRules after the identity is made and whenever they differ
// (the list is the whole set), and read with listAttestationRules; a
// managed identity in state DELETED counts as absent.

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
	registry.RegisterModel(krm.IAMWorkloadIdentityPoolManagedIdentityGVK, NewWorkloadIdentityPoolManagedIdentityModel)
}

func NewWorkloadIdentityPoolManagedIdentityModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &managedIdentityModel{namespaceModel{config: *config}}, nil
}

type managedIdentityModel struct {
	namespaceModel
}

var _ directbase.Model = &managedIdentityModel{}

func (m *managedIdentityModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.IAMWorkloadIdentityPoolManagedIdentity{}
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
	return &managedIdentityAdapter{id: id.(*krm.IAMWorkloadIdentityPoolManagedIdentityIdentity), svc: svc, desired: obj}, nil
}

func (m *managedIdentityModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type managedIdentityAdapter struct {
	id      *krm.IAMWorkloadIdentityPoolManagedIdentityIdentity
	svc     *api.Service
	desired *krm.IAMWorkloadIdentityPoolManagedIdentity
	actual  *api.WorkloadIdentityPoolManagedIdentity
}

var _ directbase.Adapter = &managedIdentityAdapter{}

func (a *managedIdentityAdapter) identities() *api.ProjectsLocationsWorkloadIdentityPoolsNamespacesManagedIdentitiesService {
	return a.svc.Projects.Locations.WorkloadIdentityPools.Namespaces.ManagedIdentities
}

func (a *managedIdentityAdapter) wait(ctx context.Context, op *api.Operation) error {
	return waitOperation(ctx, op, func(name string) (*api.Operation, error) {
		return a.identities().Operations.Get(name).Context(ctx).Do()
	})
}

func (a *managedIdentityAdapter) Find(ctx context.Context) (bool, error) {
	actual, err := a.identities().Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
	}
	if actual.State == "DELETED" {
		return false, nil
	}
	a.actual = actual
	return true, nil
}

func (a *managedIdentityAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating IAMWorkloadIdentityPoolManagedIdentity", "name", a.id)
	desired := &api.WorkloadIdentityPoolManagedIdentity{
		Description: direct.ValueOf(a.desired.Spec.Description),
		Disabled:    direct.ValueOf(a.desired.Spec.Disabled),
	}
	op, err := a.identities().Create(a.id.ParentString(), desired).WorkloadIdentityPoolManagedIdentityId(a.id.WorkloadIdentityPoolManagedIdentity).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return fmt.Errorf("creating IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
	}
	if len(a.desired.Spec.AttestationRules) > 0 {
		if err := a.setAttestationRules(ctx); err != nil {
			return err
		}
	}
	return a.readAndUpdateStatus(ctx, createOp)
}

func (a *managedIdentityAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	var mask []string
	if a.desired.Spec.Description != nil && direct.ValueOf(a.desired.Spec.Description) != a.actual.Description {
		mask = append(mask, "description")
	}
	if a.desired.Spec.Disabled != nil && direct.ValueOf(a.desired.Spec.Disabled) != a.actual.Disabled {
		mask = append(mask, "disabled")
	}
	if len(mask) > 0 {
		klog.FromContext(ctx).V(2).Info("updating IAMWorkloadIdentityPoolManagedIdentity", "name", a.id, "mask", mask)
		desired := &api.WorkloadIdentityPoolManagedIdentity{
			Description:     direct.ValueOf(a.desired.Spec.Description),
			Disabled:        direct.ValueOf(a.desired.Spec.Disabled),
			ForceSendFields: []string{"Description", "Disabled"},
		}
		op, err := a.identities().Patch(a.id.String(), desired).UpdateMask(strings.Join(mask, ",")).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("updating IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
		}
		if err := a.wait(ctx, op); err != nil {
			return fmt.Errorf("updating IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
		}
	}
	// nil leaves the rules unmanaged; an empty list removes them all.
	if a.desired.Spec.AttestationRules != nil {
		actual, err := a.listAttestationRules(ctx)
		if err != nil {
			return err
		}
		if !slices.Equal(actual, desiredRules(a.desired.Spec.AttestationRules)) {
			if err := a.setAttestationRules(ctx); err != nil {
				return err
			}
		}
	}
	return a.readAndUpdateStatus(ctx, updateOp)
}

func (a *managedIdentityAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting IAMWorkloadIdentityPoolManagedIdentity", "name", a.id)
	op, err := a.identities().Delete(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return false, fmt.Errorf("deleting IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
	}
	return true, nil
}

func (a *managedIdentityAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	obj := &krm.IAMWorkloadIdentityPoolManagedIdentity{}
	obj.Spec.Location = direct.LazyPtr(a.id.Location)
	obj.Spec.ResourceID = direct.LazyPtr(a.id.WorkloadIdentityPoolManagedIdentity)
	obj.Spec.WorkloadIdentityPoolNamespaceRef = &krm.IAMWorkloadIdentityPoolNamespaceRef{External: a.id.ParentString()}
	obj.Spec.Description = direct.LazyPtr(a.actual.Description)
	obj.Spec.Disabled = direct.LazyPtr(a.actual.Disabled)
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	out := &unstructured.Unstructured{Object: u}
	out.SetName(a.id.WorkloadIdentityPoolManagedIdentity)
	out.SetGroupVersionKind(krm.IAMWorkloadIdentityPoolManagedIdentityGVK)
	return out, nil
}

// desiredRules is the spec's rules as a sorted set of resources.
func desiredRules(rules []krm.AttestationRule) []string {
	out := []string{}
	for _, r := range rules {
		out = append(out, r.GoogleCloudResource)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func (a *managedIdentityAdapter) listAttestationRules(ctx context.Context) ([]string, error) {
	out := []string{}
	call := a.identities().ListAttestationRules(a.id.String())
	if err := call.Pages(ctx, func(page *api.ListAttestationRulesResponse) error {
		for _, r := range page.AttestationRules {
			out = append(out, r.GoogleCloudResource)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("listing attestation rules of %s: %w", a.id, err)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

func (a *managedIdentityAdapter) setAttestationRules(ctx context.Context) error {
	req := &api.SetAttestationRulesRequest{AttestationRules: []*api.AttestationRule{}, ForceSendFields: []string{"AttestationRules"}}
	for _, r := range desiredRules(a.desired.Spec.AttestationRules) {
		req.AttestationRules = append(req.AttestationRules, &api.AttestationRule{GoogleCloudResource: r})
	}
	klog.FromContext(ctx).V(2).Info("setting attestation rules", "name", a.id, "rules", len(req.AttestationRules))
	op, err := a.identities().SetAttestationRules(a.id.String(), req).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("setting attestation rules of %s: %w", a.id, err)
	}
	if err := a.wait(ctx, op); err != nil {
		return fmt.Errorf("setting attestation rules of %s: %w", a.id, err)
	}
	return nil
}

func (a *managedIdentityAdapter) readAndUpdateStatus(ctx context.Context, op directbase.Operation) error {
	actual, err := a.identities().Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("reading IAMWorkloadIdentityPoolManagedIdentity %s: %w", a.id, err)
	}
	status := &krm.IAMWorkloadIdentityPoolManagedIdentityStatus{
		ExternalRef: direct.LazyPtr(a.id.String()),
		ObservedState: &krm.IAMWorkloadIdentityPoolManagedIdentityObservedState{
			State:      direct.LazyPtr(actual.State),
			ExpireTime: direct.LazyPtr(actual.ExpireTime),
		},
	}
	return op.UpdateStatus(ctx, status, nil)
}
