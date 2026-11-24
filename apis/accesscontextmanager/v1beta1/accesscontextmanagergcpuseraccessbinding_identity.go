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
// apis/accesscontextmanager/v1alpha1/accesscontextmanagergcpuseraccessbinding_identity.go
// and accesscontextmanagergcpuseraccessbinding_identity.generated.go
// (UPSTREAM.md); the package is v1beta1; the organization is
// organizationRef.external only (upstream's other forms resolve to it too),
// and status.externalRef is cross-checked as upstream checks status.name.

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.ServerGeneratedIdentity = &AccessContextManagerGCPUserAccessBindingIdentity{}
	_ identity.Resource                = &AccessContextManagerGCPUserAccessBinding{}
)

var AccessContextManagerGCPUserAccessBindingIdentityFormat = gcpurls.Template[AccessContextManagerGCPUserAccessBindingIdentity]("accesscontextmanager.googleapis.com", "organizations/{organization}/gcpUserAccessBindings/{gcpUserAccessBinding}")

// AccessContextManagerGCPUserAccessBindingIdentity is the identity of a GCP AccessContextManagerGCPUserAccessBinding resource.
// +k8s:deepcopy-gen=false
type AccessContextManagerGCPUserAccessBindingIdentity struct {
	Organization         string
	GcpUserAccessBinding string
}

func (i *AccessContextManagerGCPUserAccessBindingIdentity) HasIdentitySpecified() bool {
	return i.GcpUserAccessBinding != ""
}

func (i *AccessContextManagerGCPUserAccessBindingIdentity) String() string {
	return AccessContextManagerGCPUserAccessBindingIdentityFormat.ToString(*i)
}

func (i *AccessContextManagerGCPUserAccessBindingIdentity) FromExternal(ref string) error {
	parsed, match, err := AccessContextManagerGCPUserAccessBindingIdentityFormat.Parse(ref)
	if err != nil {
		return fmt.Errorf("format of AccessContextManagerGCPUserAccessBinding external=%q was not known (use %s): %w", ref, AccessContextManagerGCPUserAccessBindingIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of AccessContextManagerGCPUserAccessBinding external=%q was not known (use %s)", ref, AccessContextManagerGCPUserAccessBindingIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *AccessContextManagerGCPUserAccessBindingIdentity) Host() string {
	return AccessContextManagerGCPUserAccessBindingIdentityFormat.Host()
}

func (i *AccessContextManagerGCPUserAccessBindingIdentity) ParentString() string {
	return "organizations/" + i.Organization
}

func getIdentityFromAccessContextManagerGCPUserAccessBindingSpec(obj *AccessContextManagerGCPUserAccessBinding) (*AccessContextManagerGCPUserAccessBindingIdentity, error) {
	if obj.Spec.OrganizationRef == nil || obj.Spec.OrganizationRef.External == "" {
		return nil, fmt.Errorf("spec.organizationRef.external is required")
	}
	org := strings.TrimPrefix(obj.Spec.OrganizationRef.External, "organizations/")
	if org == "" || strings.Contains(org, "/") {
		return nil, fmt.Errorf("spec.organizationRef.external %q is not organizations/{id} or {id}", obj.Spec.OrganizationRef.External)
	}
	return &AccessContextManagerGCPUserAccessBindingIdentity{
		Organization:         org,
		GcpUserAccessBinding: common.ValueOf(obj.Spec.ResourceID),
	}, nil
}

func (obj *AccessContextManagerGCPUserAccessBinding) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromAccessContextManagerGCPUserAccessBindingSpec(obj)
	if err != nil {
		return nil, err
	}

	// Cross-check the identity against the status value, if present.
	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef == "" {
		externalRef = common.ValueOf(obj.Status.Name)
	}
	if externalRef != "" {
		statusIdentity := &AccessContextManagerGCPUserAccessBindingIdentity{}
		if err := statusIdentity.FromExternal(externalRef); err != nil {
			return nil, err
		}

		if specIdentity.GcpUserAccessBinding == "" {
			specIdentity.GcpUserAccessBinding = statusIdentity.GcpUserAccessBinding
		}

		if statusIdentity.String() != specIdentity.String() {
			return nil, fmt.Errorf("cannot change AccessContextManagerGCPUserAccessBinding identity (old=%q, new=%q)", statusIdentity.String(), specIdentity.String())
		}
	}

	return specIdentity, nil
}
