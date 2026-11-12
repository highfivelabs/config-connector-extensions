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
// pkg/controller/direct/networksecurity/networksecuritysecurityprofilegroup_controller.go
// (UPSTREAM.md), in the shape of our securityprofile_controller.go. Changed:
// the REST clients (SecurityProfileGroupClient, and
// OrganizationSecurityProfileGroupClient for an organization's group; their
// methods and operations are the same Go types) in place of upstream's gRPC
// connection, which our manager does not configure; an unchanged object still
// has its status written; an unset and an empty labels map compare equal; no
// export; the model is returned to our manager instead of registered with
// Config Connector's registry; our KRM types.

package networksecurity

import (
	"context"
	"fmt"

	gcp "cloud.google.com/go/networksecurity/apiv1"
	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/networksecurity/v1alpha1"
)

func NewSecurityProfileGroupModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &securityProfileGroupModel{config: *config}, nil
}

var _ directbase.Model = &securityProfileGroupModel{}

type securityProfileGroupModel struct {
	config config.ControllerConfig
}

// securityProfileGroupClient is what the adapter calls: the methods the project
// service's client (SecurityProfileGroupClient) and the organization
// service's (OrganizationSecurityProfileGroupClient) share. Written here.
type securityProfileGroupClient interface {
	GetSecurityProfileGroup(context.Context, *pb.GetSecurityProfileGroupRequest, ...gax.CallOption) (*pb.SecurityProfileGroup, error)
	CreateSecurityProfileGroup(context.Context, *pb.CreateSecurityProfileGroupRequest, ...gax.CallOption) (*gcp.CreateSecurityProfileGroupOperation, error)
	UpdateSecurityProfileGroup(context.Context, *pb.UpdateSecurityProfileGroupRequest, ...gax.CallOption) (*gcp.UpdateSecurityProfileGroupOperation, error)
	DeleteSecurityProfileGroup(context.Context, *pb.DeleteSecurityProfileGroupRequest, ...gax.CallOption) (*gcp.DeleteSecurityProfileGroupOperation, error)
}

func (m *securityProfileGroupModel) client(ctx context.Context, id *krm.NetworkSecuritySecurityProfileGroupIdentity) (securityProfileGroupClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	if id.Organization != "" {
		gcpClient, err := gcp.NewOrganizationSecurityProfileGroupRESTClient(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("building networksecurity organization security profile REST client: %w", err)
		}
		return gcpClient, nil
	}
	gcpClient, err := gcp.NewSecurityProfileGroupRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building networksecurity security profile REST client: %w", err)
	}
	return gcpClient, nil
}

func (m *securityProfileGroupModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.NetworkSecuritySecurityProfileGroup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}
	spgID := id.(*krm.NetworkSecuritySecurityProfileGroupIdentity)

	gcpClient, err := m.client(ctx, spgID)
	if err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	desired := NetworkSecuritySecurityProfileGroupSpec_v1alpha1_ToProto(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	desired.Name = spgID.String()

	return &securityProfileGroupAdapter{
		gcpClient: gcpClient,
		id:        spgID,
		desired:   desired,
	}, nil
}

func (m *securityProfileGroupModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type securityProfileGroupAdapter struct {
	gcpClient securityProfileGroupClient
	id        *krm.NetworkSecuritySecurityProfileGroupIdentity
	desired   *pb.SecurityProfileGroup
	actual    *pb.SecurityProfileGroup
}

var _ directbase.Adapter = &securityProfileGroupAdapter{}

func (a *securityProfileGroupAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting SecurityProfileGroup", "name", a.id)

	req := &pb.GetSecurityProfileGroupRequest{Name: a.id.String()}
	actual, err := a.gcpClient.GetSecurityProfileGroup(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting SecurityProfileGroup %q from gcp: %w", a.id.String(), err)
	}

	a.actual = actual
	return true, nil
}

func (a *securityProfileGroupAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating SecurityProfileGroup", "name", a.id)

	req := &pb.CreateSecurityProfileGroupRequest{
		Parent:                 a.id.ParentString(),
		SecurityProfileGroupId: a.id.Securityprofilegroup,
		SecurityProfileGroup:   a.desired,
	}
	op, err := a.gcpClient.CreateSecurityProfileGroup(ctx, req)
	if err != nil {
		return fmt.Errorf("creating SecurityProfileGroup %s: %w", a.id, err)
	}

	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("SecurityProfileGroup %s waiting creation: %w", a.id, err)
	}

	latest, err := a.gcpClient.GetSecurityProfileGroup(ctx, &pb.GetSecurityProfileGroupRequest{Name: a.id.String()})
	if err != nil {
		return fmt.Errorf("getting SecurityProfileGroup after creation: %w", err)
	}

	log.V(2).Info("successfully created SecurityProfileGroup", "name", a.id)

	return a.updateStatus(ctx, createOp, latest)
}

