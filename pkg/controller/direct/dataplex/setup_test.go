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

package dataplex_test

// What a DataplexZone and a DataplexAsset need live: a zone its lake, an
// asset its lake, zone and Cloud Storage bucket (the fixtures' ${NAME}-lake,
// ${NAME}-zone and ${NAME}-bucket), for the test's object and its
// abandoned one. The fake does not check a parent, so nothing is made
// there. Everything is deleted when the test ends, children first. Every
// call here may be refused for quota (Dataplex: 10 requests a minute per
// user and region, shared with the controller): kindtest.RetryQuota.

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

const region = "us-central1"

func init() {
	kindtest.RegisterSetup("DataplexZone", func(t *testing.T, env *mockenv.Env) { makeParents(t, env, "dataplexzone", false) })
	kindtest.RegisterSetup("DataplexAsset", func(t *testing.T, env *mockenv.Env) { makeParents(t, env, "dataplexasset", true) })
}

func makeParents(t *testing.T, env *mockenv.Env, base string, forAsset bool) {
	if !mockenv.Live() {
		return
	}
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://dataplex.googleapis.com/v1/"}
	gcs := &discokind.API{HTTP: env.HTTP, BaseURL: "https://storage.googleapis.com/storage/v1/"}
	parent := "projects/" + mockenv.Project + "/locations/" + region
	labels := map[string]any{}
	for k, v := range mockenv.Labels() {
		labels[k] = v
	}
	for _, b := range []string{base, base + "-kept"} {
		n := mockenv.Name(b)
		lake := parent + "/lakes/" + n + "-lake"
		make1(t, env, api, parent+"/lakes", "lakeId", n+"-lake", map[string]any{
			"description": "made by a config-connector-extensions test, run " + mockenv.RunID,
			"labels":      labels,
		})
		if !forAsset {
			continue
		}
		make1(t, env, api, lake+"/zones", "zoneId", n+"-zone", map[string]any{
			"type":          "RAW",
			"resourceSpec":  map[string]any{"locationType": "SINGLE_REGION"},
			"discoverySpec": map[string]any{"enabled": false},
			"labels":        labels,
		})
		bucket := n + "-bucket"
		if err := kindtest.RetryQuota(func() error {
			_, err := gcs.Do(env.Ctx, http.MethodPost, "b", map[string]string{"project": mockenv.Project}, map[string]any{
				"name": bucket, "location": region, "labels": labels,
				"iamConfiguration": map[string]any{"uniformBucketLevelAccess": map[string]any{"enabled": true}},
			})
			return err
		}); err != nil {
			t.Fatalf("creating the bucket %s: %v", bucket, err)
		}
		t.Logf("created the bucket %s", bucket)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if _, err := gcs.Do(ctx, http.MethodDelete, "b/"+bucket, nil, nil); err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting the bucket %s: %v", bucket, err)
			}
		})
	}
}

// make1 creates coll/id and waits for it; when the test ends, deletes it
// and waits (cleanups run last-made first, so children go first).
func make1(t *testing.T, env *mockenv.Env, api *discokind.API, coll, idParam, id string, body map[string]any) {
	t.Helper()
	name := coll + "/" + id
	if err := kindtest.RetryQuota(func() error {
		op, err := api.Do(env.Ctx, http.MethodPost, coll, map[string]string{idParam: id}, body)
		if err == nil {
			_, err = api.Wait(env.Ctx, op)
		}
		if direct.HasHTTPCode(err, http.StatusConflict) {
			// Made by a refused try that went through after all.
			return nil
		}
		return err
	}); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	t.Logf("created %s", name)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		err := kindtest.RetryQuota(func() error {
			op, err := api.Do(ctx, http.MethodDelete, name, nil, nil)
			if err == nil {
				_, err = api.Wait(ctx, op)
			}
			return err
		})
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("cleanup: deleting %s: %v", name, err)
		}
	})
}
