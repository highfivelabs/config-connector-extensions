// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/iam/mappers.go (UPSTREAM.md), the hand-written
// mappers upstream's IAMDenyPolicy controller uses in place of the generated
// ones: only the IAMDenyPolicy functions and the Expr mapping they call,
// unchanged but for the KRM types, which are this project's
// (iam.gcp.hfl.dev/v1beta1), and the names, which carry DenyPolicy so they
// do not collide with this package's other kinds.

package iam

import (
	pb "cloud.google.com/go/iam/apiv2/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	exprpb "google.golang.org/genproto/googleapis/type/expr"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
)

func DenyPolicyExpr_FromProto(mapCtx *direct.MapContext, in *exprpb.Expr) *krm.Expr {
	if in == nil {
		return nil
	}
	out := &krm.Expr{}
	out.Expression = direct.LazyPtr(in.Expression)
	out.Title = direct.LazyPtr(in.Title)
	out.Description = direct.LazyPtr(in.Description)
	out.Location = direct.LazyPtr(in.Location)
	return out
}

func DenyPolicyExpr_ToProto(mapCtx *direct.MapContext, in *krm.Expr) *exprpb.Expr {
	if in == nil {
		return nil
	}
	out := &exprpb.Expr{}
	out.Expression = direct.ValueOf(in.Expression)
	out.Title = direct.ValueOf(in.Title)
	out.Description = direct.ValueOf(in.Description)
	out.Location = direct.ValueOf(in.Location)
	return out
}

func IAMDenyPolicySpec_FromProto(mapCtx *direct.MapContext, in *pb.Policy) *krm.IAMDenyPolicySpec {
	if in == nil {
		return nil
	}
	out := &krm.IAMDenyPolicySpec{}
	// MISSING: Name
	// MISSING: Uid
	// MISSING: Kind
	out.DisplayName = direct.LazyPtr(in.GetDisplayName())
	// MISSING: Annotations
	// MISSING: Etag
	// MISSING: CreateTime
	// MISSING: UpdateTime
	// MISSING: DeleteTime
	out.Rules = direct.Slice_FromProto(mapCtx, in.Rules, DenyPolicyRule_FromProto)
	// MISSING: ManagingAuthority
	return out
}

func IAMDenyPolicySpec_ToProto(mapCtx *direct.MapContext, in *krm.IAMDenyPolicySpec) *pb.Policy {
	if in == nil {
		return nil
	}
	out := &pb.Policy{}
	// MISSING: Name
	// MISSING: Uid
	// MISSING: Kind
	out.DisplayName = direct.ValueOf(in.DisplayName)
	// MISSING: Annotations
	// MISSING: Etag
	// MISSING: CreateTime
	// MISSING: UpdateTime
	// MISSING: DeleteTime
	out.Rules = direct.Slice_ToProto(mapCtx, in.Rules, DenyPolicyRule_ToProto)
	// MISSING: ManagingAuthority
	return out
}

func DenyPolicyRule_FromProto(mapCtx *direct.MapContext, in *pb.PolicyRule) *krm.PolicyRule {
	if in == nil {
		return nil
	}
	out := &krm.PolicyRule{}
	out.DenyRule = DenyPolicyDenyRule_FromProto(mapCtx, in.GetDenyRule())
	out.Description = direct.LazyPtr(in.GetDescription())
	return out
}

func DenyPolicyRule_ToProto(mapCtx *direct.MapContext, in *krm.PolicyRule) *pb.PolicyRule {
	if in == nil {
		return nil
	}
	out := &pb.PolicyRule{}
	if oneof := DenyPolicyDenyRule_ToProto(mapCtx, in.DenyRule); oneof != nil {
		out.Kind = &pb.PolicyRule_DenyRule{DenyRule: oneof}
	}
	out.Description = direct.ValueOf(in.Description)
	return out
}

func DenyPolicyDenyRule_FromProto(mapCtx *direct.MapContext, in *pb.DenyRule) *krm.DenyRule {
	if in == nil {
		return nil
	}
	out := &krm.DenyRule{}
	if len(in.DeniedPrincipals) > 0 {
		out.DeniedPrincipals = in.DeniedPrincipals
	}
	if len(in.ExceptionPrincipals) > 0 {
		out.ExceptionPrincipals = in.ExceptionPrincipals
	}
	if len(in.DeniedPermissions) > 0 {
		out.DeniedPermissions = in.DeniedPermissions
	}
	if len(in.ExceptionPermissions) > 0 {
		out.ExceptionPermissions = in.ExceptionPermissions
	}
	out.DenialCondition = DenyPolicyExpr_FromProto(mapCtx, in.GetDenialCondition())
	return out
}

func DenyPolicyDenyRule_ToProto(mapCtx *direct.MapContext, in *krm.DenyRule) *pb.DenyRule {
	if in == nil {
		return nil
	}
	out := &pb.DenyRule{}
	out.DeniedPrincipals = in.DeniedPrincipals
	out.ExceptionPrincipals = in.ExceptionPrincipals
	out.DeniedPermissions = in.DeniedPermissions
	out.ExceptionPermissions = in.ExceptionPermissions
	out.DenialCondition = DenyPolicyExpr_ToProto(mapCtx, in.DenialCondition)
	return out
}
