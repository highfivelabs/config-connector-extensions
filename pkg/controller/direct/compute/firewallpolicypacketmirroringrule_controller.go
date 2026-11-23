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

// Written here: Config Connector has no packet mirroring rule kind. The
// hierarchical policy's rule, after our global policy's
// (networkfirewallpolicypacketmirroringrule_controller.go), whose spec
// mapping, comparison and patch body it shares. The calls,
// firewallPolicies.getPacketMirroringRule, addPacketMirroringRule,
// patchPacketMirroringRule and removePacketMirroringRule, are in the
// compute beta API only and in neither pinned Go client, so they are sent
// as REST; their operations are global organization operations, read with
// the v1 client. The priority and the policy are immutable.

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
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	htransport "google.golang.org/api/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	hflcomputev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

// NewFirewallPolicyPacketMirroringRuleModel is the
// ComputeFirewallPolicyPacketMirroringRule model.
func NewFirewallPolicyPacketMirroringRuleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &firewallPolicyPacketMirroringRuleModel{config: *config}, nil
}

type firewallPolicyPacketMirroringRuleModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &firewallPolicyPacketMirroringRuleModel{}

func (m *firewallPolicyPacketMirroringRuleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeFirewallPolicyPacketMirroringRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	id := &krm.ComputeFirewallPolicyPacketMirroringRuleIdentity{}
	if ext := direct.ValueOf(obj.Status.ExternalRef); op.IsDeleting() && ext != "" {
		// Deleting a rule that was made: its name is in status, so the
		// policy object is not resolved.
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
	} else {
		idVal, err := obj.GetIdentity(ctx, op.Reader)
		if err != nil {
			return nil, err
		}
		got, ok := idVal.(*krm.ComputeFirewallPolicyPacketMirroringRuleIdentity)
		if !ok {
			return nil, fmt.Errorf("unexpected identity type: %T", idVal)
		}
		id = got
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	ops, err := gcp.NewGlobalOrganizationOperationsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building GlobalOrganizationOperations client: %w", err)
	}
	httpClient, _, err := htransport.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Compute Engine HTTP client: %w", err)
	}
	// The spec's fields are those of the global policy's rule.
	spec := &hflcomputev1beta1.ComputeNetworkFirewallPolicyPacketMirroringRuleSpec{
		Priority: obj.Spec.Priority, Action: obj.Spec.Action, Direction: obj.Spec.Direction,
		Match: obj.Spec.Match, RuleName: obj.Spec.RuleName, Description: obj.Spec.Description,
		Disabled: obj.Spec.Disabled, SecurityProfileGroup: obj.Spec.SecurityProfileGroup,
		TargetSecureTags: obj.Spec.TargetSecureTags,
	}
	return &firewallPolicyPacketMirroringRuleAdapter{
		operations: ops,
		httpClient: httpClient,
		id:         id,
		desired:    packetMirroringRuleSpecToProto(spec),
	}, nil
}

func (m *firewallPolicyPacketMirroringRuleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type firewallPolicyPacketMirroringRuleAdapter struct {
	operations *gcp.GlobalOrganizationOperationsClient
	httpClient *http.Client
	id         *krm.ComputeFirewallPolicyPacketMirroringRuleIdentity
	desired    *pb.FirewallPolicyRule
	actual     *pb.FirewallPolicyRule
}

var _ directbase.Adapter = &firewallPolicyPacketMirroringRuleAdapter{}

// restError is a non-2xx answer of the compute beta API.
type restError struct {
	code    int
	message string
}

func (e *restError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.code, e.message) }

// hierarchicalRuleMissing is a 404 (the policy is gone), or the 400 the API
// answers for a priority the policy has no packet mirroring rule at
// (assumed after the global policy's; mockgcp patch 0058 answers "does not
// contain a packet mirroring rule at priority").
func hierarchicalRuleMissing(err error) bool {
	var re *restError
	if !errors.As(err, &re) {
		return false
	}
	return re.code == http.StatusNotFound ||
		(re.code == http.StatusBadRequest && strings.Contains(re.message, "does not contain a") && strings.Contains(re.message, "at priority"))
}

