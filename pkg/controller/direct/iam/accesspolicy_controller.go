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

// Written by High Five Labs for config-connector-extensions after our
// principalaccessboundarypolicy_controller.go (copied from Config
// Connector): an IAM v3 access policy (IAMAccessPolicy) of an organization,
// folder or project. Config Connector has the alpha types (organization
// only) and no controller. The API's update replaces the policy (it has no
// update mask), so a difference in the display name, annotations or details
// sends the whole spec. The spec maps to the proto through JSON: upstream's
// details type has the proto's JSON names.

package iam

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/iam/apiv3"
	iampb "cloud.google.com/go/iam/apiv3/iampb"
	kcciamv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
)

func init() {
	registry.RegisterModel(krm.IAMAccessPolicyGVK, NewAccessPolicyModel)
}

func NewAccessPolicyModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelAccessPolicy{config: *config}, nil
}

var _ directbase.Model = &modelAccessPolicy{}

type modelAccessPolicy struct {
	config config.ControllerConfig
}

func (m *modelAccessPolicy) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.IAMAccessPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	id := &krm.IAMAccessPolicyIdentity{}
	if ext := direct.ValueOf(obj.Status.ExternalRef); !obj.GetDeletionTimestamp().IsZero() && ext != "" {
		// Deleting: the name is in status, so a parent object already gone
		// does not hold the delete.
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
	} else {
		i, err := obj.GetIdentity(ctx, op.Reader)
		if err != nil {
			return nil, err
		}
		id = i.(*krm.IAMAccessPolicyIdentity)
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp.NewAccessPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building AccessPolicies client: %w", err)
	}
	return &AccessPolicyAdapter{id: id, gcpClient: gcpClient, desired: obj}, nil
}

func (m *modelAccessPolicy) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type AccessPolicyAdapter struct {
	id        *krm.IAMAccessPolicyIdentity
	gcpClient *gcp.AccessPoliciesClient
	desired   *krm.IAMAccessPolicy
	actual    *iampb.AccessPolicy
}

var _ directbase.Adapter = &AccessPolicyAdapter{}

func (a *AccessPolicyAdapter) Find(ctx context.Context) (bool, error) {
	got, err := a.gcpClient.GetAccessPolicy(ctx, &iampb.GetAccessPolicyRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting AccessPolicy %q: %w", a.id, err)
	}
	a.actual = got
	return true, nil
}

// accessPolicyFromSpec is the policy the spec asks for: display name,
// annotations and details.
func accessPolicyFromSpec(spec *krm.IAMAccessPolicySpec) (*iampb.AccessPolicy, error) {
	out := &iampb.AccessPolicy{DisplayName: direct.ValueOf(spec.DisplayName), Annotations: spec.Annotations}
	if spec.Details != nil {
		b, err := json.Marshal(spec.Details)
		if err != nil {
			return nil, err
		}
		out.Details = &iampb.AccessPolicyDetails{}
		if err := protojson.Unmarshal(b, out.Details); err != nil {
			return nil, fmt.Errorf("spec.details: %w", err)
		}
	}
	return out, nil
}

// specOf is the part of a policy the spec owns, to compare.
func specOf(p *iampb.AccessPolicy) *iampb.AccessPolicy {
	out := &iampb.AccessPolicy{DisplayName: p.GetDisplayName(), Annotations: p.GetAnnotations(), Details: p.GetDetails()}
	if len(out.Annotations) == 0 {
		out.Annotations = nil
	}
	if len(out.GetDetails().GetRules()) == 0 {
		out.Details = nil
	}
	return out
}

func (a *AccessPolicyAdapter) status(p *iampb.AccessPolicy) *krm.IAMAccessPolicyStatus {
	s := &krm.IAMAccessPolicyStatus{ExternalRef: direct.LazyPtr(a.id.String()), Etag: direct.LazyPtr(p.GetEtag())}
	obs := &kcciamv1alpha1.IAMAccessPolicyObservedState{Uid: direct.LazyPtr(p.GetUid())}
	if p.GetCreateTime() != nil {
		obs.CreateTime = direct.LazyPtr(p.GetCreateTime().AsTime().UTC().Format("2006-01-02T15:04:05.000000Z"))
	}
	if p.GetUpdateTime() != nil {
		obs.UpdateTime = direct.LazyPtr(p.GetUpdateTime().AsTime().UTC().Format("2006-01-02T15:04:05.000000Z"))
	}
	s.ObservedState = obs
	return s
}

func (a *AccessPolicyAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating AccessPolicy", "name", a.id)
	resource, err := accessPolicyFromSpec(&a.desired.Spec)
	if err != nil {
		return err
	}
	op, err := a.gcpClient.CreateAccessPolicy(ctx, &iampb.CreateAccessPolicyRequest{
		Parent:         a.id.ParentString(),
		AccessPolicyId: a.id.AccessPolicy,
		AccessPolicy:   resource,
	})
	if err != nil {
		return fmt.Errorf("creating AccessPolicy %s: %w", a.id, err)
	}
	created, err := op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("AccessPolicy %s waiting creation: %w", a.id, err)
	}
	return createOp.UpdateStatus(ctx, a.status(created), nil)
}

func (a *AccessPolicyAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	desired, err := accessPolicyFromSpec(&a.desired.Spec)
	if err != nil {
		return err
	}
	updated := a.actual
	if !proto.Equal(specOf(desired), specOf(a.actual)) {
		klog.FromContext(ctx).V(2).Info("updating AccessPolicy", "name", a.id)
		desired.Name = a.id.String()
		desired.Etag = a.actual.GetEtag()
		op, err := a.gcpClient.UpdateAccessPolicy(ctx, &iampb.UpdateAccessPolicyRequest{AccessPolicy: desired})
		if err != nil {
			return fmt.Errorf("updating AccessPolicy %s: %w", a.id, err)
		}
		if updated, err = op.Wait(ctx); err != nil {
			return fmt.Errorf("AccessPolicy %s waiting update: %w", a.id, err)
		}
	}
	return updateOp.UpdateStatus(ctx, a.status(updated), nil)
}

func (a *AccessPolicyAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("IAMAccessPolicy does not support export")
}

// Delete deletes the policy. The API refuses a policy a binding names
// (without force, which this never sends): the delete is retried until the
// binding is gone.
func (a *AccessPolicyAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	op, err := a.gcpClient.DeleteAccessPolicy(ctx, &iampb.DeleteAccessPolicyRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting AccessPolicy %s: %w", a.id, err)
	}
	if err := op.Wait(ctx); err != nil {
		if direct.IsNotFound(err) || strings.Contains(err.Error(), "missing \"value\" field") {
			return true, nil
		}
		return false, fmt.Errorf("waiting delete AccessPolicy %s: %w", a.id, err)
	}
	return true, nil
}
