// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here, in the shape of Config Connector's identities.

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ComputeRegionNetworkFirewallPolicyRuleIdentity{}

// ComputeRegionNetworkFirewallPolicyRuleIdentity is
// projects/{project}/regions/{region}/firewallPolicies/{policy}/rules/{priority}.
// The API names a rule only by its priority in the policy; this spelling is
// ours.
// +k8s:deepcopy-gen=false
type ComputeRegionNetworkFirewallPolicyRuleIdentity struct {
	Policy   ComputeRegionNetworkFirewallPolicyIdentity
	Priority int64
}

func (i *ComputeRegionNetworkFirewallPolicyRuleIdentity) String() string {
	return fmt.Sprintf("%s/rules/%d", i.Policy.String(), i.Priority)
}

func (i *ComputeRegionNetworkFirewallPolicyRuleIdentity) FromExternal(ref string) error {
	policy, rule, ok := strings.Cut(strings.TrimPrefix(ref, "/"), "/rules/")
	if !ok {
		return fmt.Errorf("format of ComputeRegionNetworkFirewallPolicyRule external=%q was not known (use projects/{{projectID}}/regions/{{region}}/firewallPolicies/{{policy}}/rules/{{priority}})", ref)
	}
	priority, err := strconv.ParseInt(rule, 10, 64)
	if err != nil {
		return fmt.Errorf("format of ComputeRegionNetworkFirewallPolicyRule external=%q was not known: the priority is not a number", ref)
	}
	var p ComputeRegionNetworkFirewallPolicyIdentity
	if err := p.FromExternal(policy); err != nil {
		return err
	}
	*i = ComputeRegionNetworkFirewallPolicyRuleIdentity{Policy: p, Priority: priority}
	return nil
}

// GetIdentity is the rule of the referenced policy at spec.priority. A
// rule whose policy or priority differs from the one it was made with is
// refused.
func (obj *ComputeRegionNetworkFirewallPolicyRule) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	policy, err := obj.Spec.FirewallPolicyRef.Resolve(ctx, reader, obj.GetNamespace())
	if err != nil {
		return nil, err
	}
	id := &ComputeRegionNetworkFirewallPolicyRuleIdentity{Policy: *policy, Priority: obj.Spec.Priority}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeRegionNetworkFirewallPolicyRule identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
