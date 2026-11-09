// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here, in the shape of Config Connector's identities.

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ComputeRegionNetworkFirewallPolicyAssociationIdentity{}

// ComputeRegionNetworkFirewallPolicyAssociationIdentity is
// projects/{project}/regions/{region}/firewallPolicies/{policy}/associations/{name}.
// The API names an association by its name in the policy; this spelling is
// ours.
// +k8s:deepcopy-gen=false
type ComputeRegionNetworkFirewallPolicyAssociationIdentity struct {
	Policy ComputeRegionNetworkFirewallPolicyIdentity
	Name   string
}

func (i *ComputeRegionNetworkFirewallPolicyAssociationIdentity) String() string {
	return fmt.Sprintf("%s/associations/%s", i.Policy.String(), i.Name)
}

func (i *ComputeRegionNetworkFirewallPolicyAssociationIdentity) FromExternal(ref string) error {
	policy, name, ok := strings.Cut(strings.TrimPrefix(ref, "/"), "/associations/")
	if !ok || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("format of ComputeRegionNetworkFirewallPolicyAssociation external=%q was not known (use projects/{{projectID}}/regions/{{region}}/firewallPolicies/{{policy}}/associations/{{name}})", ref)
	}
	var p ComputeRegionNetworkFirewallPolicyIdentity
	if err := p.FromExternal(policy); err != nil {
		return err
	}
	*i = ComputeRegionNetworkFirewallPolicyAssociationIdentity{Policy: p, Name: name}
	return nil
}

// GetIdentity is the association of the referenced policy named by
// spec.resourceID (else metadata.name). An association whose policy or name
// differs from the one it was made with is refused.
func (obj *ComputeRegionNetworkFirewallPolicyAssociation) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	policy, err := obj.Spec.FirewallPolicyRef.Resolve(ctx, reader, obj.GetNamespace())
	if err != nil {
		return nil, err
	}
	name := common.ValueOf(obj.Spec.ResourceID)
	if name == "" {
		name = obj.GetName()
	}
	id := &ComputeRegionNetworkFirewallPolicyAssociationIdentity{Policy: *policy, Name: name}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeRegionNetworkFirewallPolicyAssociation identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
