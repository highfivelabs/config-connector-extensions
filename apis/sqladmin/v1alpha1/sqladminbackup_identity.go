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

// Modified by High Five Labs for gaps-gcp. Copied from Config Connector
// apis/sqladmin/v1alpha1/sqladminbackup_identity.go (UPSTREAM.md) with the
// change from upstream pull request #12553 (head 9611a6de): a backup run's
// ID is chosen by Cloud SQL when it is created, so before creation the
// identity has no backup ID (not metadata.name), and afterwards it comes
// from status.externalRef unless spec.resourceID names one.

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &SQLAdminBackupIdentity{}
	_ identity.Resource   = &SQLAdminBackup{}
)

var SQLAdminBackupIdentityFormat = gcpurls.Template[SQLAdminBackupIdentity]("sqladmin.googleapis.com", "projects/{project}/backups/{backup}")

// +k8s:deepcopy-gen=false
type SQLAdminBackupIdentity struct {
	Project string
	Backup  string
}

func (i *SQLAdminBackupIdentity) String() string {
	return SQLAdminBackupIdentityFormat.ToString(*i)
}

func (i *SQLAdminBackupIdentity) FromExternal(ref string) error {
	parsed, match, err := SQLAdminBackupIdentityFormat.Parse(ref)
	if err != nil {
		return fmt.Errorf("format of SQLAdminBackup external=%q was not known (use %s): %w", ref, SQLAdminBackupIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of SQLAdminBackup external=%q was not known (use %s)", ref, SQLAdminBackupIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *SQLAdminBackupIdentity) Host() string {
	return SQLAdminBackupIdentityFormat.Host()
}

func getIdentityFromSQLAdminBackupSpec(ctx context.Context, reader client.Reader, obj *SQLAdminBackup) (*SQLAdminBackupIdentity, error) {
	resourceID := common.ValueOf(obj.Spec.ResourceID)

	projectID, err := refsv1beta1.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}

	identity := &SQLAdminBackupIdentity{
		Project: projectID,
		Backup:  resourceID,
	}
	return identity, nil
}

func (obj *SQLAdminBackup) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromSQLAdminBackupSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	// Cross-check the identity against the status value, if present.
	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef != "" {
		// Validate desired with actual
		statusIdentity := &SQLAdminBackupIdentity{}
		if err := statusIdentity.FromExternal(externalRef); err != nil {
			return nil, err
		}

		specID := common.ValueOf(obj.Spec.ResourceID)
		if specID == "" {
			if statusIdentity.Project != specIdentity.Project {
				return nil, fmt.Errorf("cannot change SQLAdminBackup identity (old project=%q, new project=%q)", statusIdentity.Project, specIdentity.Project)
			}
			return statusIdentity, nil
		}

		if statusIdentity.Backup != specID {
			return nil, fmt.Errorf("cannot change SQLAdminBackup identity (old=%q, new=%q)", statusIdentity.Backup, specID)
		}
	}

	return specIdentity, nil
}

func (i *SQLAdminBackupIdentity) Parent() *SQLAdminBackupParent {
	return &SQLAdminBackupParent{
		Project: i.Project,
	}
}

type SQLAdminBackupParent struct {
	Project string
}

func (p *SQLAdminBackupParent) String() string {
	return "projects/" + p.Project
}
