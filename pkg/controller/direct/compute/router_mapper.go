// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// (UPSTREAM.md):
//   - pkg/controller/direct/compute/computerouter_mapper.go: the router, its
//     BGP and advertised ranges; ours adds bgp.identifierRange and the four
//     lists below;
//   - pkg/controller/direct/compute/computerouternat_mapper.go: a NAT, its
//     rules, actions, log config and subnetworks, for our RouterNAT (upstream's
//     ComputeRouterNATSpec without the router, region and resource ID); ours
//     adds name, autoNetworkTier, nat64Subnetworks and
//     sourceSubnetworkIpRangesToNat64;
//   - pkg/controller/direct/compute/computerouterinterface_mapper.go: an
//     interface, for our RouterInterface; privateIpAddress and
//     redundantInterface are the API's strings, and ipVersion is added.
// Written here, from the RouterBgpPeer mapper controllerbuilder generated
// (hack/scaffold-kind.sh): a BGP peer, with Config Connector's field names,
// enable as a boolean (the API's "TRUE" and "FALSE", as Terraform's
// google_compute_router_peer does) and the router appliance as a reference.
// MD5 keys map by name here; the controller adds their values from Secrets.

package compute

import (
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

// ComputeRouterSpec_FromProto maps a pb.Router to a krm.ComputeRouterSpec.
// MD5 keys come back by name only (the API does not return their values).
func ComputeRouterSpec_FromProto(mapCtx *direct.MapContext, in *pb.Router) *krm.ComputeRouterSpec {
	if in == nil {
		return nil
	}
	out := &krm.ComputeRouterSpec{}
	out.BGP = RouterBGP_FromProto(mapCtx, in.GetBgp())
	out.BGPPeers = direct.Slice_FromProto(mapCtx, in.BgpPeers, RouterBGPPeer_FromProto)
	out.Description = in.Description
	out.EncryptedInterconnectRouter = in.EncryptedInterconnectRouter
	out.Interfaces = direct.Slice_FromProto(mapCtx, in.Interfaces, RouterInterface_FromProto)
	for _, k := range in.Md5AuthenticationKeys {
		out.MD5AuthenticationKeys = append(out.MD5AuthenticationKeys, krm.RouterMD5AuthenticationKey{Name: k.GetName()})
	}
	out.NATs = direct.Slice_FromProto(mapCtx, in.Nats, RouterNAT_FromProto)
	if in.GetNetwork() != "" {
		out.NetworkRef = computerefs.ComputeNetworkRef{External: in.GetNetwork()}
	}
	out.Region = direct.ValueOf(in.Region)
	out.ResourceID = in.Name
	return out
}

// ComputeRouterSpec_ToProto maps a krm.ComputeRouterSpec to a pb.Router.
// MD5 keys are mapped by name; their values are the controller's to add.
func ComputeRouterSpec_ToProto(mapCtx *direct.MapContext, in *krm.ComputeRouterSpec) *pb.Router {
	if in == nil {
		return nil
	}
	out := &pb.Router{}
	out.Bgp = RouterBGP_ToProto(mapCtx, in.BGP)
	out.BgpPeers = direct.Slice_ToProto(mapCtx, in.BGPPeers, RouterBGPPeer_ToProto)
	out.Description = in.Description
	out.EncryptedInterconnectRouter = in.EncryptedInterconnectRouter
	out.Interfaces = direct.Slice_ToProto(mapCtx, in.Interfaces, RouterInterface_ToProto)
	for _, k := range in.MD5AuthenticationKeys {
		out.Md5AuthenticationKeys = append(out.Md5AuthenticationKeys, &pb.RouterMd5AuthenticationKey{Name: direct.LazyPtr(k.Name)})
	}
	out.Nats = direct.Slice_ToProto(mapCtx, in.NATs, RouterNAT_ToProto)
	if in.NetworkRef.External != "" {
		out.Network = direct.LazyPtr(in.NetworkRef.External)
	}
	out.Region = direct.LazyPtr(in.Region)
	out.Name = in.ResourceID
	return out
}

// ComputeRouterStatus_FromProto maps the output fields of a pb.Router.
func ComputeRouterStatus_FromProto(mapCtx *direct.MapContext, in *pb.Router) *krm.ComputeRouterStatus {
	if in == nil {
		return nil
	}
	return &krm.ComputeRouterStatus{CreationTimestamp: in.CreationTimestamp, SelfLink: in.SelfLink}
}

// ComputeRouterStatus_ToProto maps the output fields back to a pb.Router.
func ComputeRouterStatus_ToProto(mapCtx *direct.MapContext, in *krm.ComputeRouterStatus) *pb.Router {
	if in == nil {
		return nil
	}
	return &pb.Router{CreationTimestamp: in.CreationTimestamp, SelfLink: in.SelfLink}
}

// RouterBGP_FromProto maps a pb.RouterBgp to a krm.RouterBGP
// This is handcoded because Asn and KeepaliveInterval have different types (uint32 vs int64)
func RouterBGP_FromProto(mapCtx *direct.MapContext, in *pb.RouterBgp) *krm.RouterBGP {
	if in == nil {
		return nil
	}
	out := &krm.RouterBGP{}
	out.AdvertiseMode = in.AdvertiseMode
	out.AdvertisedGroups = in.AdvertisedGroups
	out.AdvertisedIPRanges = direct.Slice_FromProto(mapCtx, in.AdvertisedIpRanges, RouterAdvertisedIPRange_FromProto)
	if in.Asn != nil {
		out.Asn = int64(*in.Asn)
	}
	out.IdentifierRange = in.IdentifierRange
	if in.KeepaliveInterval != nil {
		val := int64(*in.KeepaliveInterval)
		out.KeepaliveInterval = &val
	}
	return out
}

// RouterBGP_ToProto maps a krm.RouterBGP to a pb.RouterBgp
// This is handcoded because Asn and KeepaliveInterval have different types (uint32 vs int64)
func RouterBGP_ToProto(mapCtx *direct.MapContext, in *krm.RouterBGP) *pb.RouterBgp {
	if in == nil {
		return nil
	}
	out := &pb.RouterBgp{}
	out.AdvertiseMode = in.AdvertiseMode
	out.AdvertisedGroups = in.AdvertisedGroups
	out.AdvertisedIpRanges = direct.Slice_ToProto(mapCtx, in.AdvertisedIPRanges, RouterAdvertisedIPRange_ToProto)
	if in.Asn != 0 {
		asn32 := uint32(in.Asn)
		out.Asn = &asn32
	}
	out.IdentifierRange = in.IdentifierRange
	if in.KeepaliveInterval != nil {
		val32 := uint32(*in.KeepaliveInterval)
		out.KeepaliveInterval = &val32
	}
	return out
}

// RouterAdvertisedIPRange_FromProto maps a pb.RouterAdvertisedIpRange to a krm.RouterAdvertisedIPRange
// This is handcoded because Range is a pointer string in proto but a non-pointer in KRM.
func RouterAdvertisedIPRange_FromProto(mapCtx *direct.MapContext, in *pb.RouterAdvertisedIpRange) *krm.RouterAdvertisedIPRange {
	if in == nil {
		return nil
	}
	out := &krm.RouterAdvertisedIPRange{}
	out.Description = in.Description
	out.Range = direct.ValueOf(in.Range)
	return out
}

// RouterAdvertisedIPRange_ToProto maps a krm.RouterAdvertisedIPRange to a pb.RouterAdvertisedIpRange
// This is handcoded because Range is a pointer string in proto but a non-pointer in KRM.
func RouterAdvertisedIPRange_ToProto(mapCtx *direct.MapContext, in *krm.RouterAdvertisedIPRange) *pb.RouterAdvertisedIpRange {
	if in == nil {
		return nil
	}
	out := &pb.RouterAdvertisedIpRange{}
	out.Description = in.Description
	out.Range = direct.LazyPtr(in.Range)
	return out
}

// RouterInterface_FromProto maps a pb.RouterInterface to a krm.RouterInterface.
func RouterInterface_FromProto(mapCtx *direct.MapContext, in *pb.RouterInterface) *krm.RouterInterface {
	if in == nil {
		return nil
	}
	out := &krm.RouterInterface{}
	out.Name = in.GetName()
	out.IPRange = in.IpRange
	out.IPVersion = in.IpVersion
	if in.GetLinkedInterconnectAttachment() != "" {
		out.InterconnectAttachmentRef = &computev1beta1.ComputeInterconnectAttachmentRef{External: in.GetLinkedInterconnectAttachment()}
	}
	if in.GetLinkedVpnTunnel() != "" {
		out.VPNTunnelRef = &computev1beta1.ComputeVPNTunnelRef{External: in.GetLinkedVpnTunnel()}
	}
	out.PrivateIPAddress = in.PrivateIpAddress
	out.RedundantInterface = in.RedundantInterface
	if in.GetSubnetwork() != "" {
		out.SubnetworkRef = &computev1beta1.ComputeSubnetworkRef{External: in.GetSubnetwork()}
	}
	return out
}

// RouterInterface_ToProto maps a krm.RouterInterface to a pb.RouterInterface.
func RouterInterface_ToProto(mapCtx *direct.MapContext, in *krm.RouterInterface) *pb.RouterInterface {
	if in == nil {
		return nil
	}
	out := &pb.RouterInterface{}
	out.Name = direct.LazyPtr(in.Name)
	out.IpRange = in.IPRange
	out.IpVersion = in.IPVersion
	if in.InterconnectAttachmentRef != nil {
		out.LinkedInterconnectAttachment = direct.LazyPtr(in.InterconnectAttachmentRef.External)
	}
	if in.VPNTunnelRef != nil {
		out.LinkedVpnTunnel = direct.LazyPtr(in.VPNTunnelRef.External)
	}
	out.PrivateIpAddress = in.PrivateIPAddress
	out.RedundantInterface = in.RedundantInterface
	if in.SubnetworkRef != nil {
		out.Subnetwork = direct.LazyPtr(in.SubnetworkRef.External)
	}
	return out
}

// RouterBGPPeer_FromProto maps a pb.RouterBgpPeer to a krm.RouterBGPPeer.
func RouterBGPPeer_FromProto(mapCtx *direct.MapContext, in *pb.RouterBgpPeer) *krm.RouterBGPPeer {
	if in == nil {
		return nil
	}
	out := &krm.RouterBGPPeer{}
	out.AdvertiseMode = in.AdvertiseMode
	out.AdvertisedGroups = in.AdvertisedGroups
	out.AdvertisedIPRanges = direct.Slice_FromProto(mapCtx, in.AdvertisedIpRanges, RouterAdvertisedIPRange_FromProto)
	if in.AdvertisedRoutePriority != nil {
		out.AdvertisedRoutePriority = direct.PtrTo(int64(in.GetAdvertisedRoutePriority()))
	}
	out.Bfd = RouterBGPPeerBfd_FromProto(mapCtx, in.GetBfd())
	out.CustomLearnedIPRanges = direct.Slice_FromProto(mapCtx, in.CustomLearnedIpRanges, RouterBGPPeerCustomLearnedIPRange_FromProto)
	if in.CustomLearnedRoutePriority != nil {
		out.CustomLearnedRoutePriority = direct.PtrTo(int64(in.GetCustomLearnedRoutePriority()))
	}
	if in.Enable != nil {
		out.Enable = direct.PtrTo(in.GetEnable() != "FALSE")
	}
	out.EnableIPv4 = in.EnableIpv4
	out.EnableIPv6 = in.EnableIpv6
	out.ExportPolicies = in.ExportPolicies
	out.ImportPolicies = in.ImportPolicies
	out.InterfaceName = in.GetInterfaceName()
	out.IPAddress = in.IpAddress
	out.IPv4NexthopAddress = in.Ipv4NexthopAddress
	out.IPv6NexthopAddress = in.Ipv6NexthopAddress
	out.MD5AuthenticationKeyName = in.Md5AuthenticationKeyName
	out.Name = in.GetName()
	out.PeerAsn = int64(in.GetPeerAsn())
	out.PeerIPAddress = in.PeerIpAddress
	out.PeerIPv4NexthopAddress = in.PeerIpv4NexthopAddress
	out.PeerIPv6NexthopAddress = in.PeerIpv6NexthopAddress
	if in.GetRouterApplianceInstance() != "" {
		out.RouterApplianceInstanceRef = &computev1beta1.InstanceRef{External: in.GetRouterApplianceInstance()}
	}
	return out
}

// RouterBGPPeer_ToProto maps a krm.RouterBGPPeer to a pb.RouterBgpPeer.
func RouterBGPPeer_ToProto(mapCtx *direct.MapContext, in *krm.RouterBGPPeer) *pb.RouterBgpPeer {
	if in == nil {
		return nil
	}
	out := &pb.RouterBgpPeer{}
	out.AdvertiseMode = in.AdvertiseMode
	out.AdvertisedGroups = in.AdvertisedGroups
	out.AdvertisedIpRanges = direct.Slice_ToProto(mapCtx, in.AdvertisedIPRanges, RouterAdvertisedIPRange_ToProto)
	if in.AdvertisedRoutePriority != nil {
		out.AdvertisedRoutePriority = direct.PtrTo(uint32(*in.AdvertisedRoutePriority))
	}
	out.Bfd = RouterBGPPeerBfd_ToProto(mapCtx, in.Bfd)
	out.CustomLearnedIpRanges = direct.Slice_ToProto(mapCtx, in.CustomLearnedIPRanges, RouterBGPPeerCustomLearnedIPRange_ToProto)
	if in.CustomLearnedRoutePriority != nil {
		out.CustomLearnedRoutePriority = direct.PtrTo(int32(*in.CustomLearnedRoutePriority))
	}
	if in.Enable != nil {
		out.Enable = direct.PtrTo("TRUE")
		if !*in.Enable {
			out.Enable = direct.PtrTo("FALSE")
		}
	}
	out.EnableIpv4 = in.EnableIPv4
	out.EnableIpv6 = in.EnableIPv6
	out.ExportPolicies = in.ExportPolicies
	out.ImportPolicies = in.ImportPolicies
	out.InterfaceName = direct.LazyPtr(in.InterfaceName)
	out.IpAddress = in.IPAddress
	out.Ipv4NexthopAddress = in.IPv4NexthopAddress
	out.Ipv6NexthopAddress = in.IPv6NexthopAddress
	out.Md5AuthenticationKeyName = in.MD5AuthenticationKeyName
	out.Name = direct.LazyPtr(in.Name)
	if in.PeerAsn != 0 {
		out.PeerAsn = direct.PtrTo(uint32(in.PeerAsn))
	}
	out.PeerIpAddress = in.PeerIPAddress
	out.PeerIpv4NexthopAddress = in.PeerIPv4NexthopAddress
	out.PeerIpv6NexthopAddress = in.PeerIPv6NexthopAddress
	if in.RouterApplianceInstanceRef != nil {
		out.RouterApplianceInstance = direct.LazyPtr(in.RouterApplianceInstanceRef.External)
	}
	return out
}

// RouternatLogConfig_FromProto maps a pb.RouterNatLogConfig to a krm.RouternatLogConfig
func RouternatLogConfig_FromProto(mapCtx *direct.MapContext, in *pb.RouterNatLogConfig) *computev1beta1.RouternatLogConfig {
	if in == nil {
		return nil
	}
	out := &computev1beta1.RouternatLogConfig{}
	out.Enable = direct.ValueOf(in.Enable)
	out.Filter = direct.ValueOf(in.Filter)
	return out
}

// RouternatLogConfig_ToProto maps a krm.RouternatLogConfig to a pb.RouterNatLogConfig
func RouternatLogConfig_ToProto(mapCtx *direct.MapContext, in *computev1beta1.RouternatLogConfig) *pb.RouterNatLogConfig {
	if in == nil {
		return nil
	}
	out := &pb.RouterNatLogConfig{}
	out.Enable = &in.Enable
	if in.Filter != "" {
		out.Filter = direct.LazyPtr(in.Filter)
	}
	return out
}

// RouternatAction_FromProto maps a pb.RouterNatRuleAction to a krm.RouternatAction
func RouternatAction_FromProto(mapCtx *direct.MapContext, in *pb.RouterNatRuleAction) *computev1beta1.RouternatAction {
	if in == nil {
		return nil
	}
	out := &computev1beta1.RouternatAction{}
	for _, ip := range in.SourceNatActiveIps {
		out.SourceNatActiveIpsRefs = append(out.SourceNatActiveIpsRefs, computev1beta1.ComputeAddressRef{External: ip})
	}
	for _, ip := range in.SourceNatDrainIps {
		out.SourceNatDrainIpsRefs = append(out.SourceNatDrainIpsRefs, computev1beta1.ComputeAddressRef{External: ip})
	}
	for _, r := range in.SourceNatActiveRanges {
		out.SourceNatActiveRangesRefs = append(out.SourceNatActiveRangesRefs, computev1beta1.ComputeSubnetworkRef{External: r})
	}
	for _, r := range in.SourceNatDrainRanges {
		out.SourceNatDrainRangesRefs = append(out.SourceNatDrainRangesRefs, computev1beta1.ComputeSubnetworkRef{External: r})
	}
	return out
}

// RouternatAction_ToProto maps a krm.RouternatAction to a pb.RouterNatRuleAction
func RouternatAction_ToProto(mapCtx *direct.MapContext, in *computev1beta1.RouternatAction) *pb.RouterNatRuleAction {
	if in == nil {
		return nil
	}
	out := &pb.RouterNatRuleAction{}
	for _, ref := range in.SourceNatActiveIpsRefs {
		if ref.External != "" {
			out.SourceNatActiveIps = append(out.SourceNatActiveIps, ref.External)
		}
	}
	for _, ref := range in.SourceNatDrainIpsRefs {
		if ref.External != "" {
			out.SourceNatDrainIps = append(out.SourceNatDrainIps, ref.External)
		}
	}
	for _, ref := range in.SourceNatActiveRangesRefs {
		if ref.External != "" {
			out.SourceNatActiveRanges = append(out.SourceNatActiveRanges, ref.External)
		}
	}
	for _, ref := range in.SourceNatDrainRangesRefs {
		if ref.External != "" {
			out.SourceNatDrainRanges = append(out.SourceNatDrainRanges, ref.External)
		}
	}
	return out
}

// RouternatRules_FromProto maps a pb.RouterNatRule to a krm.RouternatRules
func RouternatRules_FromProto(mapCtx *direct.MapContext, in *pb.RouterNatRule) *computev1beta1.RouternatRules {
	if in == nil {
		return nil
	}
	out := &computev1beta1.RouternatRules{}
	out.Action = RouternatAction_FromProto(mapCtx, in.Action)
	out.Description = in.Description
	out.Match = direct.ValueOf(in.Match)
	if in.RuleNumber != nil {
		out.RuleNumber = int64(*in.RuleNumber)
	}
	return out
}

// RouternatRules_ToProto maps a krm.RouternatRules to a pb.RouterNatRule
func RouternatRules_ToProto(mapCtx *direct.MapContext, in *computev1beta1.RouternatRules) *pb.RouterNatRule {
	if in == nil {
		return nil
	}
	out := &pb.RouterNatRule{}
	out.Action = RouternatAction_ToProto(mapCtx, in.Action)
	out.Description = in.Description
	if in.Match != "" {
		out.Match = direct.LazyPtr(in.Match)
	}
	if in.RuleNumber != 0 {
		val32 := uint32(in.RuleNumber)
		out.RuleNumber = &val32
	}
	return out
}

// RouternatSubnetwork_FromProto maps a pb.RouterNatSubnetworkToNat to a krm.RouternatSubnetwork
func RouternatSubnetwork_FromProto(mapCtx *direct.MapContext, in *pb.RouterNatSubnetworkToNat) *computev1beta1.RouternatSubnetwork {
	if in == nil {
		return nil
	}
	out := &computev1beta1.RouternatSubnetwork{}
	out.SecondaryIpRangeNames = in.SecondaryIpRangeNames
	out.SourceIpRangesToNat = in.SourceIpRangesToNat
	if in.GetName() != "" {
		out.SubnetworkRef = computev1beta1.ComputeSubnetworkRef{External: in.GetName()}
	}
	return out
}

// RouternatSubnetwork_ToProto maps a krm.RouternatSubnetwork to a pb.RouterNatSubnetworkToNat
func RouternatSubnetwork_ToProto(mapCtx *direct.MapContext, in *computev1beta1.RouternatSubnetwork) *pb.RouterNatSubnetworkToNat {
	if in == nil {
		return nil
	}
	out := &pb.RouterNatSubnetworkToNat{}
	out.SecondaryIpRangeNames = in.SecondaryIpRangeNames
	out.SourceIpRangesToNat = in.SourceIpRangesToNat
	if in.SubnetworkRef.External != "" {
		out.Name = direct.LazyPtr(in.SubnetworkRef.External)
	}
	return out
}

// RouterNAT_FromProto maps a pb.RouterNat to a krm.RouterNAT
func RouterNAT_FromProto(mapCtx *direct.MapContext, in *pb.RouterNat) *krm.RouterNAT {
	if in == nil {
		return nil
	}
	out := &krm.RouterNAT{}
	out.Name = in.GetName()
	out.AutoNetworkTier = in.AutoNetworkTier
	for _, ip := range in.DrainNatIps {
		out.DrainNatIps = append(out.DrainNatIps, computev1beta1.ComputeAddressRef{External: ip})
	}
	out.EnableDynamicPortAllocation = in.EnableDynamicPortAllocation
	out.EnableEndpointIndependentMapping = in.EnableEndpointIndependentMapping
	if in.IcmpIdleTimeoutSec != nil {
		val := int64(*in.IcmpIdleTimeoutSec)
		out.IcmpIdleTimeoutSec = &val
	}
	out.LogConfig = RouternatLogConfig_FromProto(mapCtx, in.LogConfig)
	if in.MaxPortsPerVm != nil {
		val := int64(*in.MaxPortsPerVm)
		out.MaxPortsPerVm = &val
	}
	if in.MinPortsPerVm != nil {
		val := int64(*in.MinPortsPerVm)
		out.MinPortsPerVm = &val
	}
	out.Nat64Subnetworks = direct.Slice_FromProto(mapCtx, in.Nat64Subnetworks, RouterNATSubnetworkToNat64_FromProto)
	out.NatIpAllocateOption = in.NatIpAllocateOption
	for _, ip := range in.NatIps {
		out.NatIps = append(out.NatIps, computev1beta1.ComputeAddressRef{External: ip})
	}
	out.Rules = direct.Slice_FromProto(mapCtx, in.Rules, RouternatRules_FromProto)
	out.SourceSubnetworkIpRangesToNat = direct.ValueOf(in.SourceSubnetworkIpRangesToNat)
	out.SourceSubnetworkIpRangesToNat64 = in.SourceSubnetworkIpRangesToNat64
	out.Subnetwork = direct.Slice_FromProto(mapCtx, in.Subnetworks, RouternatSubnetwork_FromProto)
	if in.TcpEstablishedIdleTimeoutSec != nil {
		val := int64(*in.TcpEstablishedIdleTimeoutSec)
		out.TcpEstablishedIdleTimeoutSec = &val
	}
	if in.TcpTimeWaitTimeoutSec != nil {
		val := int64(*in.TcpTimeWaitTimeoutSec)
		out.TcpTimeWaitTimeoutSec = &val
	}
	if in.TcpTransitoryIdleTimeoutSec != nil {
		val := int64(*in.TcpTransitoryIdleTimeoutSec)
		out.TcpTransitoryIdleTimeoutSec = &val
	}
	if in.UdpIdleTimeoutSec != nil {
		val := int64(*in.UdpIdleTimeoutSec)
		out.UdpIdleTimeoutSec = &val
	}
	out.Type = in.Type
	out.EndpointTypes = in.EndpointTypes
	return out
}

// RouterNAT_ToProto maps a krm.RouterNAT to a pb.RouterNat
func RouterNAT_ToProto(mapCtx *direct.MapContext, in *krm.RouterNAT) *pb.RouterNat {
	if in == nil {
		return nil
	}
	out := &pb.RouterNat{}
	out.Name = direct.LazyPtr(in.Name)
	out.AutoNetworkTier = in.AutoNetworkTier
	for _, ref := range in.DrainNatIps {
		if ref.External != "" {
			out.DrainNatIps = append(out.DrainNatIps, ref.External)
		}
	}
	out.EnableDynamicPortAllocation = in.EnableDynamicPortAllocation
	out.EnableEndpointIndependentMapping = in.EnableEndpointIndependentMapping
	if in.IcmpIdleTimeoutSec != nil {
		val32 := int32(*in.IcmpIdleTimeoutSec)
		out.IcmpIdleTimeoutSec = &val32
	}
	out.LogConfig = RouternatLogConfig_ToProto(mapCtx, in.LogConfig)
	if in.MaxPortsPerVm != nil {
		val32 := int32(*in.MaxPortsPerVm)
		out.MaxPortsPerVm = &val32
	}
	if in.MinPortsPerVm != nil {
		val32 := int32(*in.MinPortsPerVm)
		out.MinPortsPerVm = &val32
	}
	out.Nat64Subnetworks = direct.Slice_ToProto(mapCtx, in.Nat64Subnetworks, RouterNATSubnetworkToNat64_ToProto)
	out.NatIpAllocateOption = in.NatIpAllocateOption
	for _, ref := range in.NatIps {
		if ref.External != "" {
			out.NatIps = append(out.NatIps, ref.External)
		}
	}
	out.Rules = direct.Slice_ToProto(mapCtx, in.Rules, RouternatRules_ToProto)
	if in.SourceSubnetworkIpRangesToNat != "" {
		out.SourceSubnetworkIpRangesToNat = direct.LazyPtr(in.SourceSubnetworkIpRangesToNat)
	}
	out.SourceSubnetworkIpRangesToNat64 = in.SourceSubnetworkIpRangesToNat64
	out.Subnetworks = direct.Slice_ToProto(mapCtx, in.Subnetwork, RouternatSubnetwork_ToProto)
	if in.TcpEstablishedIdleTimeoutSec != nil {
		val32 := int32(*in.TcpEstablishedIdleTimeoutSec)
		out.TcpEstablishedIdleTimeoutSec = &val32
	}
	if in.TcpTimeWaitTimeoutSec != nil {
		val32 := int32(*in.TcpTimeWaitTimeoutSec)
		out.TcpTimeWaitTimeoutSec = &val32
	}
	if in.TcpTransitoryIdleTimeoutSec != nil {
		val32 := int32(*in.TcpTransitoryIdleTimeoutSec)
		out.TcpTransitoryIdleTimeoutSec = &val32
	}
	if in.UdpIdleTimeoutSec != nil {
		val32 := int32(*in.UdpIdleTimeoutSec)
		out.UdpIdleTimeoutSec = &val32
	}
	out.Type = in.Type
	out.EndpointTypes = in.EndpointTypes
	return out
}
