// Copyright 2025 Google LLC
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
// pkg/controller/direct/run/mapper.generated.go (UPSTREAM.md): the RunJob
// functions (RunJobSpec, ExecutionTemplate, TaskTemplate, BinaryAuthorization,
// ExecutionReference), for our RunJob (run.gcp.hfl.dev/v1alpha1), whose
// containers, volumes and VPC access are our RunService's types and so map
// with this package's functions for them. Written here: the fields upstream
// marks MISSING that our RunJob has (the execution template's labels, the
// task template's nodeSelector and gpuZonalRedundancyDisabled,
// binaryAuthorization.policy); maxRetries read back when it is 0 (a job
// that never retries), which upstream's LazyPtr drops; and the observed
// state, as our RunService's.

package run

import (
	pb "cloud.google.com/go/run/apiv2/runpb"
	krmkmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	apipb "google.golang.org/genproto/googleapis/api"

	krmrunv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
)

// RunJobSpec_ToProto maps the spec; the job's labels are the object's
// metadata.labels, which the controller sets.
func RunJobSpec_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.RunJobSpec) *pb.Job {
	if in == nil {
		return nil
	}
	out := &pb.Job{}
	// MISSING: Name
	// MISSING: Generation
	// MISSING: Labels (metadata.labels, set by the controller)
	out.Annotations = in.Annotations
	out.Client = direct.ValueOf(in.Client)
	out.ClientVersion = direct.ValueOf(in.ClientVersion)
	out.LaunchStage = direct.Enum_ToProto[apipb.LaunchStage](mapCtx, in.LaunchStage)
	out.BinaryAuthorization = JobBinaryAuthorization_ToProto(mapCtx, in.BinaryAuthorization)
	out.Template = ExecutionTemplate_ToProto(mapCtx, in.Template)
	// MISSING: ObservedGeneration
	// MISSING: Conditions
	// MISSING: SatisfiesPzs
	// MISSING: StartExecutionToken
	// MISSING: RunExecutionToken
	return out
}

func RunJobSpec_FromProto(mapCtx *direct.MapContext, in *pb.Job) *krmrunv1alpha1.RunJobSpec {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.RunJobSpec{}
	// MISSING: Name
	// MISSING: Generation
	// MISSING: Labels (metadata.labels)
	out.Annotations = in.Annotations
	out.Client = direct.LazyPtr(in.GetClient())
	out.ClientVersion = direct.LazyPtr(in.GetClientVersion())
	out.LaunchStage = direct.Enum_FromProto(mapCtx, in.GetLaunchStage())
	out.BinaryAuthorization = JobBinaryAuthorization_FromProto(mapCtx, in.GetBinaryAuthorization())
	out.Template = ExecutionTemplate_FromProto(mapCtx, in.GetTemplate())
	return out
}

func JobBinaryAuthorization_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.BinaryAuthorization) *pb.BinaryAuthorization {
	if in == nil {
		return nil
	}
	out := &pb.BinaryAuthorization{}
	if oneof := BinaryAuthorization_UseDefault_ToProto(mapCtx, in.UseDefault); oneof != nil {
		out.BinauthzMethod = oneof
	}
	// Written here: the policy (the other side of the same oneof).
	if in.Policy != nil {
		out.BinauthzMethod = &pb.BinaryAuthorization_Policy{Policy: *in.Policy}
	}
	out.BreakglassJustification = direct.ValueOf(in.BreakglassJustification)
	return out
}

func JobBinaryAuthorization_FromProto(mapCtx *direct.MapContext, in *pb.BinaryAuthorization) *krmrunv1alpha1.BinaryAuthorization {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.BinaryAuthorization{}
	switch x := in.BinauthzMethod.(type) {
	case *pb.BinaryAuthorization_UseDefault:
		val := x.UseDefault
		out.UseDefault = &val
	case *pb.BinaryAuthorization_Policy: // Written here.
		val := x.Policy
		out.Policy = &val
	}
	out.BreakglassJustification = direct.LazyPtr(in.GetBreakglassJustification())
	return out
}

func ExecutionTemplate_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.ExecutionTemplate) *pb.ExecutionTemplate {
	if in == nil {
		return nil
	}
	out := &pb.ExecutionTemplate{}
	out.Labels = in.Labels // Written here.
	out.Annotations = in.Annotations
	out.Parallelism = direct.ValueOf(in.Parallelism)
	out.TaskCount = direct.ValueOf(in.TaskCount)
	out.Template = TaskTemplate_ToProto(mapCtx, in.Template)
	return out
}

func ExecutionTemplate_FromProto(mapCtx *direct.MapContext, in *pb.ExecutionTemplate) *krmrunv1alpha1.ExecutionTemplate {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.ExecutionTemplate{}
	out.Labels = in.Labels // Written here.
	out.Annotations = in.Annotations
	out.Parallelism = direct.LazyPtr(in.GetParallelism())
	out.TaskCount = direct.LazyPtr(in.GetTaskCount())
	out.Template = TaskTemplate_FromProto(mapCtx, in.GetTemplate())
	return out
}

