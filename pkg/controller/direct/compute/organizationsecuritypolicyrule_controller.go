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

// Written here: Config Connector reconciles ComputeOrganizationSecurityPolicyRule
// only through Terraform (google_compute_organization_security_policy_rule,
// from Magic Modules' OrganizationSecurityPolicyRule.yaml, which this
// follows: read, not copied). Its direct mapper maps the rule to
// FirewallPolicyRule, the message of firewallPolicies, not of this API, so
// it is not copied. The shape follows regionnetworkfirewallpolicyrule_controller.go.
//
// A rule is addressed by its priority in the policy:
// organizationSecurityPolicies.getRule reads it, addRule creates it,
// patchRule changes it (every field, sent whole) and removeRule deletes it.
// A FIREWALL rule's direction, logging, targets, destination ranges and
// layer 4 configs are only in the compute beta API (Magic Modules:
// min_version beta), so the controller uses the beta REST client, as
// Terraform does.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	apirefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	computebeta "google.golang.org/api/compute/v0.beta"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

// NewOrganizationSecurityPolicyRuleModel is the model of the kind.
func NewOrganizationSecurityPolicyRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &organizationSecurityPolicyRuleModel{config: *config}, nil
}

type organizationSecurityPolicyRuleModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &organizationSecurityPolicyRuleModel{}

func (m *organizationSecurityPolicyRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeOrganizationSecurityPolicyRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeOrganizationSecurityPolicyRuleIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	priority, err := strconv.ParseInt(id.Priority, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("the priority of ComputeOrganizationSecurityPolicyRule %s: %w", id.String(), err)
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	service, err := computebeta.NewService(ctx, append([]option.ClientOption{}, opts...)...)
	if err != nil {
		return nil, fmt.Errorf("building Compute Engine beta client: %w", err)
	}
	return &organizationSecurityPolicyRuleAdapter{
		service:  service,
		id:       id,
		priority: priority,
		desired:  organizationSecurityPolicyRuleSpecToAPI(&obj.Spec, priority),
	}, nil
}

func (m *organizationSecurityPolicyRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type organizationSecurityPolicyRuleAdapter struct {
	service  *computebeta.Service
	id       *krm.ComputeOrganizationSecurityPolicyRuleIdentity
	priority int64
	desired  *computebeta.SecurityPolicyRule
	actual   *computebeta.SecurityPolicyRule
}

var _ directbase.Adapter = &organizationSecurityPolicyRuleAdapter{}

func (a *organizationSecurityPolicyRuleAdapter) get(ctx context.Context) (*computebeta.SecurityPolicyRule, error) {
	return a.service.OrganizationSecurityPolicies.GetRule(a.id.SecurityPolicy).Priority(a.priority).Context(ctx).Do()
}

func (a *organizationSecurityPolicyRuleAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeOrganizationSecurityPolicyRule", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if organizationSecurityPolicyRuleMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeOrganizationSecurityPolicyRule %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *organizationSecurityPolicyRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeOrganizationSecurityPolicyRule", "name", a.id.String())
	op, err := a.service.OrganizationSecurityPolicies.AddRule(a.id.SecurityPolicy, a.desired).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating ComputeOrganizationSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "create"); err != nil {
		return err
	}
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeOrganizationSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *organizationSecurityPolicyRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	if organizationSecurityPolicyRulesEqual(a.desired, a.actual) {
		log.V(2).Info("no diff detected for ComputeOrganizationSecurityPolicyRule", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	log.V(2).Info("updating ComputeOrganizationSecurityPolicyRule", "name", a.id.String())
	op, err := a.service.OrganizationSecurityPolicies.PatchRule(a.id.SecurityPolicy, a.desired).Priority(a.priority).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating ComputeOrganizationSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "update"); err != nil {
		return err
	}
	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeOrganizationSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, updated)
}

func (a *organizationSecurityPolicyRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *organizationSecurityPolicyRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeOrganizationSecurityPolicyRule", "name", a.id.String())
	op, err := a.service.OrganizationSecurityPolicies.RemoveRule(a.id.SecurityPolicy).Priority(a.priority).Context(ctx).Do()
	if err != nil {
		if organizationSecurityPolicyRuleMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeOrganizationSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "delete"); err != nil {
		if organizationSecurityPolicyRuleMissing(err) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

// wait polls the organization operation until it is done.
func (a *organizationSecurityPolicyRuleAdapter) wait(ctx context.Context, op *computebeta.Operation, what string) error {
	for op.Status != "DONE" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		var err error
		op, err = a.service.GlobalOrganizationOperations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("waiting for %s of ComputeOrganizationSecurityPolicyRule %s: %w", what, a.id.String(), err)
		}
	}
	if op.Error != nil && len(op.Error.Errors) > 0 {
		return fmt.Errorf("%s of ComputeOrganizationSecurityPolicyRule %s: %s: %s", what, a.id.String(), op.Error.Errors[0].Code, op.Error.Errors[0].Message)
	}
	return nil
}

