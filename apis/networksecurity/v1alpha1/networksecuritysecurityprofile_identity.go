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
// apis/networksecurity/v1alpha1/networksecuritysecurityprofile_identity.go (UPSTREAM.md).
// An identity's parent is a project or an organization:
// projects/{project}/locations/{location}/securityProfiles/{securityprofile} (upstream's)
// or organizations/{organization}/locations/{location}/securityProfiles/{securityprofile}.

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &NetworkSecuritySecurityProfileIdentity{}
	_ identity.Resource   = &NetworkSecuritySecurityProfile{}
)

var NetworkSecuritySecurityProfileIdentityFormat = gcpurls.Template[NetworkSecuritySecurityProfileIdentity]("networksecurity.googleapis.com", "projects/{project}/locations/{location}/securityProfiles/{securityprofile}")

// NetworkSecuritySecurityProfileOrganizationIdentityFormat is an
// organization's security profile (written here).
var NetworkSecuritySecurityProfileOrganizationIdentityFormat = gcpurls.Template[NetworkSecuritySecurityProfileIdentity]("networksecurity.googleapis.com", "organizations/{organization}/locations/{location}/securityProfiles/{securityprofile}")

// +k8s:deepcopy-gen=false
type NetworkSecuritySecurityProfileIdentity struct {
	// Project or Organization is set, never both.
	Project         string
	Organization    string
	Location        string
	Securityprofile string
}

func (i *NetworkSecuritySecurityProfileIdentity) String() string {
	if i.Organization != "" {
		return NetworkSecuritySecurityProfileOrganizationIdentityFormat.ToString(*i)
	}
	return NetworkSecuritySecurityProfileIdentityFormat.ToString(*i)
}

func (i *NetworkSecuritySecurityProfileIdentity) FromExternal(ref string) error {
	for _, format := range []*gcpurls.URLTemplate[NetworkSecuritySecurityProfileIdentity]{NetworkSecuritySecurityProfileIdentityFormat, NetworkSecuritySecurityProfileOrganizationIdentityFormat} {
		parsed, match, err := format.Parse(ref)
		if err != nil {
			return fmt.Errorf("format of NetworkSecuritySecurityProfile external=%q was not known (use %s or %s): %w", ref, NetworkSecuritySecurityProfileIdentityFormat.CanonicalForm(), NetworkSecuritySecurityProfileOrganizationIdentityFormat.CanonicalForm(), err)
		}
		if match {
			*i = *parsed
			return nil
		}
	}
	return fmt.Errorf("format of NetworkSecuritySecurityProfile external=%q was not known (use %s or %s)", ref, NetworkSecuritySecurityProfileIdentityFormat.CanonicalForm(), NetworkSecuritySecurityProfileOrganizationIdentityFormat.CanonicalForm())
}

func (i *NetworkSecuritySecurityProfileIdentity) Host() string {
	return NetworkSecuritySecurityProfileIdentityFormat.Host()
}

func (i *NetworkSecuritySecurityProfileIdentity) ParentString() string {
	if i.Organization != "" {
		return fmt.Sprintf("organizations/%s/locations/%s", i.Organization, i.Location)
	}
	return fmt.Sprintf("projects/%s/locations/%s", i.Project, i.Location)
}

func getIdentityFromNetworkSecuritySecurityProfileSpec(ctx context.Context, reader client.Reader, obj client.Object) (*NetworkSecuritySecurityProfileIdentity, error) {
	// A reference reads the object as unstructured (reference.go); the
	// parent fields are read from it as from the typed object.
	var organizationRef *refs.OrganizationRef
	hasProject := false
	switch o := obj.(type) {
	case *NetworkSecuritySecurityProfile:
		organizationRef = o.Spec.OrganizationRef
		hasProject = o.Spec.ProjectRef != nil
	case *unstructured.Unstructured:
		if m, found, _ := unstructured.NestedMap(o.Object, "spec", "organizationRef"); found {
			organizationRef = &refs.OrganizationRef{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, organizationRef); err != nil {
				return nil, fmt.Errorf("reading spec.organizationRef: %w", err)
			}
		}
		_, hasProject, _ = unstructured.NestedMap(o.Object, "spec", "projectRef")
	default:
		return nil, fmt.Errorf("unexpected type %T", obj)
	}
	if (organizationRef == nil) == !hasProject {
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

	identity := &NetworkSecuritySecurityProfileIdentity{
		Location:        location,
		Securityprofile: resourceID,
	}
	if organizationRef != nil {
		org, err := refs.ResolveOrganization(ctx, reader, obj, organizationRef)
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

func (obj *NetworkSecuritySecurityProfile) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromNetworkSecuritySecurityProfileSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef != "" {
		statusIdentity := &NetworkSecuritySecurityProfileIdentity{}
		if err := statusIdentity.FromExternal(externalRef); err != nil {
			return nil, err
		}

		if statusIdentity.String() != specIdentity.String() {
			return nil, fmt.Errorf("cannot change NetworkSecuritySecurityProfile identity (old=%q, new=%q)", statusIdentity.String(), specIdentity.String())
		}
	}

	return specIdentity, nil
}

func (obj *NetworkSecuritySecurityProfile) ExternalIdentifier() *string {
	if obj.Status.ExternalRef != nil {
		return obj.Status.ExternalRef
	}
	return nil
}
