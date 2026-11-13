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

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/spanner/spannerinstanceconfig_mapper.go (UPSTREAM.md). Changed:
// our KRM types for the kind and its spec (stable, spanner.gcp.hfl.dev/v1beta1,
// docs/design.md decision 26), upstream's for the replica and the reference;
// the observed state's unused ToProto dropped.

package spanner

import (
	pb "cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/spanner/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/spanner/v1beta1"
)

func ReplicaInfo_FromProto(mapCtx *direct.MapContext, in *pb.ReplicaInfo) *upstream.ReplicaInfo {
	if in == nil {
		return nil
	}
	out := &upstream.ReplicaInfo{}
	out.Location = direct.LazyPtr(in.GetLocation())
	out.Type = direct.Enum_FromProto(mapCtx, in.GetType())
	out.DefaultLeaderLocation = direct.LazyPtr(in.GetDefaultLeaderLocation())
	return out
}
func ReplicaInfo_ToProto(mapCtx *direct.MapContext, in *upstream.ReplicaInfo) *pb.ReplicaInfo {
	if in == nil {
		return nil
	}
	out := &pb.ReplicaInfo{}
	out.Location = direct.ValueOf(in.Location)
	out.Type = direct.Enum_ToProto[pb.ReplicaInfo_ReplicaType](mapCtx, in.Type)
	out.DefaultLeaderLocation = direct.ValueOf(in.DefaultLeaderLocation)
	return out
}
func SpannerInstanceConfigObservedState_FromProto(mapCtx *direct.MapContext, in *pb.InstanceConfig) *krm.SpannerInstanceConfigObservedState {
	if in == nil {
		return nil
	}
	out := &krm.SpannerInstanceConfigObservedState{}
	// MISSING: Name
	out.ConfigType = direct.Enum_FromProto(mapCtx, in.GetConfigType())
	out.OptionalReplicas = direct.Slice_FromProto(mapCtx, in.OptionalReplicas, ReplicaInfo_FromProto)
	out.Reconciling = direct.LazyPtr(in.GetReconciling())
	out.State = direct.Enum_FromProto(mapCtx, in.GetState())
	out.FreeInstanceAvailability = direct.Enum_FromProto(mapCtx, in.GetFreeInstanceAvailability())
	out.QuorumType = direct.Enum_FromProto(mapCtx, in.GetQuorumType())
	out.StorageLimitPerProcessingUnit = direct.LazyPtr(in.GetStorageLimitPerProcessingUnit())
	return out
}
func SpannerInstanceConfigSpec_FromProto(mapCtx *direct.MapContext, in *pb.InstanceConfig) *krm.SpannerInstanceConfigSpec {
	if in == nil {
		return nil
	}
	out := &krm.SpannerInstanceConfigSpec{}
	// MISSING: Name
	out.DisplayName = direct.LazyPtr(in.GetDisplayName())
	out.Replicas = direct.Slice_FromProto(mapCtx, in.Replicas, ReplicaInfo_FromProto)
	if in.GetBaseConfig() != "" {
		out.BaseConfigRef = &upstream.InstanceConfigRef{External: in.BaseConfig}
	}
	out.Labels = in.Labels
	out.LeaderOptions = in.LeaderOptions
	return out
}
func SpannerInstanceConfigSpec_ToProto(mapCtx *direct.MapContext, in *krm.SpannerInstanceConfigSpec) *pb.InstanceConfig {
	if in == nil {
		return nil
	}
	out := &pb.InstanceConfig{}
	// MISSING: Name
	out.DisplayName = direct.ValueOf(in.DisplayName)
	out.Replicas = direct.Slice_ToProto(mapCtx, in.Replicas, ReplicaInfo_ToProto)
	if in.BaseConfigRef != nil {
		out.BaseConfig = in.BaseConfigRef.External
	}
	out.Labels = in.Labels
	out.LeaderOptions = in.LeaderOptions
	return out
}
