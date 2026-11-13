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
// pkg/controller/direct/compute/computeregionperinstanceconfig_mapper.go and the spec
// functions in mapper.generated.go (UPSTREAM.md). Changed: our KRM types
// (stable, compute.gcp.hfl.dev/v1beta1, docs/design.md decision 26);
// v1beta1 in the functions' names.

package compute

import (
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
)

func ComputeRegionPerInstanceConfigSpec_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.PerInstanceConfig) *krm.ComputeRegionPerInstanceConfigSpec {
	if in == nil {
		return nil
	}
	out := &krm.ComputeRegionPerInstanceConfigSpec{}
	// MISSING: Fingerprint
	// MISSING: Name
	out.PreservedState = RegionperinstanceconfigPreservedState_v1beta1_FromProto(mapCtx, in.GetPreservedState())
	// MISSING: Status
	return out
}
func ComputeRegionPerInstanceConfigSpec_v1beta1_ToProto(mapCtx *direct.MapContext, in *krm.ComputeRegionPerInstanceConfigSpec) *pb.PerInstanceConfig {
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

func RegionperinstanceconfigPreservedState_v1beta1_FromProto(mapCtx *direct.MapContext, in *pb.PreservedState) *krm.RegionperinstanceconfigPreservedState {
	if in == nil {
		return nil
	}
	out := &krm.RegionperinstanceconfigPreservedState{}

	if in.Disks != nil {
		out.Disk = make([]krm.RegionperinstanceconfigDisk, 0, len(in.Disks))
		for k, v := range in.Disks {
			disk := krm.RegionperinstanceconfigDisk{
				DeviceName: k,
			}
			if v != nil {
				disk.DeleteRule = v.AutoDelete
				disk.Mode = v.Mode
				if v.Source != nil {
					disk.Source = *v.Source
				}
			}
			out.Disk = append(out.Disk, disk)
		}
	}

	if in.ExternalIPs != nil {
		out.ExternalIp = make([]krm.RegionperinstanceconfigExternalIp, 0, len(in.ExternalIPs))
		for k, v := range in.ExternalIPs {
			ip := krm.RegionperinstanceconfigExternalIp{
				InterfaceName: k,
			}
			if v != nil {
				ip.AutoDelete = v.AutoDelete
				if v.IpAddress != nil {
					ip.IpAddress = &krm.RegionperinstanceconfigIpAddress{
						Address: v.IpAddress.Address,
					}
				}
			}
			out.ExternalIp = append(out.ExternalIp, ip)
		}
	}

	if in.InternalIPs != nil {
		out.InternalIp = make([]krm.RegionperinstanceconfigInternalIp, 0, len(in.InternalIPs))
		for k, v := range in.InternalIPs {
			ip := krm.RegionperinstanceconfigInternalIp{
				InterfaceName: k,
			}
			if v != nil {
				ip.AutoDelete = v.AutoDelete
				if v.IpAddress != nil {
					ip.IpAddress = &krm.RegionperinstanceconfigIpAddress{
						Address: v.IpAddress.Address,
					}
				}
			}
			out.InternalIp = append(out.InternalIp, ip)
		}
	}

	out.Metadata = in.Metadata

	return out
}

func RegionperinstanceconfigPreservedState_v1beta1_ToProto(mapCtx *direct.MapContext, in *krm.RegionperinstanceconfigPreservedState) *pb.PreservedState {
	if in == nil {
		return nil
	}
	out := &pb.PreservedState{}

	if len(in.Disk) > 0 {
		out.Disks = make(map[string]*pb.PreservedStatePreservedDisk)
		for _, disk := range in.Disk {
			pd := &pb.PreservedStatePreservedDisk{
				AutoDelete: disk.DeleteRule,
				Mode:       disk.Mode,
			}
			if disk.Source != "" {
				pd.Source = &disk.Source
			}
			out.Disks[disk.DeviceName] = pd
		}
	}

	if len(in.ExternalIp) > 0 {
		out.ExternalIPs = make(map[string]*pb.PreservedStatePreservedNetworkIp)
		for _, ip := range in.ExternalIp {
			pni := &pb.PreservedStatePreservedNetworkIp{
				AutoDelete: ip.AutoDelete,
			}
			if ip.IpAddress != nil && ip.IpAddress.Address != nil {
				pni.IpAddress = &pb.PreservedStatePreservedNetworkIpIpAddress{
					Address: ip.IpAddress.Address,
				}
			}
			out.ExternalIPs[ip.InterfaceName] = pni
		}
	}

	if len(in.InternalIp) > 0 {
		out.InternalIPs = make(map[string]*pb.PreservedStatePreservedNetworkIp)
		for _, ip := range in.InternalIp {
			pni := &pb.PreservedStatePreservedNetworkIp{
				AutoDelete: ip.AutoDelete,
			}
			if ip.IpAddress != nil && ip.IpAddress.Address != nil {
				pni.IpAddress = &pb.PreservedStatePreservedNetworkIpIpAddress{
					Address: ip.IpAddress.Address,
				}
			}
			out.InternalIPs[ip.InterfaceName] = pni
		}
	}

	out.Metadata = in.Metadata

	return out
}
