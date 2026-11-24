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

// Written here: ComputeNetworkFields and ComputeSubnetworkFields, field
// kinds (docs/design.md section 6.6, pattern 2; decision 37). No Config
// Connector code or pull request sets a network's BGP best path selection
// or a subnet's allowSubnetCidrRoutesOverlap. Each resource is read with
// get and written with patch, a JSON merge patch whose body names only the
// owned fields that differ (and, for a subnet, the fingerprint just read),
// so nothing else is touched. Config Connector's ComputeNetwork and
// ComputeSubnetwork run Terraform, whose updates send only the fields that
// changed, none of these: a network's routingConfig with routingMode alone
// (merged, so the BGP fields stay), a subnet's fields one patch at a time
// (section 6.6).

import (
	"context"
	"fmt"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewNetworkFieldsModel is the ComputeNetworkFields model.
func NewNetworkFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &networkFieldsModel{config: *config}, nil
}

type networkFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &networkFieldsModel{}

func (m *networkFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeNetworkFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	if obj.Spec.RoutingConfig == nil {
		return nil, fmt.Errorf("spec.routingConfig is required")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if fieldkind.Deleting(obj, prev) {
		return fieldkind.Gone{}, nil
	}
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the network is in status; Config Connector's
		// ComputeNetwork may be gone already.
		ref = &computerefs.ComputeNetworkRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &computerefs.ComputeNetworkIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the network of ComputeNetworkFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.ComputeNetworkFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewNetworksRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Networks client: %w", err)
	}
	return &networkFieldsAdapter{id: id, gcpClient: c, desired: obj.Spec.RoutingConfig}, nil
}

func (m *networkFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type networkFieldsAdapter struct {
	id        *computerefs.ComputeNetworkIdentity
	gcpClient *gcp.NetworksClient
	desired   *krm.NetworkRoutingConfig
	actual    *pb.Network
}

var _ directbase.Adapter = &networkFieldsAdapter{}

// Find reports whether the network exists. A missing one is "not found" so
// that a delete finishes when it is gone; a create then waits (Create).
func (a *networkFieldsAdapter) Find(ctx context.Context) (bool, error) {
	n, err := a.gcpClient.Get(ctx, &pb.GetNetworkRequest{Project: a.id.Project, Network: a.id.Network})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting network %s: %w", a.id, err)
	}
	a.actual = n
	return true, nil
}

// Create is called only while the network does not exist: this kind never
// makes it, so it waits for it.
func (a *networkFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("ComputeNetwork", a.id.String())
}

// patchBody is the owned routingConfig fields that differ from the
// network's, or nil when none does. The mode is sent with the other two
// whenever the spec sets it, as the API checks them against it.
func (a *networkFieldsAdapter) patchBody() *pb.NetworkRoutingConfig {
	actual := a.actual.GetRoutingConfig()
	rc := &pb.NetworkRoutingConfig{}
	changed := false
	if want := a.desired.BgpBestPathSelectionMode; want != nil && actual.GetBgpBestPathSelectionMode() != *want {
		changed = true
	}
	if want := a.desired.BgpAlwaysCompareMed; want != nil && (actual.BgpAlwaysCompareMed == nil || actual.GetBgpAlwaysCompareMed() != *want) {
		rc.BgpAlwaysCompareMed = direct.PtrTo(*want)
		changed = true
	}
	if want := a.desired.BgpInterRegionCost; want != nil && actual.GetBgpInterRegionCost() != *want {
		rc.BgpInterRegionCost = direct.PtrTo(*want)
		changed = true
	}
	if !changed {
		return nil
	}
	if want := a.desired.BgpBestPathSelectionMode; want != nil {
		rc.BgpBestPathSelectionMode = direct.PtrTo(*want)
	}
	return rc
}

