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

// Modified by High Five Labs for config-connector-extensions: adapted from Config Connector
// apis/compute/v1alpha1/computeautoscaler_identity.go (UPSTREAM.md), the zonal
// autoscaler's identity, for our stable regional kind (docs/design.md
// decision 26; upstream has no identity for the regional one). Changed: the
// package; a region in place of the zone.

package v1beta1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &ComputeRegionAutoscalerIdentity{}
	_ identity.Resource   = &ComputeRegionAutoscaler{}
)

var ComputeRegionAutoscalerIdentityFormat = gcpurls.Template[ComputeRegionAutoscalerIdentity](
	"compute.googleapis.com",
	"projects/{project}/regions/{region}/autoscalers/{autoscaler}",
)

// ComputeRegionAutoscalerIdentity is the identity of a GCP ComputeRegionAutoscaler resource.
// +k8s:deepcopy-gen=false
type ComputeRegionAutoscalerIdentity struct {
	Project    string
	Region     string
	Autoscaler string
}

func (i *ComputeRegionAutoscalerIdentity) String() string {
	return ComputeRegionAutoscalerIdentityFormat.ToString(*i)
}

func (i *ComputeRegionAutoscalerIdentity) FromExternal(ref string) error {
	ref = refs.TrimComputeURIPrefix(ref)
	parsed, match, err := ComputeRegionAutoscalerIdentityFormat.Parse(ref)
	if err != nil {
		return fmt.Errorf("format of ComputeRegionAutoscaler external=%q was not known (use %s): %w", ref, ComputeRegionAutoscalerIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of ComputeRegionAutoscaler external=%q was not known (use %s)", ref, ComputeRegionAutoscalerIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *ComputeRegionAutoscalerIdentity) Host() string {
	return ComputeRegionAutoscalerIdentityFormat.Host()
}

func getIdentityFromComputeRegionAutoscalerSpec(ctx context.Context, reader client.Reader, obj *ComputeRegionAutoscaler) (*ComputeRegionAutoscalerIdentity, error) {
	resourceID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID")
	}

	region := obj.Spec.Region
	if region == "" {
		return nil, fmt.Errorf("cannot resolve region: spec.region is empty")
	}

	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project")
	}

	identity := &ComputeRegionAutoscalerIdentity{
		Project:    projectID,
		Region:     region,
		Autoscaler: resourceID,
	}
	return identity, nil
}

func (obj *ComputeRegionAutoscaler) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromComputeRegionAutoscalerSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	return specIdentity, nil
}
