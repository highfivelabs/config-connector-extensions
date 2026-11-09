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
// pkg/controller/direct/run/mapper.generated.go (UPSTREAM.md): only the v1alpha1
// (RunWorkerPool) functions, without the worker pool's instance splits and
// scaling; the KRM types are this project's (run.gcp.hfl.dev); the worker
// pool's template functions map a service's RevisionTemplate, with the
// fields marked "Written here" added from the Cloud Run v2 API. The spec
// mapping is in runservice_mapper.go. The three Condition reason helpers the
// copied functions use are copied too, at the end.

package run

import (
	pb "cloud.google.com/go/run/apiv2/runpb"
	krmcomputerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	krmcomputev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	krmkmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	krmsecretmanagerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/secretmanager/v1beta1"
	krmstoragev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	krmvpcaccessv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/vpcaccess/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	krmrunv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
)

func BinaryAuthorization_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.BinaryAuthorization) *pb.BinaryAuthorization {
	if in == nil {
		return nil
	}
	out := &pb.BinaryAuthorization{}
	if oneof := BinaryAuthorization_UseDefault_ToProto(mapCtx, in.UseDefault); oneof != nil {
		out.BinauthzMethod = oneof
	}
	// MISSING: Policy
	out.BreakglassJustification = direct.ValueOf(in.BreakglassJustification)
	return out
}
func BuildInfoObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.BuildInfo) *krmrunv1alpha1.BuildInfoObservedState {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.BuildInfoObservedState{}
	out.FunctionTarget = direct.LazyPtr(in.GetFunctionTarget())
	out.SourceLocation = direct.LazyPtr(in.GetSourceLocation())
	return out
}
func BuildInfoObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.BuildInfoObservedState) *pb.BuildInfo {
	if in == nil {
		return nil
	}
	out := &pb.BuildInfo{}
	out.FunctionTarget = direct.ValueOf(in.FunctionTarget)
	out.SourceLocation = direct.ValueOf(in.SourceLocation)
	return out
}
func CloudSQLInstance_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.CloudSqlInstance) *krmrunv1alpha1.CloudSQLInstance {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.CloudSQLInstance{}

	if v := in.GetInstances(); len(v) != 0 {
		for i := range v {
			out.InstanceRefs = append(out.InstanceRefs, &refsv1beta1.SQLInstanceRef{External: v[i]})
		}
	}

	return out
}
func CloudSQLInstance_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.CloudSQLInstance) *pb.CloudSqlInstance {
	if in == nil {
		return nil
	}
	out := &pb.CloudSqlInstance{}

	if v := in.InstanceRefs; len(v) != 0 {
		for i := range v {
			out.Instances = append(out.Instances, v[i].External)
		}
	}

	return out
}
func Container_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.Container) *krmrunv1alpha1.Container {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.Container{}
	out.Name = direct.LazyPtr(in.GetName())
	out.Image = direct.LazyPtr(in.GetImage())
	out.Command = in.Command
	out.Args = in.Args
	out.Env = direct.Slice_FromProto(mapCtx, in.Env, EnvVar_v1alpha1_FromProto)
	out.Resources = ResourceRequirements_v1alpha1_FromProto(mapCtx, in.GetResources())
	out.Ports = direct.Slice_FromProto(mapCtx, in.Ports, ContainerPort_v1alpha1_FromProto)
	out.VolumeMounts = direct.Slice_FromProto(mapCtx, in.VolumeMounts, VolumeMount_v1alpha1_FromProto)
	out.WorkingDir = direct.LazyPtr(in.GetWorkingDir())
	out.LivenessProbe = Probe_v1alpha1_FromProto(mapCtx, in.GetLivenessProbe())
	out.StartupProbe = Probe_v1alpha1_FromProto(mapCtx, in.GetStartupProbe())
	out.DependsOn = in.DependsOn
	out.BaseImageURI = direct.LazyPtr(in.GetBaseImageUri())
	// MISSING: BuildInfo
	return out
}
func Container_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.Container) *pb.Container {
	if in == nil {
		return nil
	}
	out := &pb.Container{}
	out.Name = direct.ValueOf(in.Name)
	out.Image = direct.ValueOf(in.Image)
	out.Command = in.Command
	out.Args = in.Args
	out.Env = direct.Slice_ToProto(mapCtx, in.Env, EnvVar_v1alpha1_ToProto)
	out.Resources = ResourceRequirements_v1alpha1_ToProto(mapCtx, in.Resources)
	out.Ports = direct.Slice_ToProto(mapCtx, in.Ports, ContainerPort_v1alpha1_ToProto)
	out.VolumeMounts = direct.Slice_ToProto(mapCtx, in.VolumeMounts, VolumeMount_v1alpha1_ToProto)
	out.WorkingDir = direct.ValueOf(in.WorkingDir)
	out.LivenessProbe = Probe_v1alpha1_ToProto(mapCtx, in.LivenessProbe)
	out.StartupProbe = Probe_v1alpha1_ToProto(mapCtx, in.StartupProbe)
	out.DependsOn = in.DependsOn
	out.BaseImageUri = direct.ValueOf(in.BaseImageURI)
	// MISSING: BuildInfo
	return out
}
func ContainerObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.Container) *krmrunv1alpha1.ContainerObservedState {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.ContainerObservedState{}
	// MISSING: Name
	// MISSING: Image
	// MISSING: Command
	// MISSING: Args
	// MISSING: Env
	// MISSING: Resources
	// MISSING: Ports
	// MISSING: VolumeMounts
	// MISSING: WorkingDir
	// MISSING: LivenessProbe
	// MISSING: StartupProbe
	// MISSING: DependsOn
	// MISSING: BaseImageURI
	out.BuildInfo = BuildInfoObservedState_v1alpha1_FromProto(mapCtx, in.GetBuildInfo())
	return out
}
func ContainerObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.ContainerObservedState) *pb.Container {
	if in == nil {
		return nil
	}
	out := &pb.Container{}
	// MISSING: Name
	// MISSING: Image
	// MISSING: Command
	// MISSING: Args
	// MISSING: Env
	// MISSING: Resources
	// MISSING: Ports
	// MISSING: VolumeMounts
	// MISSING: WorkingDir
	// MISSING: LivenessProbe
	// MISSING: StartupProbe
	// MISSING: DependsOn
	// MISSING: BaseImageURI
	out.BuildInfo = BuildInfoObservedState_v1alpha1_ToProto(mapCtx, in.BuildInfo)
	return out
}
func ContainerPort_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.ContainerPort) *krmrunv1alpha1.ContainerPort {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.ContainerPort{}
	out.Name = direct.LazyPtr(in.GetName())
	out.ContainerPort = direct.LazyPtr(in.GetContainerPort())
	return out
}
func ContainerPort_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.ContainerPort) *pb.ContainerPort {
	if in == nil {
		return nil
	}
	out := &pb.ContainerPort{}
	out.Name = direct.ValueOf(in.Name)
	out.ContainerPort = direct.ValueOf(in.ContainerPort)
	return out
}
func EmptyDirVolumeSource_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.EmptyDirVolumeSource) *krmrunv1alpha1.EmptyDirVolumeSource {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.EmptyDirVolumeSource{}
	out.Medium = direct.Enum_FromProto(mapCtx, in.GetMedium())
	out.SizeLimit = direct.LazyPtr(in.GetSizeLimit())
	return out
}
func EmptyDirVolumeSource_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.EmptyDirVolumeSource) *pb.EmptyDirVolumeSource {
	if in == nil {
		return nil
	}
	out := &pb.EmptyDirVolumeSource{}
	out.Medium = direct.Enum_ToProto[pb.EmptyDirVolumeSource_Medium](mapCtx, in.Medium)
	out.SizeLimit = direct.ValueOf(in.SizeLimit)
	return out
}
func EnvVar_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.EnvVar) *krmrunv1alpha1.EnvVar {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.EnvVar{}
	out.Name = direct.LazyPtr(in.GetName())
	out.Value = direct.LazyPtr(in.GetValue())
	out.ValueSource = EnvVarSource_v1alpha1_FromProto(mapCtx, in.GetValueSource())
	return out
}
func EnvVar_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.EnvVar) *pb.EnvVar {
	if in == nil {
		return nil
	}
	out := &pb.EnvVar{}
	out.Name = direct.ValueOf(in.Name)
	if oneof := EnvVar_Value_ToProto(mapCtx, in.Value); oneof != nil {
		out.Values = oneof
	}
	if oneof := EnvVarSource_v1alpha1_ToProto(mapCtx, in.ValueSource); oneof != nil {
		out.Values = &pb.EnvVar_ValueSource{ValueSource: oneof}
	}
	return out
}
func EnvVarSource_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.EnvVarSource) *krmrunv1alpha1.EnvVarSource {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.EnvVarSource{}
	out.SecretKeyRef = SecretKeySelector_v1alpha1_FromProto(mapCtx, in.GetSecretKeyRef())
	return out
}
func EnvVarSource_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.EnvVarSource) *pb.EnvVarSource {
	if in == nil {
		return nil
	}
	out := &pb.EnvVarSource{}
	out.SecretKeyRef = SecretKeySelector_v1alpha1_ToProto(mapCtx, in.SecretKeyRef)
	return out
}
func GCSVolumeSource_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.GCSVolumeSource) *krmrunv1alpha1.GCSVolumeSource {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.GCSVolumeSource{}
	if in.GetBucket() != "" {
		out.BucketRef = &krmstoragev1beta1.StorageBucketRef{External: in.GetBucket()}
	}
	out.ReadOnly = direct.LazyPtr(in.GetReadOnly())
	out.MountOptions = in.MountOptions
	return out
}
func GCSVolumeSource_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.GCSVolumeSource) *pb.GCSVolumeSource {
	if in == nil {
		return nil
	}
	out := &pb.GCSVolumeSource{}
	if in.BucketRef != nil {
		out.Bucket = in.BucketRef.External
	}
	out.ReadOnly = direct.ValueOf(in.ReadOnly)
	out.MountOptions = in.MountOptions
	return out
}
func GrpcAction_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.GRPCAction) *krmrunv1alpha1.GrpcAction {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.GrpcAction{}
	out.Port = direct.LazyPtr(in.GetPort())
	out.Service = direct.LazyPtr(in.GetService())
	return out
}
func GrpcAction_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.GrpcAction) *pb.GRPCAction {
	if in == nil {
		return nil
	}
	out := &pb.GRPCAction{}
	out.Port = direct.ValueOf(in.Port)
	out.Service = direct.ValueOf(in.Service)
	return out
}
func HTTPGetAction_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.HTTPGetAction) *krmrunv1alpha1.HTTPGetAction {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.HTTPGetAction{}
	out.Path = direct.LazyPtr(in.GetPath())
	out.HTTPHeaders = direct.Slice_FromProto(mapCtx, in.HttpHeaders, HTTPHeader_v1alpha1_FromProto)
	out.Port = direct.LazyPtr(in.GetPort())
	return out
}
func HTTPGetAction_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.HTTPGetAction) *pb.HTTPGetAction {
	if in == nil {
		return nil
	}
	out := &pb.HTTPGetAction{}
	out.Path = direct.ValueOf(in.Path)
	out.HttpHeaders = direct.Slice_ToProto(mapCtx, in.HTTPHeaders, HTTPHeader_v1alpha1_ToProto)
	out.Port = direct.ValueOf(in.Port)
	return out
}
func HTTPHeader_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.HTTPHeader) *krmrunv1alpha1.HTTPHeader {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.HTTPHeader{}
	out.Name = direct.LazyPtr(in.GetName())
	out.Value = direct.LazyPtr(in.GetValue())
	return out
}
func HTTPHeader_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.HTTPHeader) *pb.HTTPHeader {
	if in == nil {
		return nil
	}
	out := &pb.HTTPHeader{}
	out.Name = direct.ValueOf(in.Name)
	out.Value = direct.ValueOf(in.Value)
	return out
}
func NfsVolumeSource_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.NFSVolumeSource) *krmrunv1alpha1.NfsVolumeSource {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.NfsVolumeSource{}
	out.Server = direct.LazyPtr(in.GetServer())
	out.Path = direct.LazyPtr(in.GetPath())
	out.ReadOnly = direct.LazyPtr(in.GetReadOnly())
	return out
}
func NfsVolumeSource_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.NfsVolumeSource) *pb.NFSVolumeSource {
	if in == nil {
		return nil
	}
	out := &pb.NFSVolumeSource{}
	out.Server = direct.ValueOf(in.Server)
	out.Path = direct.ValueOf(in.Path)
	out.ReadOnly = direct.ValueOf(in.ReadOnly)
	return out
}
func NodeSelector_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.NodeSelector) *krmrunv1alpha1.NodeSelector {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.NodeSelector{}
	out.Accelerator = direct.LazyPtr(in.GetAccelerator())
	return out
}
func NodeSelector_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.NodeSelector) *pb.NodeSelector {
	if in == nil {
		return nil
	}
	out := &pb.NodeSelector{}
	out.Accelerator = direct.ValueOf(in.Accelerator)
	return out
}
func Probe_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.Probe) *krmrunv1alpha1.Probe {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.Probe{}
	out.InitialDelaySeconds = direct.LazyPtr(in.GetInitialDelaySeconds())
	out.TimeoutSeconds = direct.LazyPtr(in.GetTimeoutSeconds())
	out.PeriodSeconds = direct.LazyPtr(in.GetPeriodSeconds())
	out.FailureThreshold = direct.LazyPtr(in.GetFailureThreshold())
	out.HTTPGet = HTTPGetAction_v1alpha1_FromProto(mapCtx, in.GetHttpGet())
	out.TCPSocket = TCPSocketAction_v1alpha1_FromProto(mapCtx, in.GetTcpSocket())
	out.Grpc = GrpcAction_v1alpha1_FromProto(mapCtx, in.GetGrpc())
	return out
}
func Probe_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.Probe) *pb.Probe {
	if in == nil {
		return nil
	}
	out := &pb.Probe{}
	out.InitialDelaySeconds = direct.ValueOf(in.InitialDelaySeconds)
	out.TimeoutSeconds = direct.ValueOf(in.TimeoutSeconds)
	out.PeriodSeconds = direct.ValueOf(in.PeriodSeconds)
	out.FailureThreshold = direct.ValueOf(in.FailureThreshold)
	if oneof := HTTPGetAction_v1alpha1_ToProto(mapCtx, in.HTTPGet); oneof != nil {
		out.ProbeType = &pb.Probe_HttpGet{HttpGet: oneof}
	}
	if oneof := TCPSocketAction_v1alpha1_ToProto(mapCtx, in.TCPSocket); oneof != nil {
		out.ProbeType = &pb.Probe_TcpSocket{TcpSocket: oneof}
	}
	if oneof := GrpcAction_v1alpha1_ToProto(mapCtx, in.Grpc); oneof != nil {
		out.ProbeType = &pb.Probe_Grpc{Grpc: oneof}
	}
	return out
}
func ResourceRequirements_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.ResourceRequirements) *krmrunv1alpha1.ResourceRequirements {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.ResourceRequirements{}
	out.Limits = in.Limits
	out.CPUIdle = direct.LazyPtr(in.GetCpuIdle())
	out.StartupCPUBoost = direct.LazyPtr(in.GetStartupCpuBoost())
	return out
}
func ResourceRequirements_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.ResourceRequirements) *pb.ResourceRequirements {
	if in == nil {
		return nil
	}
	out := &pb.ResourceRequirements{}
	out.Limits = in.Limits
	out.CpuIdle = direct.ValueOf(in.CPUIdle)
	out.StartupCpuBoost = direct.ValueOf(in.StartupCPUBoost)
	return out
}
func SecretKeySelector_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.SecretKeySelector) *krmrunv1alpha1.SecretKeySelector {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.SecretKeySelector{}
	if in.GetSecret() != "" {
		out.SecretRef = &krmsecretmanagerv1beta1.SecretRef{External: in.GetSecret()}
	}
	if in.GetVersion() != "" {
		out.VersionRef = &krmsecretmanagerv1beta1.SecretVersionRef{External: in.GetVersion()}
	}
	return out
}
func SecretKeySelector_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.SecretKeySelector) *pb.SecretKeySelector {
	if in == nil {
		return nil
	}
	out := &pb.SecretKeySelector{}
	if in.SecretRef != nil {
		out.Secret = in.SecretRef.External
	}
	if in.VersionRef != nil {
		out.Version = in.VersionRef.External
	}
	return out
}
func SecretVolumeSource_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.SecretVolumeSource) *krmrunv1alpha1.SecretVolumeSource {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.SecretVolumeSource{}
	if in.GetSecret() != "" {
		out.SecretRef = &krmsecretmanagerv1beta1.SecretRef{External: in.GetSecret()}
	}
	out.Items = direct.Slice_FromProto(mapCtx, in.Items, VersionToPath_v1alpha1_FromProto)
	out.DefaultMode = direct.LazyPtr(in.GetDefaultMode())
	return out
}
func SecretVolumeSource_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.SecretVolumeSource) *pb.SecretVolumeSource {
	if in == nil {
		return nil
	}
	out := &pb.SecretVolumeSource{}
	if in.SecretRef != nil {
		out.Secret = in.SecretRef.External
	}
	out.Items = direct.Slice_ToProto(mapCtx, in.Items, VersionToPath_v1alpha1_ToProto)
	out.DefaultMode = direct.ValueOf(in.DefaultMode)
	return out
}
func TCPSocketAction_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.TCPSocketAction) *krmrunv1alpha1.TCPSocketAction {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.TCPSocketAction{}
	out.Port = direct.LazyPtr(in.GetPort())
	return out
}
func TCPSocketAction_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.TCPSocketAction) *pb.TCPSocketAction {
	if in == nil {
		return nil
	}
	out := &pb.TCPSocketAction{}
	out.Port = direct.ValueOf(in.Port)
	return out
}
func VPCAccess_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.VpcAccess) *krmrunv1alpha1.VPCAccess {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.VPCAccess{}
	if in.GetConnector() != "" {
		out.ConnectorRef = &krmvpcaccessv1beta1.VPCAccessConnectorRef{External: in.GetConnector()}
	}
	out.Egress = direct.Enum_FromProto(mapCtx, in.GetEgress())
	out.NetworkInterfaces = direct.Slice_FromProto(mapCtx, in.NetworkInterfaces, VPCAccess_NetworkInterface_v1alpha1_FromProto)
	return out
}
func VPCAccess_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.VPCAccess) *pb.VpcAccess {
	if in == nil {
		return nil
	}
	out := &pb.VpcAccess{}
	if in.ConnectorRef != nil {
		out.Connector = in.ConnectorRef.External
	}
	out.Egress = direct.Enum_ToProto[pb.VpcAccess_VpcEgress](mapCtx, in.Egress)
	out.NetworkInterfaces = direct.Slice_ToProto(mapCtx, in.NetworkInterfaces, VPCAccess_NetworkInterface_v1alpha1_ToProto)
	return out
}
func VPCAccess_NetworkInterface_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.VpcAccess_NetworkInterface) *krmrunv1alpha1.VPCAccess_NetworkInterface {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.VPCAccess_NetworkInterface{}
	if in.GetNetwork() != "" {
		out.NetworkRef = &krmcomputerefs.ComputeNetworkRef{External: in.GetNetwork()}
	}
	if in.GetSubnetwork() != "" {
		out.SubnetworkRef = &krmcomputev1beta1.ComputeSubnetworkRef{External: in.GetSubnetwork()}
	}
	out.Tags = in.Tags
	return out
}
func VPCAccess_NetworkInterface_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.VPCAccess_NetworkInterface) *pb.VpcAccess_NetworkInterface {
	if in == nil {
		return nil
	}
	out := &pb.VpcAccess_NetworkInterface{}
	if in.NetworkRef != nil {
		out.Network = in.NetworkRef.External
	}
	if in.SubnetworkRef != nil {
		out.Subnetwork = in.SubnetworkRef.External
	}
	out.Tags = in.Tags
	return out
}
func VersionToPath_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.VersionToPath) *krmrunv1alpha1.VersionToPath {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.VersionToPath{}
	out.Path = direct.LazyPtr(in.GetPath())
	if in.GetVersion() != "" {
		out.VersionRef = &krmsecretmanagerv1beta1.SecretVersionRef{External: in.GetVersion()}
	}
	out.Mode = direct.LazyPtr(in.GetMode())
	return out
}
func VersionToPath_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.VersionToPath) *pb.VersionToPath {
	if in == nil {
		return nil
	}
	out := &pb.VersionToPath{}
	out.Path = direct.ValueOf(in.Path)
	if in.VersionRef != nil {
		out.Version = in.VersionRef.External
	}
	out.Mode = direct.ValueOf(in.Mode)
	return out
}
func Volume_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.Volume) *krmrunv1alpha1.Volume {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.Volume{}
	out.Name = direct.LazyPtr(in.GetName())
	out.Secret = SecretVolumeSource_v1alpha1_FromProto(mapCtx, in.GetSecret())
	out.CloudSQLInstance = CloudSQLInstance_v1alpha1_FromProto(mapCtx, in.GetCloudSqlInstance())
	out.EmptyDir = EmptyDirVolumeSource_v1alpha1_FromProto(mapCtx, in.GetEmptyDir())
	out.Nfs = NfsVolumeSource_v1alpha1_FromProto(mapCtx, in.GetNfs())
	out.GCS = GCSVolumeSource_v1alpha1_FromProto(mapCtx, in.GetGcs())
	return out
}
func Volume_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.Volume) *pb.Volume {
	if in == nil {
		return nil
	}
	out := &pb.Volume{}
	out.Name = direct.ValueOf(in.Name)
	if oneof := SecretVolumeSource_v1alpha1_ToProto(mapCtx, in.Secret); oneof != nil {
		out.VolumeType = &pb.Volume_Secret{Secret: oneof}
	}
	if oneof := CloudSQLInstance_v1alpha1_ToProto(mapCtx, in.CloudSQLInstance); oneof != nil {
		out.VolumeType = &pb.Volume_CloudSqlInstance{CloudSqlInstance: oneof}
	}
	if oneof := EmptyDirVolumeSource_v1alpha1_ToProto(mapCtx, in.EmptyDir); oneof != nil {
		out.VolumeType = &pb.Volume_EmptyDir{EmptyDir: oneof}
	}
	if oneof := NfsVolumeSource_v1alpha1_ToProto(mapCtx, in.Nfs); oneof != nil {
		out.VolumeType = &pb.Volume_Nfs{Nfs: oneof}
	}
	if oneof := GCSVolumeSource_v1alpha1_ToProto(mapCtx, in.GCS); oneof != nil {
		out.VolumeType = &pb.Volume_Gcs{Gcs: oneof}
	}
	return out
}
func VolumeMount_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.VolumeMount) *krmrunv1alpha1.VolumeMount {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.VolumeMount{}
	out.Name = direct.LazyPtr(in.GetName())
	out.MountPath = direct.LazyPtr(in.GetMountPath())
	return out
}
func VolumeMount_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.VolumeMount) *pb.VolumeMount {
	if in == nil {
		return nil
	}
	out := &pb.VolumeMount{}
	out.Name = direct.ValueOf(in.Name)
	out.MountPath = direct.ValueOf(in.MountPath)
	return out
}
func RevisionTemplate_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.RevisionTemplate) *krmrunv1alpha1.RevisionTemplate {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.RevisionTemplate{}
	out.Revision = direct.LazyPtr(in.GetRevision())
	// MISSING: Labels
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
	// Written here: the RevisionTemplate fields a worker pool's template lacks.
	out.Scaling = RevisionScaling_FromProto(mapCtx, in.GetScaling())
	out.Timeout = direct.StringDuration_FromProto(mapCtx, in.GetTimeout())
	out.MaxInstanceRequestConcurrency = direct.LazyPtr(in.GetMaxInstanceRequestConcurrency())
	out.ExecutionEnvironment = direct.Enum_FromProto(mapCtx, in.GetExecutionEnvironment())
	out.SessionAffinity = direct.LazyPtr(in.GetSessionAffinity())
	out.HealthCheckDisabled = direct.LazyPtr(in.GetHealthCheckDisabled())
	out.GpuZonalRedundancyDisabled = in.GpuZonalRedundancyDisabled
	return out
}
func RevisionTemplate_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.RevisionTemplate) *pb.RevisionTemplate {
	if in == nil {
		return nil
	}
	out := &pb.RevisionTemplate{}
	out.Revision = direct.ValueOf(in.Revision)
	// MISSING: Labels
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
	// Written here: the RevisionTemplate fields a worker pool's template lacks.
	out.Scaling = RevisionScaling_ToProto(mapCtx, in.Scaling)
	out.Timeout = direct.StringDuration_ToProto(mapCtx, in.Timeout)
	out.MaxInstanceRequestConcurrency = direct.ValueOf(in.MaxInstanceRequestConcurrency)
	out.ExecutionEnvironment = direct.Enum_ToProto[pb.ExecutionEnvironment](mapCtx, in.ExecutionEnvironment)
	out.SessionAffinity = direct.ValueOf(in.SessionAffinity)
	out.HealthCheckDisabled = direct.ValueOf(in.HealthCheckDisabled)
	out.GpuZonalRedundancyDisabled = in.GpuZonalRedundancyDisabled
	return out
}
func RevisionTemplateObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.RevisionTemplate) *krmrunv1alpha1.RevisionTemplateObservedState {
	if in == nil {
		return nil
	}
	out := &krmrunv1alpha1.RevisionTemplateObservedState{}
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
func RevisionTemplateObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krmrunv1alpha1.RevisionTemplateObservedState) *pb.RevisionTemplate {
	if in == nil {
		return nil
	}
	out := &pb.RevisionTemplate{}
	out.Revision = direct.ValueOf(in.Revision)
	// MISSING: Labels
	// MISSING: Annotations
	// MISSING: VPCAccess
	// MISSING: ServiceAccount
	out.Containers = direct.Slice_ToProto(mapCtx, in.Containers, ContainerObservedState_v1alpha1_ToProto)
	// MISSING: Volumes
	// MISSING: EncryptionKey
	// MISSING: ServiceMesh
	// MISSING: EncryptionKeyRevocationAction
	// MISSING: EncryptionKeyShutdownDuration
	// MISSING: NodeSelector
	return out
}
func Condition_Reason_ToProto(mapCtx *direct.MapContext, in *string) *pb.Condition_Reason {
	if in == nil {
		return nil
	}
	return &pb.Condition_Reason{Reason: direct.Enum_ToProto[pb.Condition_CommonReason](mapCtx, in)}
}
func Condition_RevisionReason_ToProto(mapCtx *direct.MapContext, in *string) *pb.Condition_RevisionReason_ {
	if in == nil {
		return nil
	}
	return &pb.Condition_RevisionReason_{RevisionReason: direct.Enum_ToProto[pb.Condition_RevisionReason](mapCtx, in)}
}
func Condition_ExecutionReason_ToProto(mapCtx *direct.MapContext, in *string) *pb.Condition_ExecutionReason_ {
	if in == nil {
		return nil
	}
	return &pb.Condition_ExecutionReason_{ExecutionReason: direct.Enum_ToProto[pb.Condition_ExecutionReason](mapCtx, in)}
}
