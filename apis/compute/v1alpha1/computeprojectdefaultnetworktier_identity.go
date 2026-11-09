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

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ComputeProjectDefaultNetworkTierIdentity{}

// ComputeProjectDefaultNetworkTierIdentity is
// projects/{project}/defaultNetworkTier.
type ComputeProjectDefaultNetworkTierIdentity struct {
	Project string
}

func (i *ComputeProjectDefaultNetworkTierIdentity) String() string {
	return "projects/" + i.Project + "/defaultNetworkTier"
}

func (i *ComputeProjectDefaultNetworkTierIdentity) FromExternal(ref string) error {
	t := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if len(t) != 3 || t[0] != "projects" || t[1] == "" || t[2] != "defaultNetworkTier" {
		return fmt.Errorf("format of ComputeProjectDefaultNetworkTier external=%q was not known (use projects/{project}/defaultNetworkTier)", ref)
	}
	i.Project = t[1]
	return nil
}

func (obj *ComputeProjectDefaultNetworkTier) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}
	id := &ComputeProjectDefaultNetworkTierIdentity{Project: projectID}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeProjectDefaultNetworkTier identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
