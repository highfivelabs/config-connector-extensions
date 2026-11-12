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

package secretmanager_test

// A regional secret moves from one version slot to another (pkg/test/slotmove),
// on the fake cloud or, with task test-live KINDS=secretmanager, a real
// project. A secret has labels, so the slot guard marks and checks it.

import (
	"testing"

	pb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	smv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/secretmanager/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/slotmove"
)

func TestSlotMoveSecretManagerSecret(t *testing.T) {
	env := mockenv.NewSlots(t, slotmove.Old, slotmove.New)
	cloud := regionalClient(t, env, region)
	name := mockenv.Name("slot-move")
	ref := "projects/" + mockenv.Project + "/locations/" + region + "/secrets/" + name

	slotmove.Run(t, env, slotmove.Case{
		GVK:  smv1beta1.SecretManagerSecretGVK,
		Name: name,
		Object: func(ns string) client.Object {
			labels := mockenv.Labels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels["app"] = "payments"
			return &smv1beta1.SecretManagerSecret{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Annotations: map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project}, Labels: labels},
				Spec:       smv1beta1.SecretManagerSecretSpec{Location: mockenv.Ptr(region)},
			}
		},
		Read: func(t *testing.T) (slotmove.Cloud, bool) {
			t.Helper()
			got, err := cloud.GetSecret(env.Ctx, &pb.GetSecretRequest{Name: ref})
			if direct.IsNotFound(err) {
				return slotmove.Cloud{}, false
			}
			if err != nil {
				t.Fatalf("reading %s: %v", ref, err)
			}
			rest := proto.Clone(got).(*pb.Secret)
			rest.Labels, rest.Etag = nil, ""
			return slotmove.Cloud{
				ID:      got.GetName(),
				Created: got.GetCreateTime().AsTime().String(),
				Labels:  got.GetLabels(),
				Fields:  protojson.Format(rest),
			}, true
		},
		DeleteCloud: func(t *testing.T) {
			if err := cloud.DeleteSecret(env.Ctx, &pb.DeleteSecretRequest{Name: ref}); err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting %s: %v", ref, err)
			}
		},
		Labeled: true,
	})
}