func (a *organizationSecurityPolicyRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *computebeta.SecurityPolicyRule) error {
	status := &krm.ComputeOrganizationSecurityPolicyRuleStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if latest.RuleTupleCount != 0 {
		status.RuleTupleCount = direct.PtrTo(latest.RuleTupleCount)
	}
	return op.UpdateStatus(ctx, status, nil)
}

// organizationSecurityPolicyRuleMissing reports whether err says the rule
// (or its policy) does not exist: a missing policy is a 404; a policy with
// no rule at a priority answers 400, as a firewall policy does (assumed
// for this API; live pending, needs the test folder).
func organizationSecurityPolicyRuleMissing(err error) bool {
	if direct.IsNotFound(err) {
		return true
	}
	var ge *googleapi.Error
	return errors.As(err, &ge) && ge.Code == http.StatusBadRequest && strings.Contains(err.Error(), "does not contain a rule at priority")
}

// organizationSecurityPolicyRuleSpecToAPI is the spec as the beta API's
// rule. Every field is sent, the unset ones as their defaults (direction
// INGRESS, versionedExpr FIREWALL, "", false, []), so that taking a field
// off the spec puts the default back on patchRule.
func organizationSecurityPolicyRuleSpecToAPI(in *krm.ComputeOrganizationSecurityPolicyRuleSpec, priority int64) *computebeta.SecurityPolicyRule {
	out := &computebeta.SecurityPolicyRule{
		Priority:              priority,
		Action:                in.Action,
		Description:           direct.ValueOf(in.Description),
		Direction:             direct.ValueOf(in.Direction),
		EnableLogging:         direct.ValueOf(in.EnableLogging),
		Preview:               direct.ValueOf(in.Preview),
		TargetResources:       in.TargetResources,
		TargetServiceAccounts: in.TargetServiceAccounts,
		Match:                 &computebeta.SecurityPolicyRuleMatcher{Config: &computebeta.SecurityPolicyRuleMatcherConfig{}},
		ForceSendFields:       []string{"Priority", "Description", "EnableLogging", "Preview", "TargetResources", "TargetServiceAccounts"},
	}
	if out.Direction == "" {
		out.Direction = "INGRESS"
	}
	if in.Match != nil {
		out.Match.VersionedExpr = direct.ValueOf(in.Match.VersionedExpr)
		if c := in.Match.Config; c != nil {
			out.Match.Config.SrcIpRanges = c.SrcIPRanges
			out.Match.Config.DestIpRanges = c.DestIPRanges
			for _, l := range c.Layer4Config {
				out.Match.Config.Layer4Configs = append(out.Match.Config.Layer4Configs, &computebeta.SecurityPolicyRuleMatcherConfigLayer4Config{
					IpProtocol: l.IPProtocol,
					Ports:      l.Ports,
				})
			}
		}
	}
	if out.Match.VersionedExpr == "" {
		out.Match.VersionedExpr = "FIREWALL"
	}
	out.Match.Config.ForceSendFields = []string{"SrcIpRanges", "DestIpRanges"}
	return out
}

// organizationSecurityPolicyRulesEqual compares the fields the spec
// manages, in the spelling the API returns: output-only fields (kind,
// ruleTupleCount, ruleNumber) and fields this kind does not have are left
// out; an empty list and a missing one are the same; a target network's
// URL and its relative name are the same; the case of the action,
// direction and protocols is not compared.
func organizationSecurityPolicyRulesEqual(want, have *computebeta.SecurityPolicyRule) bool {
	return organizationSecurityPolicyRuleKey(want) == organizationSecurityPolicyRuleKey(have)
}

func organizationSecurityPolicyRuleKey(r *computebeta.SecurityPolicyRule) string {
	direction := strings.ToUpper(r.Direction)
	if direction == "" {
		direction = "INGRESS"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "priority=%d\naction=%s\ndescription=%q\ndirection=%s\nlogging=%t\npreview=%t\n",
		r.Priority, strings.ToLower(r.Action), r.Description, direction, r.EnableLogging, r.Preview)
	var targets []string
	for _, t := range r.TargetResources {
		targets = append(targets, apirefs.TrimComputeURIPrefix(t))
	}
	fmt.Fprintf(&b, "targetResources=%q\ntargetServiceAccounts=%q\n", targets, lowered(r.TargetServiceAccounts))
	if m := r.Match; m != nil {
		versionedExpr := m.VersionedExpr
		if versionedExpr == "" {
			versionedExpr = "FIREWALL"
		}
		fmt.Fprintf(&b, "versionedExpr=%s\n", versionedExpr)
		if c := m.Config; c != nil {
			fmt.Fprintf(&b, "src=%q\ndest=%q\n", c.SrcIpRanges, c.DestIpRanges)
			for _, l := range c.Layer4Configs {
				fmt.Fprintf(&b, "l4=%s %q\n", strings.ToLower(l.IpProtocol), l.Ports)
			}
		}
	}
	return b.String()
}
