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
// pkg/controller/direct/apigee/nataddress_mappings.go (UPSTREAM.md). Changed:
// our KRM types (stable, apigee.gcp.hfl.dev/v1beta1, docs/design.md decision 26).

package apigee

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	krm "github.com/highfivelabs/config-connector-extensions/apis/apigee/v1beta1"

	api "google.golang.org/api/apigee/v1"
)

func ApigeeNATAddressSpec_FromAPI(mapCtx *direct.MapContext, in *api.GoogleCloudApigeeV1NatAddress) *krm.ApigeeNATAddressSpec {
	if in == nil {
		return nil
	}
	out := &krm.ApigeeNATAddressSpec{}
	return out
}

func ApigeeNATAddressSpec_ToAPI(mapCtx *direct.MapContext, in *krm.ApigeeNATAddressSpec) *api.GoogleCloudApigeeV1NatAddress {
	if in == nil {
		return nil
	}
	out := &api.GoogleCloudApigeeV1NatAddress{}
	return out
}

func ApigeeNATAddressStatus_FromAPI(mapCtx *direct.MapContext, in *api.GoogleCloudApigeeV1NatAddress) *krm.ApigeeNATAddressStatus {
	if in == nil {
		return nil
	}
	out := &krm.ApigeeNATAddressStatus{}
	out.IpAddress = direct.LazyPtr(in.IpAddress)
	out.State = direct.LazyPtr(in.State)
	return out
}

func ApigeeNATAddressStatus_ToAPI(mapCtx *direct.MapContext, in *krm.ApigeeNATAddressStatus) *api.GoogleCloudApigeeV1NatAddress {
	if in == nil {
		return nil
	}
	out := &api.GoogleCloudApigeeV1NatAddress{}
	out.IpAddress = direct.ValueOf(in.IpAddress)
	out.State = direct.ValueOf(in.State)
	return out
}
