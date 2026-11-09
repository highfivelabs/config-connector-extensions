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
// controllerbuilder generated: only the NetworkSecurityAddressGroup and
// NetworkSecuritySecurityProfile functions, unchanged but for the KRM types,
// which are this project's (networksecurity.gcp.hfl.dev); the intercept and
// mirroring endpoint group references stay Config Connector's.

package networksecurity

import (
	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/networksecurity/v1alpha1"
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
func AntivirusOverride_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.AntivirusOverride) *krm.AntivirusOverride {
	if in == nil {
		return nil
	}
	out := &krm.AntivirusOverride{}
	out.Protocol = direct.Enum_FromProto(mapCtx, in.GetProtocol())
	out.Action = direct.Enum_FromProto(mapCtx, in.GetAction())
	return out
}
func AntivirusOverride_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.AntivirusOverride) *pb.AntivirusOverride {
	if in == nil {
		return nil
	}
	out := &pb.AntivirusOverride{}
	out.Protocol = direct.Enum_ToProto[pb.Protocol](mapCtx, in.Protocol)
	out.Action = direct.Enum_ToProto[pb.ThreatAction](mapCtx, in.Action)
	return out
}
func CustomInterceptProfile_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.CustomInterceptProfile) *krm.CustomInterceptProfile {
	if in == nil {
		return nil
	}
	out := &krm.CustomInterceptProfile{}
	if in.GetInterceptEndpointGroup() != "" {
		out.InterceptEndpointGroupRef = &upstream.NetworkSecurityInterceptEndpointGroupRef{External: in.GetInterceptEndpointGroup()}
	}
	return out
}
func CustomInterceptProfile_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.CustomInterceptProfile) *pb.CustomInterceptProfile {
	if in == nil {
		return nil
	}
	out := &pb.CustomInterceptProfile{}
	if in.InterceptEndpointGroupRef != nil {
		out.InterceptEndpointGroup = in.InterceptEndpointGroupRef.External
	}
	return out
}
func CustomMirroringProfile_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.CustomMirroringProfile) *krm.CustomMirroringProfile {
	if in == nil {
		return nil
	}
	out := &krm.CustomMirroringProfile{}
	if in.GetMirroringEndpointGroup() != "" {
		out.MirroringEndpointGroupRef = &upstream.NetworkSecurityMirroringEndpointGroupRef{External: in.GetMirroringEndpointGroup()}
	}
	return out
}
func CustomMirroringProfile_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.CustomMirroringProfile) *pb.CustomMirroringProfile {
	if in == nil {
		return nil
	}
	out := &pb.CustomMirroringProfile{}
	if in.MirroringEndpointGroupRef != nil {
		out.MirroringEndpointGroup = in.MirroringEndpointGroupRef.External
	}
	return out
}
func NetworkSecuritySecurityProfileObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.SecurityProfile) *krm.NetworkSecuritySecurityProfileObservedState {
	if in == nil {
		return nil
	}
	out := &krm.NetworkSecuritySecurityProfileObservedState{}
	out.ThreatPreventionProfile = ThreatPreventionProfileObservedState_v1alpha1_FromProto(mapCtx, in.GetThreatPreventionProfile())
	// MISSING: Name
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	out.Etag = direct.LazyPtr(in.GetEtag())
	return out
}
func NetworkSecuritySecurityProfileObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.NetworkSecuritySecurityProfileObservedState) *pb.SecurityProfile {
	if in == nil {
		return nil
	}
	out := &pb.SecurityProfile{}
	if oneof := ThreatPreventionProfileObservedState_v1alpha1_ToProto(mapCtx, in.ThreatPreventionProfile); oneof != nil {
		out.Profile = &pb.SecurityProfile_ThreatPreventionProfile{ThreatPreventionProfile: oneof}
	}
	// MISSING: Name
	out.CreateTime = direct.StringTimestamp_ToProto(mapCtx, in.CreateTime)
	out.UpdateTime = direct.StringTimestamp_ToProto(mapCtx, in.UpdateTime)
	out.Etag = direct.ValueOf(in.Etag)
	return out
}
func NetworkSecuritySecurityProfileSpec_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.SecurityProfile) *krm.NetworkSecuritySecurityProfileSpec {
	if in == nil {
		return nil
	}
	out := &krm.NetworkSecuritySecurityProfileSpec{}
	out.ThreatPreventionProfile = ThreatPreventionProfile_v1alpha1_FromProto(mapCtx, in.GetThreatPreventionProfile())
	out.CustomMirroringProfile = CustomMirroringProfile_v1alpha1_FromProto(mapCtx, in.GetCustomMirroringProfile())
	out.CustomInterceptProfile = CustomInterceptProfile_v1alpha1_FromProto(mapCtx, in.GetCustomInterceptProfile())
	out.URLFilteringProfile = URLFilteringProfile_v1alpha1_FromProto(mapCtx, in.GetUrlFilteringProfile())
	// MISSING: Name
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Labels = in.Labels
	out.Type = direct.Enum_FromProto(mapCtx, in.GetType())
	return out
}
func NetworkSecuritySecurityProfileSpec_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.NetworkSecuritySecurityProfileSpec) *pb.SecurityProfile {
	if in == nil {
		return nil
	}
	out := &pb.SecurityProfile{}
	if oneof := ThreatPreventionProfile_v1alpha1_ToProto(mapCtx, in.ThreatPreventionProfile); oneof != nil {
		out.Profile = &pb.SecurityProfile_ThreatPreventionProfile{ThreatPreventionProfile: oneof}
	}
	if oneof := CustomMirroringProfile_v1alpha1_ToProto(mapCtx, in.CustomMirroringProfile); oneof != nil {
		out.Profile = &pb.SecurityProfile_CustomMirroringProfile{CustomMirroringProfile: oneof}
	}
	if oneof := CustomInterceptProfile_v1alpha1_ToProto(mapCtx, in.CustomInterceptProfile); oneof != nil {
		out.Profile = &pb.SecurityProfile_CustomInterceptProfile{CustomInterceptProfile: oneof}
	}
	if oneof := URLFilteringProfile_v1alpha1_ToProto(mapCtx, in.URLFilteringProfile); oneof != nil {
		out.Profile = &pb.SecurityProfile_UrlFilteringProfile{UrlFilteringProfile: oneof}
	}
	// MISSING: Name
	out.Description = direct.ValueOf(in.Description)
	out.Labels = in.Labels
	out.Type = direct.Enum_ToProto[pb.SecurityProfile_ProfileType](mapCtx, in.Type)
	return out
}
func SeverityOverride_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.SeverityOverride) *krm.SeverityOverride {
	if in == nil {
		return nil
	}
	out := &krm.SeverityOverride{}
	out.Severity = direct.Enum_FromProto(mapCtx, in.GetSeverity())
	out.Action = direct.Enum_FromProto(mapCtx, in.GetAction())
	return out
}
func SeverityOverride_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.SeverityOverride) *pb.SeverityOverride {
	if in == nil {
		return nil
	}
	out := &pb.SeverityOverride{}
	out.Severity = direct.Enum_ToProto[pb.Severity](mapCtx, in.Severity)
	out.Action = direct.Enum_ToProto[pb.ThreatAction](mapCtx, in.Action)
	return out
}
func ThreatOverride_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.ThreatOverride) *krm.ThreatOverride {
	if in == nil {
		return nil
	}
	out := &krm.ThreatOverride{}
	out.ThreatID = direct.LazyPtr(in.GetThreatId())
	// MISSING: Type
	out.Action = direct.Enum_FromProto(mapCtx, in.GetAction())
	return out
}
func ThreatOverride_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.ThreatOverride) *pb.ThreatOverride {
	if in == nil {
		return nil
	}
	out := &pb.ThreatOverride{}
	out.ThreatId = direct.ValueOf(in.ThreatID)
	// MISSING: Type
	out.Action = direct.Enum_ToProto[pb.ThreatAction](mapCtx, in.Action)
	return out
}
func ThreatOverrideObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.ThreatOverride) *krm.ThreatOverrideObservedState {
	if in == nil {
		return nil
	}
	out := &krm.ThreatOverrideObservedState{}
	// MISSING: ThreatID
	out.Type = direct.Enum_FromProto(mapCtx, in.GetType())
	// MISSING: Action
	return out
}
func ThreatOverrideObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.ThreatOverrideObservedState) *pb.ThreatOverride {
	if in == nil {
		return nil
	}
	out := &pb.ThreatOverride{}
	// MISSING: ThreatID
	out.Type = direct.Enum_ToProto[pb.ThreatType](mapCtx, in.Type)
	// MISSING: Action
	return out
}
func ThreatPreventionProfile_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.ThreatPreventionProfile) *krm.ThreatPreventionProfile {
	if in == nil {
		return nil
	}
	out := &krm.ThreatPreventionProfile{}
	out.SeverityOverrides = direct.Slice_FromProto(mapCtx, in.SeverityOverrides, SeverityOverride_v1alpha1_FromProto)
	out.ThreatOverrides = direct.Slice_FromProto(mapCtx, in.ThreatOverrides, ThreatOverride_v1alpha1_FromProto)
	out.AntivirusOverrides = direct.Slice_FromProto(mapCtx, in.AntivirusOverrides, AntivirusOverride_v1alpha1_FromProto)
	return out
}
func ThreatPreventionProfile_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.ThreatPreventionProfile) *pb.ThreatPreventionProfile {
	if in == nil {
		return nil
	}
	out := &pb.ThreatPreventionProfile{}
	out.SeverityOverrides = direct.Slice_ToProto(mapCtx, in.SeverityOverrides, SeverityOverride_v1alpha1_ToProto)
	out.ThreatOverrides = direct.Slice_ToProto(mapCtx, in.ThreatOverrides, ThreatOverride_v1alpha1_ToProto)
	out.AntivirusOverrides = direct.Slice_ToProto(mapCtx, in.AntivirusOverrides, AntivirusOverride_v1alpha1_ToProto)
	return out
}
func ThreatPreventionProfileObservedState_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.ThreatPreventionProfile) *krm.ThreatPreventionProfileObservedState {
	if in == nil {
		return nil
	}
	out := &krm.ThreatPreventionProfileObservedState{}
	// MISSING: SeverityOverrides
	out.ThreatOverrides = direct.Slice_FromProto(mapCtx, in.ThreatOverrides, ThreatOverrideObservedState_v1alpha1_FromProto)
	// MISSING: AntivirusOverrides
	return out
}
func ThreatPreventionProfileObservedState_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.ThreatPreventionProfileObservedState) *pb.ThreatPreventionProfile {
	if in == nil {
		return nil
	}
	out := &pb.ThreatPreventionProfile{}
	// MISSING: SeverityOverrides
	out.ThreatOverrides = direct.Slice_ToProto(mapCtx, in.ThreatOverrides, ThreatOverrideObservedState_v1alpha1_ToProto)
	// MISSING: AntivirusOverrides
	return out
}
func URLFilter_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.UrlFilter) *krm.URLFilter {
	if in == nil {
		return nil
	}
	out := &krm.URLFilter{}
	out.FilteringAction = direct.Enum_FromProto(mapCtx, in.GetFilteringAction())
	out.Urls = in.Urls
	out.Priority = in.Priority
	return out
}
func URLFilter_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.URLFilter) *pb.UrlFilter {
	if in == nil {
		return nil
	}
	out := &pb.UrlFilter{}
	out.FilteringAction = direct.Enum_ToProto[pb.UrlFilter_UrlFilteringAction](mapCtx, in.FilteringAction)
	out.Urls = in.Urls
	out.Priority = in.Priority
	return out
}
func URLFilteringProfile_v1alpha1_FromProto(mapCtx *direct.MapContext, in *pb.UrlFilteringProfile) *krm.URLFilteringProfile {
	if in == nil {
		return nil
	}
	out := &krm.URLFilteringProfile{}
	out.URLFilters = direct.Slice_FromProto(mapCtx, in.UrlFilters, URLFilter_v1alpha1_FromProto)
	return out
}
func URLFilteringProfile_v1alpha1_ToProto(mapCtx *direct.MapContext, in *krm.URLFilteringProfile) *pb.UrlFilteringProfile {
	if in == nil {
		return nil
	}
	out := &pb.UrlFilteringProfile{}
	out.UrlFilters = direct.Slice_ToProto(mapCtx, in.URLFilters, URLFilter_v1alpha1_ToProto)
	return out
}
