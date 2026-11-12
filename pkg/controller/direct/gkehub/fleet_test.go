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

package gkehub_test

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

func init() { kindtest.RegisterSetup("GKEHubRolloutSequence", fleet) }

// fleet makes sure the run's project has a fleet: a rollout sequence's
// stages name fleet host projects, and the API refuses one whose project
// has none (404 "fleet not found", run cce-11112128-d3db). Live only: the
// fake does not check. A fleet is free and a project has at most one
// (projects/P/locations/global/fleets/default); one made here carries the
// run's labels, is deleted after the test's own resources (t.Cleanup runs
// last-in first-out), and is swept by label (hack/live/sweep.sh). A fleet
// the project already had is used and left alone.
func fleet(t *testing.T, env *mockenv.Env) {
	if !mockenv.Live() {
		return
	}
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://gkehub.googleapis.com/v1/"}
	parent := "projects/" + mockenv.Project + "/locations/global"
	name := parent + "/fleets/default"
	_, err := api.Get(env.Ctx, name)
	if err == nil {
		t.Logf("using the project's fleet %s", name)
		return
	}
	if !direct.IsNotFound(err) {
		t.Fatalf("reading %s: %v", name, err)
	}
	labels := map[string]any{}
	for k, v := range mockenv.Labels() {
		labels[k] = v
	}
	op, err := api.Do(env.Ctx, http.MethodPost, parent+"/fleets", nil, map[string]any{
		"displayName": mockenv.RunID,
		"labels":      labels,
	})
	if err == nil {
		_, err = api.Wait(env.Ctx, op)
	}
	if err != nil {
		t.Fatalf("creating the fleet %s: %v", name, err)
	}
	t.Logf("created the fleet %s", name)
	t.Cleanup(func() {
		// env.Ctx may be done by now: the test's own context.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		op, err := api.Do(ctx, http.MethodDelete, name, nil, nil)
		if err == nil {
			_, err = api.Wait(ctx, op)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("cleanup: deleting the fleet %s: %v", name, err)
		}
	})
}
