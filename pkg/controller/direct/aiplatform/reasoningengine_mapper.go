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

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/aiplatform/aiplatformreasoningengine_mapper.go and the functions it
// calls in mapper.generated.go and aiplatformschedule_mapper.go (UPSTREAM.md). Changed:
// our KRM types for the kind and its spec (stable, aiplatform.gcp.hfl.dev/v1beta1,
// docs/design.md decision 26), upstream's for the leaf types; only the
// functions this kind uses.

package aiplatform

import (
	pb "cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/aiplatform/v1alpha1"
	krmcomputev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1alpha1"
	krmcomputev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/aiplatform/v1beta1"
)

func AIPlatformReasoningEngineSpec_FromProto(mapCtx *direct.MapContext, in *pb.ReasoningEngine) *krm.AIPlatformReasoningEngineSpec {
	if in == nil {
		return nil
	}
	out := &krm.AIPlatformReasoningEngineSpec{}
	// MISSING: Name
	out.DisplayName = direct.LazyPtr(in.GetDisplayName())
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Spec = ReasoningEngineSpec_FromProto(mapCtx, in.GetSpec())
	// MISSING: Etag
	out.EncryptionSpec = EncryptionSpec_FromProto(mapCtx, in.GetEncryptionSpec())
	out.Labels = in.GetLabels()
	return out
}

func AIPlatformReasoningEngineSpec_ToProto(mapCtx *direct.MapContext, in *krm.AIPlatformReasoningEngineSpec) *pb.ReasoningEngine {
	if in == nil {
		return nil
	}
	out := &pb.ReasoningEngine{}
	// MISSING: Name
	out.DisplayName = direct.ValueOf(in.DisplayName)
	out.Description = direct.ValueOf(in.Description)
	out.Spec = ReasoningEngineSpec_ToProto(mapCtx, in.Spec)
	// MISSING: Etag
	out.EncryptionSpec = EncryptionSpec_ToProto(mapCtx, in.EncryptionSpec)
	out.Labels = in.Labels
	return out
}

func AIPlatformReasoningEngineObservedState_FromProto(mapCtx *direct.MapContext, in *pb.ReasoningEngine) *krm.AIPlatformReasoningEngineObservedState {
	if in == nil {
		return nil
	}
	out := &krm.AIPlatformReasoningEngineObservedState{}
	// MISSING: Name
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	// MISSING: Etag
	return out
}

func ReasoningEngineSpec_FromProto(mapCtx *direct.MapContext, in *pb.ReasoningEngineSpec) *krm.ReasoningEngineSpec {
	if in == nil {
		return nil
	}
	out := &krm.ReasoningEngineSpec{}
	if in.GetServiceAccount() != "" {
		out.ServiceAccountRef = &refsv1beta1.IAMServiceAccountRef{External: in.GetServiceAccount()}
	}
	out.PackageSpec = ReasoningEngineSpec_PackageSpec_FromProto(mapCtx, in.GetPackageSpec())
	out.DeploymentSpec = ReasoningEngineSpec_DeploymentSpec_FromProto(mapCtx, in.GetDeploymentSpec())
	out.AgentFramework = direct.LazyPtr(in.GetAgentFramework())
	return out
}

func ReasoningEngineSpec_ToProto(mapCtx *direct.MapContext, in *krm.ReasoningEngineSpec) *pb.ReasoningEngineSpec {
	if in == nil {
		return nil
	}
	out := &pb.ReasoningEngineSpec{}
	if in.ServiceAccountRef != nil {
		out.ServiceAccount = direct.LazyPtr(in.ServiceAccountRef.External)
	}
	out.PackageSpec = ReasoningEngineSpec_PackageSpec_ToProto(mapCtx, in.PackageSpec)
	out.DeploymentSpec = ReasoningEngineSpec_DeploymentSpec_ToProto(mapCtx, in.DeploymentSpec)
	out.AgentFramework = direct.ValueOf(in.AgentFramework)
	return out
}

