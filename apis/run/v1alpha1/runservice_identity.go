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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// apis/run/v1alpha1/runworkerpool_identity.go (UPSTREAM.md): a Cloud Run service
// (projects/{project}/locations/{location}/services/{service}) instead of a
// worker pool; otherwise unchanged.

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &RunServiceIdentity{}
	_ identity.Resource   = &RunService{}
)

var RunServiceIdentityFormat = gcpurls.Template[RunServiceIdentity](
	"run.googleapis.com",
	"projects/{project}/locations/{location}/services/{service}",
)

// RunServiceIdentity is the identity of a GCP RunService resource.
// +k8s:deepcopy-gen=false
type RunServiceIdentity struct {
	Project  string
	Location string
	Service  string
}

func (i *RunServiceIdentity) String() string {
	return RunServiceIdentityFormat.ToString(*i)
}

func (i *RunServiceIdentity) FromExternal(ref string) error {
	parsed, match, err := RunServiceIdentityFormat.Parse(ref)
	if err != nil {
		return fmt.Errorf("format of RunService external=%q was not known (use %s): %w", ref, RunServiceIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of RunService external=%q was not known (use %s)", ref, RunServiceIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *RunServiceIdentity) Host() string {
	return RunServiceIdentityFormat.Host()
}

func getIdentityFromRunServiceSpec(ctx context.Context, reader client.Reader, obj *RunService) (*RunServiceIdentity, error) {
	resourceID := common.ValueOf(obj.Spec.ResourceID)
	if resourceID == "" {
		resourceID = obj.GetName()
	}
	if resourceID == "" {
		return nil, fmt.Errorf("cannot resolve resource ID")
	}

	location := common.ValueOf(obj.Spec.Location)
	if location == "" {
		return nil, fmt.Errorf("cannot resolve location")
	}

	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}

	return &RunServiceIdentity{
		Project:  projectID,
		Location: location,
		Service:  resourceID,
	}, nil
}

func (obj *RunService) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromRunServiceSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef != "" {
		statusIdentity := &RunServiceIdentity{}
		if err := statusIdentity.FromExternal(externalRef); err != nil {
			return nil, err
		}

		if statusIdentity.String() != specIdentity.String() {
			return nil, fmt.Errorf("cannot change RunService identity (old=%q, new=%q)", statusIdentity.String(), specIdentity.String())
		}
	}

	return specIdentity, nil
}

func (i *RunServiceIdentity) ParentString() string {
	return fmt.Sprintf("projects/%s/locations/%s", i.Project, i.Location)
}
