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

// Modified by High Five Labs for config-connector-extensions: adapted from
// Config Connector's apis/compute/v1alpha1/computediskresourcepolicyattachment_identity.go
// (UPSTREAM.md), the zonal attachment's identity, for a regional disk
// (upstream has no direct regional kind). Changed: the package; region for
// zone; the referenced disk must be a regional disk in spec.region.

package v1beta1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	apirefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &ComputeRegionDiskResourcePolicyAttachmentIdentity{}
	_ identity.Resource   = &ComputeRegionDiskResourcePolicyAttachment{}
)

var ComputeRegionDiskResourcePolicyAttachmentIdentityFormat = gcpurls.Template[ComputeRegionDiskResourcePolicyAttachmentIdentity](
	"compute.googleapis.com",
	"projects/{project}/regions/{region}/disks/{disk}/{name}",
)

// ComputeRegionDiskResourcePolicyAttachmentIdentity is the identity of a GCP ComputeRegionDiskResourcePolicyAttachment resource.
// +k8s:deepcopy-gen=false
type ComputeRegionDiskResourcePolicyAttachmentIdentity struct {
	Project string
	Region  string
	Disk    string
	Name    string
}

func (i *ComputeRegionDiskResourcePolicyAttachmentIdentity) String() string {
	return ComputeRegionDiskResourcePolicyAttachmentIdentityFormat.ToString(*i)
}

func (i *ComputeRegionDiskResourcePolicyAttachmentIdentity) FromExternal(ref string) error {
	trimmedRef := apirefs.TrimComputeURIPrefix(ref)
	parsed, match, err := ComputeRegionDiskResourcePolicyAttachmentIdentityFormat.Parse(trimmedRef)
	if err != nil {
		return fmt.Errorf("format of ComputeRegionDiskResourcePolicyAttachment external=%q was not known (use %s): %w", ref, ComputeRegionDiskResourcePolicyAttachmentIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of ComputeRegionDiskResourcePolicyAttachment external=%q was not known (use %s)", ref, ComputeRegionDiskResourcePolicyAttachmentIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *ComputeRegionDiskResourcePolicyAttachmentIdentity) Host() string {
	return ComputeRegionDiskResourcePolicyAttachmentIdentityFormat.Host()
}

func (i *ComputeRegionDiskResourcePolicyAttachmentIdentity) ParentString() string {
	return fmt.Sprintf("projects/%s/regions/%s/disks/%s", i.Project, i.Region, i.Disk)
}

func getIdentityFromComputeRegionDiskResourcePolicyAttachmentSpec(ctx context.Context, reader client.Reader, obj *ComputeRegionDiskResourcePolicyAttachment) (*ComputeRegionDiskResourcePolicyAttachmentIdentity, error) {
	resourceID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID")
	}

	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project")
	}

	region := obj.Spec.Region
	if region == "" {
		return nil, fmt.Errorf("cannot resolve region: spec.region is empty")
	}

	diskRef := obj.Spec.DiskRef
	if err := diskRef.Normalize(ctx, reader, obj.Namespace); err != nil {
		return nil, fmt.Errorf("cannot resolve diskRef: %w", err)
	}

	diskId, err := computev1beta1.ParseComputeDiskExternal(diskRef.External)
	if err != nil {
		return nil, fmt.Errorf("cannot parse disk external reference: %w", err)
	}
	if !diskId.IsRegional() {
		return nil, fmt.Errorf("diskRef %q is not a regional disk (use projects/{project}/regions/{region}/disks/{disk})", diskRef.External)
	}
	if diskId.Region != region {
		return nil, fmt.Errorf("diskRef %q is in region %q, not spec.region %q", diskRef.External, diskId.Region, region)
	}
	if diskId.Project != projectID {
		return nil, fmt.Errorf("diskRef %q is in project %q, not the attachment's project %q", diskRef.External, diskId.Project, projectID)
	}

	identity := &ComputeRegionDiskResourcePolicyAttachmentIdentity{
		Project: projectID,
		Region:  region,
		Disk:    diskId.Disk,
		Name:    resourceID,
	}
	return identity, nil
}

func (obj *ComputeRegionDiskResourcePolicyAttachment) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromComputeRegionDiskResourcePolicyAttachmentSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}
	return specIdentity, nil
}
