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

package fake

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
)

type unreachable struct{ t *testing.T }

func (u unreachable) RoundTrip(req *http.Request) (*http.Response, error) {
	u.t.Errorf("passed on: %s %s", req.Method, req.URL)
	return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody, Request: req}, nil
}

func TestRuleDeployment(t *testing.T) {
	ctx := context.Background()
	api := &discokind.API{HTTP: &http.Client{Transport: New(unreachable{t})}, BaseURL: "https://us-chronicle.googleapis.com/v1/"}
	parent := "projects/p/locations/us/instances/i"
	rule, err := api.Do(ctx, http.MethodPost, parent+"/rules", nil, map[string]any{"text": "rule r { condition: $e }"})
	if err != nil {
		t.Fatal(err)
	}
	name := rule["name"].(string)
	if !strings.HasPrefix(name, parent+"/rules/ru_") {
		t.Fatalf("rule name %q", name)
	}
	dep := name + "/deployment"
	got, err := api.Get(ctx, dep)
	if err != nil {
		t.Fatal(err)
	}
	if got["enabled"] != false || got["name"] != dep {
		t.Errorf("a new rule's deployment: %v", got)
	}

	patch := func(mask string, body map[string]any) (map[string]any, error) {
		return api.Do(ctx, http.MethodPatch, dep, map[string]string{"updateMask": mask}, body)
	}
	if got, err = patch("enabled,alerting", map[string]any{"enabled": true, "alerting": true, "archived": true}); err != nil {
		t.Fatal(err)
	}
	if got["enabled"] != true || got["alerting"] != true || got["archived"] != false {
		t.Errorf("a field outside the mask changed, or one in it did not: %v", got)
	}
	if got["lastAlertStatusChangeTime"] == nil {
		t.Error("lastAlertStatusChangeTime is not set after alerting changed")
	}
	if _, err := patch("archived", map[string]any{"archived": true}); err == nil || !strings.Contains(err.Error(), "disable it first") {
		t.Errorf("archiving an enabled deployment: %v", err)
	}
	if _, err := patch("enabled", map[string]any{"enabled": false}); err != nil {
		t.Fatal(err)
	}
	if got, err = patch("archived", map[string]any{"archived": true}); err != nil {
		t.Fatal(err)
	}
	if got["alerting"] != false || got["archiveTime"] == nil {
		t.Errorf("an archived deployment: %v", got)
	}
	if _, err := patch("enabled", map[string]any{"enabled": true}); err == nil || !strings.Contains(err.Error(), "archived") {
		t.Errorf("enabling an archived deployment: %v", err)
	}
	if got, err = patch("archived,enabled", map[string]any{"archived": false, "enabled": true}); err != nil {
		t.Fatalf("unarchiving and enabling together: %v", err)
	}
	if got["archiveTime"] != nil || got["enabled"] != true {
		t.Errorf("an unarchived deployment: %v", got)
	}
	sc := map[string]any{"ensureEnrichmentCompleteness": true}
	if got, err = patch("runFrequency", map[string]any{"runFrequency": "LIVE_CUSTOMIZABLE", "scheduleCustomizations": sc}); err != nil {
		t.Fatal(err)
	}
	if got["runFrequency"] != "LIVE_CUSTOMIZABLE" || got["scheduleCustomizations"] == nil {
		t.Errorf("the schedule did not go with the run frequency: %v", got)
	}
	if _, err := patch("executionState", map[string]any{"executionState": "PAUSED"}); err == nil {
		t.Error("an output-only field in the mask was taken")
	}

	if _, err := api.Do(ctx, http.MethodDelete, name, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Get(ctx, dep); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("the deployment of a deleted rule: %v", err)
	}
	if _, err := patch("enabled", map[string]any{"enabled": true}); err == nil {
		t.Error("patched the deployment of a deleted rule")
	}
}
