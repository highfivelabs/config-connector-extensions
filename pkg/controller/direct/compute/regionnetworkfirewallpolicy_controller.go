// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/compute/computenetworkfirewallpolicy_controller.go and
// computenetworkfirewallpolicy_mapper.go (UPSTREAM.md). The regional client
// (RegionNetworkFirewallPolicies) and the region in every request; our KRM
// types. Written here: the spec mapping (description only; upstream's is
// generated), and an unset description is sent as "" on update so that
// removing it from the spec clears it, as Terraform's
// google_compute_region_network_firewall_policy does (read, not copied).

package compute

import (
	"context"
	"fmt"
	"strconv"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
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
	registry.RegisterModel(krm.ComputeRegionNetworkFirewallPolicyGVK, NewRegionNetworkFirewallPolicyModel)
}

func NewRegionNetworkFirewallPolicyModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &regionNetworkFirewallPolicyModel{config: *config}, nil
}

type regionNetworkFirewallPolicyModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &regionNetworkFirewallPolicyModel{}

func (m *regionNetworkFirewallPolicyModel) client(ctx context.Context) (*gcp.RegionNetworkFirewallPoliciesClient, error) {
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

func (m *regionNetworkFirewallPolicyModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeRegionNetworkFirewallPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	if err := common.NormalizeReferences(ctx, op.Reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeRegionNetworkFirewallPolicyIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	return &regionNetworkFirewallPolicyAdapter{
		gcpClient: gcpClient,
		id:        id,
		desired:   obj,
	}, nil
}

func (m *regionNetworkFirewallPolicyModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	id := &krm.ComputeRegionNetworkFirewallPolicyIdentity{}
	if err := id.FromExternal(url); err != nil {
		return nil, nil
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	return &regionNetworkFirewallPolicyAdapter{
		gcpClient: gcpClient,
		id:        id,
	}, nil
}

type regionNetworkFirewallPolicyAdapter struct {
	gcpClient *gcp.RegionNetworkFirewallPoliciesClient
	id        *krm.ComputeRegionNetworkFirewallPolicyIdentity
	desired   *krm.ComputeRegionNetworkFirewallPolicy
	actual    *pb.FirewallPolicy
}

var _ directbase.Adapter = &regionNetworkFirewallPolicyAdapter{}

func (a *regionNetworkFirewallPolicyAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ComputeRegionNetworkFirewallPolicy", "name", a.id.String())

	actual, err := a.get(ctx)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeRegionNetworkFirewallPolicy %q: %w", a.id.String(), err)
	}

	a.actual = actual
	return true, nil
}

func (a *regionNetworkFirewallPolicyAdapter) get(ctx context.Context) (*pb.FirewallPolicy, error) {
	req := &pb.GetRegionNetworkFirewallPolicyRequest{
		Project:        a.id.Project,
		Region:         a.id.Region,
		FirewallPolicy: a.id.FirewallPolicy,
	}
	return a.gcpClient.Get(ctx, req)
}

func (a *regionNetworkFirewallPolicyAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ComputeRegionNetworkFirewallPolicy", "name", a.id.String())

	firewallPolicy := regionNetworkFirewallPolicySpecToProto(&a.desired.Spec)
	firewallPolicy.Name = direct.PtrTo(a.id.FirewallPolicy)

	req := &pb.InsertRegionNetworkFirewallPolicyRequest{
		Project:                a.id.Project,
		Region:                 a.id.Region,
		FirewallPolicyResource: firewallPolicy,
	}

	op, err := a.gcpClient.Insert(ctx, req)
	if err != nil {
		return fmt.Errorf("creating ComputeRegionNetworkFirewallPolicy %s: %w", a.id.String(), err)
	}

	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting ComputeRegionNetworkFirewallPolicy %s create failed: %w", a.id.String(), err)
		}
	}
	log.V(2).Info("successfully created ComputeRegionNetworkFirewallPolicy", "name", a.id.String())

	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeRegionNetworkFirewallPolicy %s: %w", a.id.String(), err)
	}

	return a.updateStatus(ctx, createOp, created)
}

func (a *regionNetworkFirewallPolicyAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating ComputeRegionNetworkFirewallPolicy", "name", a.id.String())

	desired := regionNetworkFirewallPolicySpecToProto(&a.desired.Spec)

	// Description is the only field that changes in place.
	if desired.GetDescription() == a.actual.GetDescription() {
		log.V(2).Info("no diff detected for ComputeRegionNetworkFirewallPolicy", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}

	resourceToPatch := &pb.FirewallPolicy{
		Description: direct.PtrTo(desired.GetDescription()),
		Fingerprint: a.actual.Fingerprint,
	}

	req := &pb.PatchRegionNetworkFirewallPolicyRequest{
		Project:                a.id.Project,
		Region:                 a.id.Region,
		FirewallPolicy:         a.id.FirewallPolicy,
		FirewallPolicyResource: resourceToPatch,
	}

	op, err := a.gcpClient.Patch(ctx, req)
	if err != nil {
		return fmt.Errorf("updating ComputeRegionNetworkFirewallPolicy %s: %w", a.id.String(), err)
	}

	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for ComputeRegionNetworkFirewallPolicy %s update: %w", a.id.String(), err)
		}
	}
	log.V(2).Info("successfully updated ComputeRegionNetworkFirewallPolicy", "name", a.id.String())

	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeRegionNetworkFirewallPolicy %s: %w", a.id.String(), err)
	}

	return a.updateStatus(ctx, updateOp, updated)
}

func (a *regionNetworkFirewallPolicyAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("ComputeRegionNetworkFirewallPolicy %s not found", a.id.String())
	}

	obj := &krm.ComputeRegionNetworkFirewallPolicy{}
	obj.Spec.Description = a.actual.Description
	obj.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: a.id.Project}
	obj.Spec.Region = a.id.Region
	obj.Spec.ResourceID = direct.PtrTo(a.id.FirewallPolicy)
	obj.Name = a.id.FirewallPolicy
	obj.SetGroupVersionKind(krm.ComputeRegionNetworkFirewallPolicyGVK)

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("converting to unstructured: %w", err)
	}
	return &unstructured.Unstructured{Object: u}, nil
}

func (a *regionNetworkFirewallPolicyAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ComputeRegionNetworkFirewallPolicy", "name", a.id.String())

	req := &pb.DeleteRegionNetworkFirewallPolicyRequest{
		Project:        a.id.Project,
		Region:         a.id.Region,
		FirewallPolicy: a.id.FirewallPolicy,
	}

	op, err := a.gcpClient.Delete(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeRegionNetworkFirewallPolicy %s: %w", a.id.String(), err)
	}

	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			if direct.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("waiting for deletion of ComputeRegionNetworkFirewallPolicy %s: %w", a.id.String(), err)
		}
	}
	log.V(2).Info("successfully deleted ComputeRegionNetworkFirewallPolicy", "name", a.id.String())

	return true, nil
}

func (a *regionNetworkFirewallPolicyAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.FirewallPolicy) error {
	return op.UpdateStatus(ctx, regionNetworkFirewallPolicyStatusFromProto(latest), nil)
}

// regionNetworkFirewallPolicySpecToProto is written here: the spec's one
// API field.
func regionNetworkFirewallPolicySpecToProto(in *krm.ComputeRegionNetworkFirewallPolicySpec) *pb.FirewallPolicy {
	return &pb.FirewallPolicy{Description: in.Description}
}

// regionNetworkFirewallPolicyStatusFromProto is upstream's
// ComputeNetworkFirewallPolicyStatus_v1beta1_FromProto for our status.
func regionNetworkFirewallPolicyStatusFromProto(in *pb.FirewallPolicy) *krm.ComputeRegionNetworkFirewallPolicyStatus {
	out := &krm.ComputeRegionNetworkFirewallPolicyStatus{}
	out.CreationTimestamp = in.CreationTimestamp
	out.Fingerprint = in.Fingerprint
	if in.Id != nil {
		idStr := strconv.FormatUint(*in.Id, 10)
		out.RegionNetworkFirewallPolicyId = &idStr
	}
	if in.RuleTupleCount != nil {
		count := int64(*in.RuleTupleCount)
		out.RuleTupleCount = &count
	}
	out.SelfLink = in.SelfLink
	out.SelfLinkWithId = in.SelfLinkWithId
	return out
}
