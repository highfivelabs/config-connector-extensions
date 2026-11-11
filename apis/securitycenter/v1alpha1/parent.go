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

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector pull request #7673 (head 423fea9f, open, unmerged),
// apis/securitycenter/v1alpha1/identity.go (UPSTREAM.md): the parent of a
// Security Command Center resource, an organization, folder or project.
// Changed: folderRef is refs.FolderRefDeprecated, the type refs.FolderRef
// aliases (controller-gen does not take aliases); ParseParent takes the
// parent from the first two segments and returns the rest unparsed.

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +k8s:deepcopy-gen=false
type SecurityCenterParent struct {
	OrganizationID string
	FolderID       string
	ProjectID      string
}

func (p *SecurityCenterParent) String() string {
	if p.OrganizationID != "" {
		return "organizations/" + p.OrganizationID
	}
	if p.FolderID != "" {
		return "folders/" + p.FolderID
	}
	if p.ProjectID != "" {
		return "projects/" + p.ProjectID
	}
	return ""
}

func ResolveParent(ctx context.Context, reader client.Reader, obj client.Object, orgRef *refs.OrganizationRef, folderRef *refs.FolderRefDeprecated, projectRef *refs.ProjectRef) (*SecurityCenterParent, error) {
	count := 0
	if orgRef != nil {
		count++
	}
	if folderRef != nil {
		count++
	}
	if projectRef != nil {
		count++
	}

	if count != 1 {
		return nil, fmt.Errorf("exactly one of organizationRef, folderRef, or projectRef must be specified")
	}

	if orgRef != nil {
		org, err := refs.ResolveOrganization(ctx, reader, obj, orgRef)
		if err != nil {
			return nil, err
		}
		return &SecurityCenterParent{OrganizationID: org.OrganizationID}, nil
	}

	if folderRef != nil {
		folder, err := refs.ResolveFolder(ctx, reader, obj, folderRef)
		if err != nil {
			return nil, err
		}
		return &SecurityCenterParent{FolderID: folder.FolderID}, nil
	}

	if projectRef != nil {
		project, err := refs.ResolveProject(ctx, reader, obj.GetNamespace(), projectRef)
		if err != nil {
			return nil, err
		}
		return &SecurityCenterParent{ProjectID: project.ProjectID}, nil
	}

	return nil, fmt.Errorf("could not resolve parent")
}

// ParseParent splits {organizations|folders|projects}/{id}/{rest} into the
// parent and rest.
func ParseParent(external string) (*SecurityCenterParent, string, error) {
	tokens := strings.SplitN(strings.TrimPrefix(external, "/"), "/", 3)
	if len(tokens) < 3 || tokens[1] == "" || tokens[2] == "" {
		return nil, "", fmt.Errorf("invalid external reference format: %s", external)
	}

	p := &SecurityCenterParent{}
	switch tokens[0] {
	case "organizations":
		p.OrganizationID = tokens[1]
	case "folders":
		p.FolderID = tokens[1]
	case "projects":
		p.ProjectID = tokens[1]
	default:
		return nil, "", fmt.Errorf("invalid parent type in external reference: %s", external)
	}
	return p, tokens[2], nil
}
