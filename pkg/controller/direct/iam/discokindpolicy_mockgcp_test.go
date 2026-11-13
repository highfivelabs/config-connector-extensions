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

package iam_test

import (
	"slices"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"

	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// The IAM policies of resources whose collections the discokind fake serves
// (pkg/discokind/fake), in front of mockgcp. The fake took
// ".../lakes/l:getIamPolicy" for an object called "l:getIamPolicy" and
// answered 404, so IAMBinding and IAMMember on these kinds never became
// Ready on the fake cloud; the catalog's charts found it. Each row sets a
// policy and reads it back through iampolicy.Client, as the controllers do.
//
// Fake only: the fake cloud's policy store does not need the resource to
// exist, and a live run of these rows would need each service's resources.
// IAMBinding's live run covers the policy protocol (docs/fake-cloud.md).

type policyRow struct{ kind, name string }

func checkPolicyRoundTrip(t *testing.T, rows []policyRow) {
	t.Helper()
	if mockenv.Live() {
		t.Skip("the fake cloud's routing of policy requests; nothing to check live")
	}
	env := mockenv.New(t, namespace)
	policies := &iampolicy.Client{HTTP: env.HTTP}
	member := "serviceAccount:reader@" + mockenv.Project + ".iam.gserviceaccount.com"
	for _, r := range rows {
		t.Run(r.kind, func(t *testing.T) {
			rt, ok := iampolicy.Lookup(r.kind)
			if !ok {
				t.Fatalf("%s has no row in the IAM resource table", r.kind)
			}
			before, err := policies.Get(env.Ctx, rt, r.name)
			if err != nil {
				t.Fatalf("reading %s's policy: %v", r.name, err)
			}
			p := &iampb.Policy{Etag: before.Etag, Bindings: []*iampb.Binding{{Role: "roles/viewer", Members: []string{member}}}}
			if _, err := policies.Set(env.Ctx, rt, r.name, p); err != nil {
				t.Fatalf("setting %s's policy: %v", r.name, err)
			}
			after, err := policies.Get(env.Ctx, rt, r.name)
			if err != nil {
				t.Fatalf("reading %s's policy back: %v", r.name, err)
			}
			if m, _ := iampolicy.Members(after, "roles/viewer", nil); !slices.Contains(m, member) {
				t.Errorf("%s's policy after the write: %v, want %s in roles/viewer", r.name, after.Bindings, member)
			}
		})
	}
}

func TestDiscokindPolicyCloudBuild(t *testing.T) {
	checkPolicyRoundTrip(t, []policyRow{
		{"CloudBuildConnection", "projects/" + mockenv.Project + "/locations/us-central1/connections/c"},
	})
}

func TestDiscokindPolicyAPIGateway(t *testing.T) {
	checkPolicyRoundTrip(t, []policyRow{
		{"APIGatewayAPI", "projects/" + mockenv.Project + "/locations/global/apis/a"},
		{"APIGatewayAPIConfig", "projects/" + mockenv.Project + "/locations/global/apis/a/configs/c"},
		{"APIGatewayGateway", "projects/" + mockenv.Project + "/locations/us-central1/gateways/g"},
	})
}

func TestDiscokindPolicyDataplex(t *testing.T) {
	checkPolicyRoundTrip(t, []policyRow{
		{"DataplexLake", "projects/" + mockenv.Project + "/locations/us-central1/lakes/l"},
		{"DataplexZone", "projects/" + mockenv.Project + "/locations/us-central1/lakes/l/zones/z"},
		{"DataplexAspectType", "projects/" + mockenv.Project + "/locations/us-central1/aspectTypes/a"},
		{"DataplexDataScan", "projects/" + mockenv.Project + "/locations/us-central1/dataScans/s"},
	})
}
