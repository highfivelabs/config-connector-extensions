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

package discokind

import (
	"reflect"
	"testing"
)

func TestName(t *testing.T) {
	k := &Kind{Name: "{parent}/locations/{location}/savedQueries/{id}", Parents: []string{"project", "billingAccount"}}
	got, err := k.Render("projects/p", "q1", map[string]string{"location": "global"})
	if err != nil || got != "projects/p/locations/global/savedQueries/q1" {
		t.Errorf("Render = %q, %v", got, err)
	}
	if _, err := k.Render("projects/p", "q1", nil); err == nil {
		t.Error("Render without a location should fail")
	}
	re := k.NameRegexp()
	for name, want := range map[string]bool{
		"projects/p/locations/global/savedQueries/q1":         true,
		"billingAccounts/0-1/locations/global/savedQueries/q": true,
		"folders/f/locations/global/savedQueries/q1":          false,
		"projects/p/locations/global/savedQueries":            false,
		"projects/p/locations/global/savedQueries/q1/x":       false,
	} {
		if re.MatchString(name) != want {
			t.Errorf("NameRegexp(%q) = %v", name, !want)
		}
	}
	if got := k.PathParams(); !reflect.DeepEqual(got, []string{"location"}) {
		t.Errorf("PathParams = %v", got)
	}
	body := k.Body(map[string]any{"projectRef": map[string]any{"external": "p"}, "location": "global", "resourceID": "q1", "displayName": "d"})
	if !reflect.DeepEqual(body, map[string]any{"displayName": "d"}) {
		t.Errorf("Body = %v", body)
	}
}

func TestDiff(t *testing.T) {
	actual := map[string]any{
		"displayName":  "d",
		"size":         "42", // an int64, as the API writes it
		"ratio":        0.5,
		"labels":       map[string]any{"a": "1", "b": "2"},
		"list":         []any{map[string]any{"x": "1", "y": "default"}},
		"createTime":   "2026-01-01T00:00:00Z",
		"subnetworks":  []any{"https://www.googleapis.com/compute/v1/projects/p/regions/r/subnetworks/s"},
		"resourceSpec": map[string]any{"name": "b1", "type": "STORAGE_BUCKET", "readAccessMode": "DIRECT"},
	}
	for _, c := range []struct {
		desired map[string]any
		want    []string
	}{
		{map[string]any{"displayName": "d", "size": int64(42), "ratio": 0.5}, nil},
		{map[string]any{"labels": map[string]any{"a": "1"}}, nil},
		{map[string]any{"list": []any{map[string]any{"x": "1"}}}, nil},
		{map[string]any{"displayName": "e", "size": int64(43)}, []string{"displayName", "size"}},
		{map[string]any{"list": []any{}}, []string{"list"}},
		{map[string]any{"missing": "x", "disabled": false}, []string{"missing"}},
		// Compute Engine answers a path with its full URL.
		{map[string]any{"subnetworks": []any{"projects/p/regions/r/subnetworks/s"}}, nil},
		{map[string]any{"subnetworks": []any{"https://www.googleapis.com/compute/v1/projects/p/regions/r/subnetworks/s"}}, nil},
		{map[string]any{"subnetworks": []any{"projects/p/regions/r/subnetworks/t"}}, []string{"subnetworks"}},
		{map[string]any{"subnetworks": []any{"regions/r/subnetworks/s"}}, []string{"subnetworks"}},
		// Dataplex answers a bucket's path with its bare name.
		{map[string]any{"resourceSpec": map[string]any{"name": "projects/p/buckets/b1", "type": "STORAGE_BUCKET"}}, nil},
		{map[string]any{"resourceSpec": map[string]any{"name": "projects/p/buckets/b2"}}, []string{"resourceSpec"}},
		{map[string]any{"resourceSpec": map[string]any{"name": "b1"}}, nil},
	} {
		if got := Diff(c.desired, actual); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Diff(%v) = %v, want %v", c.desired, got, c.want)
		}
	}
}
