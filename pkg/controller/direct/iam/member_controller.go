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

package iam

// Written here: IAMMember, one member of one role on one resource,
// additive (Terraform's google_*_iam_member, read, not copied). Create and
// update are the same: read the policy, add the member to the role's
// binding if it is not there, and write the policy back with the etag it
// read (retrying when another writer changed it meanwhile). Every other
// member, role and audit config is carried through unchanged. Delete
// removes only this member (and the binding, if it was the last). A
// resource that is gone while the object is being deleted leaves nothing
// to remove.

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
)

func init() {
	registry.RegisterModel(krm.IAMMemberGVK, NewMemberModel)
}

func NewMemberModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &memberModel{config: *config}, nil
}

type memberModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &memberModel{}

func (m *memberModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.IAMMember{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	t, name, gone, err := resolveResource(ctx, op, policyRef(obj.Spec.ResourceRef))
	if err != nil {
		return nil, err
	}
	if gone {
		return &memberAdapter{desired: obj, gone: true}, nil
	}
	member, err := desiredMember(ctx, op, obj)
	if err != nil {
		return nil, err
	}
	id := &krm.IAMMemberIdentity{Resource: name, Role: obj.Spec.Role, Member: member}
	if obj.Spec.Condition != nil {
		id.ConditionTitle = obj.Spec.Condition.Title
	}
	if ref := direct.ValueOf(obj.Status.ExternalRef); ref != "" && ref != id.String() {
		return nil, fmt.Errorf("cannot change IAMMember identity (old=%q, new=%q)", ref, id.String())
	}
	c, err := policyClient(ctx, m.config)
	if err != nil {
		return nil, err
	}
	return &memberAdapter{id: id, rtype: t, client: c, desired: obj}, nil
}

// desiredMember is spec.member, or the service account of spec.memberFrom,
// as the API writes it.
func desiredMember(ctx context.Context, op *directbase.AdapterForObjectOperation, obj *krm.IAMMember) (string, error) {
	switch {
	case obj.Spec.Member != nil && obj.Spec.MemberFrom == nil:
		if strings.TrimSpace(*obj.Spec.Member) == "" {
			return "", fmt.Errorf("spec.member is empty")
		}
		return *obj.Spec.Member, nil
	case obj.Spec.MemberFrom != nil && obj.Spec.Member == nil:
		ref := obj.Spec.MemberFrom.ServiceAccountRef
		if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return "", err
		}
		return "serviceAccount:" + ref.External, nil
	}
	return "", fmt.Errorf("exactly one of spec.member and spec.memberFrom is required")
}

func (m *memberModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type memberAdapter struct {
	id      *krm.IAMMemberIdentity
	rtype   *iampolicy.ResourceType
	client  *iampolicy.Client
	desired *krm.IAMMember
	gone    bool
}

var _ directbase.Adapter = &memberAdapter{}

func (a *memberAdapter) condition() *iampolicy.Condition {
	return conditionOf(a.desired.Spec.Condition)
}

// Find reports whether the member is in the role's binding.
func (a *memberAdapter) Find(ctx context.Context) (bool, error) {
	if a.gone {
		return false, nil
	}
	p, err := a.client.Get(ctx, a.rtype, a.id.Resource)
	if err != nil {
		if iampolicy.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading the IAM policy of %s: %w", a.id.Resource, err)
	}
	// AddMembers on a copy says whether the member is missing.
	return !iampolicy.AddMembers(clonePolicy(p), a.desired.Spec.Role, a.condition(), []string{a.id.Member}), nil
}

func (a *memberAdapter) add(ctx context.Context) error {
	_, err := a.client.Modify(ctx, a.rtype, a.id.Resource, func(p *iampb.Policy) bool {
		return iampolicy.AddMembers(p, a.desired.Spec.Role, a.condition(), []string{a.id.Member})
	})
	return err
}

func (a *memberAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("adding IAMMember", "name", a.id)
	if err := a.add(ctx); err != nil {
		return err
	}
	return a.updateStatus(ctx, createOp)
}

// Update runs when Find saw the member; every field is immutable, so there
// is nothing to change.
func (a *memberAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	return a.updateStatus(ctx, updateOp)
}

func (a *memberAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	if a.gone {
		return false, nil
	}
	klog.FromContext(ctx).V(2).Info("removing IAMMember", "name", a.id)
	_, err := a.client.Modify(ctx, a.rtype, a.id.Resource, func(p *iampb.Policy) bool {
		return iampolicy.RemoveMembers(p, a.desired.Spec.Role, a.condition(), []string{a.id.Member})
	})
	if err != nil {
		if iampolicy.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *memberAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *memberAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.IAMMemberStatus{
		ExternalRef: direct.LazyPtr(a.id.String()),
	}, nil)
}
