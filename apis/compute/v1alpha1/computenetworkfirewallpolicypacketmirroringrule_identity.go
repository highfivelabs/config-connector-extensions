// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here, after our ComputeNetworkFirewallPolicyRule's identity
// (apis/compute/v1beta1), with packetMirroringRules in place of rules.

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity{}

// ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity is
// projects/{project}/global/firewallPolicies/{policy}/packetMirroringRules/{priority}.
// The API names a rule only by its priority in the policy; this spelling is
// ours.
// +k8s:deepcopy-gen=false
type ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity struct {
	Policy   computev1beta1.ComputeNetworkFirewallPolicyIdentity
	Priority int64
}

func (i *ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity) String() string {
	return fmt.Sprintf("%s/packetMirroringRules/%d", i.Policy.String(), i.Priority)
}

func (i *ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity) FromExternal(ref string) error {
	policy, rule, ok := strings.Cut(strings.TrimPrefix(ref, "/"), "/packetMirroringRules/")
	if !ok {
		return fmt.Errorf("format of ComputeNetworkFirewallPolicyPacketMirroringRule external=%q was not known (use projects/{{projectID}}/global/firewallPolicies/{{policy}}/packetMirroringRules/{{priority}})", ref)
	}
	priority, err := strconv.ParseInt(rule, 10, 64)
	if err != nil {
		return fmt.Errorf("format of ComputeNetworkFirewallPolicyPacketMirroringRule external=%q was not known: the priority is not a number", ref)
	}
	var p computev1beta1.ComputeNetworkFirewallPolicyIdentity
	if err := p.FromExternal(policy); err != nil {
		return err
	}
	*i = ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity{Policy: p, Priority: priority}
	return nil
}

// GetIdentity is the packet mirroring rule of the referenced policy at
// spec.priority. A projectRef that names another project than the policy's
// is refused, and so is a rule whose policy or priority differs from the
// one it was made with.
func (obj *ComputeNetworkFirewallPolicyPacketMirroringRule) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	ref := obj.Spec.FirewallPolicyRef
	if err := ref.Normalize(ctx, reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	var policy computev1beta1.ComputeNetworkFirewallPolicyIdentity
	if err := policy.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if obj.Spec.ProjectRef != nil {
		project, err := refsv1beta1.ResolveProject(ctx, reader, obj.GetNamespace(), obj.Spec.ProjectRef)
		if err != nil {
			return nil, err
		}
		if project.ProjectID != policy.Project {
			return nil, fmt.Errorf("spec.projectRef is project %q but the policy %q is in project %q", project.ProjectID, policy.String(), policy.Project)
		}
	}
	id := &ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity{Policy: policy, Priority: obj.Spec.Priority}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeNetworkFirewallPolicyPacketMirroringRule identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
