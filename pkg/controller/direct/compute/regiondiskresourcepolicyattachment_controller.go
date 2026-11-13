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

// Modified by High Five Labs for config-connector-extensions: adapted from
// Config Connector's pkg/controller/direct/compute/computediskresourcepolicyattachment_controller.go
// (UPSTREAM.md), the zonal attachment's controller, for a regional disk
// (upstream's ComputeRegionDiskResourcePolicyAttachment is Terraform-based,
// with no direct controller). Changed: our KRM types (stable,
// compute.gcp.hfl.dev/v1beta1, docs/design.md decision 26); the model is
// returned to our manager, not registered with Config Connector's
// registry; the RegionDisks REST client built from the controller
// config's REST options; regionDisks' get, addResourcePolicies and
// removeResourcePolicies for the disks' ones; the policy in the disk's
// own region; Export names the disk by its full path, not its bare name.

// +tool:controller
// proto.service: google.cloud.compute.v1.RegionDisks
// proto.message: google.cloud.compute.v1.AddResourcePoliciesRegionDiskRequest
// crd.type: ComputeRegionDiskResourcePolicyAttachment
// crd.version: v1beta1

package compute

import (
	"context"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func NewComputeRegionDiskResourcePolicyAttachmentModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &computeRegionDiskResourcePolicyAttachmentModel{config: config}, nil
}

var _ directbase.Model = &computeRegionDiskResourcePolicyAttachmentModel{}

type computeRegionDiskResourcePolicyAttachmentModel struct {
	config *config.ControllerConfig
}

func (m *computeRegionDiskResourcePolicyAttachmentModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.ComputeRegionDiskResourcePolicyAttachment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}
	attachmentId := id.(*krm.ComputeRegionDiskResourcePolicyAttachmentIdentity)

	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	disksClient, err := gcp.NewRegionDisksRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building region disks client: %w", err)
	}

	return &ComputeRegionDiskResourcePolicyAttachmentAdapter{
		disksClient: disksClient,
		id:          attachmentId,
		reader:      reader,
	}, nil
}

func (m *computeRegionDiskResourcePolicyAttachmentModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type ComputeRegionDiskResourcePolicyAttachmentAdapter struct {
	disksClient *gcp.RegionDisksClient
	id          *krm.ComputeRegionDiskResourcePolicyAttachmentIdentity
	actual      string
	reader      client.Reader
}

var _ directbase.Adapter = &ComputeRegionDiskResourcePolicyAttachmentAdapter{}

func (a *ComputeRegionDiskResourcePolicyAttachmentAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ComputeRegionDiskResourcePolicyAttachment", "id", a.id)

	disk, err := a.disksClient.Get(ctx, &pb.GetRegionDiskRequest{
		Project: a.id.Project,
		Region:  a.id.Region,
		Disk:    a.id.Disk,
	})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting regional ComputeDisk %s: %w", a.id.Disk, err)
	}

	for _, policyUrl := range disk.ResourcePolicies {
		tokens := strings.Split(policyUrl, "/")
		if len(tokens) == 0 {
			continue
		}
		policyName := tokens[len(tokens)-1]
		if policyName == a.id.Name {
			a.actual = policyUrl
			return true, nil
		}
	}

	return false, nil
}

func (a *ComputeRegionDiskResourcePolicyAttachmentAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ComputeRegionDiskResourcePolicyAttachment", "id", a.id)

	policyUrl := fmt.Sprintf("projects/%s/regions/%s/resourcePolicies/%s", a.id.Project, a.id.Region, a.id.Name)

	req := &pb.AddResourcePoliciesRegionDiskRequest{
		Project: a.id.Project,
		Region:  a.id.Region,
		Disk:    a.id.Disk,
		RegionDisksAddResourcePoliciesRequestResource: &pb.RegionDisksAddResourcePoliciesRequest{
			ResourcePolicies: []string{policyUrl},
		},
	}

	op, err := a.disksClient.AddResourcePolicies(ctx, req)
	if err != nil {
		return fmt.Errorf("attaching resource policy %s to disk %s: %w", a.id.Name, a.id.Disk, err)
	}

	if !op.Done() {
		err = op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("waiting for resource policy %s to attach to disk %s: %w", a.id.Name, a.id.Disk, err)
		}
	}

	log.V(2).Info("successfully attached resource policy to disk", "id", a.id)

	status := &krm.ComputeRegionDiskResourcePolicyAttachmentStatus{}
	status.ObservedGeneration = direct.PtrTo(createOp.GetUnstructured().GetGeneration())
	return createOp.UpdateStatus(ctx, status, nil)
}

func (a *ComputeRegionDiskResourcePolicyAttachmentAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating ComputeRegionDiskResourcePolicyAttachment", "id", a.id)

	status := &krm.ComputeRegionDiskResourcePolicyAttachmentStatus{}
	status.ObservedGeneration = direct.PtrTo(updateOp.GetUnstructured().GetGeneration())
	return updateOp.UpdateStatus(ctx, status, nil)
}

func (a *ComputeRegionDiskResourcePolicyAttachmentAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ComputeRegionDiskResourcePolicyAttachment", "id", a.id)

	policyUrl := fmt.Sprintf("projects/%s/regions/%s/resourcePolicies/%s", a.id.Project, a.id.Region, a.id.Name)

	req := &pb.RemoveResourcePoliciesRegionDiskRequest{
		Project: a.id.Project,
		Region:  a.id.Region,
		Disk:    a.id.Disk,
		RegionDisksRemoveResourcePoliciesRequestResource: &pb.RegionDisksRemoveResourcePoliciesRequest{
			ResourcePolicies: []string{policyUrl},
		},
	}

	op, err := a.disksClient.RemoveResourcePolicies(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("removing resource policy %s from disk %s: %w", a.id.Name, a.id.Disk, err)
	}

	if !op.Done() {
		err = op.Wait(ctx)
		if err != nil {
			return false, fmt.Errorf("waiting for resource policy %s to remove from disk %s: %w", a.id.Name, a.id.Disk, err)
		}
	}

	log.V(2).Info("successfully removed resource policy from disk", "id", a.id)
	return true, nil
}

func (a *ComputeRegionDiskResourcePolicyAttachmentAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == "" {
		return nil, fmt.Errorf("Find() not called or attachment not found")
	}

	u := &unstructured.Unstructured{}

	obj := &krm.ComputeRegionDiskResourcePolicyAttachment{}
	obj.Spec.DiskRef = computev1beta1.ComputeDiskRef{
		External: fmt.Sprintf("projects/%s/regions/%s/disks/%s", a.id.Project, a.id.Region, a.id.Disk),
	}
	obj.Spec.ProjectRef = refs.ProjectRef{
		External: a.id.Project,
	}
	obj.Spec.Region = a.id.Region
	obj.Spec.ResourceID = &a.id.Name

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.Object = uObj
	u.SetName(a.id.Name)
	u.SetGroupVersionKind(krm.ComputeRegionDiskResourcePolicyAttachmentGVK)
	return u, nil
}
