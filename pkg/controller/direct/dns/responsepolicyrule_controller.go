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

// DNSResponsePolicyRule's controller, written here after the response
// policy's (responsepolicy_controller.go): Config Connector has the kind's
// types, identity and mappers but only a Terraform controller. A rule is
// created with its ruleName in the body, read and deleted by name, and
// written whole with update (PUT), so a rule that moves from behavior to
// localData, or back, keeps nothing of the old answer.

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/diffs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"
	api "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/dns/v1beta1"
)

// NewResponsePolicyRuleModel is the DNSResponsePolicyRule model.
func NewResponsePolicyRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &responsePolicyRuleModel{config: config}, nil
}

var _ directbase.Model = &responsePolicyRuleModel{}

type responsePolicyRuleModel struct {
	config *config.ControllerConfig
}

func (m *responsePolicyRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.DNSResponsePolicyRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.DNSResponsePolicyRuleIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	if ext := direct.ValueOf(obj.Status.ExternalRef); ext != "" && ext != id.String() {
		return nil, fmt.Errorf("cannot change DNSResponsePolicyRule identity (old=%q, new=%q)", ext, id.String())
	}

	mapCtx := &direct.MapContext{}
	desired := DNSResponsePolicyRuleSpec_ToAPI(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}
	desired.RuleName = id.Rule

	var opts []option.ClientOption
	opts, err = m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building DNS client: %w", err)
	}
	return &DNSResponsePolicyRuleAdapter{id: id, gcpClient: gcpClient, desired: desired}, nil
}

func (m *responsePolicyRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type DNSResponsePolicyRuleAdapter struct {
	id        *krm.DNSResponsePolicyRuleIdentity
	gcpClient *api.Service
	desired   *api.ResponsePolicyRule
	actual    *api.ResponsePolicyRule
}

var _ directbase.Adapter = &DNSResponsePolicyRuleAdapter{}

func (a *DNSResponsePolicyRuleAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting DNSResponsePolicyRule", "name", a.id)
	got, err := a.gcpClient.ResponsePolicyRules.Get(a.id.Project, a.id.ResponsePolicy, a.id.Rule).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting DNSResponsePolicyRule %s: %w", a.id, err)
	}
	a.actual = got
	return true, nil
}

func (a *DNSResponsePolicyRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating DNSResponsePolicyRule", "name", a.id)
	created, err := a.gcpClient.ResponsePolicyRules.Create(a.id.Project, a.id.ResponsePolicy, a.desired).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating DNSResponsePolicyRule %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *DNSResponsePolicyRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	diff, err := compareResponsePolicyRule(ctx, a.actual, a.desired)
	if err != nil {
		return err
	}
	if !diff.HasDiff() {
		log.V(2).Info("no field needs update", "name", a.id)
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	diff.Object = updateOp.GetUnstructured()
	structuredreporting.ReportDiff(ctx, diff)

	resp, err := a.gcpClient.ResponsePolicyRules.Update(a.id.Project, a.id.ResponsePolicy, a.id.Rule, a.desired).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating DNSResponsePolicyRule %s: %w", a.id, err)
	}
	log.V(2).Info("successfully updated DNSResponsePolicyRule", "name", a.id)
	latest := resp.ResponsePolicyRule
	if latest == nil {
		latest = a.desired
	}
	return a.updateStatus(ctx, updateOp, latest)
}

// comparableRule is a rule as the spec says it: the answer fields only,
// without kind or server fields, and an empty list or TTL as unset.
func comparableRule(in *api.ResponsePolicyRule) *api.ResponsePolicyRule {
	out := &api.ResponsePolicyRule{Behavior: in.Behavior, DnsName: in.DnsName, RuleName: in.RuleName}
	if in.Behavior == "behaviorUnspecified" {
		out.Behavior = ""
	}
	if in.LocalData != nil && len(in.LocalData.LocalDatas) > 0 {
		out.LocalData = &api.ResponsePolicyRuleLocalData{}
		for _, rr := range in.LocalData.LocalDatas {
			out.LocalData.LocalDatas = append(out.LocalData.LocalDatas, &api.ResourceRecordSet{
				Name: rr.Name, Type: rr.Type, Ttl: rr.Ttl, Rrdatas: rr.Rrdatas,
			})
		}
	}
	return out
}

func compareResponsePolicyRule(ctx context.Context, actual, desired *api.ResponsePolicyRule) (*structuredreporting.Diff, error) {
	a := comparableRule(actual)
	d := comparableRule(desired)
	// A TTL the spec leaves out is the API's default.
	if a.LocalData != nil && d.LocalData != nil && len(a.LocalData.LocalDatas) == len(d.LocalData.LocalDatas) {
		for i, rr := range d.LocalData.LocalDatas {
			if rr.Ttl == 0 {
				rr.Ttl = a.LocalData.LocalDatas[i].Ttl
			}
		}
	}
	diff, _, err := diffs.GoogleAPI.Diff(ctx, a, d)
	return diff, err
}

func (a *DNSResponsePolicyRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *api.ResponsePolicyRule) error {
	status := &krm.DNSResponsePolicyRuleStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	return op.UpdateStatus(ctx, status, nil)
}

func (a *DNSResponsePolicyRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("export of DNSResponsePolicyRule is not supported")
}

func (a *DNSResponsePolicyRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting DNSResponsePolicyRule", "name", a.id)
	err := a.gcpClient.ResponsePolicyRules.Delete(a.id.Project, a.id.ResponsePolicy, a.id.Rule).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting DNSResponsePolicyRule %s: %w", a.id, err)
	}
	return true, nil
}
