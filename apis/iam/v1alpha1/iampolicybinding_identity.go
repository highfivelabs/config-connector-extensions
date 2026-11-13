// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

// Written by High Five Labs for config-connector-extensions after Config
// Connector pull request #14133's iampolicybinding_identity.generated.go
// (UPSTREAM.md), which knows an organization's binding only: a binding
// here has an organization, folder or project parent, as Terraform's
// google_iam_{organizations,folders,projects}_policy_binding.

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
	_ identity.IdentityV2 = &IAMPolicyBindingIdentity{}
	_ identity.Resource   = &IAMPolicyBinding{}
)

// IAMPolicyBindingIdentity is {organizations|folders|projects}/{id}/locations/{location}/policyBindings/{policyBinding}.
// +k8s:deepcopy-gen=false
type IAMPolicyBindingIdentity struct {
	// Parent is organizations/{id}, folders/{id} or projects/{id}.
	Parent        string
	Location      string
	PolicyBinding string
}

func (i *IAMPolicyBindingIdentity) String() string {
	return i.ParentString() + "/policyBindings/" + i.PolicyBinding
}

// ParentString is the collection's parent, {parent}/locations/{location}.
func (i *IAMPolicyBindingIdentity) ParentString() string {
	return i.Parent + "/locations/" + i.Location
}

func (i *IAMPolicyBindingIdentity) Host() string {
	return "iam.googleapis.com"
}

func (i *IAMPolicyBindingIdentity) FromExternal(ref string) error {
	s := identity.StripReferencePrefixes(ref, i.Host())
	s = strings.TrimPrefix(s, "v3/")
	t := strings.Split(s, "/")
	if len(t) != 6 || (t[0] != "organizations" && t[0] != "folders" && t[0] != "projects") ||
		t[1] == "" || t[2] != "locations" || t[3] == "" || t[4] != "policyBindings" || t[5] == "" {
		return fmt.Errorf("format of IAMPolicyBinding external=%q was not known (use {organizations|folders|projects}/{id}/locations/{location}/policyBindings/{policyBinding})", ref)
	}
	*i = IAMPolicyBindingIdentity{Parent: t[0] + "/" + t[1], Location: t[3], PolicyBinding: t[5]}
	return nil
}

func getIdentityFromIAMPolicyBindingSpec(ctx context.Context, reader client.Reader, obj *IAMPolicyBinding) (*IAMPolicyBindingIdentity, error) {
	resourceID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID: %w", err)
	}
	org, folder, project := obj.Spec.OrganizationRef, obj.Spec.FolderRef, obj.Spec.ProjectRef
	n := 0
	for _, set := range []bool{org != nil, folder != nil, project != nil} {
		if set {
			n++
		}
	}
	if n != 1 {
		return nil, fmt.Errorf("exactly one of spec.organizationRef, spec.folderRef and spec.projectRef is required")
	}
	var parent string
	switch {
	case org != nil:
		o, err := refs.ResolveOrganization(ctx, reader, obj, org)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve organization: %w", err)
		}
		parent = "organizations/" + o.OrganizationID
	case folder != nil:
		f, err := refs.ResolveFolder(ctx, reader, obj, folder)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve folder: %w", err)
		}
		parent = "folders/" + f.FolderID
	default:
		p, err := refs.ResolveProject(ctx, reader, obj.GetNamespace(), project)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve project: %w", err)
		}
		parent = "projects/" + p.ProjectID
	}
	location := common.ValueOf(obj.Spec.Location)
	if location == "" {
		location = "global"
	}
	return &IAMPolicyBindingIdentity{Parent: parent, Location: location, PolicyBinding: resourceID}, nil
}

func (obj *IAMPolicyBinding) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromIAMPolicyBindingSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}
	// Cross-check the identity against the status value, if present.
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" {
		statusIdentity := &IAMPolicyBindingIdentity{}
		if err := statusIdentity.FromExternal(externalRef); err != nil {
			return nil, err
		}
		if statusIdentity.String() != specIdentity.String() {
			return nil, fmt.Errorf("cannot change IAMPolicyBinding identity (old=%q, new=%q)", statusIdentity.String(), specIdentity.String())
		}
	}
	return specIdentity, nil
}
