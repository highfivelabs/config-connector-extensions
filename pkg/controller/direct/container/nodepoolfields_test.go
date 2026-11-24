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

package container

import (
	"testing"

	pb "cloud.google.com/go/container/apiv1/containerpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	krm "github.com/highfivelabs/config-connector-extensions/apis/container/v1beta1"
)

func ptr[T any](v T) *T { return &v }

// TestApplyEvictionAndManagers: apply sets the eviction and manager
// settings whole, leaves every field the spec does not own, clears only
// its own on delete, and takes an empty message the API answers for an
// unset one as the same.
func TestApplyEvictionAndManagers(t *testing.T) {
	d := &krm.NodePoolKubeletConfig{
		EvictionSoft:                     &krm.NodePoolEvictionSignals{MemoryAvailable: ptr("200Mi")},
		EvictionSoftGracePeriod:          &krm.NodePoolEvictionSignals{MemoryAvailable: ptr("30s")},
		EvictionMinimumReclaim:           &krm.NodePoolEvictionSignals{PidAvailable: ptr("5%")},
		EvictionMaxPodGracePeriodSeconds: ptr(int32(60)),
		MemoryManager:                    &krm.NodePoolMemoryManager{Policy: ptr("Static")},
		TopologyManager:                  &krm.NodePoolTopologyManager{Policy: ptr("single-numa-node"), Scope: ptr("pod")},
		SingleProcessOomKill:             ptr(false),
	}
	// What the node pool holds: Config Connector's fields, and an
	// eviction signal this object does not set, which it owns whole.
	actual := &pb.NodeKubeletConfig{
		CpuCfsQuota:  wrapperspb.Bool(true),
		PodPidsLimit: 4096,
		EvictionSoft: &pb.EvictionSignals{MemoryAvailable: "100Mi", NodefsAvailable: "10%"},
	}
	out, changed := apply(actual, d, false)
	if !changed {
		t.Fatal("apply reports no change")
	}
	want := &pb.NodeKubeletConfig{
		CpuCfsQuota:                      wrapperspb.Bool(true),
		PodPidsLimit:                     4096,
		EvictionSoft:                     &pb.EvictionSignals{MemoryAvailable: "200Mi"},
		EvictionSoftGracePeriod:          &pb.EvictionGracePeriod{MemoryAvailable: "30s"},
		EvictionMinimumReclaim:           &pb.EvictionMinimumReclaim{PidAvailable: "5%"},
		EvictionMaxPodGracePeriodSeconds: 60,
		MemoryManager:                    &pb.MemoryManager{Policy: "Static"},
		TopologyManager:                  &pb.TopologyManager{Policy: "single-numa-node", Scope: "pod"},
		SingleProcessOomKill:             ptr(false),
	}
	if !proto.Equal(out, want) {
		t.Errorf("apply = %v, want %v", out, want)
	}
	if _, changed := apply(out, d, false); changed {
		t.Error("apply on what it wrote reports a change")
	}

	cleared, changed := apply(out, d, true)
	if !changed {
		t.Fatal("clearing reports no change")
	}
	if !proto.Equal(cleared, &pb.NodeKubeletConfig{CpuCfsQuota: wrapperspb.Bool(true), PodPidsLimit: 4096}) {
		t.Errorf("cleared = %v, want only Config Connector's fields", cleared)
	}

	// The API may answer a cleared message as an empty one.
	empty := proto.Clone(cleared).(*pb.NodeKubeletConfig)
	empty.EvictionSoft, empty.MemoryManager, empty.TopologyManager = &pb.EvictionSignals{}, &pb.MemoryManager{}, &pb.TopologyManager{}
	if _, changed := apply(empty, d, true); changed {
		t.Error("an empty message is taken for a set one")
	}
}
