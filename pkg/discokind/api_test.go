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
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
)

// operationServer answers reads of one operation: refused for quota
// (429) the first refuse times, not done until the done-th read.
type operationServer struct {
	mu     sync.Mutex
	reads  []time.Time
	refuse int
	done   int
}

func (s *operationServer) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads = append(s.reads, time.Now())
	n := len(s.reads)
	answer := func(code int, body string) (*http.Response, error) {
		return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	if n <= s.refuse {
		return answer(http.StatusTooManyRequests, `{"error":{"code":429,"message":"Quota exceeded for quota metric 'Requests' and limit 'Requests per minute per user per region'","status":"RESOURCE_EXHAUSTED"}}`)
	}
	if n < s.done {
		return answer(http.StatusOK, `{"name":"projects/p/locations/us-central1/operations/op1","done":false}`)
	}
	return answer(http.StatusOK, `{"name":"projects/p/locations/us-central1/operations/op1","done":true,"response":{"name":"projects/p/locations/us-central1/lakes/l1"}}`)
}

// The live run that found it: Dataplex allows 10 requests a minute per
// user and region, and an operation read every 2s was refused (429),
// which ended the create and marked the object failed.
func TestWaitReadsLessOftenAndOutlastsQuota(t *testing.T) {
	srv := &operationServer{refuse: 3, done: 8}
	api := &API{HTTP: &http.Client{Transport: srv}, BaseURL: "https://dataplex.googleapis.com/v1/", PollInterval: 4 * time.Millisecond, MaxPollInterval: 40 * time.Millisecond}
	op := map[string]any{"name": "projects/p/locations/us-central1/operations/op1", "done": false}
	got, err := api.Wait(context.Background(), op)
	if err != nil {
		t.Fatalf("Wait with reads refused for quota: %v, want the operation's response", err)
	}
	if got["name"] != "projects/p/locations/us-central1/lakes/l1" {
		t.Errorf("Wait = %v", got)
	}
	if len(srv.reads) != 8 {
		t.Errorf("reads = %d, want 8 (3 refused, 4 not done, 1 done)", len(srv.reads))
	}
	// The waits grow: the last gap is longer than the first.
	first := srv.reads[1].Sub(srv.reads[0])
	last := srv.reads[len(srv.reads)-1].Sub(srv.reads[len(srv.reads)-2])
	if last <= first {
		t.Errorf("the gap between reads went from %v to %v, want it to grow", first, last)
	}
}

func TestWaitEndsWithItsContextWhileRefused(t *testing.T) {
	srv := &operationServer{refuse: 1 << 30}
	api := &API{HTTP: &http.Client{Transport: srv}, BaseURL: "https://dataplex.googleapis.com/v1/", PollInterval: time.Millisecond, MaxPollInterval: 5 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := api.Wait(ctx, map[string]any{"name": "projects/p/locations/us-central1/operations/op1"})
	if err == nil || ctx.Err() == nil {
		t.Fatalf("Wait = %v, want the context's end", err)
	}
}

// An operation that fails FAILED_PRECONDITION (API Gateway's config still
// in use by a gateway being deleted) is InUse, so the delete is tried
// again; a denial or a plain bad request is not.
func TestInUse(t *testing.T) {
	failed := func(code int, msg string) error {
		_, err := (&API{}).Wait(context.Background(), map[string]any{"name": "operations/op1", "done": true,
			"error": map[string]any{"code": float64(code), "message": msg}})
		return err
	}
	for _, c := range []struct {
		err  error
		want bool
	}{
		{failed(9, "Config is still used by a gateway"), true},
		{failed(9, "anything"), true},
		{failed(3, "API projects/p/locations/global/apis/a has nested resources"), true},
		{failed(3, "invalid document"), false},
		{failed(7, "Permission denied, or it is in use"), false},
		{&googleapi.Error{Code: 400, Body: `{"error":{"code":400,"message":"x","status":"FAILED_PRECONDITION"}}`}, true},
		{&googleapi.Error{Code: 409, Message: "resource is in use by projects/p/x"}, true},
		{&googleapi.Error{Code: 404, Message: "in use"}, false},
		{errors.New("in use"), false},
	} {
		if got := InUse(c.err); got != c.want {
			t.Errorf("InUse(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
