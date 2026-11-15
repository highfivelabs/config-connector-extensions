// Copyright 2025 Google LLC
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
// apis/asset/v1beta1/assetfeed_identity.go (UPSTREAM.md). Changed: the
// parent comparison with status.externalRef treats a project's number and
// its ID as the same project. Google Cloud names a project's feed
// projects/<number>/feeds/<id> whatever the create named it, and upstream
// compared that number with the project ID the projectRef resolves to, so
// every reconcile after the create failed ("spec.projectRef changed, expect
// <number>, got <id>") and the feed could not be deleted. NewFeedIdentity
// takes a ProjectNumber function to resolve the ID's number when they
// differ, and the identity is then the recorded name. NewFeedIdentityFromExternal
// is new: a delete names the feed from status.externalRef without resolving
// the parent, which may already be gone.

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FeedIdentity is the identity of an AssetFeed.
// +kubebuilder:object:generate=false
type FeedIdentity struct {
	parent *FeedParent
	id     string
}

func (i *FeedIdentity) String() string {
	if i.parent == nil || i.parent.String() == "" {
		return "" // Or handle error appropriately
	}
	return i.parent.String() + "/feeds/" + i.id
}

func (i *FeedIdentity) ID() string {
	return i.id
}

func (i *FeedIdentity) Parent() *FeedParent {
	return i.parent
}

// FeedParent represents the parent of an AssetFeed, which can be a Project, Folder, or Organization.
// +kubebuilder:object:generate=false
type FeedParent struct {
	ProjectID      string
	FolderID       string
	OrganizationID string
}

// String returns the string representation of the FeedParent.
// Format: "projects/{projectID}" or "folders/{folderID}" or "organizations/{organizationID}"
func (p *FeedParent) String() string {
	if p.ProjectID != "" {
		return "projects/" + p.ProjectID
	}
	if p.FolderID != "" {
		return "folders/" + p.FolderID
	}
	if p.OrganizationID != "" {
		return "organizations/" + p.OrganizationID
	}
	// Should not happen if constructed correctly
	return ""
}

// ProjectNumber returns the number of the project with the given ID.
// +kubebuilder:object:generate=false
type ProjectNumber func(ctx context.Context, projectID string) (string, error)

// isProjectNumber says whether a project segment is a number (project IDs
// start with a letter).
func isProjectNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// sameProject says whether the project segments desired (from the spec)
// and actual (from status.externalRef) name the same project: equal, or
// one the number of the other, which projectNumber resolves.
func sameProject(ctx context.Context, desired, actual string, projectNumber ProjectNumber) (bool, error) {
	if desired == actual {
		return true, nil
	}
	if projectNumber == nil || isProjectNumber(desired) == isProjectNumber(actual) {
		return false, nil
	}
	id, number := desired, actual
	if isProjectNumber(desired) {
		id, number = actual, desired
	}
	n, err := projectNumber(ctx, id)
	if err != nil {
		return false, fmt.Errorf("reading the number of project %q: %w", id, err)
	}
	return n == number, nil
}

