// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0
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

package compute

import (
	"testing"

	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
)

// The API lists a hybrid group's endpoints with "instance": "" and refuses
// that field on detach (run cce-11121922-5984): the detach target drops it,
// keeps a real instance, and does not change what was read.
func TestNetworkEndpointDetachTarget(t *testing.T) {
	read := &pb.NetworkEndpoint{IpAddress: direct.PtrTo("10.20.0.5"), Port: direct.PtrTo(int32(8080)), Instance: direct.PtrTo("")}
	got := networkEndpointDetachTarget(read, nil)
	if got.Instance != nil {
		t.Errorf("instance = %q, want unset", got.GetInstance())
	}
	if got.GetIpAddress() != "10.20.0.5" || got.GetPort() != 8080 {
		t.Errorf("target = %v, want the read address and port", got)
	}
	if read.Instance == nil {
		t.Error("the endpoint read was changed")
	}
	vm := &pb.NetworkEndpoint{IpAddress: direct.PtrTo("10.0.0.2"), Instance: direct.PtrTo("vm-1")}
	if got := networkEndpointDetachTarget(nil, vm); got.GetInstance() != "vm-1" {
		t.Errorf("instance = %q, want vm-1 from the desired endpoint", got.GetInstance())
	}
}
