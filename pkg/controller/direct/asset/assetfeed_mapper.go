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
// pkg/controller/direct/asset/assetfeed_mapper.go, with AssetFeedSpec's
// and FeedOutputConfig's functions from mapper.generated.go (UPSTREAM.md);
// the KRM types are ours (apis/asset/v1beta1). No other change.

package asset

import (
	pb "cloud.google.com/go/asset/apiv1/assetpb"
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	exprpb "google.golang.org/genproto/googleapis/type/expr"

	krm "github.com/highfivelabs/config-connector-extensions/apis/asset/v1beta1"
)

func Expr_FromProto(mapCtx *direct.MapContext, in *exprpb.Expr) *krm.Expr {
	if in == nil {
		return nil
	}
	out := &krm.Expr{}
	out.Expression = direct.LazyPtr(in.GetExpression())
	out.Title = direct.LazyPtr(in.GetTitle())
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Location = direct.LazyPtr(in.GetLocation())
	return out
}

func Expr_ToProto(mapCtx *direct.MapContext, in *krm.Expr) *exprpb.Expr {
	if in == nil {
		return nil
	}
	out := &exprpb.Expr{}
	out.Expression = direct.ValueOf(in.Expression)
	out.Title = direct.ValueOf(in.Title)
	out.Description = direct.ValueOf(in.Description)
	out.Location = direct.ValueOf(in.Location)
	return out
}

func PubsubDestination_FromProto(mapCtx *direct.MapContext, in *pb.PubsubDestination) *krm.PubsubDestination {
	if in == nil {
		return nil
	}
	out := &krm.PubsubDestination{}
	out.TopicRef = &pubsubv1beta1.PubSubTopicRef{External: in.GetTopic()}
	return out
}
func PubsubDestination_ToProto(mapCtx *direct.MapContext, in *krm.PubsubDestination) *pb.PubsubDestination {
	if in == nil {
		return nil
	}
	out := &pb.PubsubDestination{}
	out.Topic = in.TopicRef.External
	return out
}

func AssetFeedSpec_FromProto(mapCtx *direct.MapContext, in *pb.Feed) *krm.AssetFeedSpec {
	if in == nil {
		return nil
	}
	out := &krm.AssetFeedSpec{}
	// MISSING: Name
	out.AssetNames = in.AssetNames
	out.AssetTypes = in.AssetTypes
	out.ContentType = direct.Enum_FromProto(mapCtx, in.GetContentType())
	out.FeedOutputConfig = FeedOutputConfig_FromProto(mapCtx, in.GetFeedOutputConfig())
	out.Condition = Expr_FromProto(mapCtx, in.GetCondition())
	out.RelationshipTypes = in.RelationshipTypes
	return out
}
func AssetFeedSpec_ToProto(mapCtx *direct.MapContext, in *krm.AssetFeedSpec) *pb.Feed {
	if in == nil {
		return nil
	}
	out := &pb.Feed{}
	// MISSING: Name
	out.AssetNames = in.AssetNames
	out.AssetTypes = in.AssetTypes
	out.ContentType = direct.Enum_ToProto[pb.ContentType](mapCtx, in.ContentType)
	out.FeedOutputConfig = FeedOutputConfig_ToProto(mapCtx, in.FeedOutputConfig)
	out.Condition = Expr_ToProto(mapCtx, in.Condition)
	out.RelationshipTypes = in.RelationshipTypes
	return out
}
func FeedOutputConfig_FromProto(mapCtx *direct.MapContext, in *pb.FeedOutputConfig) *krm.FeedOutputConfig {
	if in == nil {
		return nil
	}
	out := &krm.FeedOutputConfig{}
	out.PubsubDestination = PubsubDestination_FromProto(mapCtx, in.GetPubsubDestination())
	return out
}
func FeedOutputConfig_ToProto(mapCtx *direct.MapContext, in *krm.FeedOutputConfig) *pb.FeedOutputConfig {
	if in == nil {
		return nil
	}
	out := &pb.FeedOutputConfig{}
	if oneof := PubsubDestination_ToProto(mapCtx, in.PubsubDestination); oneof != nil {
		out.Destination = &pb.FeedOutputConfig_PubsubDestination{PubsubDestination: oneof}
	}
	return out
}
