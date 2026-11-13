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
// pkg/controller/direct/dns/zz_generated.v1alpha1.mappers.go (UPSTREAM.md),
// generated there by dev/tools/openapi-to-krm: only the DNSResponsePolicyRule
// spec functions, on our KRM types (dns.gcp.hfl.dev/v1beta1).

package dns

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	api "google.golang.org/api/dns/v1"

	krm "github.com/highfivelabs/config-connector-extensions/apis/dns/v1beta1"
)

func DNSResponsePolicyRuleSpec_FromAPI(mapCtx *direct.MapContext, in *api.ResponsePolicyRule) *krm.DNSResponsePolicyRuleSpec {
	if in == nil {
		return nil
	}
	out := &krm.DNSResponsePolicyRuleSpec{}
	out.Behavior = direct.LazyPtr(in.Behavior)
	out.DnsName = in.DnsName
	out.LocalData = ResponsepolicyruleLocalData_FromAPI(mapCtx, in.LocalData)
	// MISSING: ProjectRef
	// MISSING: ResourceID
	// MISSING: ResponsePolicy
	return out
}

func DNSResponsePolicyRuleSpec_ToAPI(mapCtx *direct.MapContext, in *krm.DNSResponsePolicyRuleSpec) *api.ResponsePolicyRule {
	if in == nil {
		return nil
	}
	out := &api.ResponsePolicyRule{}
	out.Behavior = direct.ValueOf(in.Behavior)
	out.DnsName = in.DnsName
	out.LocalData = ResponsepolicyruleLocalData_ToAPI(mapCtx, in.LocalData)
	// MISSING: RuleName
	// MISSING: HTTPStatusCode
	// MISSING: Header
	return out
}

func ResponsepolicyruleLocalData_FromAPI(mapCtx *direct.MapContext, in *api.ResponsePolicyRuleLocalData) *krm.ResponsepolicyruleLocalData {
	if in == nil {
		return nil
	}
	out := &krm.ResponsepolicyruleLocalData{}
	out.LocalDatas = direct.Slice_FromProto(mapCtx, in.LocalDatas, ResponsepolicyruleLocalDatas_FromAPI)
	return out
}

func ResponsepolicyruleLocalData_ToAPI(mapCtx *direct.MapContext, in *krm.ResponsepolicyruleLocalData) *api.ResponsePolicyRuleLocalData {
	if in == nil {
		return nil
	}
	out := &api.ResponsePolicyRuleLocalData{}
	out.LocalDatas = direct.Slice_ToProto(mapCtx, in.LocalDatas, ResponsepolicyruleLocalDatas_ToAPI)
	return out
}

func ResponsepolicyruleLocalDatas_FromAPI(mapCtx *direct.MapContext, in *api.ResourceRecordSet) *krm.ResponsepolicyruleLocalDatas {
	if in == nil {
		return nil
	}
	out := &krm.ResponsepolicyruleLocalDatas{}
	out.Name = in.Name
	out.Rrdatas = in.Rrdatas
	out.Ttl = direct.LazyPtr(in.Ttl)
	out.Type = in.Type
	return out
}

func ResponsepolicyruleLocalDatas_ToAPI(mapCtx *direct.MapContext, in *krm.ResponsepolicyruleLocalDatas) *api.ResourceRecordSet {
	if in == nil {
		return nil
	}
	out := &api.ResourceRecordSet{}
	out.Name = in.Name
	out.Rrdatas = in.Rrdatas
	out.Ttl = direct.ValueOf(in.Ttl)
	out.Type = in.Type
	// MISSING: RoutingPolicy
	// MISSING: SignatureRrdatas
	// MISSING: HTTPStatusCode
	// MISSING: Header
	return out
}
