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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// pkg/controller/direct/iam/iamdenypolicy/iamdenypolicy_controller.go
// (UPSTREAM.md). Changed: status.externalRef is written (upstream sets it on
// the typed copy only, so it never reaches the object), also when nothing
// changed; an object being deleted is deleted by status.externalRef without
// resolving its parent reference, and one with neither goes without a call;
// the model is returned to our manager instead of registered with
// Config Connector's registry; the model and adapter are named for the kind,
// in this package beside the other IAM kinds; our KRM types (stable,
// iam.gcp.hfl.dev/v1beta1, docs/design.md decision 13).

package iam

import (
	"context"
	"fmt"

	gcp_iam "cloud.google.com/go/iam/apiv2"
	pb "cloud.google.com/go/iam/apiv2/iampb"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
)

// NewDenyPolicyModel is the IAMDenyPolicy model.
func NewDenyPolicyModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &denyPolicyModel{config: *config}, nil
}

var _ directbase.Model = &denyPolicyModel{}

type denyPolicyModel struct {
	config config.ControllerConfig
}

func (m *denyPolicyModel) client(ctx context.Context) (*gcp_iam.PoliciesClient, error) {
	var opts []option.ClientOption
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp_iam.NewPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building policies client: %w", err)
	}
	return gcpClient, err
}

func (m *denyPolicyModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.IAMDenyPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.Object.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	ext := direct.ValueOf(obj.Status.ExternalRef)
	var id *krm.IAMDenyPolicyIdentity
	switch deleting := !obj.GetDeletionTimestamp().IsZero(); {
	case deleting && ext != "":
		// Deleting a policy that was made or acquired: its name is in status,
		// and the parts of a name cannot change, so the parent is not
		// resolved. A parent object already gone or going (everything deleted
		// at once) would otherwise hold the delete forever (the generated
		// kinds' rule, pkg/discokind).
		id = &krm.IAMDenyPolicyIdentity{}
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
	default:
		got, err := obj.GetIdentity(ctx, op.Reader)
		if err != nil {
			if deleting {
				// Never made or acquired (no status.externalRef) and its
				// parent cannot be resolved: there is nothing of ours to
				// delete.
				klog.FromContext(ctx).Info("deleting IAMDenyPolicy without a call: no status.externalRef and the parent cannot be resolved", "error", err.Error())
				return &denyPolicyAdapter{desired: obj}, nil
			}
			return nil, err
		}
		id = got.(*krm.IAMDenyPolicyIdentity)
	}

	// Get gcp client
	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &denyPolicyAdapter{
		id:        id,
		gcpClient: gcpClient,
		desired:   obj,
	}, nil
}

func (m *denyPolicyModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type denyPolicyAdapter struct {
	id        *krm.IAMDenyPolicyIdentity
	gcpClient *gcp_iam.PoliciesClient
	desired   *krm.IAMDenyPolicy
	actual    *pb.Policy
}

var _ directbase.Adapter = &denyPolicyAdapter{}

func (a *denyPolicyAdapter) Find(ctx context.Context) (bool, error) {
	if a.id == nil {
		// Deleting an object that never had a policy (AdapterForObject).
		return false, nil
	}
	log := klog.FromContext(ctx)
	log.V(2).Info("getting IAMDenyPolicy", "name", a.id.String())

	req := &pb.GetPolicyRequest{
		Name: a.id.String(),
	}
	policy, err := a.gcpClient.GetPolicy(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting IAMDenyPolicy %q: %w", a.id.String(), err)
	}

	a.actual = policy

	return true, nil
}

func (a *denyPolicyAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating IAMDenyPolicy", "name", a.id.String())
	mapCtx := &direct.MapContext{}

	desiredProto := IAMDenyPolicySpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	parent := "policies/" + a.id.AttachmentPoint + "/denypolicies"

	req := &pb.CreatePolicyRequest{
		Parent:   parent,
		Policy:   desiredProto,
		PolicyId: a.id.DenyPolicy,
	}
	op, err := a.gcpClient.CreatePolicy(ctx, req)
	if err != nil {
		return fmt.Errorf("creating IAMDenyPolicy %q: %w", a.id.String(), err)
	}

	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting for IAMDenyPolicy %q creation: %w", a.id.String(), err)
	}

	log.V(2).Info("successfully created IAMDenyPolicy", "name", a.id.String())

	return a.updateStatus(ctx, createOp)
}

func (a *denyPolicyAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating IAMDenyPolicy", "name", a.id.String())
	mapCtx := &direct.MapContext{}

	desiredProto := IAMDenyPolicySpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	desiredProto.Name = a.actual.Name
	desiredProto.Uid = a.actual.Uid
	desiredProto.Kind = a.actual.Kind
	desiredProto.Etag = a.actual.Etag
	desiredProto.Annotations = a.actual.Annotations
	desiredProto.CreateTime = a.actual.CreateTime
	desiredProto.UpdateTime = a.actual.UpdateTime
	desiredProto.DeleteTime = a.actual.DeleteTime
	desiredProto.ManagingAuthority = a.actual.ManagingAuthority

	paths, err := common.CompareProtoMessage(desiredProto, a.actual, common.BasicDiff)
	if err != nil {
		return err
	}

	if paths.Len() == 0 {
		log.V(2).Info("no changes detected for IAMDenyPolicy, skipping update", "name", a.id.String())
		return a.updateStatus(ctx, updateOp)
	}

	getReq := &pb.GetPolicyRequest{
		Name: a.id.String(),
	}
	existing, err := a.gcpClient.GetPolicy(ctx, getReq)
	if err != nil {
		return fmt.Errorf("getting IAMDenyPolicy %q before update: %w", a.id.String(), err)
	}
	desiredProto.Etag = existing.Etag

	req := &pb.UpdatePolicyRequest{
		Policy: desiredProto,
	}

	op, err := a.gcpClient.UpdatePolicy(ctx, req)
	if err != nil {
		return fmt.Errorf("updating IAMDenyPolicy %q: %w", a.id.String(), err)
	}

	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting for IAMDenyPolicy %q update: %w", a.id.String(), err)
	}

	log.V(2).Info("successfully updated IAMDenyPolicy", "name", a.id.String())

	return a.updateStatus(ctx, updateOp)
}

// updateStatus writes status.externalRef. Written here.
func (a *denyPolicyAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.IAMDenyPolicyStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	return op.UpdateStatus(ctx, status, nil)
}

func (a *denyPolicyAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting IAMDenyPolicy", "name", a.id.String())

	req := &pb.DeletePolicyRequest{
		Name: a.id.String(),
	}

	op, err := a.gcpClient.DeletePolicy(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("deleting IAMDenyPolicy %q: %w", a.id.String(), err)
	}

	_, err = op.Wait(ctx)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("waiting for IAMDenyPolicy %q deletion: %w", a.id.String(), err)
	}

	log.V(2).Info("successfully deleted IAMDenyPolicy", "name", a.id.String())
	return true, nil
}

func (a *denyPolicyAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.IAMDenyPolicy{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(IAMDenyPolicySpec_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	// Restore references
	obj.Spec.ProjectRef = a.desired.Spec.ProjectRef
	obj.Spec.FolderRef = a.desired.Spec.FolderRef
	obj.Spec.OrganizationRef = a.desired.Spec.OrganizationRef

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u.Object = uObj
	return u, nil
}
