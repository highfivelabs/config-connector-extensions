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

package logging_test

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

func init() { kindtest.RegisterSetup("LoggingLink", linkLogBuckets) }

// linkLogBuckets makes the log buckets the LoggingLink fixtures name
// (${NAME}-bucket, for the test's object and for its abandoned one), with
// Log Analytics on: a link is a BigQuery dataset of such a bucket, and the
// API refuses one without it. Live only: the fake does not check. The
// buckets are global, in the run's project, labelled with nothing (log
// buckets take no labels) but named with the run, deleted after the
// test's own links (t.Cleanup runs last-in first-out), and swept by name
// with their links (hack/live/sweep.sh). A deleted log bucket is kept for
// seven days in DELETE_REQUESTED and its ID is not reused; the run ID in
// the name keeps runs apart.
func linkLogBuckets(t *testing.T, env *mockenv.Env) {
	if !mockenv.Live() {
		return
	}
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://logging.googleapis.com/v2/"}
	parent := "projects/" + mockenv.Project + "/locations/global"
	for _, base := range []string{"logginglink", "logginglink-kept"} {
		id := mockenv.Name(base) + "-bucket"
		name := parent + "/buckets/" + id
		_, err := api.Do(env.Ctx, http.MethodPost, parent+"/buckets", map[string]string{"bucketId": id}, map[string]any{
			"description":      "made by a config-connector-extensions test, run " + mockenv.RunID,
			"analyticsEnabled": true,
			"retentionDays":    1,
		})
		if err != nil {
			t.Fatalf("creating the log bucket %s: %v", name, err)
		}
		t.Logf("created the log bucket %s", name)
		t.Cleanup(func() {
			// env.Ctx may be done by now: the test's own context.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if _, err := api.Do(ctx, http.MethodDelete, name, nil, nil); err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting the log bucket %s: %v", name, err)
			}
		})
	}
}
