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
// apis/networksecurity/v1alpha1/networksecurityaddressgroup_identity.go (UPSTREAM.md).
// An identity's parent is a project or an organization:
// projects/{project}/locations/{location}/addressGroups/{addressgroup} (upstream's)
// or organizations/{organization}/locations/{location}/addressGroups/{addressgroup}.

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &NetworkSecurityAddressGroupIdentity{}
	_ identity.Resource   = &NetworkSecurityAddressGroup{}
)

var NetworkSecurityAddressGroupIdentityFormat = gcpurls.Template[NetworkSecurityAddressGroupIdentity]("networksecurity.googleapis.com", "projects/{project}/locations/{location}/addressGroups/{addressgroup}")

// NetworkSecurityAddressGroupOrganizationIdentityFormat is an organization's
// address group (written here).
var NetworkSecurityAddressGroupOrganizationIdentityFormat = gcpurls.Template[NetworkSecurityAddressGroupIdentity]("networksecurity.googleapis.com", "organizations/{organization}/locations/{location}/addressGroups/{addressgroup}")

// +k8s:deepcopy-gen=false
type NetworkSecurityAddressGroupIdentity struct {
	// Project or Organization is set, never both.
	Project      string
	Organization string
	Location     string
	Addressgroup string
}

func (i *NetworkSecurityAddressGroupIdentity) String() string {
	if i.Organization != "" {
		return NetworkSecurityAddressGroupOrganizationIdentityFormat.ToString(*i)
	}
	return NetworkSecurityAddressGroupIdentityFormat.ToString(*i)
}

func (i *NetworkSecurityAddressGroupIdentity) ParentString() string {
	if i.Organization != "" {
		return fmt.Sprintf("organizations/%s/locations/%s", i.Organization, i.Location)
	}
	return fmt.Sprintf("projects/%s/locations/%s", i.Project, i.Location)
}

func (i *NetworkSecurityAddressGroupIdentity) FromExternal(ref string) error {
	for _, format := range []*gcpurls.URLTemplate[NetworkSecurityAddressGroupIdentity]{NetworkSecurityAddressGroupIdentityFormat, NetworkSecurityAddressGroupOrganizationIdentityFormat} {
		parsed, match, err := format.Parse(ref)
		if err != nil {
			return fmt.Errorf("format of NetworkSecurityAddressGroup external=%q was not known (use %s or %s): %w", ref, NetworkSecurityAddressGroupIdentityFormat.CanonicalForm(), NetworkSecurityAddressGroupOrganizationIdentityFormat.CanonicalForm(), err)
		}
		if match {
			*i = *parsed
			return nil
		}
	}
	return fmt.Errorf("format of NetworkSecurityAddressGroup external=%q was not known (use %s or %s)", ref, NetworkSecurityAddressGroupIdentityFormat.CanonicalForm(), NetworkSecurityAddressGroupOrganizationIdentityFormat.CanonicalForm())
}

func (i *NetworkSecurityAddressGroupIdentity) Host() string {
	return NetworkSecurityAddressGroupIdentityFormat.Host()
}

func getIdentityFromNetworkSecurityAddressGroupSpec(ctx context.Context, reader client.Reader, obj *NetworkSecurityAddressGroup) (*NetworkSecurityAddressGroupIdentity, error) {
	if (obj.Spec.OrganizationRef == nil) == (obj.Spec.ProjectRef == nil) {
		return nil, fmt.Errorf("exactly one of spec.organizationRef and spec.projectRef is required")
	}

	resourceID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID")
	}

	location, err := refs.GetLocation(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve location")
	}

	identity := &NetworkSecurityAddressGroupIdentity{
		Location:     location,
		Addressgroup: resourceID,
	}
	if obj.Spec.OrganizationRef != nil {
		org, err := refs.ResolveOrganization(ctx, reader, obj, obj.Spec.OrganizationRef)
		if err != nil {
			return nil, err
		}
		identity.Organization = org.OrganizationID
	} else {
		projectID, err := refs.ResolveProjectID(ctx, reader, obj)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve project")
		}
		identity.Project = projectID
	}
	return identity, nil
}

func (obj *NetworkSecurityAddressGroup) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromNetworkSecurityAddressGroupSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef != "" {
		statusIdentity := &NetworkSecurityAddressGroupIdentity{}
		if err := statusIdentity.FromExternal(externalRef); err != nil {
			return nil, err
		}

		if statusIdentity.String() != specIdentity.String() {
			return nil, fmt.Errorf("cannot change NetworkSecurityAddressGroup identity (old=%q, new=%q)", statusIdentity.String(), specIdentity.String())
		}
	}

	return specIdentity, nil
}

// ExternalIdentifier returns the GCP external identifier (the GCP URL).
func (obj *NetworkSecurityAddressGroup) ExternalIdentifier() *string {
	if obj.Status.ExternalRef != nil {
		return obj.Status.ExternalRef
	}
	return nil
}
