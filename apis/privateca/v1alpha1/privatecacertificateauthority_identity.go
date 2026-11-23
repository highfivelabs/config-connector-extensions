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

package v1alpha1

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector's apis/privateca/v1beta1/privatecacertificateauthority_identity.go.
// Changed: the name is parsed and printed here rather than with gcpurls
// (the same format); a projectRef or location that is not the CA pool's,
// and a name that differs from status.externalRef, are refused.

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/privatecarefs"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &PrivateCACertificateAuthorityIdentity{}

// PrivateCACertificateAuthorityIdentity is
// projects/{project}/locations/{location}/caPools/{caPool}/certificateAuthorities/{certificateAuthority}.
// +k8s:deepcopy-gen=false
type PrivateCACertificateAuthorityIdentity struct {
	Project              string
	Location             string
	CAPool               string
	CertificateAuthority string
}

func (i *PrivateCACertificateAuthorityIdentity) String() string {
	return i.ParentString() + "/certificateAuthorities/" + i.CertificateAuthority
}

func (i *PrivateCACertificateAuthorityIdentity) FromExternal(ref string) error {
	tokens := strings.Split(strings.TrimPrefix(strings.TrimPrefix(ref, "//privateca.googleapis.com/"), "/"), "/")
	if len(tokens) != 8 || tokens[0] != "projects" || tokens[2] != "locations" || tokens[4] != "caPools" || tokens[6] != "certificateAuthorities" {
		return fmt.Errorf("format of PrivateCACertificateAuthority external=%q was not known (use projects/{{project}}/locations/{{location}}/caPools/{{caPool}}/certificateAuthorities/{{certificateAuthority}})", ref)
	}
	*i = PrivateCACertificateAuthorityIdentity{Project: tokens[1], Location: tokens[3], CAPool: tokens[5], CertificateAuthority: tokens[7]}
	return nil
}

func (i *PrivateCACertificateAuthorityIdentity) ParentString() string {
	return fmt.Sprintf("projects/%s/locations/%s/caPools/%s", i.Project, i.Location, i.CAPool)
}

// GetIdentity is the authority spec.resourceID (else metadata.name) in the
// referenced CA pool.
func (obj *PrivateCACertificateAuthority) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	resourceID := common.ValueOf(obj.Spec.ResourceID)
	if resourceID == "" {
		resourceID = obj.GetName()
	}
	ref := obj.Spec.CaPoolRef
	if err := ref.Normalize(ctx, reader, obj.GetNamespace()); err != nil {
		return nil, fmt.Errorf("cannot resolve caPoolRef: %w", err)
	}
	pool := &privatecarefs.PrivateCACAPoolIdentity{}
	if err := pool.FromExternal(ref.External); err != nil {
		return nil, fmt.Errorf("cannot parse caPoolRef external string %q: %w", ref.External, err)
	}
	project, err := refsv1beta1.ResolveProject(ctx, reader, obj.GetNamespace(), &obj.Spec.ProjectRef)
	if err != nil {
		return nil, err
	}
	if project.ProjectID != pool.Project {
		return nil, fmt.Errorf("spec.projectRef is project %q but the CA pool %q is in project %q", project.ProjectID, ref.External, pool.Project)
	}
	if obj.Spec.Location != pool.Location {
		return nil, fmt.Errorf("spec.location is %q but the CA pool %q is in %q", obj.Spec.Location, ref.External, pool.Location)
	}
	id := &PrivateCACertificateAuthorityIdentity{Project: pool.Project, Location: pool.Location, CAPool: pool.CAPool, CertificateAuthority: resourceID}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change PrivateCACertificateAuthority identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
