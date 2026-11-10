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

// Written here: Config Connector's ComputeVPNTunnel is Terraform-based at the
// pin, with direct types and a mapper (vpntunnel_mapper.go) but no direct
// controller. In the shape of our other compute controllers. A VPN tunnel
// changes in place only in its labels (vpnTunnels.setLabels); every other
// field, cipherSuite included, is taken only by vpnTunnels.insert, so the
// kind is insert, get, setLabels and delete, and the immutable fields are
// CEL rules (docs/design.md, "Field gaps on large Terraform-based kinds").

import (
	"context"
	"fmt"
	"maps"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func init() {
	registry.RegisterModel(krm.ComputeVPNTunnelGVK, NewVPNTunnelModel)
}

func NewVPNTunnelModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &vpnTunnelModel{config: *config}, nil
}

type vpnTunnelModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &vpnTunnelModel{}

func (m *vpnTunnelModel) client(ctx context.Context) (*gcp.VpnTunnelsClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewVpnTunnelsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building VpnTunnels client: %w", err)
	}
	return c, nil
}

func (m *vpnTunnelModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeVPNTunnel{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeVPNTunnelIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	ns := obj.GetNamespace()
	spec := &obj.Spec
	if r := spec.VpnGatewayRef; r != nil {
		if r.External, err = configConnectorSelfLink(ctx, op.Reader, ns, "ComputeVPNGateway", r.External, r.Name, r.Namespace); err != nil {
			return nil, fmt.Errorf("spec.vpnGatewayRef: %w", err)
		}
	}
	if r := spec.TargetVPNGatewayRef; r != nil {
		if r.External, err = configConnectorSelfLink(ctx, op.Reader, ns, "ComputeTargetVPNGateway", r.External, r.Name, r.Namespace); err != nil {
			return nil, fmt.Errorf("spec.targetVPNGatewayRef: %w", err)
		}
	}
	if r := spec.PeerGCPGatewayRef; r != nil {
		if r.External, err = configConnectorSelfLink(ctx, op.Reader, ns, "ComputeVPNGateway", r.External, r.Name, r.Namespace); err != nil {
			return nil, fmt.Errorf("spec.peerGCPGatewayRef: %w", err)
		}
	}
	if r := spec.PeerExternalGatewayRef; r != nil {
		if r.External, err = configConnectorSelfLink(ctx, op.Reader, ns, "ComputeExternalVPNGateway", r.External, r.Name, r.Namespace); err != nil {
			return nil, fmt.Errorf("spec.peerExternalGatewayRef: %w", err)
		}
	}
	if r := spec.RouterRef; r != nil {
		project, region, router, err := r.Resolve(ctx, op.Reader, ns)
		if err != nil {
			return nil, fmt.Errorf("spec.routerRef: %w", err)
		}
		r.External = fmt.Sprintf("projects/%s/regions/%s/routers/%s", project, region, router)
	}
	// The shared secret is read on every reconcile, though only an insert
	// sends it: a missing Secret shows on the object before a create.
	sharedSecret, err := spec.SharedSecret.ReadSecretValue(ctx, "spec.sharedSecret", ns, op.Reader)
	if err != nil {
		return nil, err
	}
	c, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &vpnTunnelAdapter{
		sharedSecret: sharedSecret,
		gcpClient:    c,
		id:           id,
		obj:          obj,
	}, nil
}

func (m *vpnTunnelModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type vpnTunnelAdapter struct {
	gcpClient *gcp.VpnTunnelsClient
	id        *krm.ComputeVPNTunnelIdentity
	obj       *krm.ComputeVPNTunnel
	// sharedSecret is spec.sharedSecret's value, from its Secret.
	sharedSecret *string
	actual       *pb.VpnTunnel
}

var _ directbase.Adapter = &vpnTunnelAdapter{}

func (a *vpnTunnelAdapter) get(ctx context.Context) (*pb.VpnTunnel, error) {
	return a.gcpClient.Get(ctx, &pb.GetVpnTunnelRequest{Project: a.id.Project, Region: a.id.Region, VpnTunnel: a.id.Name})
}

func (a *vpnTunnelAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeVPNTunnel", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeVPNTunnel %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *vpnTunnelAdapter) wait(ctx context.Context, op *gcp.Operation, what string) error {
	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for %s of ComputeVPNTunnel %s: %w", what, a.id.String(), err)
		}
	}
	return nil
}

func (a *vpnTunnelAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeVPNTunnel", "name", a.id.String())
	mapCtx := &direct.MapContext{}
	desired := ComputeVPNTunnelSpec_v1beta1_ToProto(mapCtx, &a.obj.Spec, a.id.Name, a.sharedSecret)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	op, err := a.gcpClient.Insert(ctx, &pb.InsertVpnTunnelRequest{Project: a.id.Project, Region: a.id.Region, VpnTunnelResource: desired})
	if err != nil {
		return fmt.Errorf("creating ComputeVPNTunnel %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "create"); err != nil {
		return err
	}
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeVPNTunnel %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

// Update changes the labels, the only field the API changes in place. The
// immutable fields cannot differ from what the tunnel was made with (CEL);
// a tunnel acquired by name with other values keeps them.
func (a *vpnTunnelAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	if maps.Equal(a.obj.Spec.Labels, a.actual.GetLabels()) {
		log.V(2).Info("no diff detected for ComputeVPNTunnel", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	log.V(2).Info("updating ComputeVPNTunnel labels", "name", a.id.String())
	op, err := a.gcpClient.SetLabels(ctx, &pb.SetLabelsVpnTunnelRequest{
		Project:  a.id.Project,
		Region:   a.id.Region,
		Resource: a.id.Name,
		RegionSetLabelsRequestResource: &pb.RegionSetLabelsRequest{
			Labels:           a.obj.Spec.Labels,
			LabelFingerprint: a.actual.LabelFingerprint,
		},
	})
	if err != nil {
		return fmt.Errorf("updating ComputeVPNTunnel %s labels: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "update"); err != nil {
		return err
	}
	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeVPNTunnel %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, updated)
}

func (a *vpnTunnelAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *vpnTunnelAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeVPNTunnel", "name", a.id.String())
	op, err := a.gcpClient.Delete(ctx, &pb.DeleteVpnTunnelRequest{Project: a.id.Project, Region: a.id.Region, VpnTunnel: a.id.Name})
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeVPNTunnel %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "delete"); err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

func (a *vpnTunnelAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.VpnTunnel) error {
	mapCtx := &direct.MapContext{}
	status := ComputeVPNTunnelStatus_v1beta1_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}
