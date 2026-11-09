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

// Written here: one key of a project's common instance metadata. The
// controller follows Terraform's google_compute_project_metadata_item
// (read, not copied): read the project's metadata, change the one key,
// write the whole metadata back with the fingerprint that was read, and
// read again and retry when another writer got there first (412). Unlike
// Terraform, a key that is already there is taken over, as Config
// Connector takes over existing resources.

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/googleapis/gax-go/v2/apierror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeProjectMetadataItemGVK, NewProjectMetadataItemModel)
}

func NewProjectMetadataItemModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &metadataItemModel{config: *config}, nil
}

type metadataItemModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &metadataItemModel{}

func (m *metadataItemModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeProjectMetadataItem{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if err := common.NormalizeReferences(ctx, op.Reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}
	id, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewProjectsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building compute projects client: %w", err)
	}
	return &metadataItemAdapter{id: id.(*krm.ComputeProjectMetadataItemIdentity), gcpClient: c, desired: obj}, nil
}

func (m *metadataItemModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type metadataItemAdapter struct {
	id        *krm.ComputeProjectMetadataItemIdentity
	gcpClient *gcp.ProjectsClient
	desired   *krm.ComputeProjectMetadataItem
	actual    *string
}

var _ directbase.Adapter = &metadataItemAdapter{}

// metadata reads the project's common instance metadata.
func (a *metadataItemAdapter) metadata(ctx context.Context) (*pb.Metadata, error) {
	p, err := a.gcpClient.Get(ctx, &pb.GetProjectRequest{Project: a.id.Project})
	if err != nil {
		return nil, fmt.Errorf("reading project %s: %w", a.id.Project, err)
	}
	if p.CommonInstanceMetadata == nil {
		return &pb.Metadata{}, nil
	}
	return p.CommonInstanceMetadata, nil
}

func (a *metadataItemAdapter) Find(ctx context.Context) (bool, error) {
	md, err := a.metadata(ctx)
	if err != nil {
		return false, err
	}
	for _, item := range md.GetItems() {
		if item.GetKey() == a.id.Key {
			a.actual = direct.PtrTo(item.GetValue())
			return true, nil
		}
	}
	return false, nil
}

// set writes the key's value, or removes the key when value is nil, and
// retries on a fingerprint conflict with another writer.
func (a *metadataItemAdapter) set(ctx context.Context, value *string) error {
	const attempts = 5
	for i := 0; ; i++ {
		md, err := a.metadata(ctx)
		if err != nil {
			return err
		}
		var items []*pb.Items
		found := false
		for _, item := range md.GetItems() {
			if item.GetKey() != a.id.Key {
				items = append(items, item)
				continue
			}
			found = true
			if value != nil && item.GetValue() == *value {
				return nil
			}
		}
		if value == nil && !found {
			return nil
		}
		if value != nil {
			items = append(items, &pb.Items{Key: direct.PtrTo(a.id.Key), Value: direct.PtrTo(*value)})
		}
		op, err := a.gcpClient.SetCommonInstanceMetadata(ctx, &pb.SetCommonInstanceMetadataProjectRequest{
			Project:          a.id.Project,
			MetadataResource: &pb.Metadata{Fingerprint: md.Fingerprint, Items: items},
		})
		if err == nil {
			err = op.Wait(ctx)
		}
		if err == nil {
			return nil
		}
		var ae *apierror.APIError
		if i+1 < attempts && errors.As(err, &ae) && ae.HTTPCode() == http.StatusPreconditionFailed {
			klog.FromContext(ctx).V(2).Info("project metadata changed under us; reading it again", "name", a.id)
			continue
		}
		return fmt.Errorf("setting project %s's metadata: %w", a.id.Project, err)
	}
}

func (a *metadataItemAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("adding ComputeProjectMetadataItem", "name", a.id)
	if err := a.set(ctx, direct.PtrTo(a.desired.Spec.Value)); err != nil {
		return err
	}
	return a.updateStatus(ctx, createOp)
}

func (a *metadataItemAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if a.actual == nil || *a.actual != a.desired.Spec.Value {
		klog.FromContext(ctx).V(2).Info("updating ComputeProjectMetadataItem", "name", a.id)
		if err := a.set(ctx, direct.PtrTo(a.desired.Spec.Value)); err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, updateOp)
}

func (a *metadataItemAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("removing ComputeProjectMetadataItem", "name", a.id)
	if err := a.set(ctx, nil); err != nil {
		return false, err
	}
	return true, nil
}

func (a *metadataItemAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *metadataItemAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.ComputeProjectMetadataItemStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}
