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

// Written here: Config Connector's ComputeNetworkFirewallPolicyRule is
// Terraform-only at the pin. Adapted from our regional rule's controller
// (regionnetworkfirewallpolicyrule_controller.go), which follows Terraform's
// google_compute_region_network_firewall_policy_rule (read, not copied), with
// Config Connector's computefirewallpolicyrule_controller.go (the rule of an
// organization or folder policy) read for the same calls: a rule is
// addressed by its priority in the policy; networkFirewallPolicies.getRule
// reads it, addRule creates it, patchRule changes it (every field the spec
// has, sent whole) and removeRule deletes it. The priority and the policy
// are immutable.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	computev1 "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func init() {
	registry.RegisterModel(krm.ComputeNetworkFirewallPolicyRuleGVK, NewNetworkFirewallPolicyRuleModel)
}

func NewNetworkFirewallPolicyRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &networkFirewallPolicyRuleModel{config: *config}, nil
}

type networkFirewallPolicyRuleModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &networkFirewallPolicyRuleModel{}

func (m *networkFirewallPolicyRuleModel) client(ctx context.Context) (*gcp.NetworkFirewallPoliciesClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewNetworkFirewallPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building NetworkFirewallPolicies client: %w", err)
	}
	return c, nil
}

func (m *networkFirewallPolicyRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeNetworkFirewallPolicyRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if ext := direct.ValueOf(obj.Status.ExternalRef); op.IsDeleting() && ext != "" {
		// Deleting a rule that was made: its name is in status, and Find and
		// Delete need nothing else, so neither the policy nor the target
		// service accounts are resolved. An object they name already gone
		// (everything deleted at once) would otherwise hold the delete forever.
		id := &krm.ComputeNetworkFirewallPolicyRuleIdentity{}
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
		c, err := m.client(ctx)
		if err != nil {
			return nil, err
		}
		return &networkFirewallPolicyRuleAdapter{gcpClient: c, id: id}, nil
	}
	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeNetworkFirewallPolicyRuleIdentity)
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
	return &networkFirewallPolicyRuleAdapter{
		gcpClient: c,
		patcher:   patcher,
		id:        id,
		desired:   networkFirewallPolicyRuleSpecToProto(&obj.Spec, serviceAccounts),
	}, nil
}

func (m *networkFirewallPolicyRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type networkFirewallPolicyRuleAdapter struct {
	gcpClient *gcp.NetworkFirewallPoliciesClient
	// patcher sends patchRule, which the generated client cannot do for a
	// list the spec empties (see patchBody).
	patcher *computev1.Service
	id      *krm.ComputeNetworkFirewallPolicyRuleIdentity
	desired *pb.FirewallPolicyRule
	actual  *pb.FirewallPolicyRule
}

var _ directbase.Adapter = &networkFirewallPolicyRuleAdapter{}

func (a *networkFirewallPolicyRuleAdapter) get(ctx context.Context) (*pb.FirewallPolicyRule, error) {
	return a.gcpClient.GetRule(ctx, &pb.GetRuleNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	})
}

func (a *networkFirewallPolicyRuleAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeNetworkFirewallPolicyRule", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if firewallPolicyRuleMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeNetworkFirewallPolicyRule %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *networkFirewallPolicyRuleAdapter) wait(ctx context.Context, op *gcp.Operation, what string) error {
	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for %s of ComputeNetworkFirewallPolicyRule %s: %w", what, a.id.String(), err)
		}
	}
	return nil
}

