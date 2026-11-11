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
// pkg/controller/direct/securitycenter/mapper.generated.go (UPSTREAM.md),
// its SecurityCenterMuteConfig functions only, on our KRM types.

package securitycenter

import (
	pb "cloud.google.com/go/securitycenter/apiv1/securitycenterpb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/securitycenter/v1alpha1"
)

func SecurityCenterMuteConfigObservedState_FromProto(mapCtx *direct.MapContext, in *pb.MuteConfig) *krm.SecurityCenterMuteConfigObservedState {
	if in == nil {
		return nil
	}
	out := &krm.SecurityCenterMuteConfigObservedState{}
	// MISSING: Name
	// MISSING: DisplayName
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	out.MostRecentEditor = direct.LazyPtr(in.GetMostRecentEditor())
	return out
}

func SecurityCenterMuteConfigObservedState_ToProto(mapCtx *direct.MapContext, in *krm.SecurityCenterMuteConfigObservedState) *pb.MuteConfig {
	if in == nil {
		return nil
	}
	out := &pb.MuteConfig{}
	// MISSING: Name
	// MISSING: DisplayName
	out.CreateTime = direct.StringTimestamp_ToProto(mapCtx, in.CreateTime)
	out.UpdateTime = direct.StringTimestamp_ToProto(mapCtx, in.UpdateTime)
	out.MostRecentEditor = direct.ValueOf(in.MostRecentEditor)
	return out
}

func SecurityCenterMuteConfigSpec_FromProto(mapCtx *direct.MapContext, in *pb.MuteConfig) *krm.SecurityCenterMuteConfigSpec {
	if in == nil {
		return nil
	}
	out := &krm.SecurityCenterMuteConfigSpec{}
	// MISSING: Name
	// MISSING: DisplayName
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Filter = direct.LazyPtr(in.GetFilter())
	out.Type = direct.Enum_FromProto(mapCtx, in.GetType())
	out.ExpiryTime = direct.StringTimestamp_FromProto(mapCtx, in.GetExpiryTime())
	return out
}

func SecurityCenterMuteConfigSpec_ToProto(mapCtx *direct.MapContext, in *krm.SecurityCenterMuteConfigSpec) *pb.MuteConfig {
	if in == nil {
		return nil
	}
	out := &pb.MuteConfig{}
	// MISSING: Name
	// MISSING: DisplayName
	out.Description = direct.ValueOf(in.Description)
	out.Filter = direct.ValueOf(in.Filter)
	out.Type = direct.Enum_ToProto[pb.MuteConfig_MuteConfigType](mapCtx, in.Type)
	out.ExpiryTime = direct.StringTimestamp_ToProto(mapCtx, in.ExpiryTime)
	return out
}