func (a *securityProfileGroupAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating SecurityProfileGroup", "name", a.id)

	diffs, updateMask, err := compareSecurityProfileGroup(ctx, a.actual, a.desired)
	if err != nil {
		return err
	}

	diffs.Object = updateOp.GetUnstructured()
	structuredreporting.ReportDiff(ctx, diffs)

	if !diffs.HasDiff() {
		log.V(2).Info("no field needs update", "name", a.id)
		return a.updateStatus(ctx, updateOp, a.actual)
	}

	log.V(2).Info("fields need update", "name", a.id, "updateMask", updateMask)

	req := &pb.UpdateSecurityProfileGroupRequest{
		UpdateMask:           updateMask,
		SecurityProfileGroup: a.desired,
	}
	req.SecurityProfileGroup.Name = a.id.String()

	op, err := a.gcpClient.UpdateSecurityProfileGroup(ctx, req)
	if err != nil {
		return fmt.Errorf("updating SecurityProfileGroup %s: %w", a.id, err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("SecurityProfileGroup %s waiting update: %w", a.id, err)
	}

	latest, err := a.gcpClient.GetSecurityProfileGroup(ctx, &pb.GetSecurityProfileGroupRequest{Name: a.id.String()})
	if err != nil {
		return fmt.Errorf("getting SecurityProfileGroup after update: %w", err)
	}

	log.V(2).Info("successfully updated SecurityProfileGroup", "name", a.id)

	return a.updateStatus(ctx, updateOp, latest)
}

func compareSecurityProfileGroup(ctx context.Context, actual, desired *pb.SecurityProfileGroup) (*structuredreporting.Diff, *fieldmaskpb.FieldMask, error) {
	mapCtx := &direct.MapContext{}
	spec := NetworkSecuritySecurityProfileGroupSpec_v1alpha1_FromProto(mapCtx, actual)
	if mapCtx.Err() != nil {
		return nil, nil, mapCtx.Err()
	}
	maskedActual := NetworkSecuritySecurityProfileGroupSpec_v1alpha1_ToProto(mapCtx, spec)
	if mapCtx.Err() != nil {
		return nil, nil, mapCtx.Err()
	}
	maskedActual.Name = desired.Name

	// An unset map and an empty one are the same (written here).
	cmpDesired := proto.CloneOf(desired)
	if len(cmpDesired.Labels) == 0 {
		cmpDesired.Labels = nil
	}
	if len(maskedActual.Labels) == 0 {
		maskedActual.Labels = nil
	}

	diffs, updateMask, err := common.DiffForTopLevelFields(ctx, cmpDesired.ProtoReflect(), maskedActual.ProtoReflect())
	if err != nil {
		return nil, nil, err
	}
	return diffs, updateMask, nil
}

func (a *securityProfileGroupAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *securityProfileGroupAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting SecurityProfileGroup", "name", a.id)

	req := &pb.DeleteSecurityProfileGroupRequest{Name: a.id.String()}
	op, err := a.gcpClient.DeleteSecurityProfileGroup(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent SecurityProfileGroup, assuming it was already deleted", "name", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting SecurityProfileGroup %s: %w", a.id, err)
	}

	if err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("waiting delete SecurityProfileGroup %s: %w", a.id, err)
	}

	log.V(2).Info("successfully deleted SecurityProfileGroup", "name", a.id)
	return true, nil
}

func (a *securityProfileGroupAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.SecurityProfileGroup) error {
	mapCtx := &direct.MapContext{}
	status := &krm.NetworkSecuritySecurityProfileGroupStatus{}
	status.ObservedState = NetworkSecuritySecurityProfileGroupObservedState_v1alpha1_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	externalRef := a.id.String()
	status.ExternalRef = &externalRef

	return op.UpdateStatus(ctx, status, nil)
}

// CloudLabels are the found resource's labels, for the slot guard
// (pkg/slotguard). Written here.
func (a *securityProfileGroupAdapter) CloudLabels() map[string]string {
	return a.actual.GetLabels()
}
