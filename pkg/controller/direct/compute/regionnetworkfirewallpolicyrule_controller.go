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

// Written here: no Config Connector code covers a regional network
// firewall policy's rules. The controller follows Terraform's
// google_compute_region_network_firewall_policy_rule (read, not copied) and
// the Compute Engine API: a rule is addressed by its priority in the policy;
// regionNetworkFirewallPolicies.getRule reads it, addRule creates it,
// patchRule changes it (every field the spec has, sent whole) and
// removeRule deletes it. The priority and the policy are immutable.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/googleapis/gax-go/v2/apierror"
	computev1 "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeRegionNetworkFirewallPolicyRuleGVK, NewRegionNetworkFirewallPolicyRuleModel)
}

func NewRegionNetworkFirewallPolicyRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &regionNetworkFirewallPolicyRuleModel{config: *config}, nil
}

type regionNetworkFirewallPolicyRuleModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &regionNetworkFirewallPolicyRuleModel{}

func (m *regionNetworkFirewallPolicyRuleModel) client(ctx context.Context) (*gcp.RegionNetworkFirewallPoliciesClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewRegionNetworkFirewallPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building RegionNetworkFirewallPolicies client: %w", err)
	}
	return c, nil
}

func (m *regionNetworkFirewallPolicyRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeRegionNetworkFirewallPolicyRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeRegionNetworkFirewallPolicyRuleIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	var serviceAccounts []string
	for i := range obj.Spec.TargetServiceAccounts {
		ref := &obj.Spec.TargetServiceAccounts[i]
		if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		serviceAccounts = append(serviceAccounts, ref.External)
	}
	c, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	patcher, err := computev1.NewService(ctx, append([]option.ClientOption{}, opts...)...)
	if err != nil {
		return nil, fmt.Errorf("building Compute Engine client: %w", err)
	}
	return &regionNetworkFirewallPolicyRuleAdapter{
		gcpClient: c,
		patcher:   patcher,
		id:        id,
		desired:   regionNetworkFirewallPolicyRuleSpecToProto(&obj.Spec, serviceAccounts),
	}, nil
}

func (m *regionNetworkFirewallPolicyRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type regionNetworkFirewallPolicyRuleAdapter struct {
	gcpClient *gcp.RegionNetworkFirewallPoliciesClient
	// patcher sends patchRule, which the generated client cannot do for a
	// list the spec empties (see patchBody).
	patcher *computev1.Service
	id      *krm.ComputeRegionNetworkFirewallPolicyRuleIdentity
	desired *pb.FirewallPolicyRule
	actual  *pb.FirewallPolicyRule
}

var _ directbase.Adapter = &regionNetworkFirewallPolicyRuleAdapter{}

func (a *regionNetworkFirewallPolicyRuleAdapter) get(ctx context.Context) (*pb.FirewallPolicyRule, error) {
	return a.gcpClient.GetRule(ctx, &pb.GetRuleRegionNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		Region:         a.id.Policy.Region,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	})
}

func (a *regionNetworkFirewallPolicyRuleAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeRegionNetworkFirewallPolicyRule", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if regionFirewallPolicyRuleMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeRegionNetworkFirewallPolicyRule %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *regionNetworkFirewallPolicyRuleAdapter) wait(ctx context.Context, op *gcp.Operation, what string) error {
	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for %s of ComputeRegionNetworkFirewallPolicyRule %s: %w", what, a.id.String(), err)
		}
	}
	return nil
}

func (a *regionNetworkFirewallPolicyRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeRegionNetworkFirewallPolicyRule", "name", a.id.String())
	op, err := a.gcpClient.AddRule(ctx, &pb.AddRuleRegionNetworkFirewallPolicyRequest{
		Project:                    a.id.Policy.Project,
		Region:                     a.id.Policy.Region,
		FirewallPolicy:             a.id.Policy.FirewallPolicy,
		FirewallPolicyRuleResource: a.desired,
	})
	if err != nil {
		return fmt.Errorf("creating ComputeRegionNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "create"); err != nil {
		return err
	}
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeRegionNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *regionNetworkFirewallPolicyRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	if proto.Equal(comparableRule(a.desired), comparableRule(a.actual)) {
		log.V(2).Info("no diff detected for ComputeRegionNetworkFirewallPolicyRule", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	log.V(2).Info("updating ComputeRegionNetworkFirewallPolicyRule", "name", a.id.String())
	body, err := a.patchBody()
	if err != nil {
		return fmt.Errorf("updating ComputeRegionNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	computeOp, err := a.patcher.RegionNetworkFirewallPolicies.PatchRule(a.id.Policy.Project, a.id.Policy.Region, a.id.Policy.FirewallPolicy, body).
		Priority(a.id.Priority).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating ComputeRegionNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.waitPatch(ctx, computeOp); err != nil {
		return err
	}
	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeRegionNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, updated)
}

// patchBody is the rule to send to patchRule. patchRule keeps a list the
// request leaves out (a live run: a rule patched without its srcIpRanges
// kept them), and the generated client leaves an empty list out, so a list
// the cloud's rule has and the spec has not is sent as [] (a ForceSendFields
// entry): that clears it.
func (a *regionNetworkFirewallPolicyRuleAdapter) patchBody() (*computev1.FirewallPolicyRule, error) {
	raw, err := protojson.Marshal(a.desired)
	if err != nil {
		return nil, err
	}
	body := &computev1.FirewallPolicyRule{}
	if err := json.Unmarshal(raw, body); err != nil {
		return nil, err
	}
	if body.Match == nil {
		body.Match = &computev1.FirewallPolicyRuleMatcher{}
	}
	have, want := a.actual.GetMatch(), a.desired.GetMatch()
	emptied := func(field string, haveLen, wantLen int, force *[]string) {
		if haveLen > 0 && wantLen == 0 {
			*force = append(*force, field)
		}
	}
	m := &body.Match.ForceSendFields
	emptied("SrcIpRanges", len(have.GetSrcIpRanges()), len(want.GetSrcIpRanges()), m)
	emptied("DestIpRanges", len(have.GetDestIpRanges()), len(want.GetDestIpRanges()), m)
	emptied("SrcAddressGroups", len(have.GetSrcAddressGroups()), len(want.GetSrcAddressGroups()), m)
	emptied("DestAddressGroups", len(have.GetDestAddressGroups()), len(want.GetDestAddressGroups()), m)
	emptied("SrcFqdns", len(have.GetSrcFqdns()), len(want.GetSrcFqdns()), m)
	emptied("DestFqdns", len(have.GetDestFqdns()), len(want.GetDestFqdns()), m)
	emptied("SrcRegionCodes", len(have.GetSrcRegionCodes()), len(want.GetSrcRegionCodes()), m)
	emptied("DestRegionCodes", len(have.GetDestRegionCodes()), len(want.GetDestRegionCodes()), m)
	emptied("SrcThreatIntelligences", len(have.GetSrcThreatIntelligences()), len(want.GetSrcThreatIntelligences()), m)
	emptied("DestThreatIntelligences", len(have.GetDestThreatIntelligences()), len(want.GetDestThreatIntelligences()), m)
	emptied("SrcSecureTags", len(have.GetSrcSecureTags()), len(want.GetSrcSecureTags()), m)
	r := &body.ForceSendFields
	emptied("TargetServiceAccounts", len(a.actual.GetTargetServiceAccounts()), len(a.desired.GetTargetServiceAccounts()), r)
	emptied("TargetSecureTags", len(a.actual.GetTargetSecureTags()), len(a.desired.GetTargetSecureTags()), r)
	return body, nil
}

// waitPatch waits for the regional operation patchRule started.
func (a *regionNetworkFirewallPolicyRuleAdapter) waitPatch(ctx context.Context, op *computev1.Operation) error {
	for op.Status != "DONE" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		var err error
		op, err = a.patcher.RegionOperations.Get(a.id.Policy.Project, a.id.Policy.Region, op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("waiting for update of ComputeRegionNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
		}
	}
	if op.Error != nil && len(op.Error.Errors) > 0 {
		return fmt.Errorf("updating ComputeRegionNetworkFirewallPolicyRule %s: %s: %s", a.id.String(), op.Error.Errors[0].Code, op.Error.Errors[0].Message)
	}
	return nil
}

func (a *regionNetworkFirewallPolicyRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *regionNetworkFirewallPolicyRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeRegionNetworkFirewallPolicyRule", "name", a.id.String())
	op, err := a.gcpClient.RemoveRule(ctx, &pb.RemoveRuleRegionNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		Region:         a.id.Policy.Region,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	})
	if err != nil {
		if regionFirewallPolicyRuleMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeRegionNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "delete"); err != nil {
		if regionFirewallPolicyRuleMissing(err) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

func (a *regionNetworkFirewallPolicyRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.FirewallPolicyRule) error {
	status := &krm.ComputeRegionNetworkFirewallPolicyRuleStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if latest.RuleTupleCount != nil {
		status.RuleTupleCount = direct.PtrTo(int64(latest.GetRuleTupleCount()))
	}
	return op.UpdateStatus(ctx, status, nil)
}

// regionFirewallPolicyRuleMissing reports whether err says the rule (or its
// policy) does not exist. A policy that has no rule at a priority answers
// 400, "The firewall policy does not contain a rule at priority N", not 404
// (as for a router's route policy, docs/live-runs.md); a missing policy is a
// 404.
func regionFirewallPolicyRuleMissing(err error) bool {
	if direct.IsNotFound(err) {
		return true
	}
	var ae *apierror.APIError
	return errors.As(err, &ae) && ae.HTTPCode() == http.StatusBadRequest && strings.Contains(err.Error(), "does not contain a rule at priority")
}

// regionNetworkFirewallPolicyRuleSpecToProto is written here: the spec's
// fields, as the API's FirewallPolicyRule. serviceAccounts are the
// spec.targetServiceAccounts as emails. The optional strings and booleans
// the spec leaves unset are sent as "" and false, so that taking one off
// the spec puts the default back.
func regionNetworkFirewallPolicyRuleSpecToProto(in *krm.ComputeRegionNetworkFirewallPolicyRuleSpec, serviceAccounts []string) *pb.FirewallPolicyRule {
	out := &pb.FirewallPolicyRule{
		Priority:      direct.PtrTo(int32(in.Priority)),
		Action:        direct.PtrTo(in.Action),
		Direction:     direct.PtrTo(in.Direction),
		RuleName:      direct.PtrTo(direct.ValueOf(in.RuleName)),
		Description:   direct.PtrTo(direct.ValueOf(in.Description)),
		Disabled:      direct.PtrTo(direct.ValueOf(in.Disabled)),
		EnableLogging: direct.PtrTo(direct.ValueOf(in.EnableLogging)),
		Match: &pb.FirewallPolicyRuleMatcher{
			SrcIpRanges:             in.Match.SrcIPRanges,
			DestIpRanges:            in.Match.DestIPRanges,
			SrcAddressGroups:        in.Match.SrcAddressGroups,
			DestAddressGroups:       in.Match.DestAddressGroups,
			SrcFqdns:                in.Match.SrcFqdns,
			DestFqdns:               in.Match.DestFqdns,
			SrcRegionCodes:          in.Match.SrcRegionCodes,
			DestRegionCodes:         in.Match.DestRegionCodes,
			SrcThreatIntelligences:  in.Match.SrcThreatIntelligences,
			DestThreatIntelligences: in.Match.DestThreatIntelligences,
		},
		TargetServiceAccounts: serviceAccounts,
	}
	for _, c := range in.Match.Layer4Configs {
		out.Match.Layer4Configs = append(out.Match.Layer4Configs, &pb.FirewallPolicyRuleMatcherLayer4Config{
			IpProtocol: direct.PtrTo(c.IPProtocol),
			Ports:      c.Ports,
		})
	}
	for _, t := range in.Match.SrcSecureTags {
		out.Match.SrcSecureTags = append(out.Match.SrcSecureTags, &pb.FirewallPolicyRuleSecureTag{Name: direct.PtrTo(t.Name)})
	}
	for _, t := range in.TargetSecureTags {
		out.TargetSecureTags = append(out.TargetSecureTags, &pb.FirewallPolicyRuleSecureTag{Name: direct.PtrTo(t.Name)})
	}
	return out
}

// comparableRule is the rule reduced to the fields the spec manages, in the
// spelling the API returns, so a rule read can be compared with the one
// wanted: the output-only fields (kind, ruleTupleCount, a secure tag's
// state), fields this kind does not have (a security profile group, target
// resources, network contexts) and the case of the action and direction are
// left out.
func comparableRule(in *pb.FirewallPolicyRule) *pb.FirewallPolicyRule {
	out := &pb.FirewallPolicyRule{
		Priority:              in.Priority,
		Action:                direct.PtrTo(strings.ToLower(in.GetAction())),
		Direction:             direct.PtrTo(strings.ToUpper(in.GetDirection())),
		RuleName:              direct.PtrTo(in.GetRuleName()),
		Description:           direct.PtrTo(in.GetDescription()),
		Disabled:              direct.PtrTo(in.GetDisabled()),
		EnableLogging:         direct.PtrTo(in.GetEnableLogging()),
		TargetServiceAccounts: lowered(in.TargetServiceAccounts),
		TargetSecureTags:      secureTagNames(in.TargetSecureTags),
	}
	m := in.GetMatch()
	out.Match = &pb.FirewallPolicyRuleMatcher{
		SrcIpRanges:             m.GetSrcIpRanges(),
		DestIpRanges:            m.GetDestIpRanges(),
		SrcAddressGroups:        m.GetSrcAddressGroups(),
		DestAddressGroups:       m.GetDestAddressGroups(),
		SrcFqdns:                m.GetSrcFqdns(),
		DestFqdns:               m.GetDestFqdns(),
		SrcRegionCodes:          m.GetSrcRegionCodes(),
		DestRegionCodes:         m.GetDestRegionCodes(),
		SrcThreatIntelligences:  m.GetSrcThreatIntelligences(),
		DestThreatIntelligences: m.GetDestThreatIntelligences(),
		SrcSecureTags:           secureTagNames(m.GetSrcSecureTags()),
	}
	for _, c := range m.GetLayer4Configs() {
		out.Match.Layer4Configs = append(out.Match.Layer4Configs, &pb.FirewallPolicyRuleMatcherLayer4Config{
			IpProtocol: direct.PtrTo(strings.ToLower(c.GetIpProtocol())),
			Ports:      c.Ports,
		})
	}
	return out
}

func lowered(in []string) []string {
	var out []string
	for _, s := range in {
		out = append(out, strings.ToLower(s))
	}
	return out
}

func secureTagNames(in []*pb.FirewallPolicyRuleSecureTag) []*pb.FirewallPolicyRuleSecureTag {
	var out []*pb.FirewallPolicyRuleSecureTag
	for _, t := range in {
		out = append(out, &pb.FirewallPolicyRuleSecureTag{Name: t.Name})
	}
	return out
}
