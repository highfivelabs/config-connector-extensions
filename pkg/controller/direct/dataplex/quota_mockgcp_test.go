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

// A DataplexLake whose every step the API first refuses for quota (429
// RESOURCE_EXHAUSTED), as Dataplex does past 10 requests a minute per user
// and region. The catalog's live runs in hfl-dev-514205 found the lakes,
// zones and assets marked failed (UpdateFailed) on such a refusal and
// tried again within a second, which the quota refused again: an asset
// never became Ready, and deletes stalled for more than 25 minutes. Now a
// refusal requeues with a growing wait, and the object is never marked
// failed (patch 0047, pkg/controller QuotaWait). Live, the same refusals
// are made by the test in front of the real API, on top of the real quota.

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataplexv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/dataplex/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestDataplexLakeRefusedForQuota(t *testing.T) {
	const namespace = "quota"
	gvk := dataplexv1beta1.DataplexLakeGVK
	k := discokind.ByKind(gvk.Kind)
	env := mockenv.New(t, namespace)
	api := &discokind.API{HTTP: env.HTTP, BaseURL: k.BaseURL}
	if !mockenv.Live() {
		api.PollInterval = 200 * time.Millisecond
	}
	id := mockenv.Name("lake-quota")
	parent := "projects/" + mockenv.Project + "/locations/us-central1"
	name := parent + "/lakes/" + id
	isLake := func(method string) func(*http.Request) bool {
		return func(req *http.Request) bool {
			if req.Method != method {
				return false
			}
			if method == http.MethodPost {
				return strings.HasSuffix(req.URL.Path, "/"+parent+"/lakes") && req.URL.Query().Get(k.IDParam) == id
			}
			return strings.HasSuffix(req.URL.Path, "/"+name)
		}
	}
	failedEvents := func() []string {
		var out []string
		for _, r := range env.Events(namespace, id) {
			if r == "UpdateFailed" || r == "DeleteFailed" {
				out = append(out, r)
			}
		}
		return out
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		for {
			op, err := api.Do(ctx, http.MethodDelete, name, nil, nil)
			if err == nil {
				_, err = api.Wait(ctx, op)
			}
			if err == nil || direct.IsNotFound(err) {
				return
			}
			if !direct.HasHTTPCode(err, http.StatusTooManyRequests) || ctx.Err() != nil {
				t.Errorf("cleanup: deleting %s: %v", name, err)
				return
			}
			time.Sleep(20 * time.Second)
		}
	})

	// The create, and the reads before and after it, are each refused
	// first.
	createRefused := env.RefuseForQuota(1, isLake(http.MethodPost))
	getRefused := env.RefuseForQuota(3, isLake(http.MethodGet))
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(id)
	u.SetLabels(mockenv.Labels())
	u.Object["spec"] = map[string]any{
		"projectRef":  map[string]any{"external": mockenv.Project},
		"location":    "us-central1",
		"displayName": "cce quota test lake",
		"description": "made by a config-connector-extensions test, run " + mockenv.RunID,
	}
	if err := env.Kube.Create(env.Ctx, u); err != nil {
		t.Fatal(err)
	}

	t.Run("refused for quota, the create waits and goes on", func(t *testing.T) {
		env.WaitReady(gvk, namespace, id)
		if n := createRefused(); n != 1 {
			t.Errorf("creates refused = %d, want 1", n)
		}
		if n := getRefused(); n < 1 {
			t.Errorf("reads refused = %d, want at least 1", n)
		}
		if got, err := api.Get(env.Ctx, name); err != nil {
			t.Errorf("reading the lake: %v", err)
		} else if got["displayName"] != "cce quota test lake" {
			t.Errorf("the lake is %v", got)
		}
		if f := failedEvents(); len(f) > 0 {
			t.Errorf("events %v: a refusal for quota marked the object failed", f)
		}
		if !slices.Contains(env.Events(namespace, id), "QuotaExhausted") {
			t.Errorf("no QuotaExhausted event, events %v", env.Events(namespace, id))
		}
	})

	t.Run("refused for quota, the delete waits and goes on", func(t *testing.T) {
		delRefused := env.RefuseForQuota(2, isLake(http.MethodDelete))
		getRefused := env.RefuseForQuota(1, isLake(http.MethodGet))
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		if err := env.Kube.Get(env.Ctx, client.ObjectKey{Namespace: namespace, Name: id}, obj); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		if n := delRefused(); n != 2 {
			t.Errorf("deletes refused = %d, want 2", n)
		}
		if n := getRefused(); n != 1 {
			t.Errorf("reads refused = %d, want 1", n)
		}
		if _, err := api.Get(env.Ctx, name); !direct.IsNotFound(err) {
			t.Errorf("reading the deleted lake: %v, want not found", err)
		}
		if f := failedEvents(); len(f) > 0 {
			t.Errorf("events %v: a refusal for quota marked the object failed", f)
		}
	})
}
