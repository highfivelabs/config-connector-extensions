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

// Written here: IAMBinding, one role on one resource with its members,
// authoritative for that role and condition (Terraform's
// google_*_iam_binding, read, not copied). Create and update are the same:
// read the policy, make the role's binding have exactly the spec's members,
// and write the policy back with the etag it read (retrying when another
// writer changed it meanwhile). Other roles, audit configs and the same
// role under another condition are carried through unchanged. Delete
// removes the binding. A resource that is gone while the binding is being
// deleted leaves nothing to remove.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
)

func init() {
	registry.RegisterModel(krm.IAMBindingGVK, NewBindingModel)
}

func NewBindingModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &bindingModel{config: *config}, nil
}

type bindingModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &bindingModel{}

func (m *bindingModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.IAMBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	t, name, gone, err := resolveResource(ctx, op, obj.Spec.ResourceRef)
	if err != nil {
		return nil, err
	}
	if gone {
		return &bindingAdapter{desired: obj, gone: true}, nil
	}
	id := &krm.IAMBindingIdentity{Resource: name, Role: obj.Spec.Role}
	if obj.Spec.Condition != nil {
		id.ConditionTitle = obj.Spec.Condition.Title
	}
	if ref := direct.ValueOf(obj.Status.ExternalRef); ref != "" && ref != id.String() {
		return nil, fmt.Errorf("cannot change IAMBinding identity (old=%q, new=%q)", ref, id.String())
	}
	var members []string
	if !op.IsDeleting() {
		if members, err = desiredMembers(ctx, op, obj); err != nil {
			return nil, err
		}
	}
	c, err := policyClient(ctx, m.config)
	if err != nil {
		return nil, err
	}
	return &bindingAdapter{id: id, rtype: t, client: c, desired: obj, members: members}, nil
}

// desiredMembers is spec.members and the service accounts of
// spec.membersFrom, as the API writes them.
func desiredMembers(ctx context.Context, op *directbase.AdapterForObjectOperation, obj *krm.IAMBinding) ([]string, error) {
	var members []string
	for _, m := range obj.Spec.Members {
		if strings.TrimSpace(m) == "" {
			return nil, fmt.Errorf("spec.members has an empty member")
		}
		members = append(members, m)
	}
	for i := range obj.Spec.MembersFrom {
		ref := obj.Spec.MembersFrom[i].ServiceAccountRef
		if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		members = append(members, "serviceAccount:"+ref.External)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("a binding needs at least one member")
	}
	return members, nil
}

func (m *bindingModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type bindingAdapter struct {
	id      *krm.IAMBindingIdentity
	rtype   *iampolicy.ResourceType
	client  *iampolicy.Client
	desired *krm.IAMBinding
	members []string
	gone    bool

	policy *iampb.Policy
}

var _ directbase.Adapter = &bindingAdapter{}

func (a *bindingAdapter) condition() *iampolicy.Condition {
	return conditionOf(a.desired.Spec.Condition)
}

func (a *bindingAdapter) Find(ctx context.Context) (bool, error) {
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
	a.policy = p
	_, found := iampolicy.Members(p, a.desired.Spec.Role, a.condition())
	return found, nil
}

// apply makes the binding have exactly the spec's members.
func (a *bindingAdapter) apply(ctx context.Context) error {
	_, err := a.client.Modify(ctx, a.rtype, a.id.Resource, func(p *iampb.Policy) bool {
		return iampolicy.SetMembers(p, a.desired.Spec.Role, a.condition(), a.members)
	})
	return err
}

func (a *bindingAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("adding IAMBinding", "name", a.id)
	if err := a.apply(ctx); err != nil {
		return err
	}
	return a.updateStatus(ctx, createOp)
}

func (a *bindingAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	// Find read the policy; write only if it differs from the spec.
	if a.policy == nil || iampolicy.SetMembers(proto.Clone(a.policy).(*iampb.Policy), a.desired.Spec.Role, a.condition(), a.members) {
		klog.FromContext(ctx).V(2).Info("updating IAMBinding", "name", a.id)
		if err := a.apply(ctx); err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, updateOp)
}

func (a *bindingAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	if a.gone {
		return false, nil
	}
	klog.FromContext(ctx).V(2).Info("removing IAMBinding", "name", a.id)
	_, err := a.client.Modify(ctx, a.rtype, a.id.Resource, func(p *iampb.Policy) bool {
		return iampolicy.Remove(p, a.desired.Spec.Role, a.condition())
	})
	if err != nil {
		if iampolicy.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *bindingAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *bindingAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	applied := append([]string(nil), a.members...)
	sort.Strings(applied)
	return op.UpdateStatus(ctx, &krm.IAMBindingStatus{
		ExternalRef:    direct.LazyPtr(a.id.String()),
		AppliedMembers: applied,
	}, nil)
}
