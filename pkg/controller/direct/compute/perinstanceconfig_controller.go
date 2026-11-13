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
// our regionperinstanceconfig_controller.go (itself copied from Config
// Connector's pkg/controller/direct/compute/computeregionperinstanceconfig_controller.go,
// UPSTREAM.md) for ComputePerInstanceConfig, the zonal per-instance config,
// which Config Connector has only as a Terraform-based alpha kind: the
// zonal InstanceGroupManagers client and its requests for the regional
// ones. Our KRM types (stable, compute.gcp.hfl.dev/v1beta1, docs/design.md
// decision 26); the model is returned to our manager; the Compute Engine
// REST client built from the controller config's REST options.

package compute

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/mappers"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func NewComputePerInstanceConfigModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &computePerInstanceConfigModel{config: config}, nil
}

type computePerInstanceConfigModel struct {
	config *config.ControllerConfig
}

var _ directbase.Model = &computePerInstanceConfigModel{}

func (m *computePerInstanceConfigModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.ComputePerInstanceConfig{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}
	picId, ok := id.(*krm.ComputePerInstanceConfigIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type %T", id)
	}

	if picId.Project == "" || picId.Zone == "" || picId.InstanceGroupManager == "" || picId.Name == "" {
		return nil, fmt.Errorf("incomplete identity: %+v", picId)
	}

	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	zonalClient, err := gcp.NewInstanceGroupManagersRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building zonal instance group managers client: %w", err)
	}

	return &ComputePerInstanceConfigAdapter{
		zonalClient: zonalClient,
		id:          picId,
		desired:     obj,
		reader:      reader,
	}, nil
}

func (m *computePerInstanceConfigModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type ComputePerInstanceConfigAdapter struct {
	zonalClient *gcp.InstanceGroupManagersClient
	id          *krm.ComputePerInstanceConfigIdentity
	desired     *krm.ComputePerInstanceConfig
	actual      *pb.PerInstanceConfig
	reader      client.Reader
}

var _ directbase.Adapter = &ComputePerInstanceConfigAdapter{}

func (a *ComputePerInstanceConfigAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ComputePerInstanceConfig", "name", a.id.String())

	req := &pb.ListPerInstanceConfigsInstanceGroupManagersRequest{
		Project:              a.id.Project,
		Zone:                 a.id.Zone,
		InstanceGroupManager: a.id.InstanceGroupManager,
	}
	it := a.zonalClient.ListPerInstanceConfigs(ctx, req)
	for {
		item, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			if direct.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("listing per instance configs for %s: %w", a.id.ParentString(), err)
		}
		if item.GetName() == a.id.Name {
			a.actual = item
			return true, nil
		}
	}

	return false, nil
}

func (a *ComputePerInstanceConfigAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ComputePerInstanceConfig", "name", a.id.String())

	mapCtx := &direct.MapContext{}
	desiredPb := ComputePerInstanceConfigSpec_v1beta1_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	desiredPb.Name = direct.LazyPtr(a.id.Name)

	req := &pb.CreateInstancesInstanceGroupManagerRequest{
		Project:              a.id.Project,
		Zone:                 a.id.Zone,
		InstanceGroupManager: a.id.InstanceGroupManager,
		InstanceGroupManagersCreateInstancesRequestResource: &pb.InstanceGroupManagersCreateInstancesRequest{
			Instances: []*pb.PerInstanceConfig{desiredPb},
		},
	}
	op, err := a.zonalClient.CreateInstances(ctx, req)
	if err != nil {
		return fmt.Errorf("creating ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for creation of ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}

	return a.updateStatus(ctx, createOp)
}

func (a *ComputePerInstanceConfigAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating ComputePerInstanceConfig", "name", a.id.String())

	mapCtx := &direct.MapContext{}
	desiredPb := ComputePerInstanceConfigSpec_v1beta1_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	desiredPb.Name = direct.LazyPtr(a.id.Name)

	diffs, err := compareComputePerInstanceConfig(ctx, a.actual, desiredPb)
	if err != nil {
		return err
	}

	if !diffs.HasDiff() {
		log.V(2).Info("no diff detected for ComputePerInstanceConfig", "name", a.id.String())
		return a.updateStatus(ctx, updateOp)
	}

	structuredreporting.ReportDiff(ctx, diffs)

	req := &pb.UpdatePerInstanceConfigsInstanceGroupManagerRequest{
		Project:              a.id.Project,
		Zone:                 a.id.Zone,
		InstanceGroupManager: a.id.InstanceGroupManager,
		InstanceGroupManagersUpdatePerInstanceConfigsReqResource: &pb.InstanceGroupManagersUpdatePerInstanceConfigsReq{
			PerInstanceConfigs: []*pb.PerInstanceConfig{desiredPb},
		},
	}
	op, err := a.zonalClient.UpdatePerInstanceConfigs(ctx, req)
	if err != nil {
		return fmt.Errorf("updating ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for update of ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}

	// Instance name in applyUpdatesToInstances request must include zone
	instanceURL, err := a.findInstanceURL(ctx)
	if err != nil {
		return err
	}

	minAction := "NONE"
	if a.desired.Spec.MinimalAction != nil && *a.desired.Spec.MinimalAction != "" {
		minAction = *a.desired.Spec.MinimalAction
	}
	mostDisruptiveAction := "REPLACE"
	if a.desired.Spec.MostDisruptiveAllowedAction != nil && *a.desired.Spec.MostDisruptiveAllowedAction != "" {
		mostDisruptiveAction = *a.desired.Spec.MostDisruptiveAllowedAction
	}

	applyReq := &pb.ApplyUpdatesToInstancesInstanceGroupManagerRequest{
		Project:              a.id.Project,
		Zone:                 a.id.Zone,
		InstanceGroupManager: a.id.InstanceGroupManager,
		InstanceGroupManagersApplyUpdatesRequestResource: &pb.InstanceGroupManagersApplyUpdatesRequest{
			Instances:                   []string{instanceURL},
			MinimalAction:               direct.LazyPtr(minAction),
			MostDisruptiveAllowedAction: direct.LazyPtr(mostDisruptiveAction),
		},
	}
	applyOp, err := a.zonalClient.ApplyUpdatesToInstances(ctx, applyReq)
	if err != nil {
		return fmt.Errorf("applying updates to ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}
	if err := applyOp.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for applyUpdatesToInstances of ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}

	return a.updateStatus(ctx, updateOp)
}

func (a *ComputePerInstanceConfigAdapter) findInstanceURL(ctx context.Context) (string, error) {
	req := &pb.ListManagedInstancesInstanceGroupManagersRequest{
		Project:              a.id.Project,
		Zone:                 a.id.Zone,
		InstanceGroupManager: a.id.InstanceGroupManager,
		MaxResults:           direct.LazyPtr(uint32(1)),
	}
	instanceNameToFind := "/" + a.id.Name
	it := a.zonalClient.ListManagedInstances(ctx, req)
	for {
		item, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("listing managed instances for %s: %w", a.id.ParentString(), err)
		}
		if item.GetName() == a.id.Name || strings.HasSuffix(item.GetInstance(), instanceNameToFind) {
			return item.GetInstance(), nil
		}
	}
	return "", fmt.Errorf("could not find instance %q in managed instances of %s", a.id.Name, a.id.ParentString())
}

func (a *ComputePerInstanceConfigAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ComputePerInstanceConfig", "name", a.id.String())

	req := &pb.DeletePerInstanceConfigsInstanceGroupManagerRequest{
		Project:              a.id.Project,
		Zone:                 a.id.Zone,
		InstanceGroupManager: a.id.InstanceGroupManager,
		InstanceGroupManagersDeletePerInstanceConfigsReqResource: &pb.InstanceGroupManagersDeletePerInstanceConfigsReq{
			Names: []string{a.id.Name},
		},
	}
	op, err := a.zonalClient.DeletePerInstanceConfigs(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}
	if err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("waiting for deletion of ComputePerInstanceConfig %s: %w", a.id.String(), err)
	}

	if a.desired.Spec.RemoveInstanceStateOnDestroy != nil && *a.desired.Spec.RemoveInstanceStateOnDestroy {
		instanceURL, err := a.findInstanceURL(ctx)
		if err == nil {
			applyReq := &pb.ApplyUpdatesToInstancesInstanceGroupManagerRequest{
				Project:              a.id.Project,
				Zone:                 a.id.Zone,
				InstanceGroupManager: a.id.InstanceGroupManager,
				InstanceGroupManagersApplyUpdatesRequestResource: &pb.InstanceGroupManagersApplyUpdatesRequest{
					Instances: []string{instanceURL},
				},
			}
			applyOp, err := a.zonalClient.ApplyUpdatesToInstances(ctx, applyReq)
			if err != nil {
				return false, fmt.Errorf("applying updates on destroy of ComputePerInstanceConfig %s: %w", a.id.String(), err)
			}
			if err := applyOp.Wait(ctx); err != nil {
				return false, fmt.Errorf("waiting for applyUpdatesToInstances on destroy of ComputePerInstanceConfig %s: %w", a.id.String(), err)
			}
		}
	}

	log.V(2).Info("successfully deleted ComputePerInstanceConfig", "name", a.id.String())
	return true, nil
}

func (a *ComputePerInstanceConfigAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.ComputePerInstanceConfigStatus{}
	return op.UpdateStatus(ctx, status, nil)
}

func (a *ComputePerInstanceConfigAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.ComputePerInstanceConfig{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(ComputePerInstanceConfigSpec_v1beta1_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	obj.Spec.ProjectRef = refs.ProjectRef{External: a.id.Project}
	obj.Spec.Zone = a.id.Zone
	obj.Spec.InstanceGroupManagerRef = krm.ComputeInstanceGroupManagerRef{
		External: a.id.InstanceGroupManager,
	}
	obj.Spec.ResourceID = direct.LazyPtr(a.id.Name)

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.Object = uObj
	u.SetName(a.actual.GetName())
	u.SetGroupVersionKind(krm.ComputePerInstanceConfigGVK)
	return u, nil
}

func compareComputePerInstanceConfig(ctx context.Context, actual, desired *pb.PerInstanceConfig) (*structuredreporting.Diff, error) {
	maskedActual, err := mappers.OnlySpecFields(actual, ComputePerInstanceConfigSpec_v1beta1_FromProto, ComputePerInstanceConfigSpec_v1beta1_ToProto)
	if err != nil {
		return nil, err
	}
	maskedActual.Name = desired.Name

	clonedDesired := proto.Clone(desired).(*pb.PerInstanceConfig)

	diffs, _, err := common.DiffForTopLevelFields(ctx, clonedDesired.ProtoReflect(), maskedActual.ProtoReflect())
	if err != nil {
		return nil, err
	}
	return diffs, nil
}
