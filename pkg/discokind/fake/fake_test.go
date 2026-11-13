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

func TestComputeInsertStyle(t *testing.T) {
	ctx := context.Background()
	for _, put := range []bool{false, true} {
		k := &discokind.Kind{
			BaseURL: "https://compute.googleapis.com/compute/v1/", Name: "{parent}/regions/{region}/widgets/{id}",
			Parents: []string{"project"}, InsertStyle: true, UpdatePUT: put, Updatable: true,
			CreateLRO: true, UpdateLRO: true, DeleteLRO: true,
			Output: []string{"creationTimestamp", "fingerprint", "id", "selfLink"},
		}
		api := &discokind.API{HTTP: &http.Client{Transport: New(unreachable{}, []*discokind.Kind{k})}, BaseURL: k.BaseURL, PollInterval: time.Millisecond}
		path := "projects/p/regions/r/widgets/w1"
		op, err := api.Do(ctx, http.MethodPost, "projects/p/regions/r/widgets", nil, map[string]any{"name": "w1", "color": "red", "size": 3})
		if err != nil {
			t.Fatal(err)
		}
		if op["kind"] != "compute#operation" {
			t.Fatalf("insert answered %v", op)
		}
		if _, err := api.Wait(ctx, op); err != nil {
			t.Fatal(err)
		}
		got, err := api.Get(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		if got["name"] != "w1" || got["selfLink"] != k.BaseURL+path || got["fingerprint"] == nil {
			t.Errorf("inserted %v", got)
		}
		if _, err := api.Do(ctx, http.MethodPatch, path, nil, map[string]any{"name": "w1", "color": "blue", "fingerprint": "stale"}); !direct.HasHTTPCode(err, 412) && !put {
			t.Errorf("a stale fingerprint: %v, want 412", err)
		}
		method := http.MethodPatch
		if put {
			method = http.MethodPut
		}
		op, err = api.Do(ctx, method, path, nil, map[string]any{"name": "w1", "color": "blue", "fingerprint": got["fingerprint"]})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := api.Wait(ctx, op); err != nil {
			t.Fatal(err)
		}
		got, _ = api.Get(ctx, path)
		if got["color"] != "blue" || (put && got["size"] != nil) || (!put && got["size"] == nil) {
			t.Errorf("after %s: %v", method, got)
		}
		list, _ := api.Get(ctx, "projects/p/regions/r/widgets")
		if items, _ := list["items"].([]any); len(items) != 1 {
			t.Errorf("list: %v", list)
		}
		op, err = api.Do(ctx, http.MethodDelete, path, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := api.Wait(ctx, op); err != nil {
			t.Fatal(err)
		}
		if _, err := api.Get(ctx, path); !direct.IsNotFound(err) {
			t.Errorf("after delete: %v", err)
		}
	}
}

// next records the requests the fake passes on.
type next struct{ paths []string }

func (n *next) RoundTrip(req *http.Request) (*http.Response, error) {
	n.paths = append(n.paths, req.Method+" "+req.URL.Path)
	return jsonResponse(req, http.StatusOK, map[string]any{"etag": "BwE="})
}

// A custom method on an object, such as :getIamPolicy, is not the
// collection's: it reaches the next round tripper (mockgcp, which serves
// the IAM verbs for every resource), not the fake's "not found". Before,
// a kind's name pattern took ".../widgets/w1:getIamPolicy" for an object
// called "w1:getIamPolicy".
func TestCustomMethodsReachNext(t *testing.T) {
	ctx := context.Background()
	k := &discokind.Kind{
		BaseURL: "https://example.googleapis.com/v1/", Name: "{parent}/locations/{location}/widgets/{id}",
		Parents: []string{"project"}, IDParam: "widgetId", Updatable: true,
	}
	n := &next{}
	api := &discokind.API{HTTP: &http.Client{Transport: New(n, []*discokind.Kind{k})}, BaseURL: k.BaseURL, PollInterval: time.Millisecond}
	name := "projects/p/locations/global/widgets/w1"
	if _, err := api.Do(ctx, http.MethodPost, "projects/p/locations/global/widgets", map[string]string{"widgetId": "w1"}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, verb string }{
		{http.MethodGet, "getIamPolicy"}, {http.MethodPost, "getIamPolicy"},
		{http.MethodPost, "setIamPolicy"}, {http.MethodPost, "testIamPermissions"},
	} {
		if _, err := api.Do(ctx, c.method, name+":"+c.verb, nil, map[string]any{}); err != nil {
			t.Errorf("%s %s:%s: %v, want it passed to the next round tripper", c.method, name, c.verb, err)
		}
	}
	want := []string{
		"GET /v1/" + name + ":getIamPolicy", "POST /v1/" + name + ":getIamPolicy",
		"POST /v1/" + name + ":setIamPolicy", "POST /v1/" + name + ":testIamPermissions",
	}
	if len(n.paths) != len(want) {
		t.Fatalf("passed on %v, want %v", n.paths, want)
	}
	for i := range want {
		if n.paths[i] != want[i] {
			t.Errorf("passed on %q, want %q", n.paths[i], want[i])
		}
	}
	// The object itself is still the fake's.
	if got, err := api.Get(ctx, name); err != nil || got["name"] != name {
		t.Errorf("get %s: %v, %v", name, got, err)
	}
}
