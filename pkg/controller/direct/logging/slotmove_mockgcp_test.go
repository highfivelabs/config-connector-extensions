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

// A saved query (a discokind kind) moves from one version slot to another
// (pkg/test/slotmove). A saved query has no labels: it moves the same way,
// but nothing in the cloud marks it, so the guard cannot stop a resumed old
// slot; the move pauses the old slot and abandons its object.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	loggingv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/slotmove"
)

func TestSlotMoveLoggingSavedQuery(t *testing.T) {
	if mockenv.Live() {
		t.Skip("fake cloud only: the live slot move is SecretManagerSecret's (docs/live-runs.md)")
	}
	env := mockenv.NewSlots(t, slotmove.Old, slotmove.New)
	name := mockenv.Name("slot-move")
	url := "https://logging.googleapis.com/v2/projects/" + mockenv.Project + "/locations/global/savedQueries/" + name

	do := func(ctx context.Context, method string) (map[string]any, int, error) {
		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, 0, err
		}
		resp, err := env.HTTP.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, err
		}
		if resp.StatusCode/100 != 2 {
			return nil, resp.StatusCode, fmt.Errorf("%s %s: %d %s", method, url, resp.StatusCode, body)
		}
		out := map[string]any{}
		return out, resp.StatusCode, json.Unmarshal(body, &out)
	}

	slotmove.Run(t, env, slotmove.Case{
		GVK:  loggingv1alpha1.LoggingSavedQueryGVK,
		Name: name,
		Object: func(ns string) client.Object {
			return &loggingv1alpha1.LoggingSavedQuery{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
				Spec: loggingv1alpha1.LoggingSavedQuerySpec{
					ProjectRef:   &refsv1beta1.ProjectRef{External: mockenv.Project},
					Location:     "global",
					DisplayName:  mockenv.Ptr("payments errors"),
					LoggingQuery: &loggingv1alpha1.LoggingSavedQueryLoggingQuery{Filter: mockenv.Ptr(`severity>=ERROR AND resource.labels.service_name="payments"`)},
					Visibility:   mockenv.Ptr("SHARED"),
				},
			}
		},
		Read: func(t *testing.T) (slotmove.Cloud, bool) {
			t.Helper()
			got, code, err := do(env.Ctx, http.MethodGet)
			if code == http.StatusNotFound {
				return slotmove.Cloud{}, false
			}
			if err != nil {
				t.Fatal(err)
			}
			created, _ := got["createTime"].(string)
			delete(got, "updateTime")
			fields, _ := json.Marshal(got) // sorted keys
			return slotmove.Cloud{ID: fmt.Sprint(got["name"]), Created: created, Fields: string(fields)}, true
		},
		DeleteCloud: func(t *testing.T) {
			if _, code, err := do(env.Ctx, http.MethodDelete); err != nil && code != http.StatusNotFound {
				t.Errorf("deleting %s: %v", strings.TrimPrefix(url, "https://"), err)
			}
		},
		Labeled: false,
	})
}
