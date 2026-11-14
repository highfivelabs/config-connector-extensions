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

// Modified by High Five Labs for config-connector-extensions: adapted from Config Connector
// pkg/controller/direct/compute/computeautoscaler_controller.go (UPSTREAM.md),
// the zonal autoscaler's controller, for the regional one, which has types
// and a mapper upstream but no controller. Changed: our KRM types (stable,
// compute.gcp.hfl.dev/v1beta1, docs/design.md decision 26); the
// RegionAutoscalers REST client, built from the controller config's REST
// options; a region in place of the zone; the target is a URL in the spec
// (upstream's regional type has no reference), trimmed of the Compute
// Engine prefix; the model is returned to our manager, not registered with
// Config Connector's registry.

// +tool:controller
// proto.service: google.cloud.compute.v1.RegionAutoscalers
// proto.message: google.cloud.compute.v1.Autoscaler
// crd.type: ComputeRegionAutoscaler
// crd.version: v1beta1

package compute

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/mappers"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func NewComputeRegionAutoscalerModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &computeRegionAutoscalerModel{config: config}, nil
}

var _ directbase.Model = &computeRegionAutoscalerModel{}

type computeRegionAutoscalerModel struct {
	config *config.ControllerConfig
}

func (m *computeRegionAutoscalerModel) client(ctx context.Context) (*compute.RegionAutoscalersClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := compute.NewRegionAutoscalersRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building region autoscalers client: %w", err)
	}
	return c, nil
}

func (m *computeRegionAutoscalerModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.ComputeRegionAutoscaler{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}

	autoscalersClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	obj.Spec.Target = refs.TrimComputeURIPrefix(obj.Spec.Target)

	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	mapCtx := &direct.MapContext{}
	desired := obj.DeepCopy()
	resource := ComputeRegionAutoscalerSpec_v1beta1_ToProto(mapCtx, &desired.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	return &ComputeRegionAutoscalerAdapter{
		gcpClient: autoscalersClient,
		id:        id.(*krm.ComputeRegionAutoscalerIdentity),
		desired:   resource,
		reader:    reader,
	}, nil
}

func (m *computeRegionAutoscalerModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	id := &krm.ComputeRegionAutoscalerIdentity{}
	if err := id.FromExternal(url); err != nil {
		// Not recognized
		return nil, nil
	}

	autoscalersClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	return &ComputeRegionAutoscalerAdapter{
		gcpClient: autoscalersClient,
		id:        id,
	}, nil
}

type ComputeRegionAutoscalerAdapter struct {
	gcpClient *compute.RegionAutoscalersClient
	id        *krm.ComputeRegionAutoscalerIdentity
	desired   *pb.Autoscaler
	actual    *pb.Autoscaler
	reader    client.Reader
}

var _ directbase.Adapter = &ComputeRegionAutoscalerAdapter{}

func (a *ComputeRegionAutoscalerAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ComputeRegionAutoscaler", "name", a.id)

	req := &pb.GetRegionAutoscalerRequest{
		Project:    a.id.Project,
		Region:     a.id.Region,
		Autoscaler: a.id.Autoscaler,
	}
	actual, err := a.gcpClient.Get(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeRegionAutoscaler %q: %w", a.id, err)
	}

	a.actual = actual
	return true, nil
}

func (a *ComputeRegionAutoscalerAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ComputeRegionAutoscaler", "name", a.id)

	a.desired.Name = proto.String(a.id.Autoscaler)

	req := &pb.InsertRegionAutoscalerRequest{
		Project:            a.id.Project,
		Region:             a.id.Region,
		AutoscalerResource: a.desired,
	}
	op, err := a.gcpClient.Insert(ctx, req)
	if err != nil {
		return fmt.Errorf("creating ComputeRegionAutoscaler %s: %w", a.id, err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting ComputeRegionAutoscaler %s create failed: %w", a.id, err)
	}
	log.V(2).Info("successfully created ComputeRegionAutoscaler", "name", a.id)

	// Get latest state
	latest, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeRegionAutoscaler %s after creation: %w", a.id, err)
	}

	return a.updateStatus(ctx, createOp, latest)
}

func (a *ComputeRegionAutoscalerAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating ComputeRegionAutoscaler", "name", a.id)

	diffs, _, err := compareComputeRegionAutoscaler(ctx, a.actual, a.desired)
	if err != nil {
		return err
	}

	var updated *pb.Autoscaler
	if !diffs.HasDiff() {
		log.V(2).Info("no field needs update", "name", a.id.String())
		updated = a.actual
	} else {
		diffs.Object = updateOp.GetUnstructured()
		structuredreporting.ReportDiff(ctx, diffs)

		req := &pb.PatchRegionAutoscalerRequest{
			Project:            a.id.Project,
			Region:             a.id.Region,
			Autoscaler:         proto.String(a.id.Autoscaler),
			AutoscalerResource: a.desired,
		}
		op, err := a.gcpClient.Patch(ctx, req)
		if err != nil {
			return fmt.Errorf("updating compute ComputeRegionAutoscaler %s: %w", a.id.String(), err)
		}
		log.V(2).Info("successfully updated compute ComputeRegionAutoscaler", "name", a.id.String())

		err = op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("compute ComputeRegionAutoscaler %s waiting for update: %w", a.id.String(), err)
		}

		updated, err = a.get(ctx)
		if err != nil {
			return fmt.Errorf("getting ComputeRegionAutoscaler %s: %w", a.id, err)
		}
	}

	return a.updateStatus(ctx, updateOp, updated)
}

func (a *ComputeRegionAutoscalerAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ComputeRegionAutoscaler", "name", a.id)

	req := &pb.DeleteRegionAutoscalerRequest{
		Project:    a.id.Project,
		Region:     a.id.Region,
		Autoscaler: a.id.Autoscaler,
	}
	op, err := a.gcpClient.Delete(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeRegionAutoscaler %s: %w", a.id, err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return false, fmt.Errorf("waiting ComputeRegionAutoscaler %s delete failed: %w", a.id, err)
	}

	log.V(2).Info("successfully deleted ComputeRegionAutoscaler", "name", a.id)
	return true, nil
}

func (a *ComputeRegionAutoscalerAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.ComputeRegionAutoscaler{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(ComputeRegionAutoscalerSpec_v1beta1_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	obj.Spec.Region = a.id.Region
	obj.Spec.ResourceID = direct.LazyPtr(a.id.Autoscaler)

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.Object = uObj
	u.SetName(a.actual.GetName())
	u.SetGroupVersionKind(krm.ComputeRegionAutoscalerGVK)
	return u, nil
}

func (a *ComputeRegionAutoscalerAdapter) get(ctx context.Context) (*pb.Autoscaler, error) {
	req := &pb.GetRegionAutoscalerRequest{
		Project:    a.id.Project,
		Region:     a.id.Region,
		Autoscaler: a.id.Autoscaler,
	}
	return a.gcpClient.Get(ctx, req)
}

func (a *ComputeRegionAutoscalerAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.Autoscaler) error {
	mapCtx := &direct.MapContext{}
	status := ComputeRegionAutoscalerStatus_v1beta1_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	return op.UpdateStatus(ctx, status, nil)
}

func compareComputeRegionAutoscaler(ctx context.Context, actual, desired *pb.Autoscaler) (*structuredreporting.Diff, *fieldmaskpb.FieldMask, error) {
	maskedActual, err := mappers.OnlySpecFields(actual, ComputeRegionAutoscalerSpec_v1beta1_FromProto, ComputeRegionAutoscalerSpec_v1beta1_ToProto)
	if err != nil {
		return nil, nil, err
	}
	maskedActual.Name = desired.Name

	clonedDesired := proto.CloneOf(desired)

	normalizeComputeRegionAutoscalerForComparison(maskedActual)
	normalizeComputeRegionAutoscalerForComparison(clonedDesired)

	diffs, updateMask, err := common.DiffForTopLevelFields(ctx, clonedDesired.ProtoReflect(), maskedActual.ProtoReflect())
	if err != nil {
		return nil, nil, err
	}
	return diffs, updateMask, nil
}

func normalizeComputeRegionAutoscalerForComparison(obj *pb.Autoscaler) {
	if obj == nil {
		return
	}
	if obj.Region != nil {
		obj.Region = direct.PtrTo(lastComponent(*obj.Region))
	}
	if obj.Target != nil {
		obj.Target = direct.PtrTo(refs.TrimComputeURIPrefix(*obj.Target))
	}
}
