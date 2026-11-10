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

// Written here: Config Connector has no packet mirroring rule kind.
// Adapted from our ComputeNetworkFirewallPolicyRule's controller
// (networkfirewallpolicyrule_controller.go), with Terraform's
// google_compute_network_firewall_policy_packet_mirroring_rule read for the
// calls: a packet mirroring rule is addressed by its priority in the
// policy; networkFirewallPolicies.getPacketMirroringRule reads it,
// addPacketMirroringRule creates it, patchPacketMirroringRule changes it
// (every field the spec has, sent whole) and removePacketMirroringRule
// deletes it. The priority and the policy are immutable.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/googleapis/gax-go/v2/apierror"
	htransport "google.golang.org/api/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

// NewNetworkFirewallPolicyPacketMirroringRuleModel is the
// ComputeNetworkFirewallPolicyPacketMirroringRule model.
func NewNetworkFirewallPolicyPacketMirroringRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &networkFirewallPolicyPacketMirroringRuleModel{config: *config}, nil
}

type networkFirewallPolicyPacketMirroringRuleModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &networkFirewallPolicyPacketMirroringRuleModel{}

func (m *networkFirewallPolicyPacketMirroringRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeNetworkFirewallPolicyPacketMirroringRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewNetworkFirewallPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building NetworkFirewallPolicies client: %w", err)
	}
	ops, err := gcp.NewGlobalOperationsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building GlobalOperations client: %w", err)
	}
	httpClient, _, err := htransport.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Compute Engine HTTP client: %w", err)
	}
	return &networkFirewallPolicyPacketMirroringRuleAdapter{
		gcpClient:  c,
		operations: ops,
		httpClient: httpClient,
		id:         id,
		desired:    packetMirroringRuleSpecToProto(&obj.Spec),
	}, nil
}

func (m *networkFirewallPolicyPacketMirroringRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type networkFirewallPolicyPacketMirroringRuleAdapter struct {
	gcpClient  *gcp.NetworkFirewallPoliciesClient
	operations *gcp.GlobalOperationsClient
	// httpClient sends patchPacketMirroringRule, which the generated client
	// cannot do for a list the spec empties (see patchBody), and which the
	// pinned google.golang.org/api compute/v1 client does not have.
	httpClient *http.Client
	id         *krm.ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity
	desired    *pb.FirewallPolicyRule
	actual     *pb.FirewallPolicyRule
}

var _ directbase.Adapter = &networkFirewallPolicyPacketMirroringRuleAdapter{}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) get(ctx context.Context) (*pb.FirewallPolicyRule, error) {
	return a.gcpClient.GetPacketMirroringRule(ctx, &pb.GetPacketMirroringRuleNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	})
}

