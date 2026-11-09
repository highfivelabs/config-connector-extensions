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

// Written here, in the shape of Config Connector's identities: upstream's
// ServiceIdentityIdentity (apis/serviceusage/v1beta1) has a project only.

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &ServiceIdentityIdentity{}

// ServiceIdentityIdentity is folders/{id}/services/{service} or
// organizations/{id}/services/{service}, the parent of
// services.generateServiceIdentity.
type ServiceIdentityIdentity struct {
	// Parent is folders/{id} or organizations/{id}.
	Parent string
	// Service is the service's name, such as osconfig.googleapis.com.
	Service string
}

func (i *ServiceIdentityIdentity) String() string {
	return i.Parent + "/services/" + i.Service
}

func (i *ServiceIdentityIdentity) FromExternal(ref string) error {
	t := strings.Split(strings.TrimPrefix(strings.TrimPrefix(ref, "//serviceusage.googleapis.com"), "/"), "/")
	if len(t) != 4 || (t[0] != "folders" && t[0] != "organizations") || t[1] == "" || t[2] != "services" || t[3] == "" {
		return fmt.Errorf("format of ServiceIdentity external=%q was not known (use folders/{id}/services/{service} or organizations/{id}/services/{service})", ref)
	}
	i.Parent, i.Service = t[0]+"/"+t[1], t[3]
	return nil
}

func (obj *ServiceIdentity) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	org, folder := obj.Spec.OrganizationRef, obj.Spec.FolderRef
	if (org == nil) == (folder == nil) {
		return nil, fmt.Errorf("exactly one of spec.organizationRef and spec.folderRef is required")
	}
	service := common.ValueOf(obj.Spec.ResourceID)
	if service == "" {
		service = obj.GetName()
	}
	id := &ServiceIdentityIdentity{Service: service}
	if org != nil {
		o, err := refsv1beta1.ResolveOrganization(ctx, reader, obj, org)
		if err != nil {
			return nil, err
		}
		id.Parent = "organizations/" + o.OrganizationID
	} else {
		f, err := refsv1beta1.ResolveFolder(ctx, reader, obj, folder)
		if err != nil {
			return nil, err
		}
		id.Parent = "folders/" + f.FolderID
	}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ServiceIdentity identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