func TaskTemplate_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.TaskTemplate) *pb.TaskTemplate {
	if in == nil {
		return nil
	}
	out := &pb.TaskTemplate{}
	out.Containers = direct.Slice_ToProto(mapCtx, in.Containers, Container_v1alpha1_ToProto)
	out.Volumes = direct.Slice_ToProto(mapCtx, in.Volumes, Volume_v1alpha1_ToProto)
	if in.MaxRetries != nil {
		out.Retries = &pb.TaskTemplate_MaxRetries{MaxRetries: *in.MaxRetries}
	}
	out.Timeout = direct.StringDuration_ToProto(mapCtx, in.Timeout)
	if in.ServiceAccountRef != nil {
		out.ServiceAccount = in.ServiceAccountRef.External
	}
	out.ExecutionEnvironment = direct.Enum_ToProto[pb.ExecutionEnvironment](mapCtx, in.ExecutionEnvironment)
	if in.EncryptionKeyRef != nil {
		out.EncryptionKey = in.EncryptionKeyRef.External
	}
	out.VpcAccess = VPCAccess_v1alpha1_ToProto(mapCtx, in.VPCAccess)
	out.NodeSelector = NodeSelector_v1alpha1_ToProto(mapCtx, in.NodeSelector) // Written here.
	out.GpuZonalRedundancyDisabled = in.GpuZonalRedundancyDisabled            // Written here.
	return out
}

func TaskTemplate_FromProto(mapCtx *direct.MapContext, in *pb.TaskTemplate) *krmrunv1alpha1.TaskTemplate {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.TaskTemplate{}
	out.Containers = direct.Slice_FromProto(mapCtx, in.Containers, Container_v1alpha1_FromProto)
	out.Volumes = direct.Slice_FromProto(mapCtx, in.Volumes, Volume_v1alpha1_FromProto)
	// Written here: a maxRetries of 0 is kept (upstream's LazyPtr drops it).
	if r, ok := in.Retries.(*pb.TaskTemplate_MaxRetries); ok {
		out.MaxRetries = direct.PtrTo(r.MaxRetries)
	}
	out.Timeout = direct.StringDuration_FromProto(mapCtx, in.GetTimeout())
	if in.GetServiceAccount() != "" {
		out.ServiceAccountRef = &refsv1beta1.IAMServiceAccountRef{External: in.GetServiceAccount()}
	}
	out.ExecutionEnvironment = direct.Enum_FromProto(mapCtx, in.GetExecutionEnvironment())
	if in.GetEncryptionKey() != "" {
		out.EncryptionKeyRef = &krmkmsv1beta1.KMSCryptoKeyRef{External: in.GetEncryptionKey()}
	}
	out.VPCAccess = VPCAccess_v1alpha1_FromProto(mapCtx, in.GetVpcAccess())
	out.NodeSelector = NodeSelector_v1alpha1_FromProto(mapCtx, in.GetNodeSelector()) // Written here.
	out.GpuZonalRedundancyDisabled = in.GpuZonalRedundancyDisabled                   // Written here.
	return out
}

func ExecutionReference_FromProto(mapCtx *direct.MapContext, in *pb.ExecutionReference) *krmrunv1alpha1.ExecutionReference {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.ExecutionReference{}
	out.Name = direct.LazyPtr(in.GetName())
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.CompletionTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCompletionTime())
	out.DeleteTime = direct.StringTimestamp_FromProto(mapCtx, in.GetDeleteTime())
	out.CompletionStatus = direct.Enum_FromProto(mapCtx, in.GetCompletionStatus())
	return out
}

// RunJobObservedState_FromProto is written here, as our RunService's.
func RunJobObservedState_FromProto(mapCtx *direct.MapContext, in *pb.Job) *krmrunv1alpha1.RunJobObservedState {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.RunJobObservedState{}
	out.Uid = direct.LazyPtr(in.GetUid())
	out.Generation = direct.LazyPtr(in.GetGeneration())
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	out.DeleteTime = direct.StringTimestamp_FromProto(mapCtx, in.GetDeleteTime())
	out.ExpireTime = direct.StringTimestamp_FromProto(mapCtx, in.GetExpireTime())
	out.Creator = direct.LazyPtr(in.GetCreator())
	out.LastModifier = direct.LazyPtr(in.GetLastModifier())
	out.ObservedGeneration = direct.LazyPtr(in.GetObservedGeneration())
	out.TerminalCondition = RunCondition_v1alpha1_FromProto(mapCtx, in.GetTerminalCondition())
	out.ObservedConditions = direct.Slice_FromProto(mapCtx, in.Conditions, RunCondition_v1alpha1_FromProto)
	out.ExecutionCount = direct.LazyPtr(in.GetExecutionCount())
	out.LatestCreatedExecution = ExecutionReference_FromProto(mapCtx, in.GetLatestCreatedExecution())
	out.Reconciling = direct.LazyPtr(in.GetReconciling())
	out.Etag = direct.LazyPtr(in.GetEtag())
	return out
}