// packetMirroringRuleMissing is a 404, or the 400 the API answers for a
// priority the policy has no packet mirroring rule at (assumed after the
// firewall rules' "does not contain a rule at priority"; mockgcp patch
// 0031 answers "does not contain a packet mirroring rule at priority").
func packetMirroringRuleMissing(err error) bool {
	if direct.IsNotFound(err) {
		return true
	}
	var ae *apierror.APIError
	return errors.As(err, &ae) && ae.HTTPCode() == http.StatusBadRequest &&
		strings.Contains(err.Error(), "does not contain a") && strings.Contains(err.Error(), "at priority")
}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeNetworkFirewallPolicyPacketMirroringRule", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if packetMirroringRuleMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeNetworkFirewallPolicyPacketMirroringRule %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) wait(ctx context.Context, op *gcp.Operation, what string) error {
	if !op.Done() {
		if err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for %s of ComputeNetworkFirewallPolicyPacketMirroringRule %s: %w", what, a.id.String(), err)
		}
	}
	return nil
}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeNetworkFirewallPolicyPacketMirroringRule", "name", a.id.String())
	op, err := a.gcpClient.AddPacketMirroringRule(ctx, &pb.AddPacketMirroringRuleNetworkFirewallPolicyRequest{
		Project:                    a.id.Policy.Project,
		FirewallPolicy:             a.id.Policy.FirewallPolicy,
		FirewallPolicyRuleResource: a.desired,
	})
	if err != nil {
		return fmt.Errorf("creating ComputeNetworkFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "create"); err != nil {
		return err
	}
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeNetworkFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	if proto.Equal(comparablePacketMirroringRule(a.desired), comparablePacketMirroringRule(a.actual)) {
		log.V(2).Info("no diff detected for ComputeNetworkFirewallPolicyPacketMirroringRule", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	log.V(2).Info("updating ComputeNetworkFirewallPolicyPacketMirroringRule", "name", a.id.String())
	if err := a.patch(ctx); err != nil {
		return fmt.Errorf("updating ComputeNetworkFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeNetworkFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, updated)
}

// patchBody is the rule to send to patchPacketMirroringRule, as JSON. As
// for firewall rules (networkfirewallpolicyrule_controller.go), the API is
// assumed to keep a list the request leaves out, and protojson leaves an
// empty list out, so a list the cloud's rule has and the spec has not is
// sent as [], and a security profile group the spec drops as "".
func (a *networkFirewallPolicyPacketMirroringRuleAdapter) patchBody() ([]byte, error) {
	raw, err := protojson.Marshal(a.desired)
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	match, _ := body["match"].(map[string]any)
	if match == nil {
		match = map[string]any{}
		body["match"] = match
	}
	have, want := a.actual.GetMatch(), a.desired.GetMatch()
	if len(have.GetSrcIpRanges()) > 0 && len(want.GetSrcIpRanges()) == 0 {
		match["srcIpRanges"] = []string{}
	}
	if len(have.GetDestIpRanges()) > 0 && len(want.GetDestIpRanges()) == 0 {
		match["destIpRanges"] = []string{}
	}
	if len(a.actual.GetTargetSecureTags()) > 0 && len(a.desired.GetTargetSecureTags()) == 0 {
		body["targetSecureTags"] = []any{}
	}
	if a.actual.GetSecurityProfileGroup() != "" && a.desired.GetSecurityProfileGroup() == "" {
		body["securityProfileGroup"] = ""
	}
	return json.Marshal(body)
}

// patch sends patchPacketMirroringRule and waits for its global operation.
func (a *networkFirewallPolicyPacketMirroringRuleAdapter) patch(ctx context.Context) error {
	body, err := a.patchBody()
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/global/firewallPolicies/%s/patchPacketMirroringRule?priority=%d",
		url.PathEscape(a.id.Policy.Project), url.PathEscape(a.id.Policy.FirewallPolicy), a.id.Priority)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("patchPacketMirroringRule: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(answer)))
	}
	op := &pb.Operation{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(answer, op); err != nil {
		return fmt.Errorf("reading the patch's operation: %w", err)
	}
	for op.GetStatus() != pb.Operation_DONE {
		op, err = a.operations.Wait(ctx, &pb.WaitGlobalOperationRequest{Project: a.id.Policy.Project, Operation: op.GetName()})
		if err != nil {
			return fmt.Errorf("waiting for the patch: %w", err)
		}
	}
	if errs := op.GetError().GetErrors(); len(errs) > 0 {
		return fmt.Errorf("%s: %s", errs[0].GetCode(), errs[0].GetMessage())
	}
	return nil
}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeNetworkFirewallPolicyPacketMirroringRule", "name", a.id.String())
	op, err := a.gcpClient.RemovePacketMirroringRule(ctx, &pb.RemovePacketMirroringRuleNetworkFirewallPolicyRequest{
		Project:        a.id.Policy.Project,
		FirewallPolicy: a.id.Policy.FirewallPolicy,
		Priority:       direct.PtrTo(int32(a.id.Priority)),
	})
	if err != nil {
		if packetMirroringRuleMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeNetworkFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	if err := a.wait(ctx, op, "delete"); err != nil {
		if packetMirroringRuleMissing(err) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

func (a *networkFirewallPolicyPacketMirroringRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.FirewallPolicyRule) error {
	status := &krm.ComputeNetworkFirewallPolicyPacketMirroringRuleStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if latest.RuleTupleCount != nil {
		status.RuleTupleCount = direct.PtrTo(int64(latest.GetRuleTupleCount()))
	}
	return op.UpdateStatus(ctx, status, nil)
}

// packetMirroringRuleSpecToProto is the spec's fields as the API's
// FirewallPolicyRule. The optional strings and booleans the spec leaves
// unset are sent as "" and false, so that taking one off the spec puts the
// default back; the security profile group only when set. tlsInspect is
// never sent: the API refuses it on a packet mirroring rule, even false.
func packetMirroringRuleSpecToProto(in *krm.ComputeNetworkFirewallPolicyPacketMirroringRuleSpec) *pb.FirewallPolicyRule {
	out := &pb.FirewallPolicyRule{
		Priority:             direct.PtrTo(int32(in.Priority)),
		Action:               direct.PtrTo(in.Action),
		Direction:            direct.PtrTo(in.Direction),
		RuleName:             direct.PtrTo(direct.ValueOf(in.RuleName)),
		Description:          direct.PtrTo(direct.ValueOf(in.Description)),
		Disabled:             direct.PtrTo(direct.ValueOf(in.Disabled)),
		SecurityProfileGroup: in.SecurityProfileGroup,
		Match: &pb.FirewallPolicyRuleMatcher{
			SrcIpRanges:  in.Match.SrcIPRanges,
			DestIpRanges: in.Match.DestIPRanges,
		},
	}
	for _, c := range in.Match.Layer4Configs {
		out.Match.Layer4Configs = append(out.Match.Layer4Configs, &pb.FirewallPolicyRuleMatcherLayer4Config{
			IpProtocol: direct.PtrTo(c.IPProtocol),
			Ports:      c.Ports,
		})
	}
	for _, t := range in.TargetSecureTags {
		out.TargetSecureTags = append(out.TargetSecureTags, &pb.FirewallPolicyRuleSecureTag{Name: direct.PtrTo(t.Name)})
	}
	return out
}

// comparablePacketMirroringRule is what the spec manages of a rule, in one
// spelling: the action lower case, the direction upper case, unset strings
// and booleans as their zero values, secure tags by name, and the security
// profile group without the //networksecurity.googleapis.com/ prefix.
func comparablePacketMirroringRule(in *pb.FirewallPolicyRule) *pb.FirewallPolicyRule {
	out := &pb.FirewallPolicyRule{
		Priority:             in.Priority,
		Action:               direct.PtrTo(strings.ToLower(in.GetAction())),
		Direction:            direct.PtrTo(strings.ToUpper(in.GetDirection())),
		RuleName:             direct.PtrTo(in.GetRuleName()),
		Description:          direct.PtrTo(in.GetDescription()),
		Disabled:             direct.PtrTo(in.GetDisabled()),
		SecurityProfileGroup: direct.PtrTo(strings.TrimPrefix(in.GetSecurityProfileGroup(), "//networksecurity.googleapis.com/")),
		TargetSecureTags:     secureTagNames(in.TargetSecureTags),
	}
	m := in.GetMatch()
	out.Match = &pb.FirewallPolicyRuleMatcher{
		SrcIpRanges:  m.GetSrcIpRanges(),
		DestIpRanges: m.GetDestIpRanges(),
	}
	for _, c := range m.GetLayer4Configs() {
		out.Match.Layer4Configs = append(out.Match.Layer4Configs, &pb.FirewallPolicyRuleMatcherLayer4Config{
			IpProtocol: direct.PtrTo(strings.ToLower(c.GetIpProtocol())),
			Ports:      c.Ports,
		})
	}
	return out
}
