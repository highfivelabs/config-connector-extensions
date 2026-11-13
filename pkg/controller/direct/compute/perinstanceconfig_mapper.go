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

// Written here (High Five Labs, config-connector-extensions): the spec
// mapping of ComputePerInstanceConfig, the zonal per-instance config, after
// regionperinstanceconfig_mapper.go (copied from Config Connector); the
// preserved state maps with that file's functions, since the two kinds
// share its types.

package compute

import (
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func ComputePerInstanceConfigSpec_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.PerInstanceConfig) *krm.ComputePerInstanceConfigSpec {
	if in == nil {
		return nil
	}
	out := &krm.ComputePerInstanceConfigSpec{}
	// MISSING: Fingerprint
	// MISSING: Name
	out.PreservedState = RegionperinstanceconfigPreservedState_v1beta1_FromProto(mapCtx, in.GetPreservedState())
	// MISSING: Status
	return out
}

func ComputePerInstanceConfigSpec_v1beta1_ToProto(mapCtx *direct.MapContext, in *krm.ComputePerInstanceConfigSpec) *pb.PerInstanceConfig {
	if in == nil {
		return nil
	}
	out := &pb.PerInstanceConfig{}
	// MISSING: Fingerprint
	// MISSING: Name
	out.PreservedState = RegionperinstanceconfigPreservedState_v1beta1_ToProto(mapCtx, in.PreservedState)
	// MISSING: Status
	return out
}
