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

package v1alpha1

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ComputeFirewallPolicyPacketMirroringRuleIdentity{}

// ComputeFirewallPolicyPacketMirroringRuleIdentity is
// locations/global/firewallPolicies/{policy}/packetMirroringRules/{priority}.
// The API names a rule only by its priority in the policy; this spelling is
// ours (after ComputeNetworkFirewallPolicyPacketMirroringRuleIdentity).
// +k8s:deepcopy-gen=false
type ComputeFirewallPolicyPacketMirroringRuleIdentity struct {
	Policy   computev1beta1.ComputeFirewallPolicyIdentity
	Priority int64
}

func (i *ComputeFirewallPolicyPacketMirroringRuleIdentity) String() string {
	return fmt.Sprintf("%s/packetMirroringRules/%d", i.Policy.String(), i.Priority)
}

func (i *ComputeFirewallPolicyPacketMirroringRuleIdentity) FromExternal(ref string) error {
	policy, rule, ok := strings.Cut(strings.TrimPrefix(ref, "/"), "/packetMirroringRules/")
	if !ok {
		return fmt.Errorf("format of ComputeFirewallPolicyPacketMirroringRule external=%q was not known (use locations/global/firewallPolicies/{{policy}}/packetMirroringRules/{{priority}})", ref)
	}
	priority, err := strconv.ParseInt(rule, 10, 64)
	if err != nil {
		return fmt.Errorf("format of ComputeFirewallPolicyPacketMirroringRule external=%q was not known: the priority is not a number", ref)
	}
	var p computev1beta1.ComputeFirewallPolicyIdentity
	if err := p.FromExternal(policy); err != nil {
		return err
	}
	*i = ComputeFirewallPolicyPacketMirroringRuleIdentity{Policy: p, Priority: priority}
	return nil
}

// GetIdentity is the packet mirroring rule of the referenced policy at
// spec.priority. A rule whose policy or priority differs from the one it
// was made with is refused.
func (obj *ComputeFirewallPolicyPacketMirroringRule) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	ref := obj.Spec.FirewallPolicyRef
	if err := ref.Normalize(ctx, reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	var policy computev1beta1.ComputeFirewallPolicyIdentity
	if err := policy.FromExternal(ref.External); err != nil {
		return nil, err
	}
	id := &ComputeFirewallPolicyPacketMirroringRuleIdentity{Policy: policy, Priority: obj.Spec.Priority}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeFirewallPolicyPacketMirroringRule identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
