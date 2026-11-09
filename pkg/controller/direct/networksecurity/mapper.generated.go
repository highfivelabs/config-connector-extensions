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
// pkg/controller/direct/networksecurity/mapper.generated.go (UPSTREAM.md), which
// controllerbuilder generated: only the NetworkSecurityAddressGroup functions,
// unchanged but for the KRM types, which are this project's
// (networksecurity.gcp.hfl.dev).

package networksecurity

import (
	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	krm "github.com/highfivelabs/config-connector-extensions/apis/networksecurity/v1alpha1"
)

func NetworkSecurityAddressGroupObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.AddressGroup) *krm.NetworkSecurityAddressGroupObservedState {
	if in == nil {
		return nil
	}
	out := &krm.NetworkSecurityAddressGroupObservedState{}
	// MISSING: Name
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	out.SelfLink = direct.LazyPtr(in.GetSelfLink())
	return out
}
func NetworkSecurityAddressGroupObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.NetworkSecurityAddressGroupObservedState) *pb.AddressGroup {
	if in == nil {
		return nil
	}
	out := &pb.AddressGroup{}
	// MISSING: Name
	out.CreateTime = direct.StringTimestamp_ToProto(mapCtx, in.CreateTime)
	out.UpdateTime = direct.StringTimestamp_ToProto(mapCtx, in.UpdateTime)
	out.SelfLink = direct.ValueOf(in.SelfLink)
	return out
}
func NetworkSecurityAddressGroupSpec_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.AddressGroup) *krm.NetworkSecurityAddressGroupSpec {
	if in == nil {
		return nil
	}
	out := &krm.NetworkSecurityAddressGroupSpec{}
	// MISSING: Name
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Labels = in.Labels
	out.Type = direct.Enum_FromProto(mapCtx, in.GetType())
	out.Items = in.Items
	out.Capacity = direct.LazyPtr(in.GetCapacity())
	out.Purpose = direct.EnumSlice_FromProto(mapCtx, in.Purpose)
	return out
}
func NetworkSecurityAddressGroupSpec_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.NetworkSecurityAddressGroupSpec) *pb.AddressGroup {
	if in == nil {
		return nil
	}
	out := &pb.AddressGroup{}
	// MISSING: Name
	out.Description = direct.ValueOf(in.Description)
	out.Labels = in.Labels
	out.Type = direct.Enum_ToProto[pb.AddressGroup_Type](mapCtx, in.Type)
	out.Items = in.Items
	out.Capacity = direct.ValueOf(in.Capacity)
	out.Purpose = direct.EnumSlice_ToProto[pb.AddressGroup_Purpose](mapCtx, in.Purpose)
	return out
}
