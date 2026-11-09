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
// pkg/controller/direct/run/runworkerpool_mapper.go (UPSTREAM.md): the worker pool's
// spec and observed-state mappings become the service's (google.cloud.run.v2.Service),
// without instance splits; the KRM types are this project's. Written here: the
// spec's FromProto (generated upstream for the worker pool), the service's
// ingress, invokerIamDisabled, defaultUriDisabled and scaling, the URIs in
// observed state, and the scaling mappings.

package run

import (
	pb "cloud.google.com/go/run/apiv2/runpb"
	networkservicesv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/networkservices/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	krmrunv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
	apipb "google.golang.org/genproto/googleapis/api"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func BinaryAuthorization_UseDefault_ToProto(mapCtx *direct.MapContext, in *bool) *pb.BinaryAuthorization_UseDefault {
	if in == nil {
		return nil
	}
	return &pb.BinaryAuthorization_UseDefault{UseDefault: *in}
}

func BinaryAuthorization_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.BinaryAuthorization) *krmrunv1alpha1.BinaryAuthorization {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.BinaryAuthorization{}
	switch x := in.BinauthzMethod.(type) {
	case *pb.BinaryAuthorization_UseDefault:
		val := x.UseDefault
		out.UseDefault = &val
	}
	out.BreakglassJustification = direct.LazyPtr(in.GetBreakglassJustification())
	return out
}

func EnvVar_Value_ToProto(mapCtx *direct.MapContext, in *string) *pb.EnvVar_Value {
	if in == nil {
		return nil
	}
	return &pb.EnvVar_Value{Value: *in}
}

func RunServiceSpec_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.RunServiceSpec) *pb.Service {
	if in == nil {
		return nil
	}
	out := &pb.Service{}
	// MISSING: Name (handled by controller)
	out.Description = direct.ValueOf(in.Description)
	out.Labels = in.Labels
	out.Annotations = direct.MapStringString_ToProto(mapCtx, in.Annotations)
	out.Client = direct.ValueOf(in.Client)
	out.ClientVersion = direct.ValueOf(in.ClientVersion)
	out.LaunchStage = direct.Enum_ToProto[apipb.LaunchStage](mapCtx, in.LaunchStage)
	out.BinaryAuthorization = BinaryAuthorization_v1alpha1_ToProto(mapCtx, in.BinaryAuthorization)
	out.Template = RevisionTemplate_v1alpha1_ToProto(mapCtx, in.Template)
	out.Scaling = ServiceScaling_ToProto(mapCtx, in.Scaling)
	out.CustomAudiences = in.CustomAudiences
	// Written here: the service's own fields.
	out.Ingress = direct.Enum_ToProto[pb.IngressTraffic](mapCtx, in.Ingress)
	out.InvokerIamDisabled = direct.ValueOf(in.InvokerIamDisabled)
	out.DefaultUriDisabled = direct.ValueOf(in.DefaultUriDisabled)
	return out
}

// Written here (upstream generates it for the worker pool).
func RunServiceSpec_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.Service) *krmrunv1alpha1.RunServiceSpec {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.RunServiceSpec{}
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Labels = in.Labels
	out.Annotations = in.Annotations
	out.Client = direct.LazyPtr(in.GetClient())
	out.ClientVersion = direct.LazyPtr(in.GetClientVersion())
	out.LaunchStage = direct.Enum_FromProto(mapCtx, in.GetLaunchStage())
	out.BinaryAuthorization = BinaryAuthorization_v1alpha1_FromProto(mapCtx, in.GetBinaryAuthorization())
	out.Template = RevisionTemplate_v1alpha1_FromProto(mapCtx, in.GetTemplate())
	out.Scaling = ServiceScaling_FromProto(mapCtx, in.GetScaling())
	out.CustomAudiences = in.CustomAudiences
	out.Ingress = direct.Enum_FromProto(mapCtx, in.GetIngress())
	out.InvokerIamDisabled = direct.LazyPtr(in.GetInvokerIamDisabled())
	out.DefaultUriDisabled = direct.LazyPtr(in.GetDefaultUriDisabled())
	return out
}

