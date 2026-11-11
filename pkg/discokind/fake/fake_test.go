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
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
)

type unreachable struct{}

func (unreachable) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not for the fake")
}

func TestStandardCollection(t *testing.T) {
	ctx := context.Background()
	for _, lro := range []bool{false, true} {
		k := &discokind.Kind{
			BaseURL: "https://example.googleapis.com/v1/", Name: "{parent}/locations/{location}/widgets/{id}",
			Parents: []string{"project", "folder"}, IDParam: "widgetId", Updatable: true,
			CreateLRO: lro, UpdateLRO: lro, DeleteLRO: lro,
			Output:     []string{"createTime", "state", "updateTime"},
			SoftDelete: &discokind.FieldValue{Field: "state", Value: "DELETED"}, FakeDefaults: map[string]any{"state": "ACTIVE"},
		}
		api := &discokind.API{HTTP: &http.Client{Transport: New(unreachable{}, []*discokind.Kind{k})}, BaseURL: k.BaseURL, PollInterval: time.Millisecond}
		name := "projects/p/locations/global/widgets/w1"
		op, err := api.Do(ctx, http.MethodPost, "projects/p/locations/global/widgets", map[string]string{"widgetId": "w1"}, map[string]any{"color": "red", "size": 3})
		if err != nil {
			t.Fatal(err)
		}
		if lro {
			if op, err = api.Wait(ctx, op); err != nil {
				t.Fatal(err)
			}
		}
		if op["name"] != name || op["state"] != "ACTIVE" || op["createTime"] == nil {
			t.Errorf("created %v", op)
		}
		if _, err := api.Do(ctx, http.MethodPost, "projects/p/locations/global/widgets", map[string]string{"widgetId": "w1"}, map[string]any{}); !direct.HasHTTPCode(err, 409) {
			t.Errorf("a second create: %v, want 409", err)
		}
		if _, err := api.Do(ctx, http.MethodPatch, name, map[string]string{"updateMask": "color,size"}, map[string]any{"color": "blue"}); err != nil {
			t.Fatal(err)
		}
		got, err := api.Get(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if got["color"] != "blue" || got["size"] != nil {
			t.Errorf("after the patch: %v", got)
		}
		list, err := api.Get(ctx, "projects/p/locations/global/widgets")
		if err != nil {
			t.Fatal(err)
		}
		if items, _ := list["widgets"].([]any); len(items) != 1 {
			t.Errorf("list: %v", list)
		}
		if _, err := api.Get(ctx, "folders/f/locations/global/widgets/w1"); !direct.IsNotFound(err) {
			t.Errorf("another parent's widget: %v, want 404", err)
		}
		if _, err := api.Do(ctx, http.MethodDelete, name, nil, nil); err != nil {
			t.Fatal(err)
		}
		if got, _ := api.Get(ctx, name); got["state"] != "DELETED" {
			t.Errorf("a soft-deleted widget: %v", got)
		}
		if _, err := api.Do(ctx, http.MethodDelete, name, nil, nil); !direct.IsNotFound(err) {
			t.Errorf("deleting twice: %v, want 404", err)
		}
		if _, err := api.Get(ctx, "projects/p/other/x"); err == nil || direct.IsNotFound(err) {
			t.Errorf("a path the fake does not serve should reach the next round tripper: %v", err)
		}
	}
}
