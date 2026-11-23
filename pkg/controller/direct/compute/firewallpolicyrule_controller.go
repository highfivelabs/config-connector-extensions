// Copyright 2024 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

package compute

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector's pkg/controller/direct/compute/computefirewallpolicyrule_controller.go
// (UPSTREAM.md), the direct controller of a hierarchical firewall policy's
// rule: the same calls (firewallPolicies.getRule, addRule, patchRule with
// the priority as a query parameter and not in the body, removeRule) on the
// same client, and a missing rule read as the API's 400 "does not contain a
// rule at priority". Changed: our KRM types (ComputeFirewallPolicyRule in
// compute.gcp.hfl.dev, which adds the fields upstream's mapper marks
// MISSING); the references are resolved with their Normalize methods (a
// service account to its email); the spec is mapped, compared and patched
// as our ComputeNetworkFirewallPolicyRule does (networkfirewallpolicyrule_controller.go):
// upstream's CompareProtoMessage and generated-client patch leave out a
// list the spec empties, which patchRule then keeps, so the patch is sent
// with google.golang.org/api's client and clears such a list with []; a
// network and a security profile group compare by path; and a delete of a
// rule that was made uses status.externalRef and resolves nothing.

import (
	"context"
	"fmt"
	"slices"
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
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeFirewallPolicyRuleGVK, NewFirewallPolicyRuleModel)
}

func NewFirewallPolicyRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &firewallPolicyRuleModel{config: *config}, nil
}

type firewallPolicyRuleModel struct {
	config config.ControllerConfig
}

// model implements the Model interface.
var _ directbase.Model = &firewallPolicyRuleModel{}

type firewallPolicyRuleAdapter struct {
	id                     *krm.ComputeFirewallPolicyRuleIdentity
	firewallPoliciesClient *gcp.FirewallPoliciesClient
	// patcher sends patchRule, which the generated client cannot do for a
	// list the spec empties (see firewallPolicyRulePatchBody).
	patcher *computev1.Service
	desired *pb.FirewallPolicyRule
	actual  *pb.FirewallPolicyRule
}

var _ directbase.Adapter = &firewallPolicyRuleAdapter{}

func (m *firewallPolicyRuleModel) client(ctx context.Context) (*gcp.FirewallPoliciesClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp.NewFirewallPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building FirewallPolicy client: %w", err)
	}
	return gcpClient, err
}

