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
// cluster fields. The cluster is read with clusters.get; each owned field
// that differs is written with its own clusters.update, whose ClusterUpdate
// carries that one desired* field and nothing else (the API takes one
// change per update), so Config Connector's fields are never sent.
// Terraform's update of a cluster (what Config Connector's ContainerCluster
// runs) sends only the desired* fields of what it changed, none of which is
// one of these (docs/design.md section 6.6).

import (
	"context"
	"fmt"
	"time"

	gcp "cloud.google.com/go/container/apiv1"
	pb "cloud.google.com/go/container/apiv1/containerpb"
	containerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/container/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/protobuf/types/known/durationpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/container/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

func init() {
	registry.RegisterModel(krm.ContainerClusterFieldsGVK, NewClusterFieldsModel)
}

func NewClusterFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &clusterFieldsModel{config: *config}, nil
}

type clusterFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &clusterFieldsModel{}

func (m *clusterFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ContainerClusterFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the cluster is in the status; Config Connector's
		// ContainerCluster may be gone already.
		ref = &containerv1beta1.ContainerClusterRef{External: prev}
	} else if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	id := &containerv1beta1.ContainerClusterIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the cluster of ContainerClusterFields (old=%q, new=%q)", prev, id.String())
	}
	updates, err := clusterUpdates(&obj.Spec)
	if err != nil {
		return nil, err
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.ContainerClusterFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	c, err := newClient(ctx, &m.config)
	if err != nil {
		return nil, fmt.Errorf("building container client: %w", err)
	}
	return &clusterFieldsAdapter{id: id, client: c, updates: updates}, nil
}

