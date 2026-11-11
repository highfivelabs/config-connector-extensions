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
// pkg/controller/direct/compute/computevpntunnel_mapper.go (UPSTREAM.md),
// for our ComputeVPNTunnel. Changed: our KRM types; the references are
// resolved by the controller first (each ref's external is set, the router's
// too); the shared secret's value is passed in (read from its Secret);
// cipherSuite and labels are added. Only the spec-to-proto and
// status-from-proto directions are kept: the kind has no export.

package compute

import (
	"strconv"

	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func ComputeVPNTunnelSpec_v1beta1_ToProto(mapCtx *direct.MapContext, in *krm.ComputeVPNTunnelSpec, name string, sharedSecret *string) *pb.VpnTunnel {
	if in == nil {
		return nil
	}
	out := &pb.VpnTunnel{Name: direct.PtrTo(name)}
	out.Description = in.Description
	out.IkeVersion = direct.PtrInt64ToPtrInt32(in.IkeVersion)
	out.LocalTrafficSelector = in.LocalTrafficSelector
	out.PeerExternalGatewayInterface = direct.PtrInt64ToPtrInt32(in.PeerExternalGatewayInterface)
	if in.PeerExternalGatewayRef != nil && in.PeerExternalGatewayRef.External != "" {
		out.PeerExternalGateway = &in.PeerExternalGatewayRef.External
	}
	if in.PeerGCPGatewayRef != nil && in.PeerGCPGatewayRef.External != "" {
		out.PeerGcpGateway = &in.PeerGCPGatewayRef.External
	}
	out.PeerIp = in.PeerIp
	if in.Region != "" {
		out.Region = &in.Region
	}
	out.RemoteTrafficSelector = in.RemoteTrafficSelector
	if in.RouterRef != nil && in.RouterRef.External != "" {
		out.Router = &in.RouterRef.External
	}
	out.SharedSecret = sharedSecret
	if in.TargetVPNGatewayRef != nil && in.TargetVPNGatewayRef.External != "" {
		out.TargetVpnGateway = &in.TargetVPNGatewayRef.External
	}
	out.VpnGatewayInterface = direct.PtrInt64ToPtrInt32(in.VpnGatewayInterface)
	if in.VpnGatewayRef != nil && in.VpnGatewayRef.External != "" {
		out.VpnGateway = &in.VpnGatewayRef.External
	}
	out.CipherSuite = VPNTunnelCipherSuite_ToProto(mapCtx, in.CipherSuite)
	out.Labels = in.Labels

	return out
}

// VPNTunnelCipherSuite_ToProto is written here (upstream's types have no
// cipher suite).
func VPNTunnelCipherSuite_ToProto(mapCtx *direct.MapContext, in *krm.VPNTunnelCipherSuite) *pb.VpnTunnelCipherSuite {
	if in == nil {
		return nil
	}
	out := &pb.VpnTunnelCipherSuite{}
	if p := in.Phase1; p != nil {
		out.Phase1 = &pb.VpnTunnelPhase1Algorithms{Dh: p.Dh, Encryption: p.Encryption, Integrity: p.Integrity, Prf: p.Prf}
	}
	if p := in.Phase2; p != nil {
		out.Phase2 = &pb.VpnTunnelPhase2Algorithms{Encryption: p.Encryption, Integrity: p.Integrity, Pfs: p.Pfs}
	}
	return out
}

func ComputeVPNTunnelStatus_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.VpnTunnel) *krm.ComputeVPNTunnelStatus {
	if in == nil {
		return nil
	}
	out := &krm.ComputeVPNTunnelStatus{}
	out.CreationTimestamp = in.CreationTimestamp
	out.DetailedStatus = in.DetailedStatus
	out.LabelFingerprint = in.LabelFingerprint
	out.SelfLink = in.SelfLink
	out.SharedSecretHash = in.SharedSecretHash
	if in.Id != nil {
		idStr := strconv.FormatUint(*in.Id, 10)
		out.TunnelId = &idStr
	}

	return out
}