// call sends one of the four calls to the compute beta API; body is nil
// for get and remove.
func (a *firewallPolicyPacketMirroringRuleAdapter) call(ctx context.Context, method string, withPriority bool, body []byte) ([]byte, error) {
	u := fmt.Sprintf("https://compute.googleapis.com/compute/beta/locations/global/firewallPolicies/%s/%s",
		url.PathEscape(a.id.Policy.FirewallPolicy), method)
	if withPriority {
		u += fmt.Sprintf("?priority=%d", a.id.Priority)
	}
	verb := http.MethodPost
	if method == "getPacketMirroringRule" {
		verb = http.MethodGet
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, verb, u, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(answer))
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(answer, &e) == nil && e.Error.Message != "" {
			msg = e.Error.Message
		}
		return nil, &restError{code: resp.StatusCode, message: msg}
	}
	return answer, nil
}

// operation sends a call that answers an operation and waits for it.
func (a *firewallPolicyPacketMirroringRuleAdapter) operation(ctx context.Context, method string, withPriority bool, body []byte) error {
	answer, err := a.call(ctx, method, withPriority, body)
	if err != nil {
		return err
	}
	op := &pb.Operation{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(answer, op); err != nil {
		return fmt.Errorf("reading the %s operation: %w", method, err)
	}
	for op.GetStatus() != pb.Operation_DONE {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		op, err = a.operations.Get(ctx, &pb.GetGlobalOrganizationOperationRequest{Operation: op.GetName()})
		if err != nil {
			return fmt.Errorf("waiting for %s: %w", method, err)
		}
	}
	if errs := op.GetError().GetErrors(); len(errs) > 0 {
		return fmt.Errorf("%s: %s: %s", method, errs[0].GetCode(), errs[0].GetMessage())
	}
	return nil
}

func (a *firewallPolicyPacketMirroringRuleAdapter) get(ctx context.Context) (*pb.FirewallPolicyRule, error) {
	answer, err := a.call(ctx, "getPacketMirroringRule", true, nil)
	if err != nil {
		return nil, err
	}
	rule := &pb.FirewallPolicyRule{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(answer, rule); err != nil {
		return nil, fmt.Errorf("reading the rule: %w", err)
	}
	return rule, nil
}

func (a *firewallPolicyPacketMirroringRuleAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting ComputeFirewallPolicyPacketMirroringRule", "name", a.id.String())
	actual, err := a.get(ctx)
	if err != nil {
		if hierarchicalRuleMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeFirewallPolicyPacketMirroringRule %q: %w", a.id.String(), err)
	}
	a.actual = actual
	return true, nil
}

func (a *firewallPolicyPacketMirroringRuleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeFirewallPolicyPacketMirroringRule", "name", a.id.String())
	body, err := protojson.Marshal(a.desired)
	if err != nil {
		return err
	}
	if err := a.operation(ctx, "addPacketMirroringRule", false, body); err != nil {
		return fmt.Errorf("creating ComputeFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting created ComputeFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, createOp, created)
}

func (a *firewallPolicyPacketMirroringRuleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	if proto.Equal(comparablePacketMirroringRule(a.desired), comparablePacketMirroringRule(a.actual)) {
		log.V(2).Info("no diff detected for ComputeFirewallPolicyPacketMirroringRule", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	log.V(2).Info("updating ComputeFirewallPolicyPacketMirroringRule", "name", a.id.String())
	body, err := packetMirroringRulePatchBody(a.desired, a.actual)
	if err != nil {
		return err
	}
	if err := a.operation(ctx, "patchPacketMirroringRule", true, body); err != nil {
		return fmt.Errorf("updating ComputeFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	updated, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	return a.updateStatus(ctx, updateOp, updated)
}

func (a *firewallPolicyPacketMirroringRuleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeFirewallPolicyPacketMirroringRule", "name", a.id.String())
	if err := a.operation(ctx, "removePacketMirroringRule", true, nil); err != nil {
		if hierarchicalRuleMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeFirewallPolicyPacketMirroringRule %s: %w", a.id.String(), err)
	}
	return true, nil
}

func (a *firewallPolicyPacketMirroringRuleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *firewallPolicyPacketMirroringRuleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.FirewallPolicyRule) error {
	status := &krm.ComputeFirewallPolicyPacketMirroringRuleStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if latest.RuleTupleCount != nil {
		status.RuleTupleCount = direct.PtrTo(int64(latest.GetRuleTupleCount()))
	}
	return op.UpdateStatus(ctx, status, nil)
}
