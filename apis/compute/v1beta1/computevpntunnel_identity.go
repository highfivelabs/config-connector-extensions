// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// Written here, in the shape of Config Connector's identities (upstream's
// ComputeVPNTunnel has types and a mapper but no identity at the pin).

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ComputeVPNTunnelIdentity{}

// ComputeVPNTunnelIdentity is projects/{project}/regions/{region}/vpnTunnels/{name}.
// +k8s:deepcopy-gen=false
type ComputeVPNTunnelIdentity struct {
	Project, Region, Name string
}

func (i *ComputeVPNTunnelIdentity) String() string {
	return fmt.Sprintf("projects/%s/regions/%s/vpnTunnels/%s", i.Project, i.Region, i.Name)
}

func (i *ComputeVPNTunnelIdentity) FromExternal(ref string) error {
	t := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if len(t) != 6 || t[0] != "projects" || t[2] != "regions" || t[4] != "vpnTunnels" {
		return fmt.Errorf("format of ComputeVPNTunnel external=%q was not known (use projects/{{projectID}}/regions/{{region}}/vpnTunnels/{{name}})", ref)
	}
	*i = ComputeVPNTunnelIdentity{Project: t[1], Region: t[3], Name: t[5]}
	return nil
}

// GetIdentity is the tunnel spec.resourceID (metadata.name by default)
// names in the project and spec.region. A tunnel whose identity differs
// from the one it was made with is refused.
func (obj *ComputeVPNTunnel) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	project, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}
	name, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, err
	}
	id := &ComputeVPNTunnelIdentity{Project: project, Region: obj.Spec.Region, Name: name}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeVPNTunnel identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
