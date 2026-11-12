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

package networksecurity_test

// A project's address group moves from one version slot to another
// (pkg/test/slotmove). Its labels are in spec.labels, so the slot guard
// adds its label there.

import (
	"testing"

	gcp "cloud.google.com/go/networksecurity/apiv1"
	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/slotmove"
)

func TestSlotMoveNetworkSecurityAddressGroup(t *testing.T) {
	env := mockenv.NewSlots(t, slotmove.Old, slotmove.New)
	projects, err := gcp.NewAddressGroupRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { projects.Close() })
	const name = "slot-move"
	ref := "projects/" + mockenv.Project + "/locations/" + location + "/addressGroups/" + mockenv.Name(name)

	slotmove.Run(t, env, slotmove.Case{
		GVK:  gvk,
		Name: name,
		Object: func(ns string) client.Object {
			o := newProjectAddressGroup(name, "10.0.0.0/8")
			o.Namespace = ns
			return o
		},
		Read: func(t *testing.T) (slotmove.Cloud, bool) {
			t.Helper()
			got, err := projects.GetAddressGroup(env.Ctx, &pb.GetAddressGroupRequest{Name: ref})
			if direct.IsNotFound(err) {
				return slotmove.Cloud{}, false
			}
			if err != nil {
				t.Fatalf("reading %s: %v", ref, err)
			}
			rest := proto.Clone(got).(*pb.AddressGroup)
			rest.Labels, rest.UpdateTime = nil, nil
			return slotmove.Cloud{
				ID:      got.GetName(),
				Created: got.GetCreateTime().AsTime().String(),
				Labels:  got.GetLabels(),
				Fields:  protojson.Format(rest),
			}, true
		},
		DeleteCloud: func(t *testing.T) {
			op, err := projects.DeleteAddressGroup(env.Ctx, &pb.DeleteAddressGroupRequest{Name: ref})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting %s: %v", ref, err)
			}
		},
		Labeled: true,
	})
}
