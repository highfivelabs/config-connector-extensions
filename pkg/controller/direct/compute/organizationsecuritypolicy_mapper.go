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
// pkg/controller/direct/compute/mappers.go (UPSTREAM.md). Only the
// ComputeOrganizationSecurityPolicy and ComputeOrganizationSecurityPolicyAssociation
// functions, unchanged but for our KRM types (compute.gcp.hfl.dev/v1beta1)
// and the version in their names.

package compute

import (
	"strconv"

	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krmcomputev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

// ComputeOrganizationSecurityPolicySpec_v1beta1_FromProto converts a v1.SecurityPolicy proto to a v1beta1.ComputeOrganizationSecurityPolicySpec.
// We hand-code this function because KRM displayName and parent map to proto ShortName and Parent.
func ComputeOrganizationSecurityPolicySpec_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.SecurityPolicy) *krmcomputev1beta1.ComputeOrganizationSecurityPolicySpec {
	if in == nil {
		return nil
	}
	out := &krmcomputev1beta1.ComputeOrganizationSecurityPolicySpec{}
	out.Description = in.Description
	out.DisplayName = in.GetShortName()
	out.Parent = in.GetParent()
	out.Type = in.Type
	return out
}

// ComputeOrganizationSecurityPolicySpec_v1beta1_ToProto converts a v1beta1.ComputeOrganizationSecurityPolicySpec to a v1.SecurityPolicy proto.
// We hand-code this function because KRM displayName and parent map to proto ShortName and Parent.
func ComputeOrganizationSecurityPolicySpec_v1beta1_ToProto(mapCtx *direct.MapContext, in *krmcomputev1beta1.ComputeOrganizationSecurityPolicySpec) *pb.SecurityPolicy {
	if in == nil {
		return nil
	}
	out := &pb.SecurityPolicy{}
	out.Description = in.Description
	if in.DisplayName != "" {
		out.ShortName = &in.DisplayName
	}
	if in.Parent != "" {
		out.Parent = &in.Parent
	}
	out.Type = in.Type
	return out
}

// ComputeOrganizationSecurityPolicyStatus_v1beta1_FromProto converts a v1.SecurityPolicy proto to a v1beta1.ComputeOrganizationSecurityPolicyStatus.
// We hand-code this function because policyId maps to Id (which requires string to uint64 conversion).
func ComputeOrganizationSecurityPolicyStatus_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.SecurityPolicy) *krmcomputev1beta1.ComputeOrganizationSecurityPolicyStatus {
	if in == nil {
		return nil
	}
	out := &krmcomputev1beta1.ComputeOrganizationSecurityPolicyStatus{}
	out.Fingerprint = in.Fingerprint
	if in.Id != nil {
		idStr := strconv.FormatUint(*in.Id, 10)
		out.PolicyId = &idStr
	}
	return out
}

// ComputeOrganizationSecurityPolicyStatus_v1beta1_ToProto converts a v1beta1.ComputeOrganizationSecurityPolicyStatus to a v1.SecurityPolicy proto.
// We hand-code this function because policyId maps to Id (which requires string to uint64 conversion).
func ComputeOrganizationSecurityPolicyStatus_v1beta1_ToProto(mapCtx *direct.MapContext, in *krmcomputev1beta1.ComputeOrganizationSecurityPolicyStatus) *pb.SecurityPolicy {
	if in == nil {
		return nil
	}
	out := &pb.SecurityPolicy{}
	out.Fingerprint = in.Fingerprint
	if in.PolicyId != nil {
		idVal, err := strconv.ParseUint(*in.PolicyId, 10, 64)
		if err != nil {
			mapCtx.Errorf("parsing policyId %q: %v", *in.PolicyId, err)
		} else {
			out.Id = &idVal
		}
	}
	return out
}

// ComputeOrganizationSecurityPolicyAssociationSpec_v1beta1_FromProto maps a GCP proto SecurityPolicyAssociation to a KRM spec.
func ComputeOrganizationSecurityPolicyAssociationSpec_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.SecurityPolicyAssociation) *krmcomputev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec {
	if in == nil {
		return nil
	}
	out := &krmcomputev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec{}
	out.AttachmentID = in.GetAttachmentId()
	if in.SecurityPolicyId != nil {
		out.PolicyID = in.SecurityPolicyId
	}
	if in.GetName() != "" {
		out.ResourceID = direct.LazyPtr(in.GetName())
	}
	return out
}

// ComputeOrganizationSecurityPolicyAssociationSpec_v1beta1_ToProto maps a KRM spec to a GCP proto SecurityPolicyAssociation.
func ComputeOrganizationSecurityPolicyAssociationSpec_v1beta1_ToProto(mapCtx *direct.MapContext, in *krmcomputev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) *pb.SecurityPolicyAssociation {
	if in == nil {
		return nil
	}
	out := &pb.SecurityPolicyAssociation{}
	if in.AttachmentID != "" {
		out.AttachmentId = direct.LazyPtr(in.AttachmentID)
	}
	if in.PolicyID != nil && *in.PolicyID != "" {
		out.SecurityPolicyId = in.PolicyID
	}
	if in.ResourceID != nil {
		out.Name = in.ResourceID
	}
	return out
}

// ComputeOrganizationSecurityPolicyAssociationStatus_v1beta1_FromProto maps a GCP proto SecurityPolicyAssociation to a KRM status.
func ComputeOrganizationSecurityPolicyAssociationStatus_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.SecurityPolicyAssociation) *krmcomputev1beta1.ComputeOrganizationSecurityPolicyAssociationStatus {
	if in == nil {
		return nil
	}
	out := &krmcomputev1beta1.ComputeOrganizationSecurityPolicyAssociationStatus{}
	out.DisplayName = in.DisplayName
	return out
}

// ComputeOrganizationSecurityPolicyAssociationStatus_v1beta1_ToProto maps a KRM status to a GCP proto SecurityPolicyAssociation.
func ComputeOrganizationSecurityPolicyAssociationStatus_v1beta1_ToProto(mapCtx *direct.MapContext, in *krmcomputev1beta1.ComputeOrganizationSecurityPolicyAssociationStatus) *pb.SecurityPolicyAssociation {
	if in == nil {
		return nil
	}
	out := &pb.SecurityPolicyAssociation{}
	out.DisplayName = in.DisplayName
	return out
}