func (m *firewallPolicyRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeFirewallPolicyRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if ext := direct.ValueOf(obj.Status.ExternalRef); op.IsDeleting() && ext != "" {
		// Deleting a rule that was made: its name is in status, and Find and
		// Delete need nothing else, so neither the policy nor the targets
		// are resolved. An object they name already gone (everything
		// deleted at once) would otherwise hold the delete forever.
		id := &krm.ComputeFirewallPolicyRuleIdentity{}
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
		c, err := m.client(ctx)
		if err != nil {
			return nil, err
		}
		return &firewallPolicyRuleAdapter{id: id, firewallPoliciesClient: c}, nil
	}

	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeFirewallPolicyRuleIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	// Target networks, as Config Connector sends them: a network's full URL.
	var networks []string
	for _, r := range obj.Spec.TargetResources {
		if r == nil {
			continue
		}
		ref := *r
		if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		networks = append(networks, computeURL(ref.External))
	}
	// Target service accounts, as their emails.
	var serviceAccounts []string
	for _, r := range obj.Spec.TargetServiceAccounts {
		if r == nil {
			continue
		}
		ref := *r
		if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		serviceAccounts = append(serviceAccounts, ref.External)
	}

	c, err := m.client(ctx)
	if err != nil {
		return nil, fmt.Errorf("building gcp client: %w", err)
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	patcher, err := computev1.NewService(ctx, append([]option.ClientOption{}, opts...)...)
	if err != nil {
		return nil, fmt.Errorf("building Compute Engine client: %w", err)
	}
	return &firewallPolicyRuleAdapter{
		id:                     id,
		firewallPoliciesClient: c,
		patcher:                patcher,
		desired:                firewallPolicyRuleSpecToProto(&obj.Spec, networks, serviceAccounts),
	}, nil
}

func (m *firewallPolicyRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	// TODO: Support URLs
	return nil, nil
}

func (a *firewallPolicyRuleAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ComputeFirewallPolicyRule", "name", a.id.String())

	firewallPolicyRule, err := a.get(ctx)
	if err != nil {
		// When a certain rule does not exist, the error has code 400(bad request) instead of 404(not found)
		// example error message:
		// "Invalid value for field 'priority': '9000'. The firewall policy does not contain a rule at priority 9000.",
		if firewallPolicyRuleMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	a.actual = firewallPolicyRule
	return true, nil
}

func (a *firewallPolicyRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ComputeFirewallPolicyRule", "name", a.id.String())

	req := &pb.AddRuleFirewallPolicyRequest{
		FirewallPolicyRuleResource: a.desired,
		FirewallPolicy:             a.id.Policy.FirewallPolicy,
	}
	op, err := a.firewallPoliciesClient.AddRule(ctx, req)
	if err != nil {
		return fmt.Errorf("creating ComputeFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if !op.Done() {
		err = op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("waiting ComputeFirewallPolicyRule %s create failed: %w", a.id.String(), err)
		}
	}
	log.V(2).Info("successfully created ComputeFirewallPolicyRule", "name", a.id.String())

	// Get the created resource
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *firewallPolicyRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating ComputeFirewallPolicyRule", "name", a.id.String())

	if proto.Equal(comparableHierarchicalRule(a.desired), comparableHierarchicalRule(a.actual)) {
		log.V(2).Info("no field needs update", "name", a.id.String())
		// even though there is no update, we still want to update KRM status
		return a.updateStatus(ctx, updateOp, a.actual)
	}

	body, err := firewallPolicyRulePatchBody(a.desired, a.actual)
	if err != nil {
		return fmt.Errorf("updating ComputeFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	// The field priority should be removed from the patch request body and included as a query parameter.
	// See API doc: https://cloud.google.com/compute/docs/reference/rest/v1/firewallPolicies/patchRule#query-parameters
	body.Priority = 0
	body.ForceSendFields = slices.DeleteFunc(body.ForceSendFields, func(f string) bool { return f == "Priority" })
	// Target networks the spec no longer has are cleared with [], as the
	// other lists are.
	if len(a.actual.GetTargetResources()) > 0 && len(a.desired.GetTargetResources()) == 0 {
		body.ForceSendFields = append(body.ForceSendFields, "TargetResources")
	}

	computeOp, err := a.patcher.FirewallPolicies.PatchRule(a.id.Policy.FirewallPolicy, body).
		Priority(a.id.Priority).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating ComputeFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if err := a.waitPatch(ctx, computeOp); err != nil {
		return err
	}
	log.V(2).Info("successfully updated ComputeFirewallPolicyRule", "name", a.id.String())

	// Get the updated resource
	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, updated)
}

// waitPatch waits for the global organization operation patchRule started.
func (a *firewallPolicyRuleAdapter) waitPatch(ctx context.Context, op *computev1.Operation) error {
	for op.Status != "DONE" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		var err error
		op, err = a.patcher.GlobalOrganizationOperations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("waiting ComputeFirewallPolicyRule %s update: %w", a.id.String(), err)
		}
	}
	if op.Error != nil && len(op.Error.Errors) > 0 {
		return fmt.Errorf("waiting ComputeFirewallPolicyRule %s update failed: %s: %s", a.id.String(), op.Error.Errors[0].Code, op.Error.Errors[0].Message)
	}
	return nil
}

func (a *firewallPolicyRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

// Delete implements the Adapter interface.
func (a *firewallPolicyRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ComputeFirewallPolicyRule", "name", a.id.String())

	delReq := &pb.RemoveRuleFirewallPolicyRequest{
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	}
	op, err := a.firewallPoliciesClient.RemoveRule(ctx, delReq)
	if err != nil {
		if firewallPolicyRuleMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeFirewallPolicyRule %s: %w", a.id.String(), err)
	}
	if !op.Done() {
		err = op.Wait(ctx)
		if err != nil {
			if firewallPolicyRuleMissing(err) {
				return true, nil
			}
			return false, fmt.Errorf("waiting ComputeFirewallPolicyRule %s delete failed: %w", a.id.String(), err)
		}
	}
	log.V(2).Info("successfully deleted ComputeFirewallPolicyRule", "name", a.id.String())
	return true, nil
}

func (a *firewallPolicyRuleAdapter) get(ctx context.Context) (*pb.FirewallPolicyRule, error) {
	getReq := &pb.GetRuleFirewallPolicyRequest{
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	}
	return a.firewallPoliciesClient.GetRule(ctx, getReq)
}

func (a *firewallPolicyRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.FirewallPolicyRule) error {
	status := &krm.ComputeFirewallPolicyRuleStatus{Kind: latest.Kind}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	if latest.RuleTupleCount != nil {
		status.RuleTupleCount = direct.PtrTo(int64(latest.GetRuleTupleCount()))
	}
	return op.UpdateStatus(ctx, status, nil)
}

// firewallPolicyRuleSpecToProto is written here (upstream's
// ComputeFirewallPolicyRuleSpec_v1beta1_ToProto, with the fields it marks
// MISSING): the spec's fields as the API's FirewallPolicyRule. networks and
// serviceAccounts are the resolved spec.targetResources and
// spec.targetServiceAccounts. As networkFirewallPolicyRuleSpecToProto, the
// optional strings and booleans the spec leaves unset are sent as "" and
// false, so that taking one off the spec puts the default back; the
// security profile group and TLS inspection only when set.
func firewallPolicyRuleSpecToProto(in *krm.ComputeFirewallPolicyRuleSpec, networks, serviceAccounts []string) *pb.FirewallPolicyRule {
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
		TargetResources:       networks,
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

// comparableHierarchicalRule is comparableNetworkRule (our global network
// rule's) with the target networks, compared by path: the API answers a
// network's full URL.
func comparableHierarchicalRule(in *pb.FirewallPolicyRule) *pb.FirewallPolicyRule {
	out := comparableNetworkRule(in)
	for _, n := range in.GetTargetResources() {
		out.TargetResources = append(out.TargetResources, resourcePath(n))
	}
	return out
}

// computeURL is a Compute Engine resource path as the full URL Config
// Connector sends for a target network; a URL is kept as it is.
func computeURL(path string) string {
	if strings.Contains(path, "://") {
		return path
	}
	return "https://www.googleapis.com/compute/v1/" + strings.TrimPrefix(path, "/")
}
