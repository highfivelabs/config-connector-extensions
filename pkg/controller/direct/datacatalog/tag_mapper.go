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
// pkg/controller/direct/datacatalog/mapper_tag.go and the functions it calls in
// mapper.generated.go (UPSTREAM.md). Changed: our KRM types for the kind and
// its spec (stable, datacatalog.gcp.hfl.dev/v1beta1, docs/design.md decision
// 26), upstream's for the references and tag fields; only the functions this
// kind uses.

package datacatalog

import (
	pb "cloud.google.com/go/datacatalog/apiv1/datacatalogpb"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/datacatalog/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/datacatalog/v1beta1"
)

func DataCatalogTagSpec_FromProto(mapCtx *direct.MapContext, in *pb.Tag) *krm.DataCatalogTagSpec {
	if in == nil {
		return nil
	}
	out := &krm.DataCatalogTagSpec{}
	// MISSING: Name
	out.TemplateRef = &upstream.TagTemplateRef{
		External: in.GetTemplate(),
	}
	out.Column = direct.LazyPtr(in.GetColumn())
	out.Fields = make(map[string]upstream.TagField)
	for k, v := range in.GetFields() {
		out.Fields[k] = *TagField_FromProto(mapCtx, v)
	}
	return out
}

func DataCatalogTagSpec_ToProto(mapCtx *direct.MapContext, in *krm.DataCatalogTagSpec) *pb.Tag {
	if in == nil {
		return nil
	}
	out := &pb.Tag{}
	// MISSING: Name
	out.Template = in.TemplateRef.External
	if val := direct.ValueOf(in.Column); val != "" {
		out.Scope = &pb.Tag_Column{Column: val}
	}
	out.Fields = make(map[string]*pb.TagField)
	for k, v := range in.Fields {
		out.Fields[k] = TagField_ToProto(mapCtx, &v)
	}
	return out
}

func DataCatalogTagObservedState_FromProto(mapCtx *direct.MapContext, in *pb.Tag) *krm.DataCatalogTagObservedState {
	if in == nil {
		return nil
	}
	out := &krm.DataCatalogTagObservedState{}
	// MISSING: Name
	out.TemplateDisplayName = direct.LazyPtr(in.GetTemplateDisplayName())
	out.DataplexTransferStatus = direct.Enum_FromProto(mapCtx, in.GetDataplexTransferStatus())
	return out
}

func TagField_FromProto(mapCtx *direct.MapContext, in *pb.TagField) *upstream.TagField {
	if in == nil {
		return nil
	}
	out := &upstream.TagField{}
	// MISSING: DisplayName
	out.DoubleValue = direct.LazyPtr(in.GetDoubleValue())
	out.StringValue = direct.LazyPtr(in.GetStringValue())
	out.BoolValue = direct.LazyPtr(in.GetBoolValue())
	out.TimestampValue = direct.StringTimestamp_FromProto(mapCtx, in.GetTimestampValue())
	out.EnumValue = TagField_EnumValue_FromProto(mapCtx, in.GetEnumValue())
	out.RichtextValue = direct.LazyPtr(in.GetRichtextValue())
	// MISSING: Order
	return out
}

func TagField_ToProto(mapCtx *direct.MapContext, in *upstream.TagField) *pb.TagField {
	if in == nil {
		return nil
	}
	out := &pb.TagField{}
	// MISSING: DisplayName
	if in.DoubleValue != nil {
		out.Kind = &pb.TagField_DoubleValue{DoubleValue: *in.DoubleValue}
	}
	if in.StringValue != nil {
		out.Kind = &pb.TagField_StringValue{StringValue: *in.StringValue}
	}
	if in.BoolValue != nil {
		out.Kind = &pb.TagField_BoolValue{BoolValue: *in.BoolValue}
	}
	if in.TimestampValue != nil {
		out.Kind = &pb.TagField_TimestampValue{TimestampValue: direct.StringTimestamp_ToProto(mapCtx, in.TimestampValue)}
	}
	if in.EnumValue != nil {
		out.Kind = &pb.TagField_EnumValue_{EnumValue: TagField_EnumValue_ToProto(mapCtx, in.EnumValue)}
	}
	if in.RichtextValue != nil {
		out.Kind = &pb.TagField_RichtextValue{RichtextValue: *in.RichtextValue}
	}
	// MISSING: Order
	return out
}

func TagField_EnumValue_FromProto(mapCtx *direct.MapContext, in *pb.TagField_EnumValue) *upstream.TagField_EnumValue {
	if in == nil {
		return nil
	}
	out := &upstream.TagField_EnumValue{}
	out.DisplayName = direct.LazyPtr(in.GetDisplayName())
	return out
}

func TagField_EnumValue_ToProto(mapCtx *direct.MapContext, in *upstream.TagField_EnumValue) *pb.TagField_EnumValue {
	if in == nil {
		return nil
	}
	out := &pb.TagField_EnumValue{}
	out.DisplayName = direct.ValueOf(in.DisplayName)
	return out
}
