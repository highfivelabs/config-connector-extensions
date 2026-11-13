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

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func init() {
	kindtest.RegisterSetup("NetworkSecurityGatewaySecurityPolicyRule", gatewaySecurityPolicy)
}

// gatewaySecurityPolicy makes the policy a rule's fixtures name
// (${NAME}-gsp, global), since the API refuses a rule whose policy does not
// exist. Live only: the fake does not check. It is deleted after the
// test's own rule (t.Cleanup runs last-in first-out); the sweeper deletes
// a run's rules before its policies (hack/live/sweep.sh).
func gatewaySecurityPolicy(t *testing.T, env *mockenv.Env) {
	if !mockenv.Live() {
		return
	}
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://networksecurity.googleapis.com/v1/"}
	parent := "projects/" + mockenv.Project + "/locations/global"
	id := mockenv.Name("networksecuritygatewaysecuritypolicyrule") + "-gsp"
	name := parent + "/gatewaySecurityPolicies/" + id
	op, err := api.Do(env.Ctx, http.MethodPost, parent+"/gatewaySecurityPolicies", map[string]string{"gatewaySecurityPolicyId": id}, map[string]any{
		"description": "the policy of a config-connector-extensions rule test",
	})
	if err == nil {
		_, err = api.Wait(env.Ctx, op)
	}
	if err != nil {
		t.Fatalf("creating the gateway security policy %s: %v", name, err)
	}
	t.Logf("created the gateway security policy %s", name)
	t.Cleanup(func() {
		// env.Ctx may be done by now: the test's own context.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		op, err := api.Do(ctx, http.MethodDelete, name, nil, nil)
		if err == nil {
			_, err = api.Wait(ctx, op)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("cleanup: deleting the gateway security policy %s: %v", name, err)
		}
	})
}
