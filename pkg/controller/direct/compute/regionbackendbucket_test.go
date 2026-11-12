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

package compute_test

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

func init() { kindtest.RegisterSetup("ComputeRegionBackendBucket", backendBucketBuckets) }

// backendBucketBuckets makes the Cloud Storage buckets the
// ComputeRegionBackendBucket fixtures name (${NAME}-bucket, for the test's
// object and for its abandoned one): the API refuses a backend bucket
// whose bucket does not exist. Live only: the fake does not check. The
// buckets are regional, in the backend bucket's region, empty, labelled
// with the run, deleted after the test's own resources (t.Cleanup runs
// last-in first-out), and swept by name (hack/live/sweep.sh).
func backendBucketBuckets(t *testing.T, env *mockenv.Env) {
	if !mockenv.Live() {
		return
	}
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://storage.googleapis.com/storage/v1/"}
	labels := map[string]any{}
	for k, v := range mockenv.Labels() {
		labels[k] = v
	}
	for _, base := range []string{"computeregionbackendbucket", "computeregionbackendbucket-kept"} {
		name := mockenv.Name(base) + "-bucket"
		_, err := api.Do(env.Ctx, http.MethodPost, "b", map[string]string{"project": mockenv.Project}, map[string]any{
			"name":     name,
			"location": "US-CENTRAL1",
			"labels":   labels,
			"iamConfiguration": map[string]any{
				"uniformBucketLevelAccess": map[string]any{"enabled": true},
			},
		})
		if err != nil {
			t.Fatalf("creating the bucket %s: %v", name, err)
		}
		t.Logf("created the bucket %s", name)
		t.Cleanup(func() {
			// env.Ctx may be done by now: the test's own context.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if _, err := api.Do(ctx, http.MethodDelete, "b/"+name, nil, nil); err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting the bucket %s: %v", name, err)
			}
		})
	}
}
