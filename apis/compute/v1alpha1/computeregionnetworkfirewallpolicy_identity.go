// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/compute/v1beta1/computenetworkfirewallpolicy_identity.go (UPSTREAM.md).
// A regional policy: projects/{project}/regions/{region}/firewallPolicies/{firewallPolicy}
// instead of .../global/...; the region comes from spec.region.

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	apirefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &ComputeRegionNetworkFirewallPolicyIdentity{}
	_ identity.Resource   = &ComputeRegionNetworkFirewallPolicy{}
)

var ComputeRegionNetworkFirewallPolicyIdentityFormat = gcpurls.Template[ComputeRegionNetworkFirewallPolicyIdentity]("compute.googleapis.com", "projects/{project}/regions/{region}/firewallPolicies/{firewallPolicy}")

// ComputeRegionNetworkFirewallPolicyIdentity is the identity of a GCP regional network firewall policy.
// +k8s:deepcopy-gen=false
type ComputeRegionNetworkFirewallPolicyIdentity struct {
	Project        string
	Region         string
	FirewallPolicy string
}

func (i *ComputeRegionNetworkFirewallPolicyIdentity) String() string {
	return ComputeRegionNetworkFirewallPolicyIdentityFormat.ToString(*i)
}

func (i *ComputeRegionNetworkFirewallPolicyIdentity) FromExternal(ref string) error {
	ref = apirefs.TrimComputeURIPrefix(ref)
	parsed, match, err := ComputeRegionNetworkFirewallPolicyIdentityFormat.Parse(ref)
	if err != nil {
		return fmt.Errorf("format of ComputeRegionNetworkFirewallPolicy external=%q was not known (use %s): %w", ref, ComputeRegionNetworkFirewallPolicyIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of ComputeRegionNetworkFirewallPolicy external=%q was not known (use %s)", ref, ComputeRegionNetworkFirewallPolicyIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *ComputeRegionNetworkFirewallPolicyIdentity) Host() string {
	return ComputeRegionNetworkFirewallPolicyIdentityFormat.Host()
}

func (i *ComputeRegionNetworkFirewallPolicyIdentity) ParentString() string {
	return fmt.Sprintf("projects/%s/regions/%s", i.Project, i.Region)
}

func ParseComputeRegionNetworkFirewallPolicyExternal(external string) (*ComputeRegionNetworkFirewallPolicyIdentity, error) {
	if external == "" {
		return nil, fmt.Errorf("empty ComputeRegionNetworkFirewallPolicy external value")
	}
	id := &ComputeRegionNetworkFirewallPolicyIdentity{}
	if err := id.FromExternal(external); err != nil {
		return nil, err
	}
	return id, nil
}

func getIdentityFromComputeRegionNetworkFirewallPolicySpec(ctx context.Context, reader client.Reader, obj *ComputeRegionNetworkFirewallPolicy) (*ComputeRegionNetworkFirewallPolicyIdentity, error) {
	resourceID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID: %w", err)
	}

	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}

	if obj.Spec.Region == "" {
		return nil, fmt.Errorf("spec.region is required")
	}

	identity := &ComputeRegionNetworkFirewallPolicyIdentity{
		Project:        projectID,
		Region:         obj.Spec.Region,
		FirewallPolicy: resourceID,
	}
	return identity, nil
}

func (obj *ComputeRegionNetworkFirewallPolicy) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromComputeRegionNetworkFirewallPolicySpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	// Cross-check the identity against status.selfLink, if present.
	selfLink := common.ValueOf(obj.Status.SelfLink)
	if selfLink != "" {
		statusIdentity := &ComputeRegionNetworkFirewallPolicyIdentity{}
		if err := statusIdentity.FromExternal(selfLink); err != nil {
			return nil, err
		}

		if statusIdentity.String() != specIdentity.String() {
			return nil, fmt.Errorf("cannot change ComputeRegionNetworkFirewallPolicy identity (old=%q, new=%q)", statusIdentity.String(), specIdentity.String())
		}
	}

	return specIdentity, nil
}
