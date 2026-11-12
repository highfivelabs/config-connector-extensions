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

package iampolicy

import (
	"reflect"
	"slices"
	"testing"

	"google.golang.org/genproto/googleapis/type/expr"
)

func TestBigQueryAccessListAsPolicy(t *testing.T) {
	d := &bigqueryDataset{Etag: "e1", Access: []map[string]any{
		{"role": "OWNER", "specialGroup": "projectOwners"},
		{"role": "READER", "userByEmail": "a@example.com"},
		{"role": "READER", "userByEmail": "sa@p.iam.gserviceaccount.com"},
		{"role": "WRITER", "groupByEmail": "g@example.com"},
		{"role": "roles/bigquery.dataViewer", "domain": "example.com"},
		{"role": "READER", "specialGroup": "allAuthenticatedUsers"},
		{"role": "roles/bigquery.user", "iamMember": "allUsers"},
		{"view": map[string]any{"projectId": "p", "datasetId": "d", "tableId": "v"}},
		{"role": "READER", "userByEmail": "b@example.com", "condition": map[string]any{"title": "t", "expression": "request.time < timestamp(\"2099-01-01T00:00:00Z\")"}},
	}}
	p := bigqueryPolicy(d)
	if string(p.Etag) != "e1" || p.Version != 3 {
		t.Errorf("etag %q, version %d", p.Etag, p.Version)
	}
	viewers, _ := Members(p, "roles/bigquery.dataViewer", nil)
	want := []string{"allAuthenticatedUsers", "domain:example.com", "serviceAccount:sa@p.iam.gserviceaccount.com", "user:a@example.com"}
	if !slices.Equal(viewers, want) {
		t.Errorf("dataViewer = %v, want %v", viewers, want)
	}
	if got, _ := Members(p, "roles/bigquery.dataEditor", nil); !slices.Equal(got, []string{"group:g@example.com"}) {
		t.Errorf("dataEditor = %v", got)
	}
	if got, _ := Members(p, "roles/bigquery.user", nil); !slices.Equal(got, []string{"allUsers"}) {
		t.Errorf("bigquery.user = %v", got)
	}
	// projectOwners is not a member the IAM kinds manage: it is kept aside.
	if got, found := Members(p, "roles/bigquery.dataOwner", nil); found {
		t.Errorf("dataOwner = %v, want no binding (projectOwners is kept as it is)", got)
	}
	cond := &Condition{Title: "t", Expression: `request.time < timestamp("2099-01-01T00:00:00Z")`}
	if got, _ := Members(p, "roles/bigquery.dataViewer", cond); !slices.Equal(got, []string{"user:b@example.com"}) {
		t.Errorf("conditional dataViewer = %v", got)
	}
}

func TestBigQueryEntryRoundTrip(t *testing.T) {
	cond := &expr.Expr{Title: "t", Expression: "true"}
	for _, c := range []struct {
		member string
		want   map[string]any
	}{
		{"user:a@example.com", map[string]any{"role": "r", "userByEmail": "a@example.com"}},
		{"serviceAccount:sa@p.iam.gserviceaccount.com", map[string]any{"role": "r", "userByEmail": "sa@p.iam.gserviceaccount.com"}},
		{"group:g@example.com", map[string]any{"role": "r", "groupByEmail": "g@example.com"}},
		{"domain:example.com", map[string]any{"role": "r", "domain": "example.com"}},
		{"allAuthenticatedUsers", map[string]any{"role": "r", "specialGroup": "allAuthenticatedUsers"}},
		{"allUsers", map[string]any{"role": "r", "iamMember": "allUsers"}},
		{"principal://iam.googleapis.com/x", map[string]any{"role": "r", "iamMember": "principal://iam.googleapis.com/x"}},
	} {
		e := bigqueryEntry(c.member, "r", nil)
		if !reflect.DeepEqual(e, c.want) {
			t.Errorf("bigqueryEntry(%s) = %v, want %v", c.member, e, c.want)
		}
		if m, ok := bigqueryMember(e); !ok || m != c.member {
			t.Errorf("bigqueryMember(%v) = %q, %v, want %q", e, m, ok, c.member)
		}
	}
	e := bigqueryEntry("user:a@example.com", "r", cond)
	if got := bigqueryCondition(e); !sameExpr(got, cond) {
		t.Errorf("condition = %v, want %v", got, cond)
	}
	if _, ok := bigqueryMember(map[string]any{"routine": map[string]any{}}); ok {
		t.Error("an authorized routine is not a member")
	}
}

func TestLockDatasetSerializes(t *testing.T) {
	unlock := lockDataset("projects/p/datasets/d")
	got := make(chan struct{})
	go func() {
		u := lockDataset("projects/p/datasets/d")
		close(got)
		u()
	}()
	other := lockDataset("projects/p/datasets/e") // another dataset is not held up
	other()
	select {
	case <-got:
		t.Fatal("a second writer took the dataset's lock while the first held it")
	default:
	}
	unlock()
	<-got
}
