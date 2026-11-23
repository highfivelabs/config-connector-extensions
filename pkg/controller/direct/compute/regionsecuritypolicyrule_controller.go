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

// Written here: Config Connector has no regional security policy rule (its
// ComputeSecurityPolicy keeps a global policy's rules in spec.rule, and its
// opt-in direct controller sends them in the policy's own insert and patch).
// This follows Terraform's google_compute_region_security_policy_rule (read,
// not copied): a rule is addressed by its priority in the policy;
// regionSecurityPolicies.getRule reads it, addRule creates it, patchRule
// changes the fields its update mask names (Terraform's mask: the changed
// top-level fields, and network_match's eight leaves), and removeRule
// deletes it. The shape follows organizationsecuritypolicyrule_controller.go;
// the WAF exclusions map as upstream's computesecuritypolicy_mappings.go
// maps them (Terraform's names in the spec, the API's on the wire).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	computev1 "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeRegionSecurityPolicyRuleGVK, NewRegionSecurityPolicyRuleModel)
}

// regionSecurityPolicyDefaultRulePriority is the default rule's priority.
// The API adds the rule to a policy and never removes it, so deleting an
// object for it leaves the rule (as it is then).
const regionSecurityPolicyDefaultRulePriority = 2147483647

// NewRegionSecurityPolicyRuleModel is the model of the kind.
func NewRegionSecurityPolicyRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &regionSecurityPolicyRuleModel{config: *config}, nil
}

type regionSecurityPolicyRuleModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &regionSecurityPolicyRuleModel{}

func (m *regionSecurityPolicyRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeRegionSecurityPolicyRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	id := &krm.ComputeRegionSecurityPolicyRuleIdentity{}
	if ext := direct.ValueOf(obj.Status.ExternalRef); op.IsDeleting() && ext != "" {
		// Deleting a rule that was made: its name is in status, so the
		// policy is not resolved. A policy object already gone (everything
		// deleted at once) would otherwise hold the delete forever.
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
	} else {
		idVal, err := obj.GetIdentity(ctx, op.Reader)
		if err != nil {
			return nil, err
		}
		got, ok := idVal.(*krm.ComputeRegionSecurityPolicyRuleIdentity)
		if !ok {
			return nil, fmt.Errorf("unexpected identity type: %T", idVal)
		}
		id = got
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	service, err := computev1.NewService(ctx, append([]option.ClientOption{}, opts...)...)
	if err != nil {
		return nil, fmt.Errorf("building Compute Engine client: %w", err)
	}
	return &regionSecurityPolicyRuleAdapter{
		service: service,
		id:      id,
		spec:    &obj.Spec,
		desired: regionSecurityPolicyRuleSpecToAPI(&obj.Spec, id.Priority),
	}, nil
}

func (m *regionSecurityPolicyRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type regionSecurityPolicyRuleAdapter struct {
	service *computev1.Service
	id      *krm.ComputeRegionSecurityPolicyRuleIdentity
	spec    *krm.ComputeRegionSecurityPolicyRuleSpec
	desired *computev1.SecurityPolicyRule
	actual  *computev1.SecurityPolicyRule
}

var _ directbase.Adapter = &regionSecurityPolicyRuleAdapter{}

func (a *regionSecurityPolicyRuleAdapter) policy() (string, string, string) {
	return a.id.Policy.Project, a.id.Policy.Region, a.id.Policy.Name
}

func (a *regionSecurityPolicyRuleAdapter) get(ctx context.Context) (*computev1.SecurityPolicyRule, error) {
	project, region, policy := a.policy()
	return a.service.RegionSecurityPolicies.GetRule(project, region, policy).Priority(a.id.Priority).Context(ctx).Do()
}

func (a *regionSecurityPolicyRuleAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeRegionSecurityPolicyRule", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if regionSecurityPolicyRuleMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeRegionSecurityPolicyRule %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *regionSecurityPolicyRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeRegionSecurityPolicyRule", "name", a.id.String())
	project, region, policy := a.policy()
	op, err := a.service.RegionSecurityPolicies.AddRule(project, region, policy, a.desired).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating ComputeRegionSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "create"); err != nil {
		return err
	}
	return a.updateStatus(ctx, createOp)
}

func (a *regionSecurityPolicyRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	mask := regionSecurityPolicyRuleUpdateMask(a.spec, a.actual)
	if len(mask) == 0 {
		log.V(2).Info("no diff detected for ComputeRegionSecurityPolicyRule", "name", a.id.String())
		return a.updateStatus(ctx, updateOp)
	}
	log.V(2).Info("updating ComputeRegionSecurityPolicyRule", "name", a.id.String(), "updateMask", mask)
	project, region, policy := a.policy()
	op, err := a.service.RegionSecurityPolicies.PatchRule(project, region, policy, a.desired).
		Priority(a.id.Priority).UpdateMask(strings.Join(mask, ",")).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating ComputeRegionSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "update"); err != nil {
		return err
	}
	return a.updateStatus(ctx, updateOp)
}

