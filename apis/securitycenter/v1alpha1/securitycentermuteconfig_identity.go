// Copyright 2026 Google LLC
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

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/securitycenter/v1alpha1/securitycentermuteconfig_identity.go
// (UPSTREAM.md). Changed: the parent is an organization, folder or
// project (SecurityCenterParent, from pull request #7673), so the
// identity is {parent}/muteConfigs/{muteConfig} rather than a gcpurls
// template with an organization.

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &SecurityCenterMuteConfigIdentity{}
	_ identity.Resource   = &SecurityCenterMuteConfig{}
)

// +k8s:deepcopy-gen=false
type SecurityCenterMuteConfigIdentity struct {
	Parent     SecurityCenterParent
	MuteConfig string
}

func (i *SecurityCenterMuteConfigIdentity) String() string {
	return i.Parent.String() + "/muteConfigs/" + i.MuteConfig
}

func (i *SecurityCenterMuteConfigIdentity) FromExternal(ref string) error {
	parent, rest, err := ParseParent(ref)
	if err != nil {
		return fmt.Errorf("format of SecurityCenterMuteConfig external=%q was not known (use {organizations|folders|projects}/{id}/muteConfigs/{muteConfig}): %w", ref, err)
	}
	tokens := strings.Split(rest, "/")
	if len(tokens) != 2 || tokens[0] != "muteConfigs" || tokens[1] == "" {
		return fmt.Errorf("format of SecurityCenterMuteConfig external=%q was not known (use {organizations|folders|projects}/{id}/muteConfigs/{muteConfig})", ref)
	}
	i.Parent = *parent
	i.MuteConfig = tokens[1]
	return nil
}

func (i *SecurityCenterMuteConfigIdentity) Host() string {
	return "securitycenter.googleapis.com"
}

func getIdentityFromSecurityCenterMuteConfigSpec(ctx context.Context, reader client.Reader, obj *SecurityCenterMuteConfig) (*SecurityCenterMuteConfigIdentity, error) {
	resourceID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID")
	}

	parent, err := ResolveParent(ctx, reader, obj, obj.Spec.OrganizationRef, obj.Spec.FolderRef, obj.Spec.ProjectRef)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve parent: %w", err)
	}

	identity := &SecurityCenterMuteConfigIdentity{
		Parent:     *parent,
		MuteConfig: resourceID,
	}
	return identity, nil
}

func (obj *SecurityCenterMuteConfig) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromSecurityCenterMuteConfigSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	// Cross-check the identity against the status value, if present.
	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef != "" {
		// Validate desired with actual
		statusIdentity := &SecurityCenterMuteConfigIdentity{}
		if err := statusIdentity.FromExternal(externalRef); err != nil {
			return nil, err
		}

		if statusIdentity.String() != specIdentity.String() {
			return nil, fmt.Errorf("cannot change SecurityCenterMuteConfig identity (old=%q, new=%q)", statusIdentity.String(), specIdentity.String())
		}
	}

	return specIdentity, nil
}