// Written here.
func ServiceScaling_FromProto(mapCtx *direct.MapContext, in *pb.ServiceScaling) *krmrunv1alpha1.ServiceScaling {
	if in == nil {
		return nil
	}
	return &krmrunv1alpha1.ServiceScaling{
		MinInstanceCount:    direct.LazyPtr(in.GetMinInstanceCount()),
		ScalingMode:         direct.Enum_FromProto(mapCtx, in.GetScalingMode()),
		MaxInstanceCount:    direct.LazyPtr(in.GetMaxInstanceCount()),
		ManualInstanceCount: in.ManualInstanceCount,
	}
}

// Written here.
func ServiceScaling_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.ServiceScaling) *pb.ServiceScaling {
	if in == nil {
		return nil
	}
	return &pb.ServiceScaling{
		MinInstanceCount:    direct.ValueOf(in.MinInstanceCount),
		ScalingMode:         direct.Enum_ToProto[pb.ServiceScaling_ScalingMode](mapCtx, in.ScalingMode),
		MaxInstanceCount:    direct.ValueOf(in.MaxInstanceCount),
		ManualInstanceCount: in.ManualInstanceCount,
	}
}

// Written here.
func RevisionScaling_FromProto(mapCtx *direct.MapContext, in *pb.RevisionScaling) *krmrunv1alpha1.RevisionScaling {
	if in == nil {
		return nil
	}
	return &krmrunv1alpha1.RevisionScaling{
		MinInstanceCount: direct.LazyPtr(in.GetMinInstanceCount()),
		MaxInstanceCount: direct.LazyPtr(in.GetMaxInstanceCount()),
	}
}

// Written here.
func RevisionScaling_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.RevisionScaling) *pb.RevisionScaling {
	if in == nil {
		return nil
	}
	return &pb.RevisionScaling{
		MinInstanceCount: direct.ValueOf(in.MinInstanceCount),
		MaxInstanceCount: direct.ValueOf(in.MaxInstanceCount),
	}
}

func RunServiceObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.Service) *krmrunv1alpha1.RunServiceObservedState {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.RunServiceObservedState{}
	// MISSING: Name
	out.Uid = direct.LazyPtr(in.GetUid())
	out.Generation = direct.LazyPtr(in.GetGeneration())
	// MISSING: Labels
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	out.DeleteTime = direct.StringTimestamp_FromProto(mapCtx, in.GetDeleteTime())
	out.ExpireTime = direct.StringTimestamp_FromProto(mapCtx, in.GetExpireTime())
	out.Creator = direct.LazyPtr(in.GetCreator())
	out.LastModifier = direct.LazyPtr(in.GetLastModifier())
	out.Template = RevisionTemplateObservedState_v1alpha1_FromProto(mapCtx, in.GetTemplate())
	out.ObservedGeneration = direct.LazyPtr(in.GetObservedGeneration())
	out.TerminalCondition = RunCondition_v1alpha1_FromProto(mapCtx, in.GetTerminalCondition())
	out.ObservedConditions = direct.Slice_FromProto(mapCtx, in.Conditions, RunCondition_v1alpha1_FromProto)
	out.LatestReadyRevision = direct.LazyPtr(in.GetLatestReadyRevision())
	out.LatestCreatedRevision = direct.LazyPtr(in.GetLatestCreatedRevision())
	out.URI = direct.LazyPtr(in.GetUri()) // Written here.
	out.URLs = in.Urls                    // Written here.
	out.CustomAudiences = in.CustomAudiences
	out.SatisfiesPzs = direct.LazyPtr(in.GetSatisfiesPzs())
	out.Reconciling = direct.LazyPtr(in.GetReconciling())
	out.Etag = direct.LazyPtr(in.GetEtag())
	return out
}

func RunServiceObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.RunServiceObservedState) *pb.Service {
	if in == nil {
		return nil
	}
	out := &pb.Service{}
	// MISSING: Name
	out.Uid = direct.ValueOf(in.Uid)
	out.Generation = direct.ValueOf(in.Generation)
	out.CreateTime = direct.StringTimestamp_ToProto(mapCtx, in.CreateTime)
	out.UpdateTime = direct.StringTimestamp_ToProto(mapCtx, in.UpdateTime)
	out.DeleteTime = direct.StringTimestamp_ToProto(mapCtx, in.DeleteTime)
	out.ExpireTime = direct.StringTimestamp_ToProto(mapCtx, in.ExpireTime)
	out.Creator = direct.ValueOf(in.Creator)
	out.LastModifier = direct.ValueOf(in.LastModifier)
	out.Template = RevisionTemplateObservedState_v1alpha1_ToProto(mapCtx, in.Template)
	out.ObservedGeneration = direct.ValueOf(in.ObservedGeneration)
	out.TerminalCondition = RunCondition_v1alpha1_ToProto(mapCtx, in.TerminalCondition)
	out.Conditions = direct.Slice_ToProto(mapCtx, in.ObservedConditions, RunCondition_v1alpha1_ToProto)
	out.LatestReadyRevision = direct.ValueOf(in.LatestReadyRevision)
	out.LatestCreatedRevision = direct.ValueOf(in.LatestCreatedRevision)
	out.Uri = direct.ValueOf(in.URI) // Written here.
	out.Urls = in.URLs               // Written here.
	out.CustomAudiences = in.CustomAudiences
	out.SatisfiesPzs = direct.ValueOf(in.SatisfiesPzs)
	out.Reconciling = direct.ValueOf(in.Reconciling)
	out.Etag = direct.ValueOf(in.Etag)
	return out
}

func ServiceMesh_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.ServiceMesh) *pb.ServiceMesh {
	if in == nil {
		return nil
	}
	out := &pb.ServiceMesh{}
	if in.MeshRef != nil {
		out.Mesh = in.MeshRef.External
	}
	return out
}

func ServiceMesh_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.ServiceMesh) *krmrunv1alpha1.ServiceMesh {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.ServiceMesh{}
	if in.GetMesh() != "" {
		out.MeshRef = &networkservicesv1alpha1.NetworkServicesMeshRef{External: in.GetMesh()}
	}
	return out
}

func RunCondition_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.Condition) *krmrunv1alpha1.RunCondition {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.RunCondition{}
	out.Type = direct.LazyPtr(in.GetType())
	out.State = direct.Enum_FromProto(mapCtx, in.GetState())
	out.Message = direct.LazyPtr(in.GetMessage())
	out.LastTransitionTime = direct.StringTimestamp_FromProto(mapCtx, in.GetLastTransitionTime())
	out.Severity = direct.Enum_FromProto(mapCtx, in.GetSeverity())
	switch x := in.Reasons.(type) {
	case *pb.Condition_Reason:
		val := x.Reason.Descriptor().Values().ByNumber(protoreflect.EnumNumber(x.Reason))
		if val != nil {
			s := string(val.Name())
			out.Reason = &s
		}
	case *pb.Condition_RevisionReason_:
		val := x.RevisionReason.Descriptor().Values().ByNumber(protoreflect.EnumNumber(x.RevisionReason))
		if val != nil {
			s := string(val.Name())
			out.RevisionReason = &s
		}
	case *pb.Condition_ExecutionReason_:
		val := x.ExecutionReason.Descriptor().Values().ByNumber(protoreflect.EnumNumber(x.ExecutionReason))
		if val != nil {
			s := string(val.Name())
			out.ExecutionReason = &s
		}
	}
	return out
}

func RunCondition_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.RunCondition) *pb.Condition {
	if in == nil {
		return nil
	}
	out := &pb.Condition{}
	out.Type = direct.ValueOf(in.Type)
	out.State = direct.Enum_ToProto[pb.Condition_State](mapCtx, in.State)
	out.Message = direct.ValueOf(in.Message)
	out.LastTransitionTime = direct.StringTimestamp_ToProto(mapCtx, in.LastTransitionTime)
	out.Severity = direct.Enum_ToProto[pb.Condition_Severity](mapCtx, in.Severity)
	if oneof := Condition_Reason_ToProto(mapCtx, in.Reason); oneof != nil {
		out.Reasons = oneof
	}
	if oneof := Condition_RevisionReason_ToProto(mapCtx, in.RevisionReason); oneof != nil {
		out.Reasons = oneof
	}
	if oneof := Condition_ExecutionReason_ToProto(mapCtx, in.ExecutionReason); oneof != nil {
		out.Reasons = oneof
	}
	return out
}