func (a *networkFirewallPolicyRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeNetworkFirewallPolicyRule", "name", a.id.String())
	op, err := a.gcpClient.AddRule(ctx, &pb.AddRuleNetworkFirewallPolicyRequest{
		Project:                    a.id.Policy.Project,
		FirewallPolicy:             a.id.Policy.FirewallPolicy,
		FirewallPolicyRuleResource: a.desired,
	})
	if err != nil {
		return fmt.Errorf("creating ComputeNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "create"); err != nil {
		return err
	}
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *networkFirewallPolicyRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	if proto.Equal(comparableNetworkRule(a.desired), comparableNetworkRule(a.actual)) {
		log.V(2).Info("no diff detected for ComputeNetworkFirewallPolicyRule", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	log.V(2).Info("updating ComputeNetworkFirewallPolicyRule", "name", a.id.String())
	body, err := a.patchBody()
	if err != nil {
		return fmt.Errorf("updating ComputeNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	computeOp, err := a.patcher.NetworkFirewallPolicies.PatchRule(a.id.Policy.Project, a.id.Policy.FirewallPolicy, body).
		Priority(a.id.Priority).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating ComputeNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.waitPatch(ctx, computeOp); err != nil {
		return err
	}
	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, updated)
}

func (a *networkFirewallPolicyRuleAdapter) patchBody() (*computev1.FirewallPolicyRule, error) {
	return firewallPolicyRulePatchBody(a.desired, a.actual)
}

// firewallPolicyRulePatchBody is the rule to send to patchRule, desired
// as a patch of actual; ComputeFirewallPolicyRule (a hierarchical policy's)
// sends the same. patchRule keeps a list the request leaves out (a live
// run: a rule patched without its srcIpRanges kept them), and the
// generated client leaves an empty list out, so a list the cloud's rule
// has and the spec has not is sent as [] (a ForceSendFields entry): that
// clears it.
func firewallPolicyRulePatchBody(desired, actual *pb.FirewallPolicyRule) (*computev1.FirewallPolicyRule, error) {
	raw, err := protojson.Marshal(desired)
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
	have, want := actual.GetMatch(), desired.GetMatch()
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
	emptied("SrcNetworks", len(have.GetSrcNetworks()), len(want.GetSrcNetworks()), m)
	// A network context is not cleared by "" (a live run: patchRule kept
	// INTERNET when sent ""); it is cleared by UNSPECIFIED, one of the
	// API's values.
	if networkContextSet(have.GetSrcNetworkContext()) && !networkContextSet(want.GetSrcNetworkContext()) {
		body.Match.SrcNetworkContext = "UNSPECIFIED"
	}
	if networkContextSet(have.GetDestNetworkContext()) && !networkContextSet(want.GetDestNetworkContext()) {
		body.Match.DestNetworkContext = "UNSPECIFIED"
	}
	r := &body.ForceSendFields
	// The optional strings and booleans are sent as "" and false when the
	// spec leaves them unset (the generated client did this for the proto's
	// optional fields; this client leaves a zero value out unless forced).
	*r = append(*r, "Priority", "Description", "RuleName", "Disabled", "EnableLogging")
	// TLS inspection and a security profile group the spec no longer has
	// are sent as false and "" (the action that needs them has changed
	// too); otherwise they are sent only when the spec sets them, since
	// the API takes them only with apply_security_profile_group.
	if actual.GetTlsInspect() && !desired.GetTlsInspect() {
		*r = append(*r, "TlsInspect")
	}
	if actual.GetSecurityProfileGroup() != "" && desired.GetSecurityProfileGroup() == "" {
		*r = append(*r, "SecurityProfileGroup")
	}
	emptied("TargetServiceAccounts", len(actual.GetTargetServiceAccounts()), len(desired.GetTargetServiceAccounts()), r)
	emptied("TargetSecureTags", len(actual.GetTargetSecureTags()), len(desired.GetTargetSecureTags()), r)
	return body, nil
}

// waitPatch waits for the global operation patchRule started.
func (a *networkFirewallPolicyRuleAdapter) waitPatch(ctx context.Context, op *computev1.Operation) error {
	for op.Status != "DONE" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		var err error
		op, err = a.patcher.GlobalOperations.Get(a.id.Policy.Project, op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("waiting for update of ComputeNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
		}
	}
	if op.Error != nil && len(op.Error.Errors) > 0 {
		return fmt.Errorf("updating ComputeNetworkFirewallPolicyRule %s: %s: %s", a.id.String(), op.Error.Errors[0].Code, op.Error.Errors[0].Message)
	}
	return nil
}

func (a *networkFirewallPolicyRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *networkFirewallPolicyRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeNetworkFirewallPolicyRule", "name", a.id.String())
	op, err := a.gcpClient.RemoveRule(ctx, &pb.RemoveRuleNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	})
	if err != nil {
		if firewallPolicyRuleMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeNetworkFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "delete"); err != nil {
		if firewallPolicyRuleMissing(err) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

func (a *networkFirewallPolicyRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.FirewallPolicyRule) error {
	status := &krm.ComputeNetworkFirewallPolicyRuleStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if latest.RuleTupleCount != nil {
		status.RuleTupleCount = direct.PtrTo(int64(latest.GetRuleTupleCount()))
	}
	return op.UpdateStatus(ctx, status, nil)
}

// networkFirewallPolicyRuleSpecToProto is written here: the spec's fields,
// as the API's FirewallPolicyRule (as regionNetworkFirewallPolicyRuleSpecToProto,
// plus the network contexts). serviceAccounts are the
// spec.targetServiceAccounts as emails. The optional strings and booleans
// the spec leaves unset are sent as "" and false, so that taking one off
// the spec puts the default back.
func networkFirewallPolicyRuleSpecToProto(in *krm.ComputeNetworkFirewallPolicyRuleSpec, serviceAccounts []string) *pb.FirewallPolicyRule {
	out := &pb.FirewallPolicyRule{
		Priority:             direct.PtrTo(int32(in.Priority)),
		Action:               direct.PtrTo(in.Action),
		Direction:            direct.PtrTo(in.Direction),
		RuleName:             direct.PtrTo(direct.ValueOf(in.RuleName)),
		Description:          direct.PtrTo(direct.ValueOf(in.Description)),
		Disabled:             direct.PtrTo(direct.ValueOf(in.Disabled)),
		EnableLogging:        direct.PtrTo(direct.ValueOf(in.EnableLogging)),
		SecurityProfileGroup: in.SecurityProfileGroup,
		TlsInspect:           in.TLSInspect,
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
			SrcNetworkContext:       in.Match.SrcNetworkContext,
			DestNetworkContext:      in.Match.DestNetworkContext,
			SrcNetworks:             in.Match.SrcNetworks,
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

// comparableNetworkRule is comparableRule (the regional rule's) with the
// network contexts, the source networks, the security profile group and
// TLS inspection, which this kind manages and the regional kind does not.
// An unset context, an empty one and UNSPECIFIED are the same; a network
// and a security profile group compare by their path (the API may answer
// a full URL for a path sent); unset TLS inspection is false.
func comparableNetworkRule(in *pb.FirewallPolicyRule) *pb.FirewallPolicyRule {
	out := comparableRule(in)
	out.TlsInspect = direct.PtrTo(in.GetTlsInspect())
	if g := in.GetSecurityProfileGroup(); g != "" {
		out.SecurityProfileGroup = direct.PtrTo(resourcePath(g))
	}
	for _, n := range in.GetMatch().GetSrcNetworks() {
		out.Match.SrcNetworks = append(out.Match.SrcNetworks, resourcePath(n))
	}
	if c := in.GetMatch().GetSrcNetworkContext(); networkContextSet(c) {
		out.Match.SrcNetworkContext = direct.PtrTo(c)
	}
	if c := in.GetMatch().GetDestNetworkContext(); networkContextSet(c) {
		out.Match.DestNetworkContext = direct.PtrTo(c)
	}
	return out
}

// networkContextSet reports whether a network context names one: "" and
// UNSPECIFIED do not.
func networkContextSet(c string) bool {
	return c != "" && c != "UNSPECIFIED"
}

// resourcePath is a resource name without the scheme, host and API
// version a URL form adds: "projects/p/global/networks/n" for
// "https://www.googleapis.com/compute/v1/projects/p/global/networks/n",
// "organizations/o/locations/global/securityProfileGroups/g" for
// "//networksecurity.googleapis.com/organizations/o/..." or
// "https://networksecurity.googleapis.com/v1/organizations/o/...".
func resourcePath(s string) string {
	for _, root := range []string{"projects/", "organizations/"} {
		if i := strings.Index(s, "/"+root); i >= 0 && strings.Contains(s[:i], ".googleapis.com") {
			return s[i+1:]
		}
	}
	return s
}