// NewFeedIdentity builds a FeedIdentity from the Config Connector Feed object.
func NewFeedIdentity(ctx context.Context, reader client.Reader, obj *AssetFeed, projectNumber ProjectNumber) (*FeedIdentity, error) {
	var projectID, folderID, organizationID string
	// Get Parent
	if obj.Spec.AssetFeedParent.ProjectRef != nil {
		projectRef, err := refsv1beta1.ResolveProject(ctx, reader, obj.GetNamespace(), obj.Spec.AssetFeedParent.ProjectRef)
		if err != nil {
			return nil, err
		}
		projectID = projectRef.ProjectID
		if projectID == "" {
			return nil, fmt.Errorf("cannot resolve project")
		}
	} else if obj.Spec.AssetFeedParent.FolderRef != nil {
		folderRef, err := refsv1beta1.ResolveFolder(ctx, reader, obj, obj.Spec.AssetFeedParent.FolderRef)
		if err != nil {
			return nil, err
		}
		folderID = folderRef.FolderID
		if folderID == "" {
			return nil, fmt.Errorf("cannot resolve folder")
		}
	} else if obj.Spec.AssetFeedParent.OrganizationRef != nil {
		organizationRef, err := refsv1beta1.ResolveOrganization(ctx, reader, obj, obj.Spec.AssetFeedParent.OrganizationRef)
		if err != nil {
			return nil, err
		}
		organizationID = organizationRef.OrganizationID
		if organizationID == "" {
			return nil, fmt.Errorf("cannot resolve organization")
		}
	} else {
		return nil, fmt.Errorf("one of spec.parent.projectRef, spec.parent.folderRef, or spec.parent.organizationRef must be set")
	}

	// Get desired ID
	resourceID := common.ValueOf(obj.Spec.ResourceID)
	if resourceID == "" {
		resourceID = obj.GetName()
	}
	if resourceID == "" {
		return nil, fmt.Errorf("cannot resolve resource ID")
	}

	// Use approved External
	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef != "" {
		// Validate desired with actual
		actualParent, actualResourceID, err := ParseFeedExternal(externalRef)
		if err != nil {
			return nil, err
		}
		if (actualParent.ProjectID == "") != (projectID == "") {
			return nil, fmt.Errorf("spec.projectRef changed, expect %q, got %q", actualParent.ProjectID, projectID)
		}
		if projectID != "" {
			same, err := sameProject(ctx, projectID, actualParent.ProjectID, projectNumber)
			if err != nil {
				return nil, err
			}
			if !same {
				return nil, fmt.Errorf("spec.projectRef changed, expect %s, got %s", actualParent.ProjectID, projectID)
			}
		}
		if actualParent.FolderID != folderID {
			return nil, fmt.Errorf("spec.folderRef changed, expect %s, got %s", actualParent.FolderID, folderID)
		}
		if actualParent.OrganizationID != organizationID {
			return nil, fmt.Errorf("spec.organizationRef changed, expect %s, got %s", actualParent.OrganizationID, organizationID)
		}
		if actualResourceID != resourceID {
			return nil, fmt.Errorf("cannot reset `metadata.name` or `spec.resourceID` to %s, since it has already assigned to %s",
				resourceID, actualResourceID)
		}
		// The same feed: name it as Google Cloud does.
		return &FeedIdentity{parent: actualParent, id: actualResourceID}, nil
	}
	return &FeedIdentity{
		parent: &FeedParent{
			ProjectID:      projectID,
			FolderID:       folderID,
			OrganizationID: organizationID,
		},
		id: resourceID,
	}, nil
}

// NewFeedIdentityFromExternal is the identity of the feed status.externalRef
// names, read without resolving the spec's parent.
func NewFeedIdentityFromExternal(external string) (*FeedIdentity, error) {
	parent, id, err := ParseFeedExternal(external)
	if err != nil {
		return nil, err
	}
	return &FeedIdentity{parent: parent, id: id}, nil
}

// ParseFeedExternal parses the external identifier string into its components.
// Format: "projects/{projectID}/feeds/{feedID}"
//
//	or "folders/{folderID}/feeds/{feedID}"
//	or "organizations/{organizationID}/feeds/{feedID}"
func ParseFeedExternal(external string) (parent *FeedParent, resourceID string, err error) {
	tokens := strings.Split(external, "/")
	// Expect 4 tokens, e.g., projects/{id}/feeds/{feedId}
	if len(tokens) != 4 || tokens[2] != "feeds" {
		return nil, "", fmt.Errorf("format of AssetFeed external=%q was not known (use projects/{projectID}/feeds/{feedID}, folders/{folderID}/feeds/{feedID}, or organizations/{organizationID}/feeds/{feedID})", external)
	}

	parent = &FeedParent{}
	parentType := tokens[0]
	parentVal := tokens[1]
	resourceID = tokens[3]

	switch parentType {
	case "projects":
		parent.ProjectID = parentVal
	case "folders":
		parent.FolderID = parentVal
	case "organizations":
		parent.OrganizationID = parentVal
	default:
		return nil, "", fmt.Errorf("format of AssetFeed external=%q was not known (unknown parent type %q)", external, parentType)
	}

	if parentVal == "" {
		return nil, "", fmt.Errorf("format of AssetFeed external=%q was not known (empty parent ID)", external)
	}
	if resourceID == "" {
		return nil, "", fmt.Errorf("format of AssetFeed external=%q was not known (empty resource ID)", external)
	}

	return parent, resourceID, nil
}