func ReasoningEngineSpec_DeploymentSpec_FromProto(mapCtx *direct.MapContext, in *pb.ReasoningEngineSpec_DeploymentSpec) *krm.ReasoningEngineSpec_DeploymentSpec {
	if in == nil {
		return nil
	}
	out := &krm.ReasoningEngineSpec_DeploymentSpec{}
	out.Env = direct.Slice_FromProto(mapCtx, in.Env, EnvVar_FromProto)
	out.SecretEnv = direct.Slice_FromProto(mapCtx, in.SecretEnv, SecretEnvVar_FromProto)
	out.PSCInterfaceConfig = PSCInterfaceConfig_FromProto(mapCtx, in.GetPscInterfaceConfig())
	out.MinInstances = in.MinInstances
	out.MaxInstances = in.MaxInstances
	out.ResourceLimits = in.ResourceLimits
	out.ContainerConcurrency = in.ContainerConcurrency
	return out
}

func ReasoningEngineSpec_DeploymentSpec_ToProto(mapCtx *direct.MapContext, in *krm.ReasoningEngineSpec_DeploymentSpec) *pb.ReasoningEngineSpec_DeploymentSpec {
	if in == nil {
		return nil
	}
	out := &pb.ReasoningEngineSpec_DeploymentSpec{}
	out.Env = direct.Slice_ToProto(mapCtx, in.Env, EnvVar_ToProto)
	out.SecretEnv = direct.Slice_ToProto(mapCtx, in.SecretEnv, SecretEnvVar_ToProto)
	out.PscInterfaceConfig = PSCInterfaceConfig_ToProto(mapCtx, in.PSCInterfaceConfig)
	out.MinInstances = in.MinInstances
	out.MaxInstances = in.MaxInstances
	out.ResourceLimits = in.ResourceLimits
	out.ContainerConcurrency = in.ContainerConcurrency
	return out
}

func ReasoningEngineSpec_PackageSpec_FromProto(mapCtx *direct.MapContext, in *pb.ReasoningEngineSpec_PackageSpec) *krm.ReasoningEngineSpec_PackageSpec {
	if in == nil {
		return nil
	}
	out := &krm.ReasoningEngineSpec_PackageSpec{}
	out.PickleObjectGCSURI = direct.LazyPtr(in.GetPickleObjectGcsUri())
	out.DependencyFilesGCSURI = direct.LazyPtr(in.GetDependencyFilesGcsUri())
	out.RequirementsGCSURI = direct.LazyPtr(in.GetRequirementsGcsUri())
	out.PythonVersion = direct.LazyPtr(in.GetPythonVersion())
	return out
}

func ReasoningEngineSpec_PackageSpec_ToProto(mapCtx *direct.MapContext, in *krm.ReasoningEngineSpec_PackageSpec) *pb.ReasoningEngineSpec_PackageSpec {
	if in == nil {
		return nil
	}
	out := &pb.ReasoningEngineSpec_PackageSpec{}
	out.PickleObjectGcsUri = direct.ValueOf(in.PickleObjectGCSURI)
	out.DependencyFilesGcsUri = direct.ValueOf(in.DependencyFilesGCSURI)
	out.RequirementsGcsUri = direct.ValueOf(in.RequirementsGCSURI)
	out.PythonVersion = direct.ValueOf(in.PythonVersion)
	return out
}

func EnvVar_FromProto(mapCtx *direct.MapContext, in *pb.EnvVar) *upstream.EnvVar {
	if in == nil {
		return nil
	}
	out := &upstream.EnvVar{}
	out.Name = direct.LazyPtr(in.GetName())
	out.Value = direct.LazyPtr(in.GetValue())
	return out
}

func EnvVar_ToProto(mapCtx *direct.MapContext, in *upstream.EnvVar) *pb.EnvVar {
	if in == nil {
		return nil
	}
	out := &pb.EnvVar{}
	out.Name = direct.ValueOf(in.Name)
	out.Value = direct.ValueOf(in.Value)
	return out
}

func SecretEnvVar_FromProto(mapCtx *direct.MapContext, in *pb.SecretEnvVar) *upstream.SecretEnvVar {
	if in == nil {
		return nil
	}
	out := &upstream.SecretEnvVar{}
	out.Name = direct.LazyPtr(in.GetName())
	out.SecretRef = SecretRef_FromProto(mapCtx, in.GetSecretRef())
	return out
}

func SecretEnvVar_ToProto(mapCtx *direct.MapContext, in *upstream.SecretEnvVar) *pb.SecretEnvVar {
	if in == nil {
		return nil
	}
	out := &pb.SecretEnvVar{}
	out.Name = direct.ValueOf(in.Name)
	out.SecretRef = SecretRef_ToProto(mapCtx, in.SecretRef)
	return out
}

