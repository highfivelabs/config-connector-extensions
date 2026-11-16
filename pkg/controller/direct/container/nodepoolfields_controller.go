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

package container

// Written here: no Config Connector code or pull request sets these
// kubelet fields. nodePools.update takes a node pool's kubeletConfig whole,
// so the write is the node pool's own kubeletConfig, as just read, with
// only this object's fields changed: Config Connector's kubelet fields go
// back as they are. The other way round is not safe: Terraform's update of
// a node pool's kubelet_config (what Config Connector's ContainerNodePool
// runs when one of its kubelet fields changes) sends the whole config from
// Terraform's schema, which lacks these fields, so it can reset them; this
// object sets them again on its next reconcile (docs/design.md decision 37).

import (
	"context"
	"fmt"
	"slices"

	gcp "cloud.google.com/go/container/apiv1"
	pb "cloud.google.com/go/container/apiv1/containerpb"
	containerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/container/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/container/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

func init() {
	registry.RegisterModel(krm.ContainerNodePoolFieldsGVK, NewNodePoolFieldsModel)
}

func NewNodePoolFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &nodePoolFieldsModel{config: *config}, nil
}

type nodePoolFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &nodePoolFieldsModel{}

func (m *nodePoolFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ContainerNodePoolFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	if obj.Spec.Config == nil || obj.Spec.Config.KubeletConfig == nil {
		return nil, fmt.Errorf("spec.config.kubeletConfig is required")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the node pool is in the status; Config Connector's
		// ContainerNodePool may be gone already.
		ref = &containerv1beta1.ContainerNodePoolRef{External: prev}
	} else if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	id := &containerv1beta1.ContainerNodePoolIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the node pool of ContainerNodePoolFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.ContainerNodePoolFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	c, err := newClient(ctx, &m.config)
	if err != nil {
		return nil, fmt.Errorf("building container client: %w", err)
	}
	return &nodePoolFieldsAdapter{id: id, client: c, desired: obj.Spec.Config.KubeletConfig}, nil
}

func (m *nodePoolFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type nodePoolFieldsAdapter struct {
	id      *containerv1beta1.ContainerNodePoolIdentity
	client  *gcp.ClusterManagerClient
	desired *krm.NodePoolKubeletConfig
	actual  *pb.NodePool
}

var _ directbase.Adapter = &nodePoolFieldsAdapter{}

func (a *nodePoolFieldsAdapter) Find(ctx context.Context) (bool, error) {
	np, err := a.client.GetNodePool(ctx, &pb.GetNodePoolRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting GKE node pool %s: %w", a.id, err)
	}
	a.actual = np
	return true, nil
}

// Create is called only while the node pool does not exist: this kind
// never makes it, so it waits for it.
func (a *nodePoolFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("ContainerNodePool", a.id.String())
}

// apply is the node pool's kubeletConfig with this object's fields set
// (or, with clear, unset), and whether that differs from what it has.
func apply(actual *pb.NodeKubeletConfig, d *krm.NodePoolKubeletConfig, clear bool) (*pb.NodeKubeletConfig, bool) {
	if actual == nil {
		actual = &pb.NodeKubeletConfig{}
	}
	out := proto.Clone(actual).(*pb.NodeKubeletConfig)
	if d.InsecureKubeletReadonlyPortEnabled != nil {
		if clear {
			out.InsecureKubeletReadonlyPortEnabled = nil
		} else {
			out.InsecureKubeletReadonlyPortEnabled = direct.PtrTo(*d.InsecureKubeletReadonlyPortEnabled)
		}
	}
	if d.ContainerLogMaxSize != nil {
		out.ContainerLogMaxSize = ""
		if !clear {
			out.ContainerLogMaxSize = *d.ContainerLogMaxSize
		}
	}
	if d.ContainerLogMaxFiles != nil {
		out.ContainerLogMaxFiles = 0
		if !clear {
			out.ContainerLogMaxFiles = *d.ContainerLogMaxFiles
		}
	}
	if d.AllowedUnsafeSysctls != nil {
		out.AllowedUnsafeSysctls = nil
		if !clear {
			out.AllowedUnsafeSysctls = slices.Clone(d.AllowedUnsafeSysctls)
		}
	}
	if d.MaxParallelImagePulls != nil {
		out.MaxParallelImagePulls = 0
		if !clear {
			out.MaxParallelImagePulls = *d.MaxParallelImagePulls
		}
	}
	return out, !sameOwned(actual, out, d)
}

// sameOwned compares only the fields d owns: the API fills in defaults
// for the others, and they are not ours to compare. A cleared field is
// compared with the API's answer loosely: GKE puts its default back.
func sameOwned(a, b *pb.NodeKubeletConfig, d *krm.NodePoolKubeletConfig) bool {
	if d.InsecureKubeletReadonlyPortEnabled != nil && (a.InsecureKubeletReadonlyPortEnabled == nil) != (b.InsecureKubeletReadonlyPortEnabled == nil) {
		return false
	}
	if d.InsecureKubeletReadonlyPortEnabled != nil && a.GetInsecureKubeletReadonlyPortEnabled() != b.GetInsecureKubeletReadonlyPortEnabled() {
		return false
	}
	if d.ContainerLogMaxSize != nil && a.GetContainerLogMaxSize() != b.GetContainerLogMaxSize() {
		return false
	}
	if d.ContainerLogMaxFiles != nil && a.GetContainerLogMaxFiles() != b.GetContainerLogMaxFiles() {
		return false
	}
	if d.AllowedUnsafeSysctls != nil {
		x, y := slices.Clone(a.GetAllowedUnsafeSysctls()), slices.Clone(b.GetAllowedUnsafeSysctls())
		slices.Sort(x)
		slices.Sort(y)
		if !slices.Equal(x, y) {
			return false
		}
	}
	if d.MaxParallelImagePulls != nil && a.GetMaxParallelImagePulls() != b.GetMaxParallelImagePulls() {
		return false
	}
	return true
}

func (a *nodePoolFieldsAdapter) write(ctx context.Context, kc *pb.NodeKubeletConfig) error {
	op, err := a.client.UpdateNodePool(ctx, &pb.UpdateNodePoolRequest{Name: a.id.String(), KubeletConfig: kc})
	if err != nil {
		return fmt.Errorf("setting the kubeletConfig of GKE node pool %s: %w", a.id, err)
	}
	if err := waitForOperation(ctx, a.client, a.id.Project, a.id.Location, op); err != nil {
		return fmt.Errorf("setting the kubeletConfig of GKE node pool %s: %w", a.id, err)
	}
	return nil
}

func (a *nodePoolFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	kc, changed := apply(a.actual.GetConfig().GetKubeletConfig(), a.desired, false)
	if changed {
		klog.FromContext(ctx).V(2).Info("setting kubelet fields of a GKE node pool", "name", a.id)
		if err := a.write(ctx, kc); err != nil {
			return err
		}
	}
	return updateOp.UpdateStatus(ctx, &krm.ContainerNodePoolFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}

// Delete unsets this object's fields (GKE's defaults apply again), keeping
// the rest of the node pool's kubeletConfig. It never deletes the node
// pool; a node pool that is gone is not reached (Find said not found).
func (a *nodePoolFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	actual := a.actual.GetConfig().GetKubeletConfig()
	if actual == nil {
		return true, nil
	}
	kc, _ := apply(actual, a.desired, true)
	klog.FromContext(ctx).V(2).Info("clearing kubelet fields of a GKE node pool", "name", a.id)
	if err := a.write(ctx, kc); err != nil {
		return false, err
	}
	return true, nil
}

func (a *nodePoolFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}