func (a *regionSecurityPolicyRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *regionSecurityPolicyRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	if a.id.Priority == regionSecurityPolicyDefaultRulePriority {
		log.V(2).Info("leaving the default rule of the policy (the API does not remove it)", "name", a.id.String())
		return true, nil
	}
	log.V(2).Info("deleting ComputeRegionSecurityPolicyRule", "name", a.id.String())
	project, region, policy := a.policy()
	op, err := a.service.RegionSecurityPolicies.RemoveRule(project, region, policy).Priority(a.id.Priority).Context(ctx).Do()
	if err != nil {
		if regionSecurityPolicyRuleMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeRegionSecurityPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "delete"); err != nil {
		if regionSecurityPolicyRuleMissing(err) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

// wait polls the regional operation until it is done.
func (a *regionSecurityPolicyRuleAdapter) wait(ctx context.Context, op *computev1.Operation, what string) error {
	project, region, _ := a.policy()
	for op.Status != "DONE" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		var err error
		op, err = a.service.RegionOperations.Get(project, region, op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("waiting for %s of ComputeRegionSecurityPolicyRule %s: %w", what, a.id.String(), err)
		}
	}
	if op.Error != nil && len(op.Error.Errors) > 0 {
		return fmt.Errorf("%s of ComputeRegionSecurityPolicyRule %s: %s: %s", what, a.id.String(), op.Error.Errors[0].Code, op.Error.Errors[0].Message)
	}
	return nil
}

func (a *regionSecurityPolicyRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.ComputeRegionSecurityPolicyRuleStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	return op.UpdateStatus(ctx, status, nil)
}

// regionSecurityPolicyRuleMissing reports whether err says the rule (or
// its policy) does not exist: a 404, or a 400 about the priority, which is
// how the API answers a policy with no rule there for firewall policies
// (assumed for this API too; live pending).
func regionSecurityPolicyRuleMissing(err error) bool {
	if direct.IsNotFound(err) {
		return true
	}
	var ge *googleapi.Error
	return errors.As(err, &ge) && ge.Code == http.StatusBadRequest && strings.Contains(ge.Message, "priority") &&
		(strings.Contains(ge.Message, "does not") || strings.Contains(ge.Message, "not found"))
}

// regionSecurityPolicyRuleSpecToAPI is the spec as the API's rule. The
// description and preview are always sent, so that taking them off the
// spec puts the default back.
func regionSecurityPolicyRuleSpecToAPI(in *krm.ComputeRegionSecurityPolicyRuleSpec, priority int64) *computev1.SecurityPolicyRule {
	out := &computev1.SecurityPolicyRule{
		Priority:        priority,
		Action:          in.Action,
		Description:     direct.ValueOf(in.Description),
		Preview:         direct.ValueOf(in.Preview),
		ForceSendFields: []string{"Priority", "Description", "Preview"},
	}
	if m := in.Match; m != nil {
		out.Match = &computev1.SecurityPolicyRuleMatcher{VersionedExpr: direct.ValueOf(m.VersionedExpr)}
		if m.Config != nil {
			out.Match.Config = &computev1.SecurityPolicyRuleMatcherConfig{SrcIpRanges: m.Config.SrcIPRanges}
		}
		if m.Expr != nil {
			out.Match.Expr = &computev1.Expr{Expression: direct.ValueOf(m.Expr.Expression)}
		}
	}
	if n := in.NetworkMatch; n != nil {
		out.NetworkMatch = &computev1.SecurityPolicyRuleNetworkMatcher{
			DestIpRanges:   n.DestIPRanges,
			DestPorts:      n.DestPorts,
			IpProtocols:    n.IPProtocols,
			SrcAsns:        n.SrcAsns,
			SrcIpRanges:    n.SrcIPRanges,
			SrcPorts:       n.SrcPorts,
			SrcRegionCodes: n.SrcRegionCodes,
		}
		for _, f := range n.UserDefinedFields {
			out.NetworkMatch.UserDefinedFields = append(out.NetworkMatch.UserDefinedFields, &computev1.SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch{Name: f.Name, Values: f.Values})
		}
	}
	if w := in.PreconfiguredWafConfig; w != nil {
		out.PreconfiguredWafConfig = &computev1.SecurityPolicyRulePreconfiguredWafConfig{}
		params := func(in []krm.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams) []*computev1.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams {
			var out []*computev1.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams
			for _, p := range in {
				out = append(out, &computev1.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams{Op: direct.ValueOf(p.Operator), Val: direct.ValueOf(p.Value)})
			}
			return out
		}
		for _, e := range w.Exclusion {
			out.PreconfiguredWafConfig.Exclusions = append(out.PreconfiguredWafConfig.Exclusions, &computev1.SecurityPolicyRulePreconfiguredWafConfigExclusion{
				RequestCookiesToExclude:     params(e.RequestCookie),
				RequestHeadersToExclude:     params(e.RequestHeader),
				RequestQueryParamsToExclude: params(e.RequestQueryParam),
				RequestUrisToExclude:        params(e.RequestUri),
				TargetRuleIds:               e.TargetRuleIds,
				TargetRuleSet:               direct.ValueOf(e.TargetRuleSet),
			})
		}
	}
	if r := in.RateLimitOptions; r != nil {
		threshold := func(in *krm.SecurityPolicyRuleRateLimitOptionsThreshold) *computev1.SecurityPolicyRuleRateLimitOptionsThreshold {
			if in == nil {
				return nil
			}
			return &computev1.SecurityPolicyRuleRateLimitOptionsThreshold{Count: int64(direct.ValueOf(in.Count)), IntervalSec: int64(direct.ValueOf(in.IntervalSec))}
		}
		out.RateLimitOptions = &computev1.SecurityPolicyRuleRateLimitOptions{
			BanDurationSec:     int64(direct.ValueOf(r.BanDurationSec)),
			BanThreshold:       threshold(r.BanThreshold),
			ConformAction:      direct.ValueOf(r.ConformAction),
			EnforceOnKey:       direct.ValueOf(r.EnforceOnKey),
			EnforceOnKeyName:   direct.ValueOf(r.EnforceOnKeyName),
			ExceedAction:       direct.ValueOf(r.ExceedAction),
			RateLimitThreshold: threshold(r.RateLimitThreshold),
		}
		for _, c := range r.EnforceOnKeyConfigs {
			out.RateLimitOptions.EnforceOnKeyConfigs = append(out.RateLimitOptions.EnforceOnKeyConfigs, &computev1.SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig{
				EnforceOnKeyName: direct.ValueOf(c.EnforceOnKeyName),
				EnforceOnKeyType: direct.ValueOf(c.EnforceOnKeyType),
			})
		}
	}
	return out
}

// regionSecurityPolicyRuleFromAPI is the API's rule in the spec's shape,
// the fields the spec has (the reference and priority left out).
func regionSecurityPolicyRuleFromAPI(in *computev1.SecurityPolicyRule) *krm.ComputeRegionSecurityPolicyRuleSpec {
	out := &krm.ComputeRegionSecurityPolicyRuleSpec{
		Action:      in.Action,
		Description: direct.LazyPtr(in.Description),
		Preview:     direct.PtrTo(in.Preview),
	}
	if m := in.Match; m != nil {
		out.Match = &krm.SecurityPolicyRuleMatcher{VersionedExpr: direct.LazyPtr(m.VersionedExpr)}
		if m.Config != nil {
			out.Match.Config = &krm.SecurityPolicyRuleMatcherConfig{SrcIPRanges: m.Config.SrcIpRanges}
		}
		if m.Expr != nil {
			out.Match.Expr = &krm.SecurityPolicyRuleMatcherExpr{Expression: direct.LazyPtr(m.Expr.Expression)}
		}
	}
	if n := in.NetworkMatch; n != nil {
		out.NetworkMatch = &krm.SecurityPolicyRuleNetworkMatcher{
			DestIPRanges:   n.DestIpRanges,
			DestPorts:      n.DestPorts,
			IPProtocols:    n.IpProtocols,
			SrcAsns:        n.SrcAsns,
			SrcIPRanges:    n.SrcIpRanges,
			SrcPorts:       n.SrcPorts,
			SrcRegionCodes: n.SrcRegionCodes,
		}
		for _, f := range n.UserDefinedFields {
			out.NetworkMatch.UserDefinedFields = append(out.NetworkMatch.UserDefinedFields, krm.SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch{Name: f.Name, Values: f.Values})
		}
	}
	if w := in.PreconfiguredWafConfig; w != nil {
		out.PreconfiguredWafConfig = &krm.SecurityPolicyRulePreconfiguredWafConfig{}
		params := func(in []*computev1.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams) []krm.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams {
			var out []krm.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams
			for _, p := range in {
				out = append(out, krm.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams{Operator: direct.LazyPtr(p.Op), Value: direct.LazyPtr(p.Val)})
			}
			return out
		}
		for _, e := range w.Exclusions {
			out.PreconfiguredWafConfig.Exclusion = append(out.PreconfiguredWafConfig.Exclusion, krm.SecurityPolicyRulePreconfiguredWafConfigExclusion{
				RequestCookie:     params(e.RequestCookiesToExclude),
				RequestHeader:     params(e.RequestHeadersToExclude),
				RequestQueryParam: params(e.RequestQueryParamsToExclude),
				RequestUri:        params(e.RequestUrisToExclude),
				TargetRuleIds:     e.TargetRuleIds,
				TargetRuleSet:     direct.LazyPtr(e.TargetRuleSet),
			})
		}
	}
	if r := in.RateLimitOptions; r != nil {
		threshold := func(in *computev1.SecurityPolicyRuleRateLimitOptionsThreshold) *krm.SecurityPolicyRuleRateLimitOptionsThreshold {
			if in == nil {
				return nil
			}
			return &krm.SecurityPolicyRuleRateLimitOptionsThreshold{Count: direct.PtrTo(int32(in.Count)), IntervalSec: direct.PtrTo(int32(in.IntervalSec))}
		}
		out.RateLimitOptions = &krm.SecurityPolicyRuleRateLimitOptions{
			BanDurationSec:     direct.PtrTo(int32(r.BanDurationSec)),
			BanThreshold:       threshold(r.BanThreshold),
			ConformAction:      direct.LazyPtr(r.ConformAction),
			EnforceOnKey:       direct.LazyPtr(r.EnforceOnKey),
			EnforceOnKeyName:   direct.LazyPtr(r.EnforceOnKeyName),
			ExceedAction:       direct.LazyPtr(r.ExceedAction),
			RateLimitThreshold: threshold(r.RateLimitThreshold),
		}
		for _, c := range r.EnforceOnKeyConfigs {
			out.RateLimitOptions.EnforceOnKeyConfigs = append(out.RateLimitOptions.EnforceOnKeyConfigs, krm.SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig{
				EnforceOnKeyName: direct.LazyPtr(c.EnforceOnKeyName),
				EnforceOnKeyType: direct.LazyPtr(c.EnforceOnKeyType),
			})
		}
	}
	return out
}

// regionSecurityPolicyRuleUpdateMask is the update mask of a patchRule that
// makes the rule the spec: the fields that differ, in Terraform's paths. The
// comparison is of the spec's fields only, where an unset field, "", false,
// 0 and an empty list are the same; the action is compared without case;
// and a rate limit the spec keys on nothing is the API's default key, ALL.
func regionSecurityPolicyRuleUpdateMask(spec *krm.ComputeRegionSecurityPolicyRuleSpec, actual *computev1.SecurityPolicyRule) []string {
	want := regionSecurityPolicyRuleComparable(spec)
	have := regionSecurityPolicyRuleComparable(regionSecurityPolicyRuleFromAPI(actual))
	if r, ok := have["rateLimitOptions"].(map[string]any); ok {
		w, _ := want["rateLimitOptions"].(map[string]any)
		if _, set := w["enforceOnKey"]; !set && w["enforceOnKeyConfigs"] == nil && r["enforceOnKey"] == "ALL" {
			delete(r, "enforceOnKey")
		}
	}
	paths := []struct{ field, path string }{
		{"action", "action"},
		{"description", "description"},
		{"match", "match"},
		{"networkMatch", "network_match.userDefinedFields,network_match.srcIpRanges,network_match.destIpRanges,network_match.ipProtocols,network_match.srcPorts,network_match.destPorts,network_match.srcRegionCodes,network_match.srcAsns"},
		{"preconfiguredWafConfig", "preconfigured_waf_config"},
		{"preview", "preview"},
		{"rateLimitOptions", "rate_limit_options"},
	}
	var mask []string
	for _, p := range paths {
		if !reflect.DeepEqual(want[p.field], have[p.field]) {
			mask = append(mask, p.path)
		}
	}
	return mask
}

// regionSecurityPolicyRuleComparable is the spec's rule fields as JSON
// values with the empty ones pruned.
func regionSecurityPolicyRuleComparable(spec *krm.ComputeRegionSecurityPolicyRuleSpec) map[string]any {
	s := spec.DeepCopy()
	s.SecurityPolicyRef = krm.ComputeRegionSecurityPolicyRef{}
	s.Priority = 0
	s.Action = strings.ToLower(s.Action)
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	pruned, _ := pruneEmpty(m).(map[string]any)
	delete(pruned, "securityPolicyRef")
	delete(pruned, "priority")
	return pruned
}

// pruneEmpty drops "", false, 0, null, and lists and objects that are (or
// become) empty.
func pruneEmpty(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range t {
			if p := pruneEmpty(e); p != nil {
				out[k] = p
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		var out []any
		for _, e := range t {
			p := pruneEmpty(e)
			if p == nil {
				p = map[string]any{}
			}
			out = append(out, p)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case string:
		if t == "" {
			return nil
		}
	case bool:
		if !t {
			return nil
		}
	case float64:
		if t == 0 {
			return nil
		}
	case nil:
		return nil
	}
	return v
}
