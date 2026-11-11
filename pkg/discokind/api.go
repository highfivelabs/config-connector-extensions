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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"google.golang.org/api/googleapi"
)

// API is a Google API's JSON REST surface: resource names relative to
// BaseURL, bodies as JSON objects. Errors are *googleapi.Error.
type API struct {
	HTTP    *http.Client
	BaseURL string
	// PollInterval is how long Wait sleeps between reads of an operation
	// (default 2s).
	PollInterval time.Duration
}

// Get reads a resource.
func (a *API) Get(ctx context.Context, name string) (map[string]any, error) {
	return a.Do(ctx, http.MethodGet, name, nil, nil)
}

// Do sends one request and decodes the answer.
func (a *API) Do(ctx context.Context, method, path string, query map[string]string, body map[string]any) (map[string]any, error) {
	u := a.BaseURL + path
	if len(query) > 0 {
		v := url.Values{}
		for k, q := range query {
			v.Set(k, q)
		}
		u += "?" + v.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := googleapi.CheckResponse(resp); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if len(bytes.TrimSpace(b)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decoding %s %s: %w", method, path, err)
	}
	return out, nil
}

// Wait polls a long-running operation until it is done, and returns its
// response, or its error as a *googleapi.Error.
func (a *API) Wait(ctx context.Context, op map[string]any) (map[string]any, error) {
	interval := a.PollInterval
	if interval == 0 {
		interval = 2 * time.Second
	}
	for {
		if done, _ := op["done"].(bool); done {
			if e, ok := op["error"].(map[string]any); ok {
				code, _ := e["code"].(float64)
				msg, _ := e["message"].(string)
				return nil, &googleapi.Error{Code: grpcToHTTP(int(code)), Message: msg}
			}
			resp, _ := op["response"].(map[string]any)
			return resp, nil
		}
		name, _ := op["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("an operation without a name: %v", op)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		var err error
		if op, err = a.Get(ctx, name); err != nil {
			return nil, fmt.Errorf("reading operation %s: %w", name, err)
		}
	}
}

// grpcToHTTP maps an operation error's google.rpc.Code to the HTTP status
// the same error has in a REST answer.
func grpcToHTTP(code int) int {
	switch code {
	case 3, 9, 11: // INVALID_ARGUMENT, FAILED_PRECONDITION, OUT_OF_RANGE
		return http.StatusBadRequest
	case 5: // NOT_FOUND
		return http.StatusNotFound
	case 6, 10: // ALREADY_EXISTS, ABORTED
		return http.StatusConflict
	case 7: // PERMISSION_DENIED
		return http.StatusForbidden
	case 16: // UNAUTHENTICATED
		return http.StatusUnauthorized
	case 8: // RESOURCE_EXHAUSTED
		return http.StatusTooManyRequests
	case 12: // UNIMPLEMENTED
		return http.StatusNotImplemented
	case 14: // UNAVAILABLE
		return http.StatusServiceUnavailable
	case 4: // DEADLINE_EXCEEDED
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
