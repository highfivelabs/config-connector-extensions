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
// apis/sql/v1beta1/sqlsslcert_identity.go (UPSTREAM.md) and changed for a
// user: the identity is projects/{project}/instances/{instance}/users/{name}
// and the name is user-chosen (spec.resourceID, else metadata.name), not
// server-generated. Host is part of a MySQL user's identity in the API but
// not of its URL, so it is carried beside the template.

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.Identity = &SQLUserIdentity{}
)

var SQLUserIdentityFormat = gcpurls.Template[SQLUserIdentity]("sqladmin.googleapis.com", "projects/{project}/instances/{instance}/users/{name}")

// SQLUserIdentity is the identity of a Cloud SQL user.
// +k8s:deepcopy-gen=false
type SQLUserIdentity struct {
	Project  string
	Instance string
	Name     string

	// UserHost is the MySQL host the user connects from; empty for PostgreSQL
	// and SQL Server. Not part of the URL.
	UserHost string
}

func (i *SQLUserIdentity) String() string {
	return SQLUserIdentityFormat.ToString(*i)
}

func (i *SQLUserIdentity) FromExternal(ref string) error {
	ref = strings.TrimPrefix(ref, "https://")
	ref = strings.TrimPrefix(ref, "http://")
	ref = strings.TrimPrefix(ref, "//")
	ref = strings.TrimPrefix(ref, "sqladmin.googleapis.com/")
	ref = strings.TrimPrefix(ref, "sql/v1beta4/")

	parsed, match, err := SQLUserIdentityFormat.Parse(ref)
	if err != nil {
		return fmt.Errorf("format of SQLUser external=%q was not known (use %s): %w", ref, SQLUserIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of SQLUser external=%q was not known (use %s)", ref, SQLUserIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *SQLUserIdentity) Host() string {
	return SQLUserIdentityFormat.Host()
}

func (i *SQLUserIdentity) ParentString() string {
	return fmt.Sprintf("projects/%s/instances/%s", i.Project, i.Instance)
}

func getIdentityFromSQLUserSpec(ctx context.Context, reader client.Reader, obj *SQLUser) (*SQLUserIdentity, error) {
	name := common.ValueOf(obj.Spec.ResourceID)
	if name == "" {
		name = obj.GetName()
	}
	if name == "" {
		return nil, fmt.Errorf("cannot resolve resource ID")
	}

	instanceRef := obj.Spec.InstanceRef
	instance, err := refsv1beta1.ResolveSQLInstanceRef(ctx, reader, obj, &instanceRef)
	if err != nil {
		return nil, fmt.Errorf("resolving spec.instanceRef: %w", err)
	}
	if instance == nil {
		return nil, fmt.Errorf("spec.instanceRef is required")
	}
	if instance.SQLInstanceName == "" {
		return nil, fmt.Errorf("cannot resolve instance name from spec.instanceRef")
	}

	projectID := instance.ProjectID
	if projectID == "" {
		projectID, err = refsv1beta1.ResolveProjectID(ctx, reader, obj)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve project: %w", err)
		}
	}

	return &SQLUserIdentity{
		Project:  projectID,
		Instance: instance.SQLInstanceName,
		Name:     name,
		UserHost: common.ValueOf(obj.Spec.Host),
	}, nil
}

// GetIdentity returns the identity of the user. The name is user-chosen, so
// unlike SQLSSLCert there is no status value to cross-check.
func (obj *SQLUser) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	return getIdentityFromSQLUserSpec(ctx, reader, obj)
}
