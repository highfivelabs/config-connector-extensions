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
// pkg/controller/direct/compute/computenetworkendpoint_mapper.go (UPSTREAM.md). Changed: our
// KRM types (stable, compute.gcp.hfl.dev/v1beta1, docs/design.md decision 26);
// the functions named for this package; the unused status functions dropped.

package compute

import (
	"strconv"

	pb "cloud.google.com/go/compute/apiv1/computepb"
	upstreamcompute "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krmcompute "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func networkEndpointSpecFromProto(mapCtx *direct.MapContext, in *pb.NetworkEndpoint) *krmcompute.ComputeNetworkEndpointSpec {
	if in == nil {
		return nil
	}
	out := &krmcompute.ComputeNetworkEndpointSpec{}
	if in.GetInstance() != "" {
		out.InstanceRef = &upstreamcompute.InstanceRef{External: in.GetInstance()}
	}
	if in.GetIpAddress() != "" {
		out.IpAddress = in.GetIpAddress()
	}
	if in.Port != nil {
		portStr := strconv.Itoa(int(*in.Port))
		out.ResourceID = &portStr
	}
	return out
}

func networkEndpointSpecToProto(mapCtx *direct.MapContext, in *krmcompute.ComputeNetworkEndpointSpec) *pb.NetworkEndpoint {
	if in == nil {
		return nil
	}
	out := &pb.NetworkEndpoint{}
	if in.InstanceRef != nil {
		out.Instance = &in.InstanceRef.External
	}
	if in.IpAddress != "" {
		out.IpAddress = &in.IpAddress
	}
	if in.ResourceID != nil {
		portVal, err := strconv.Atoi(*in.ResourceID)
		if err != nil {
			mapCtx.Errorf("invalid port %q: %v", *in.ResourceID, err)
		} else {
			portInt32 := int32(portVal)
			out.Port = &portInt32
		}
	}
	return out
}
