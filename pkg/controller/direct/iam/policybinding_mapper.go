// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

// Written by High Five Labs for config-connector-extensions after Config
// Connector pull request #14133's generated IAMPolicyBinding mapper
// (UPSTREAM.md): the parent and the policy are references, resolved by the
// controller, so they are not mapped here; etag and the policy kind the API
// set go to status.observedState.

package iam

import (
	iamv3pb "cloud.google.com/go/iam/apiv3/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	exprpb "google.golang.org/genproto/googleapis/type/expr"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
)

func IAMPolicyBindingObservedState_FromProto(mapCtx *direct.MapContext, in *iamv3pb.PolicyBinding) *krm.IAMPolicyBindingObservedState {
	if in == nil {
		return nil
	}
	out := &krm.IAMPolicyBindingObservedState{}
	out.Uid = direct.LazyPtr(in.GetUid())
	out.Etag = direct.LazyPtr(in.GetEtag())
	out.PolicyKind = direct.Enum_FromProto(mapCtx, in.GetPolicyKind())
	out.PolicyUid = direct.LazyPtr(in.GetPolicyUid())
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	return out
}

// IAMPolicyBindingSpec_FromProto maps what the binding holds, but its
// parent; the policy goes to policyRef.external.
func IAMPolicyBindingSpec_FromProto(mapCtx *direct.MapContext, in *iamv3pb.PolicyBinding) *krm.IAMPolicyBindingSpec {
	if in == nil {
		return nil
	}
	out := &krm.IAMPolicyBindingSpec{}
	out.DisplayName = direct.LazyPtr(in.GetDisplayName())
	out.Annotations = in.Annotations
	out.Target = PolicyBinding_Target_FromProto(mapCtx, in.GetTarget())
	out.PolicyKind = direct.Enum_FromProto(mapCtx, in.GetPolicyKind())
	if in.GetPolicy() != "" {
		out.PolicyRef = &krm.PolicyBindingPolicyRef{External: in.GetPolicy()}
	}
	out.Condition = PolicyBindingExpr_FromProto(mapCtx, in.GetCondition())
	return out
}

// IAMPolicyBindingSpec_ToProto maps the spec but its parent and policy,
// which the controller resolves.
func IAMPolicyBindingSpec_ToProto(mapCtx *direct.MapContext, in *krm.IAMPolicyBindingSpec) *iamv3pb.PolicyBinding {
	if in == nil {
		return nil
	}
	out := &iamv3pb.PolicyBinding{}
	out.DisplayName = direct.ValueOf(in.DisplayName)
	out.Annotations = in.Annotations
	out.Target = PolicyBinding_Target_ToProto(mapCtx, in.Target)
	out.PolicyKind = direct.Enum_ToProto[iamv3pb.PolicyBinding_PolicyKind](mapCtx, in.PolicyKind)
	out.Condition = PolicyBindingExpr_ToProto(mapCtx, in.Condition)
	return out
}

func PolicyBinding_Target_FromProto(mapCtx *direct.MapContext, in *iamv3pb.PolicyBinding_Target) *krm.PolicyBinding_Target {
	if in == nil {
		return nil
	}
	out := &krm.PolicyBinding_Target{}
	if v := in.GetPrincipalSet(); v != "" {
		out.PrincipalSet = &v
	}
	return out
}

func PolicyBinding_Target_ToProto(mapCtx *direct.MapContext, in *krm.PolicyBinding_Target) *iamv3pb.PolicyBinding_Target {
	if in == nil {
		return nil
	}
	out := &iamv3pb.PolicyBinding_Target{}
	if in.PrincipalSet != nil {
		out.Target = &iamv3pb.PolicyBinding_Target_PrincipalSet{PrincipalSet: *in.PrincipalSet}
	}
	return out
}

func PolicyBindingExpr_FromProto(mapCtx *direct.MapContext, in *exprpb.Expr) *krm.PolicyBindingExpr {
	if in == nil {
		return nil
	}
	out := &krm.PolicyBindingExpr{}
	out.Expression = direct.LazyPtr(in.GetExpression())
	out.Title = direct.LazyPtr(in.GetTitle())
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Location = direct.LazyPtr(in.GetLocation())
	return out
}

func PolicyBindingExpr_ToProto(mapCtx *direct.MapContext, in *krm.PolicyBindingExpr) *exprpb.Expr {
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
