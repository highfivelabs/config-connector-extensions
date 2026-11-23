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

package dns

// Written here: DNSPolicyFields, a field kind (docs/design.md section 6.6,
// pattern 2; decision 37). No Config Connector code or pull request sets a
// DNS server policy's dns64Config. The policy is read with policies.get and
// written with policies.patch, a partial update whose body has only
// dns64Config, so no other field of the policy is sent. Config Connector's
// DNSPolicy runs Terraform by default, whose patch sends the policy's other
// fields (alternativeNameServerConfig, description,
// enableInboundForwarding, enableLogging, networks) and never dns64Config,
// so it leaves it alone. Its opt-in direct controller sends policies.update
// (a whole replace) and would clear it (docs/kinds/DNSPolicyFields.md).

import (
	"context"
	"fmt"

	dnsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/dns/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	api "google.golang.org/api/dns/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/dns/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewPolicyFieldsModel is the DNSPolicyFields model.
func NewPolicyFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &policyFieldsModel{config: *config}, nil
}

type policyFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &policyFieldsModel{}

func (m *policyFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.DNSPolicyFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	var allQueries *bool
	if c := obj.Spec.Dns64Config; c != nil && c.Scope != nil {
		allQueries = c.Scope.AllQueries
	}
	if allQueries == nil {
		return nil, fmt.Errorf("spec.dns64Config.scope.allQueries is required")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if fieldkind.Deleting(obj, prev) {
		return fieldkind.Gone{}, nil
	}
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the policy is in status; Config Connector's DNSPolicy
		// may be gone already.
		ref = &dnsv1beta1.DNSPolicyRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &dnsv1beta1.DNSPolicyIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the policy of DNSPolicyFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.DNSPolicyFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building dns client: %w", err)
	}
	return &policyFieldsAdapter{id: id, gcpClient: c, desired: *allQueries}, nil
}

func (m *policyFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type policyFieldsAdapter struct {
	id        *dnsv1beta1.DNSPolicyIdentity
	gcpClient *api.Service
	desired   bool
	actual    *api.Policy
}

var _ directbase.Adapter = &policyFieldsAdapter{}

// Find reports whether the policy exists. A missing one is "not found" so
// that a delete finishes when it is gone; a create then waits (Create).
func (a *policyFieldsAdapter) Find(ctx context.Context) (bool, error) {
	p, err := a.gcpClient.Policies.Get(a.id.Project, a.id.Policy).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting DNS policy %s: %w", a.id, err)
	}
	a.actual = p
	return true, nil
}

// Create is called only while the policy does not exist: this kind never
// makes it, so it waits for it.
func (a *policyFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("DNSPolicy", a.id.String())
}

func actualAllQueries(p *api.Policy) bool {
	return p != nil && p.Dns64Config != nil && p.Dns64Config.Scope != nil && p.Dns64Config.Scope.AllQueries
}

// patch sets dns64Config.scope.allQueries, and nothing else.
func (a *policyFieldsAdapter) patch(ctx context.Context, allQueries bool) error {
	body := &api.Policy{Dns64Config: &api.PolicyDns64Config{Scope: &api.PolicyDns64ConfigScope{
		AllQueries:      allQueries,
		ForceSendFields: []string{"AllQueries"},
	}}}
	resp, err := a.gcpClient.Policies.Patch(a.id.Project, a.id.Policy, body).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("setting dns64Config of DNS policy %s: %w", a.id, err)
	}
	a.actual = resp.Policy
	return nil
}

func (a *policyFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if actualAllQueries(a.actual) != a.desired || a.actual.Dns64Config == nil {
		klog.FromContext(ctx).V(2).Info("setting dns64Config of a DNS policy", "name", a.id)
		if err := a.patch(ctx, a.desired); err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, updateOp)
}

// Delete turns DNS64 off (allQueries false, the policy's state before this
// object), and never deletes the policy. A policy that is gone is not
// reached: Find said not found.
func (a *policyFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	if !actualAllQueries(a.actual) {
		return true, nil
	}
	klog.FromContext(ctx).V(2).Info("turning off DNS64 of a DNS policy", "name", a.id)
	if err := a.patch(ctx, false); err != nil {
		return false, err
	}
	return true, nil
}

func (a *policyFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *policyFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.DNSPolicyFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}
