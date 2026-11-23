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
// pkg/controller/direct/run/runworkerpool_mapper.go and, for the spec's FromProto
// and the template, instance split and scaling functions,
// pkg/controller/direct/run/mapper.generated.go (UPSTREAM.md), for our
// RunWorkerPool (run.gcp.hfl.dev/v1beta1). The functions shared with
// RunService (Binary Authorization, service mesh, conditions, containers) are
// in runservice_mapper.go and mapper.generated.go already. Written here: the
// template's labels, which upstream marks MISSING.

package run

import (
	pb "cloud.google.com/go/run/apiv2/runpb"
	krmkmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	apipb "google.golang.org/genproto/googleapis/api"

	krmrunv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1beta1"
)

func RunWorkerPoolSpec_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1beta1.RunWorkerPoolSpec) *pb.WorkerPool {
	if in == nil {
		return nil
	}
	out := &pb.WorkerPool{}
	// MISSING: Name (handled by controller)
	out.Description = direct.ValueOf(in.Description)
	out.Labels = in.Labels
	out.Annotations = direct.MapStringString_ToProto(mapCtx, in.Annotations)
	out.Client = direct.ValueOf(in.Client)
	out.ClientVersion = direct.ValueOf(in.ClientVersion)
	out.LaunchStage = direct.Enum_ToProto[apipb.LaunchStage](mapCtx, in.LaunchStage)
	out.BinaryAuthorization = BinaryAuthorization_v1alpha1_ToProto(mapCtx, in.BinaryAuthorization)
	out.Template = WorkerPoolRevisionTemplate_v1alpha1_ToProto(mapCtx, in.Template)
	out.InstanceSplits = direct.Slice_ToProto(mapCtx, in.InstanceSplits, InstanceSplit_v1alpha1_ToProto)
	out.Scaling = WorkerPoolScaling_v1alpha1_ToProto(mapCtx, in.Scaling)
	out.CustomAudiences = in.CustomAudiences
	return out
}

func RunWorkerPoolSpec_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.WorkerPool) *krmrunv1beta1.RunWorkerPoolSpec {
	if in == nil {
		return nil
	}
	out := &krmrunv1beta1.RunWorkerPoolSpec{}
	// MISSING: Name
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Labels = in.Labels
	out.Annotations = in.Annotations
	out.Client = direct.LazyPtr(in.GetClient())
	out.ClientVersion = direct.LazyPtr(in.GetClientVersion())
	out.LaunchStage = direct.Enum_FromProto(mapCtx, in.GetLaunchStage())
	out.BinaryAuthorization = BinaryAuthorization_v1alpha1_FromProto(mapCtx, in.GetBinaryAuthorization())
	out.Template = WorkerPoolRevisionTemplate_v1alpha1_FromProto(mapCtx, in.GetTemplate())
	out.InstanceSplits = direct.Slice_FromProto(mapCtx, in.InstanceSplits, InstanceSplit_v1alpha1_FromProto)
	out.Scaling = WorkerPoolScaling_v1alpha1_FromProto(mapCtx, in.GetScaling())
	// MISSING: Conditions
	out.CustomAudiences = in.CustomAudiences
	return out
}

func RunWorkerPoolObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.WorkerPool) *krmrunv1beta1.RunWorkerPoolObservedState {
	if in == nil {
		return nil
	}
	out := &krmrunv1beta1.RunWorkerPoolObservedState{}
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
	out.Template = WorkerPoolRevisionTemplateObservedState_v1alpha1_FromProto(mapCtx, in.GetTemplate())
	out.ObservedGeneration = direct.LazyPtr(in.GetObservedGeneration())
	out.TerminalCondition = RunCondition_v1alpha1_FromProto(mapCtx, in.GetTerminalCondition())
	out.ObservedConditions = direct.Slice_FromProto(mapCtx, in.Conditions, RunCondition_v1alpha1_FromProto)
	out.LatestReadyRevision = direct.LazyPtr(in.GetLatestReadyRevision())
	out.LatestCreatedRevision = direct.LazyPtr(in.GetLatestCreatedRevision())
	out.InstanceSplitStatuses = direct.Slice_FromProto(mapCtx, in.InstanceSplitStatuses, InstanceSplitStatus_v1alpha1_FromProto)
	out.CustomAudiences = in.CustomAudiences
	out.SatisfiesPzs = direct.LazyPtr(in.GetSatisfiesPzs())
	out.Reconciling = direct.LazyPtr(in.GetReconciling())
	out.Etag = direct.LazyPtr(in.GetEtag())
	return out
}