func SecretRef_FromProto(mapCtx *direct.MapContext, in *pb.SecretRef) *upstream.SecretRef {
	if in == nil {
		return nil
	}
	out := &upstream.SecretRef{}
	if in.GetSecret() != "" {
		out.SecretRef = &refsv1beta1.SecretManagerSecretRef{External: in.GetSecret()}
	}
	if in.GetVersion() != "" {
		out.VersionRef = &refsv1beta1.SecretManagerSecretVersionRef{External: in.GetVersion()}
	}
	return out
}

func SecretRef_ToProto(mapCtx *direct.MapContext, in *upstream.SecretRef) *pb.SecretRef {
	if in == nil {
		return nil
	}
	out := &pb.SecretRef{}
	if in.SecretRef != nil {
		out.Secret = in.SecretRef.External
	}
	if in.VersionRef != nil {
		out.Version = in.VersionRef.External
	}
	return out
}

func PSCInterfaceConfig_FromProto(mapCtx *direct.MapContext, in *pb.PscInterfaceConfig) *upstream.PSCInterfaceConfig {
	if in == nil {
		return nil
	}
	out := &upstream.PSCInterfaceConfig{}
	if in.GetNetworkAttachment() != "" {
		out.NetworkAttachmentRef = &krmcomputev1alpha1.ComputeNetworkAttachmentRef{External: in.GetNetworkAttachment()}
	}
	out.DNSPeeringConfigs = direct.Slice_FromProto(mapCtx, in.DnsPeeringConfigs, DNSPeeringConfig_FromProto)
	return out
}

func PSCInterfaceConfig_ToProto(mapCtx *direct.MapContext, in *upstream.PSCInterfaceConfig) *pb.PscInterfaceConfig {
	if in == nil {
		return nil
	}
	out := &pb.PscInterfaceConfig{}
	if in.NetworkAttachmentRef != nil {
		out.NetworkAttachment = in.NetworkAttachmentRef.External
	}
	out.DnsPeeringConfigs = direct.Slice_ToProto(mapCtx, in.DNSPeeringConfigs, DNSPeeringConfig_ToProto)
	return out
}

func DNSPeeringConfig_FromProto(mapCtx *direct.MapContext, in *pb.DnsPeeringConfig) *upstream.DNSPeeringConfig {
	if in == nil {
		return nil
	}
	out := &upstream.DNSPeeringConfig{}
	out.Domain = direct.LazyPtr(in.GetDomain())
	if in.GetTargetProject() != "" {
		out.TargetProjectRef = &refsv1beta1.ProjectRef{External: in.GetTargetProject()}
	}
	if in.GetTargetNetwork() != "" {
		out.TargetNetworkRef = &krmcomputev1beta1.ComputeNetworkRef{External: in.GetTargetNetwork()}
	}
	return out
}

func DNSPeeringConfig_ToProto(mapCtx *direct.MapContext, in *upstream.DNSPeeringConfig) *pb.DnsPeeringConfig {
	if in == nil {
		return nil
	}
	out := &pb.DnsPeeringConfig{}
	out.Domain = direct.ValueOf(in.Domain)
	if in.TargetProjectRef != nil {
		out.TargetProject = in.TargetProjectRef.External
	}
	if in.TargetNetworkRef != nil {
		out.TargetNetwork = in.TargetNetworkRef.External
	}
	return out
}

func EncryptionSpec_FromProto(mapCtx *direct.MapContext, in *pb.EncryptionSpec) *upstream.EncryptionSpec {
	if in == nil {
		return nil
	}
	out := &upstream.EncryptionSpec{}
	if in.GetKmsKeyName() != "" {
		out.KmsKeyRef = &kmsv1beta1.KMSCryptoKeyRef{External: in.GetKmsKeyName()}
	}
	return out
}

func EncryptionSpec_ToProto(mapCtx *direct.MapContext, in *upstream.EncryptionSpec) *pb.EncryptionSpec {
	if in == nil {
		return nil
	}
	out := &pb.EncryptionSpec{}
	if in.KmsKeyRef != nil {
		out.KmsKeyName = in.KmsKeyRef.External
	}
	return out
}
