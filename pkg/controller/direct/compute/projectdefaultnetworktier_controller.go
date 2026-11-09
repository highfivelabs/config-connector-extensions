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

package compute

// Written here: a project's default network tier. The controller follows
// Terraform's google_compute_project_default_network_tier (read, not
// copied): the setting always exists, so Find reads the project, create
// and update call projects.setDefaultNetworkTier when the tier differs,
// and delete leaves the tier as it is (there is nothing to delete; a
// project with no object goes on with the last tier set).

import (
	"context"
	"fmt"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeProjectDefaultNetworkTierGVK, NewProjectDefaultNetworkTierModel)
}

func NewProjectDefaultNetworkTierModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &networkTierModel{config: *config}, nil
}

type networkTierModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &networkTierModel{}

func (m *networkTierModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeProjectDefaultNetworkTier{}
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
	c, err := gcp.NewProjectsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building compute projects client: %w", err)
	}
	return &networkTierAdapter{id: id.(*krm.ComputeProjectDefaultNetworkTierIdentity), gcpClient: c, desired: obj}, nil
}

func (m *networkTierModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type networkTierAdapter struct {
	id        *krm.ComputeProjectDefaultNetworkTierIdentity
	gcpClient *gcp.ProjectsClient
	desired   *krm.ComputeProjectDefaultNetworkTier
	actual    string
}

var _ directbase.Adapter = &networkTierAdapter{}

// Find reads the project's default network tier, which always exists.
func (a *networkTierAdapter) Find(ctx context.Context) (bool, error) {
	p, err := a.gcpClient.Get(ctx, &pb.GetProjectRequest{Project: a.id.Project})
	if err != nil {
		return false, fmt.Errorf("reading project %s: %w", a.id.Project, err)
	}
	a.actual = p.GetDefaultNetworkTier()
	return true, nil
}

// Create runs only if Find reports nothing, which it never does; it sets
// the tier as Update does.
func (a *networkTierAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	if err := a.set(ctx); err != nil {
		return err
	}
	return a.updateStatus(ctx, createOp)
}

func (a *networkTierAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if a.actual != a.desired.Spec.NetworkTier {
		if err := a.set(ctx); err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, updateOp)
}

func (a *networkTierAdapter) set(ctx context.Context) error {
	klog.FromContext(ctx).V(2).Info("setting the default network tier", "name", a.id, "from", a.actual, "to", a.desired.Spec.NetworkTier)
	op, err := a.gcpClient.SetDefaultNetworkTier(ctx, &pb.SetDefaultNetworkTierProjectRequest{
		Project: a.id.Project,
		ProjectsSetDefaultNetworkTierRequestResource: &pb.ProjectsSetDefaultNetworkTierRequest{
			NetworkTier: direct.PtrTo(a.desired.Spec.NetworkTier),
		},
	})
	if err == nil {
		err = op.Wait(ctx)
	}
	if err != nil {
		return fmt.Errorf("setting project %s's default network tier to %s: %w", a.id.Project, a.desired.Spec.NetworkTier, err)
	}
	a.actual = a.desired.Spec.NetworkTier
	return nil
}

// Delete leaves the tier as it is, as Terraform does.
func (a *networkTierAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	return true, nil
}

func (a *networkTierAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *networkTierAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.ComputeProjectDefaultNetworkTierStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}