func (m *clusterFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

// clusterUpdate is one owned field: whether the cluster has the value
// wanted, and the ClusterUpdate that sets it.
type clusterUpdate struct {
	field  string
	has    func(*pb.Cluster) bool
	update *pb.ClusterUpdate
}

// clusterUpdates turns the spec into one update per owned field, in a
// fixed order. Only fields the spec sets are compared or sent.
func clusterUpdates(s *krm.ContainerClusterFieldsSpec) ([]clusterUpdate, error) {
	var out []clusterUpdate
	if c := s.SecretManagerConfig; c != nil {
		want := &pb.SecretManagerConfig{Enabled: c.Enabled}
		if r := c.RotationConfig; r != nil {
			want.RotationConfig = &pb.SecretManagerConfig_RotationConfig{Enabled: r.Enabled}
			if r.RotationInterval != nil {
				d, err := time.ParseDuration(*r.RotationInterval)
				if err != nil {
					return nil, fmt.Errorf("spec.secretManagerConfig.rotationConfig.rotationInterval: %w", err)
				}
				want.RotationConfig.RotationInterval = durationpb.New(d)
			}
		}
		out = append(out, clusterUpdate{
			field: "secretManagerConfig",
			has: func(cl *pb.Cluster) bool {
				got := cl.GetSecretManagerConfig()
				if want.Enabled != nil && got.GetEnabled() != want.GetEnabled() {
					return false
				}
				wr, gr := want.GetRotationConfig(), got.GetRotationConfig()
				if wr == nil {
					return true
				}
				if wr.Enabled != nil && gr.GetEnabled() != wr.GetEnabled() {
					return false
				}
				return wr.RotationInterval == nil || gr.GetRotationInterval().AsDuration() == wr.GetRotationInterval().AsDuration()
			},
			update: &pb.ClusterUpdate{DesiredSecretManagerConfig: want},
		})
	}
	if c := s.RBACBindingConfig; c != nil {
		want := &pb.RBACBindingConfig{
			EnableInsecureBindingSystemUnauthenticated: c.EnableInsecureBindingSystemUnauthenticated,
			EnableInsecureBindingSystemAuthenticated:   c.EnableInsecureBindingSystemAuthenticated,
		}
		out = append(out, clusterUpdate{
			field: "rbacBindingConfig",
			has: func(cl *pb.Cluster) bool {
				got := cl.GetRbacBindingConfig()
				if want.EnableInsecureBindingSystemUnauthenticated != nil && got.GetEnableInsecureBindingSystemUnauthenticated() != want.GetEnableInsecureBindingSystemUnauthenticated() {
					return false
				}
				return want.EnableInsecureBindingSystemAuthenticated == nil || got.GetEnableInsecureBindingSystemAuthenticated() == want.GetEnableInsecureBindingSystemAuthenticated()
			},
			update: &pb.ClusterUpdate{DesiredRbacBindingConfig: want},
		})
	}
	if c := s.AnonymousAuthenticationConfig; c != nil {
		mode, ok := pb.AnonymousAuthenticationConfig_Mode_value[c.Mode]
		if !ok || mode == 0 {
			return nil, fmt.Errorf("spec.anonymousAuthenticationConfig.mode: unknown mode %q", c.Mode)
		}
		want := &pb.AnonymousAuthenticationConfig{Mode: pb.AnonymousAuthenticationConfig_Mode(mode)}
		out = append(out, clusterUpdate{
			field: "anonymousAuthenticationConfig",
			has: func(cl *pb.Cluster) bool {
				return cl.GetAnonymousAuthenticationConfig().GetMode() == want.GetMode()
			},
			update: &pb.ClusterUpdate{DesiredAnonymousAuthenticationConfig: want},
		})
	}
	if c := s.AddonsConfig; c != nil && c.StatefulHAConfig != nil {
		want := &pb.StatefulHAConfig{Enabled: c.StatefulHAConfig.Enabled}
		out = append(out, clusterUpdate{
			field: "addonsConfig.statefulHaConfig",
			has: func(cl *pb.Cluster) bool {
				return cl.GetAddonsConfig().GetStatefulHaConfig().GetEnabled() == want.GetEnabled()
			},
			// Only this add-on is set: the API leaves the add-ons a request
			// does not name as they are.
			update: &pb.ClusterUpdate{DesiredAddonsConfig: &pb.AddonsConfig{StatefulHaConfig: want}},
		})
	}
	if c := s.NodePoolAutoConfig; c != nil && c.NodeKubeletConfig != nil && c.NodeKubeletConfig.InsecureKubeletReadonlyPortEnabled != nil {
		enabled := *c.NodeKubeletConfig.InsecureKubeletReadonlyPortEnabled
		out = append(out, clusterUpdate{
			field: "nodePoolAutoConfig.nodeKubeletConfig",
			has: func(cl *pb.Cluster) bool {
				got := cl.GetNodePoolAutoConfig().GetNodeKubeletConfig()
				return got != nil && got.InsecureKubeletReadonlyPortEnabled != nil && got.GetInsecureKubeletReadonlyPortEnabled() == enabled
			},
			update: &pb.ClusterUpdate{DesiredNodePoolAutoConfigKubeletConfig: &pb.NodeKubeletConfig{InsecureKubeletReadonlyPortEnabled: &enabled}},
		})
	}
	return out, nil
}

type clusterFieldsAdapter struct {
	id      *containerv1beta1.ContainerClusterIdentity
	client  *gcp.ClusterManagerClient
	updates []clusterUpdate
	actual  *pb.Cluster
}

var _ directbase.Adapter = &clusterFieldsAdapter{}

func (a *clusterFieldsAdapter) Find(ctx context.Context) (bool, error) {
	cl, err := a.client.GetCluster(ctx, &pb.GetClusterRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting GKE cluster %s: %w", a.id, err)
	}
	a.actual = cl
	return true, nil
}

// Create is called only while the cluster does not exist: this kind never
// makes it, so it waits for it.
func (a *clusterFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("ContainerCluster", a.id.String())
}

func (a *clusterFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	for _, u := range a.updates {
		if u.has(a.actual) {
			continue
		}
		klog.FromContext(ctx).V(2).Info("setting a field of a GKE cluster", "name", a.id, "field", u.field)
		op, err := a.client.UpdateCluster(ctx, &pb.UpdateClusterRequest{Name: a.id.String(), Update: u.update})
		if err != nil {
			return fmt.Errorf("setting %s of GKE cluster %s: %w", u.field, a.id, err)
		}
		if err := waitForOperation(ctx, a.client, a.id.Project, a.id.Location, op); err != nil {
			return fmt.Errorf("setting %s of GKE cluster %s: %w", u.field, a.id, err)
		}
		if _, err := a.Find(ctx); err != nil {
			return err
		}
	}
	return updateOp.UpdateStatus(ctx, &krm.ContainerClusterFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}

// Delete leaves the cluster's fields as they are: these settings have no
// "unset" in the API, only other values. It never deletes the cluster.
func (a *clusterFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	return true, nil
}

func (a *clusterFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}
