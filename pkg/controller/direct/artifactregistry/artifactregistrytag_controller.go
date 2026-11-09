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

package artifactregistry

// Written here: a direct controller for ArtifactRegistryTag, from the
// Artifact Registry API (tags.create, get, patch with update mask
// "version", delete), on the client the repository controller uses. No
// Config Connector code, pull request or Terraform resource has tags.

import (
	"context"
	"fmt"

	gcp "cloud.google.com/go/artifactregistry/apiv1"
	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ArtifactRegistryTagGVK, NewArtifactRegistryTagModel)
}

func NewArtifactRegistryTagModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelTag{config: *config}, nil
}

type modelTag struct {
	config config.ControllerConfig
}

var _ directbase.Model = &modelTag{}

func (m *modelTag) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ArtifactRegistryTag{}
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
	c, err := gcp.NewRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building ArtifactRegistryTag client: %w", err)
	}
	return &tagAdapter{id: id.(*krm.ArtifactRegistryTagIdentity), gcpClient: c, desired: obj}, nil
}

func (m *modelTag) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type tagAdapter struct {
	id        *krm.ArtifactRegistryTagIdentity
	gcpClient *gcp.Client
	desired   *krm.ArtifactRegistryTag
	actual    *pb.Tag
}

var _ directbase.Adapter = &tagAdapter{}

func (a *tagAdapter) Find(ctx context.Context) (bool, error) {
	actual, err := a.gcpClient.GetTag(ctx, &pb.GetTagRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ArtifactRegistryTag %s: %w", a.id, err)
	}
	a.actual = actual
	return true, nil
}

func (a *tagAdapter) desiredVersion() string {
	return a.id.VersionName(a.desired.Spec.Version)
}

func (a *tagAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ArtifactRegistryTag", "name", a.id)
	created, err := a.gcpClient.CreateTag(ctx, &pb.CreateTagRequest{
		Parent: a.id.PackageName(),
		TagId:  a.id.Tag,
		Tag:    &pb.Tag{Version: a.desiredVersion()},
	})
	if err != nil {
		return fmt.Errorf("creating ArtifactRegistryTag %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *tagAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	latest := a.actual
	if a.actual.GetVersion() != a.desiredVersion() {
		klog.FromContext(ctx).V(2).Info("moving ArtifactRegistryTag", "name", a.id, "from", a.actual.GetVersion(), "to", a.desiredVersion())
		updated, err := a.gcpClient.UpdateTag(ctx, &pb.UpdateTagRequest{
			Tag:        &pb.Tag{Name: a.id.String(), Version: a.desiredVersion()},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"version"}},
		})
		if err != nil {
			return fmt.Errorf("updating ArtifactRegistryTag %s: %w", a.id, err)
		}
		latest = updated
	}
	return a.updateStatus(ctx, updateOp, latest)
}

func (a *tagAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ArtifactRegistryTag", "name", a.id)
	if err := a.gcpClient.DeleteTag(ctx, &pb.DeleteTagRequest{Name: a.id.String()}); err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ArtifactRegistryTag %s: %w", a.id, err)
	}
	return true, nil
}

func (a *tagAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *tagAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.Tag) error {
	status := &krm.ArtifactRegistryTagStatus{
		ExternalRef:     direct.LazyPtr(a.id.String()),
		ObservedVersion: direct.LazyPtr(latest.GetVersion()),
	}
	return op.UpdateStatus(ctx, status, nil)
}
