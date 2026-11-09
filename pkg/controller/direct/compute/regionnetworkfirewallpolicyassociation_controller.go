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

// Written here: no Config Connector code covers a regional network
// firewall policy's association. The controller follows Terraform's
// google_compute_region_network_firewall_policy_association (read, not
// copied) and the Compute Engine API: regionNetworkFirewallPolicies.getAssociation
// reads it by its name in the policy, addAssociation attaches the policy to
// a network, removeAssociation removes it. Nothing about an association
// changes in place, so every field is immutable.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/googleapis/gax-go/v2/apierror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeRegionNetworkFirewallPolicyAssociationGVK, NewRegionNetworkFirewallPolicyAssociationModel)
}

func NewRegionNetworkFirewallPolicyAssociationModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &regionNetworkFirewallPolicyAssociationModel{config: *config}, nil
}

type regionNetworkFirewallPolicyAssociationModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &regionNetworkFirewallPolicyAssociationModel{}

func (m *regionNetworkFirewallPolicyAssociationModel) client(ctx context.Context) (*gcp.RegionNetworkFirewallPoliciesClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewRegionNetworkFirewallPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building RegionNetworkFirewallPolicies client: %w", err)
	}
	return c, nil
}

func (m *regionNetworkFirewallPolicyAssociationModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeRegionNetworkFirewallPolicyAssociation{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeRegionNetworkFirewallPolicyAssociationIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	if err := obj.Spec.NetworkRef.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	network := computeResourcePath(obj.Spec.NetworkRef.External)
	if network == "" {
		return nil, fmt.Errorf("networkRef did not resolve to a network")
	}
	c, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &regionNetworkFirewallPolicyAssociationAdapter{gcpClient: c, id: id, network: network}, nil
}

func (m *regionNetworkFirewallPolicyAssociationModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type regionNetworkFirewallPolicyAssociationAdapter struct {
	gcpClient *gcp.RegionNetworkFirewallPoliciesClient
	id        *krm.ComputeRegionNetworkFirewallPolicyAssociationIdentity
	// network is the network to attach, as projects/{project}/global/networks/{name}.
	network string
	actual  *pb.FirewallPolicyAssociation
}

var _ directbase.Adapter = &regionNetworkFirewallPolicyAssociationAdapter{}

func (a *regionNetworkFirewallPolicyAssociationAdapter) get(ctx context.Context) (*pb.FirewallPolicyAssociation, error) {
	return a.gcpClient.GetAssociation(ctx, &pb.GetAssociationRegionNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		Region:         a.id.Policy.Region,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Name:           direct.PtrTo(a.id.Name),
	})
}

func (a *regionNetworkFirewallPolicyAssociationAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeRegionNetworkFirewallPolicyAssociation", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if regionFirewallPolicyAssociationMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeRegionNetworkFirewallPolicyAssociation %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *regionNetworkFirewallPolicyAssociationAdapter) wait(ctx context.Context, op *gcp.Operation, what string) error {
	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for %s of ComputeRegionNetworkFirewallPolicyAssociation %s: %w", what, a.id.String(), err)
		}
	}
	return nil
}

func (a *regionNetworkFirewallPolicyAssociationAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeRegionNetworkFirewallPolicyAssociation", "name", a.id.String())
	op, err := a.gcpClient.AddAssociation(ctx, &pb.AddAssociationRegionNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		Region:         a.id.Policy.Region,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		FirewallPolicyAssociationResource: &pb.FirewallPolicyAssociation{
			Name:             direct.PtrTo(a.id.Name),
			AttachmentTarget: direct.PtrTo(a.network),
		},
	})
	if err != nil {
		return fmt.Errorf("creating ComputeRegionNetworkFirewallPolicyAssociation %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "create"); err != nil {
		return err
	}
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeRegionNetworkFirewallPolicyAssociation %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

// Update changes nothing: an association has no field that changes in place.
// One that exists with the name but another network is not ours to move
// (Terraform replaces the resource; here the spec cannot change, so it was
// changed out of band, or the name is taken): it is an error, not a silent
// removal of somebody's attachment.
func (a *regionNetworkFirewallPolicyAssociationAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if got := computeResourcePath(a.actual.GetAttachmentTarget()); got != a.network {
		return fmt.Errorf("ComputeRegionNetworkFirewallPolicyAssociation %s is attached to %q, not %q; remove the association or use another name", a.id.String(), got, a.network)
	}
	return a.updateStatus(ctx, updateOp, a.actual)
}

func (a *regionNetworkFirewallPolicyAssociationAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *regionNetworkFirewallPolicyAssociationAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeRegionNetworkFirewallPolicyAssociation", "name", a.id.String())
	op, err := a.gcpClient.RemoveAssociation(ctx, &pb.RemoveAssociationRegionNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		Region:         a.id.Policy.Region,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Name:           direct.PtrTo(a.id.Name),
	})
	if err != nil {
		if regionFirewallPolicyAssociationMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeRegionNetworkFirewallPolicyAssociation %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "delete"); err != nil {
		if regionFirewallPolicyAssociationMissing(err) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

func (a *regionNetworkFirewallPolicyAssociationAdapter) updateStatus(ctx context.Context, op directbase.Operation, _ *pb.FirewallPolicyAssociation) error {
	status := &krm.ComputeRegionNetworkFirewallPolicyAssociationStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	return op.UpdateStatus(ctx, status, nil)
}

// regionFirewallPolicyAssociationMissing reports whether err says the
// association (or its policy) does not exist. A policy without an
// association of that name answers 400 or 404 (a live run settled which,
// docs/live-runs.md), as a policy without a rule at a priority answers 400.
func regionFirewallPolicyAssociationMissing(err error) bool {
	if direct.IsNotFound(err) {
		return true
	}
	var ae *apierror.APIError
	if !errors.As(err, &ae) || ae.HTTPCode() != http.StatusBadRequest {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "association") && (strings.Contains(msg, "does not") || strings.Contains(msg, "not exist") || strings.Contains(msg, "not found"))
}

// computeResourcePath trims a Compute Engine URL to its resource path,
// projects/{project}/...: the API answers an attachment target as a full URL
// and takes it as a path.
func computeResourcePath(s string) string {
	if i := strings.Index(s, "projects/"); i >= 0 {
		return s[i:]
	}
	return s
}