func WorkerPoolRevisionTemplate_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1beta1.WorkerPoolRevisionTemplate) *pb.WorkerPoolRevisionTemplate {
	if in == nil {
		return nil
	}
	out := &pb.WorkerPoolRevisionTemplate{}
	out.Revision = direct.ValueOf(in.Revision)
	out.Labels = in.Labels // Written here.
	out.Annotations = in.Annotations
	out.VpcAccess = VPCAccess_v1alpha1_ToProto(mapCtx, in.VPCAccess)
	if in.ServiceAccountRef != nil {
		out.ServiceAccount = in.ServiceAccountRef.External
	}
	out.Containers = direct.Slice_ToProto(mapCtx, in.Containers, Container_v1alpha1_ToProto)
	out.Volumes = direct.Slice_ToProto(mapCtx, in.Volumes, Volume_v1alpha1_ToProto)
	if in.EncryptionKeyRef != nil {
		out.EncryptionKey = in.EncryptionKeyRef.External
	}
	out.ServiceMesh = ServiceMesh_v1alpha1_ToProto(mapCtx, in.ServiceMesh)
	out.EncryptionKeyRevocationAction = direct.Enum_ToProto[pb.EncryptionKeyRevocationAction](mapCtx, in.EncryptionKeyRevocationAction)
	out.EncryptionKeyShutdownDuration = direct.StringDuration_ToProto(mapCtx, in.EncryptionKeyShutdownDuration)
	out.NodeSelector = NodeSelector_v1alpha1_ToProto(mapCtx, in.NodeSelector)
	return out
}

func WorkerPoolRevisionTemplate_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.WorkerPoolRevisionTemplate) *krmrunv1beta1.WorkerPoolRevisionTemplate {
	if in == nil {
		return nil
	}
	out := &krmrunv1beta1.WorkerPoolRevisionTemplate{}
	out.Revision = direct.LazyPtr(in.GetRevision())
	out.Labels = in.Labels // Written here.
	out.Annotations = in.Annotations
	out.VPCAccess = VPCAccess_v1alpha1_FromProto(mapCtx, in.GetVpcAccess())
	if in.GetServiceAccount() != "" {
		out.ServiceAccountRef = &refsv1beta1.IAMServiceAccountRef{External: in.GetServiceAccount()}
	}
	out.Containers = direct.Slice_FromProto(mapCtx, in.Containers, Container_v1alpha1_FromProto)
	out.Volumes = direct.Slice_FromProto(mapCtx, in.Volumes, Volume_v1alpha1_FromProto)
	if in.GetEncryptionKey() != "" {
		out.EncryptionKeyRef = &krmkmsv1beta1.KMSCryptoKeyRef{External: in.GetEncryptionKey()}
	}
	out.ServiceMesh = ServiceMesh_v1alpha1_FromProto(mapCtx, in.GetServiceMesh())
	out.EncryptionKeyRevocationAction = direct.Enum_FromProto(mapCtx, in.GetEncryptionKeyRevocationAction())
	out.EncryptionKeyShutdownDuration = direct.StringDuration_FromProto(mapCtx, in.GetEncryptionKeyShutdownDuration())
	out.NodeSelector = NodeSelector_v1alpha1_FromProto(mapCtx, in.GetNodeSelector())
	return out
}

func WorkerPoolRevisionTemplateObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.WorkerPoolRevisionTemplate) *krmrunv1beta1.RevisionTemplateObservedState {
	if in == nil {
		return nil
	}
	out := &krmrunv1beta1.RevisionTemplateObservedState{}
	out.Revision = direct.LazyPtr(in.GetRevision())
	// MISSING: Labels
	// MISSING: Annotations
	// MISSING: VPCAccess
	// MISSING: ServiceAccount
	out.Containers = direct.Slice_FromProto(mapCtx, in.Containers, ContainerObservedState_v1alpha1_FromProto)
	// MISSING: Volumes
	// MISSING: EncryptionKey
	// MISSING: ServiceMesh
	// MISSING: EncryptionKeyRevocationAction
	// MISSING: EncryptionKeyShutdownDuration
	// MISSING: NodeSelector
	return out
}

func InstanceSplit_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1beta1.InstanceSplit) *pb.InstanceSplit {
	if in == nil {
		return nil
	}
	out := &pb.InstanceSplit{}
	out.Type = direct.Enum_ToProto[pb.InstanceSplitAllocationType](mapCtx, in.Type)
	out.Revision = direct.ValueOf(in.Revision)
	out.Percent = direct.ValueOf(in.Percent)
	return out
}

func InstanceSplit_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.InstanceSplit) *krmrunv1beta1.InstanceSplit {
	if in == nil {
		return nil
	}
	out := &krmrunv1beta1.InstanceSplit{}
	out.Type = direct.Enum_FromProto(mapCtx, in.GetType())
	out.Revision = direct.LazyPtr(in.GetRevision())
	out.Percent = direct.LazyPtr(in.GetPercent())
	return out
}

func InstanceSplitStatus_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.InstanceSplitStatus) *krmrunv1beta1.InstanceSplitStatus {
	if in == nil {
		return nil
	}
	out := &krmrunv1beta1.InstanceSplitStatus{}
	out.Type = direct.Enum_FromProto(mapCtx, in.GetType())
	out.Revision = direct.LazyPtr(in.GetRevision())
	out.Percent = direct.LazyPtr(in.GetPercent())
	return out
}

func WorkerPoolScaling_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1beta1.WorkerPoolScaling) *pb.WorkerPoolScaling {
	if in == nil {
		return nil
	}
	out := &pb.WorkerPoolScaling{}
	out.ManualInstanceCount = in.ManualInstanceCount
	return out
}

func WorkerPoolScaling_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.WorkerPoolScaling) *krmrunv1beta1.WorkerPoolScaling {
	if in == nil {
		return nil
	}
	out := &krmrunv1beta1.WorkerPoolScaling{}
	out.ManualInstanceCount = in.ManualInstanceCount
	return out
}
