// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0
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

package v1alpha1

// Written here, in the shape of Config Connector's identities.

import (
	"context"
	"fmt"
	"strings"

	arv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/artifactregistry/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ArtifactRegistryTagIdentity{}

// ArtifactRegistryTagIdentity is
// projects/{project}/locations/{location}/repositories/{repository}/packages/{package}/tags/{tag}.
type ArtifactRegistryTagIdentity struct {
	Repository arv1beta1.ArtifactRegistryRepositoryIdentity
	// Package is the package's ID as written in the spec ("team/app").
	Package string
	Tag     string
}

// EncodePackage is a package ID as it appears in a resource name: a "/" in
// it is "%2F".
func EncodePackage(pkg string) string {
	return strings.ReplaceAll(pkg, "/", "%2F")
}

// PackageName is the package's resource name, the tag's parent.
func (i *ArtifactRegistryTagIdentity) PackageName() string {
	return i.Repository.String() + "/packages/" + EncodePackage(i.Package)
}

func (i *ArtifactRegistryTagIdentity) String() string {
	return i.PackageName() + "/tags/" + i.Tag
}

// VersionName is the resource name of one of the package's versions.
func (i *ArtifactRegistryTagIdentity) VersionName(version string) string {
	return i.PackageName() + "/versions/" + version
}

func (obj *ArtifactRegistryTag) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	ref := obj.Spec.RepositoryRef
	if ref == nil {
		return nil, fmt.Errorf("spec.repositoryRef is required")
	}
	if err := ref.Normalize(ctx, reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	repo := arv1beta1.ArtifactRegistryRepositoryIdentity{}
	if err := repo.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if obj.Spec.Package == "" {
		return nil, fmt.Errorf("spec.package is required")
	}
	tag, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, err
	}
	id := &ArtifactRegistryTagIdentity{Repository: repo, Package: obj.Spec.Package, Tag: tag}

	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ArtifactRegistryTag identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}

// FromExternal parses
// projects/{project}/locations/{location}/repositories/{repository}/packages/{package}/tags/{tag}.
func (i *ArtifactRegistryTagIdentity) FromExternal(ref string) error {
	tokens := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if len(tokens) != 10 || tokens[6] != "packages" || tokens[8] != "tags" {
		return fmt.Errorf("format of ArtifactRegistryTag external=%q was not known (use projects/{{projectID}}/locations/{{location}}/repositories/{{repositoryID}}/packages/{{package}}/tags/{{tag}})", ref)
	}
	if err := i.Repository.FromExternal(strings.Join(tokens[:6], "/")); err != nil {
		return err
	}
	i.Package = strings.ReplaceAll(tokens[7], "%2F", "/")
	i.Tag = tokens[9]
	return nil
}
