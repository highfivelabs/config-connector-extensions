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

// Package compute has the direct controllers for this project's Compute
// Engine kinds.
//
// Written here: no Config Connector code or pull request has a regional
// network endpoint. This controller follows Terraform's
// google_compute_region_network_endpoint (read, not copied): an endpoint is
// attached with attachNetworkEndpoints, found by listing the group's
// endpoints and matching its address and port, and detached with
// detachNetworkEndpoints; every field is immutable.
package compute

import (
	"context"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/api/iterator"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeRegionNetworkEndpointGVK, NewRegionNetworkEndpointModel)
}

func NewRegionNetworkEndpointModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &endpointModel{config: *config}, nil
}

type endpointModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &endpointModel{}

func (m *endpointModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeRegionNetworkEndpoint{}
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
	c, err := gcp.NewRegionNetworkEndpointGroupsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building region network endpoint groups client: %w", err)
	}
	return &endpointAdapter{id: id.(*krm.ComputeRegionNetworkEndpointIdentity), gcpClient: c, desired: obj}, nil
}

func (m *endpointModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type endpointAdapter struct {
	id        *krm.ComputeRegionNetworkEndpointIdentity
	gcpClient *gcp.RegionNetworkEndpointGroupsClient
	desired   *krm.ComputeRegionNetworkEndpoint
	actual    *pb.NetworkEndpoint
}

var _ directbase.Adapter = &endpointAdapter{}

// endpoint is the desired endpoint as the API takes it.
func (a *endpointAdapter) endpoint() *pb.NetworkEndpoint {
	s := a.desired.Spec
	return &pb.NetworkEndpoint{
		Port:                  direct.PtrTo(s.Port),
		IpAddress:             s.IPAddress,
		Fqdn:                  s.FQDN,
		ClientDestinationPort: s.ClientDestinationPort,
		Instance:              s.Instance,
	}
}

// matches is Terraform's test: same IP address, domain name and port.
func matches(got, want *pb.NetworkEndpoint) bool {
	return got.GetIpAddress() == want.GetIpAddress() && got.GetFqdn() == want.GetFqdn() && got.GetPort() == want.GetPort()
}

func (a *endpointAdapter) Find(ctx context.Context) (bool, error) {
	it := a.gcpClient.ListNetworkEndpoints(ctx, &pb.ListNetworkEndpointsRegionNetworkEndpointGroupsRequest{
		Project:              a.id.Project,
		Region:               a.id.Region,
		NetworkEndpointGroup: a.id.Group,
	})
	want := a.endpoint()
	for {
		e, err := it.Next()
		if err == iterator.Done {
			return false, nil
		}
		if err != nil {
			if direct.IsNotFound(err) {
				return false, fmt.Errorf("the network endpoint group %s does not exist", a.id.GroupName())
			}
			return false, fmt.Errorf("listing the endpoints of %s: %w", a.id.GroupName(), err)
		}
		if matches(e.GetNetworkEndpoint(), want) {
			a.actual = e.GetNetworkEndpoint()
			return true, nil
		}
	}
}

func (a *endpointAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("attaching ComputeRegionNetworkEndpoint", "name", a.id)
	op, err := a.gcpClient.AttachNetworkEndpoints(ctx, &pb.AttachNetworkEndpointsRegionNetworkEndpointGroupRequest{
		Project:              a.id.Project,
		Region:               a.id.Region,
		NetworkEndpointGroup: a.id.Group,
		RegionNetworkEndpointGroupsAttachEndpointsRequestResource: &pb.RegionNetworkEndpointGroupsAttachEndpointsRequest{
			NetworkEndpoints: []*pb.NetworkEndpoint{a.endpoint()},
		},
	})
	if err != nil {
		return fmt.Errorf("attaching ComputeRegionNetworkEndpoint %s: %w", a.id, err)
	}
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("attaching ComputeRegionNetworkEndpoint %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, createOp)
}

func (a *endpointAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	// The address and port are the identity; the rest cannot change either.
	want := a.endpoint()
	if a.actual.GetClientDestinationPort() != want.GetClientDestinationPort() {
		return fmt.Errorf("ComputeRegionNetworkEndpoint %s: clientDestinationPort is immutable (is %d, spec asks %d)", a.id, a.actual.GetClientDestinationPort(), want.GetClientDestinationPort())
	}
	if got, w := lastComponent(a.actual.GetInstance()), lastComponent(want.GetInstance()); got != w {
		return fmt.Errorf("ComputeRegionNetworkEndpoint %s: instance is immutable (is %q, spec asks %q)", a.id, got, w)
	}
	return a.updateStatus(ctx, updateOp)
}

func lastComponent(s string) string {
	return s[strings.LastIndex(s, "/")+1:]
}

func (a *endpointAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("detaching ComputeRegionNetworkEndpoint", "name", a.id)
	op, err := a.gcpClient.DetachNetworkEndpoints(ctx, &pb.DetachNetworkEndpointsRegionNetworkEndpointGroupRequest{
		Project:              a.id.Project,
		Region:               a.id.Region,
		NetworkEndpointGroup: a.id.Group,
		RegionNetworkEndpointGroupsDetachEndpointsRequestResource: &pb.RegionNetworkEndpointGroupsDetachEndpointsRequest{
			NetworkEndpoints: []*pb.NetworkEndpoint{a.endpoint()},
		},
	})
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("detaching ComputeRegionNetworkEndpoint %s: %w", a.id, err)
	}
	if err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("detaching ComputeRegionNetworkEndpoint %s: %w", a.id, err)
	}
	return true, nil
}

func (a *endpointAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *endpointAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.ComputeRegionNetworkEndpointStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}