func (a *networkFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	rc := a.patchBody()
	if rc == nil {
		return a.updateStatus(ctx, updateOp)
	}
	klog.FromContext(ctx).V(2).Info("setting the BGP best path selection of a network", "name", a.id)
	op, err := a.gcpClient.Patch(ctx, &pb.PatchNetworkRequest{
		Project: a.id.Project, Network: a.id.Network,
		NetworkResource: &pb.Network{RoutingConfig: rc},
	})
	if err == nil && !op.Done() {
		err = op.Wait(ctx)
	}
	if err != nil {
		return fmt.Errorf("setting the routingConfig of network %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, updateOp)
}

// Delete leaves the fields as they are: putting the network back to
// LEGACY best path selection would change how its Cloud Routers choose
// routes, which deleting this object should not do. The network itself
// is never deleted. A network that is gone is not reached: Find said not
// found.
func (a *networkFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	return true, nil
}

func (a *networkFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *networkFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.ComputeNetworkFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}

// NewSubnetworkFieldsModel is the ComputeSubnetworkFields model.
func NewSubnetworkFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &subnetworkFieldsModel{config: *config}, nil
}

type subnetworkFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &subnetworkFieldsModel{}

func (m *subnetworkFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeSubnetworkFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	if obj.Spec.AllowSubnetCidrRoutesOverlap == nil {
		return nil, fmt.Errorf("spec.allowSubnetCidrRoutesOverlap is required")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if fieldkind.Deleting(obj, prev) {
		return fieldkind.Gone{}, nil
	}
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the subnet is in status; Config Connector's
		// ComputeSubnetwork may be gone already.
		ref = &computev1beta1.ComputeSubnetworkRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &computev1beta1.ComputeSubnetworkIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the subnet of ComputeSubnetworkFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.ComputeSubnetworkFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewSubnetworksRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Subnetworks client: %w", err)
	}
	return &subnetworkFieldsAdapter{id: id, gcpClient: c, desired: *obj.Spec.AllowSubnetCidrRoutesOverlap}, nil
}

func (m *subnetworkFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type subnetworkFieldsAdapter struct {
	id        *computev1beta1.ComputeSubnetworkIdentity
	gcpClient *gcp.SubnetworksClient
	desired   bool
	actual    *pb.Subnetwork
}

var _ directbase.Adapter = &subnetworkFieldsAdapter{}

// Find reports whether the subnet exists. A missing one is "not found" so
// that a delete finishes when it is gone; a create then waits (Create).
func (a *subnetworkFieldsAdapter) Find(ctx context.Context) (bool, error) {
	s, err := a.gcpClient.Get(ctx, &pb.GetSubnetworkRequest{Project: a.id.Project, Region: a.id.Region, Subnetwork: a.id.Subnetwork})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting subnet %s: %w", a.id, err)
	}
	a.actual = s
	return true, nil
}

// Create is called only while the subnet does not exist: this kind never
// makes it, so it waits for it.
func (a *subnetworkFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("ComputeSubnetwork", a.id.String())
}

func (a *subnetworkFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if a.actual.GetAllowSubnetCidrRoutesOverlap() == a.desired {
		return a.updateStatus(ctx, updateOp)
	}
	klog.FromContext(ctx).V(2).Info("setting allowSubnetCidrRoutesOverlap of a subnet", "name", a.id)
	op, err := a.gcpClient.Patch(ctx, &pb.PatchSubnetworkRequest{
		Project: a.id.Project, Region: a.id.Region, Subnetwork: a.id.Subnetwork,
		SubnetworkResource: &pb.Subnetwork{
			AllowSubnetCidrRoutesOverlap: direct.PtrTo(a.desired),
			Fingerprint:                  a.actual.Fingerprint,
		},
	})
	if err == nil && !op.Done() {
		err = op.Wait(ctx)
	}
	if err != nil {
		return fmt.Errorf("setting allowSubnetCidrRoutesOverlap of subnet %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, updateOp)
}

// Delete leaves the field as it is: turning the overlap off would make the
// API refuse a route already relying on it. The subnet itself is never
// deleted. A subnet that is gone is not reached: Find said not found.
func (a *subnetworkFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	return true, nil
}

func (a *subnetworkFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *subnetworkFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.ComputeSubnetworkFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}
