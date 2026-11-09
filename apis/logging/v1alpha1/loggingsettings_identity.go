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
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ identity.Identity = &LoggingSettingsIdentity{}

// LoggingSettingsIdentity is organizations/{id}/settings or
// folders/{id}/settings.
type LoggingSettingsIdentity struct {
	// Parent is organizations/{id} or folders/{id}.
	Parent string
}

func (i *LoggingSettingsIdentity) String() string {
	return i.Parent + "/settings"
}

func (i *LoggingSettingsIdentity) FromExternal(ref string) error {
	t := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if len(t) != 3 || (t[0] != "organizations" && t[0] != "folders") || t[1] == "" || t[2] != "settings" {
		return fmt.Errorf("format of LoggingSettings external=%q was not known (use organizations/{id}/settings or folders/{id}/settings)", ref)
	}
	i.Parent = t[0] + "/" + t[1]
	return nil
}

func (obj *LoggingSettings) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	org, folder := obj.Spec.OrganizationRef, obj.Spec.FolderRef
	if (org == nil) == (folder == nil) {
		return nil, fmt.Errorf("exactly one of spec.organizationRef and spec.folderRef is required")
	}
	id := &LoggingSettingsIdentity{}
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
		return nil, fmt.Errorf("cannot change LoggingSettings identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
