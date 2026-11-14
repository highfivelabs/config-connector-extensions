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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/api/googleapi"
)

// API is a Google API's JSON REST surface: resource names relative to
// BaseURL, bodies as JSON objects. Errors are *googleapi.Error.
type API struct {
	HTTP    *http.Client
	BaseURL string
	// PollInterval is how long Wait sleeps before its first read of an
	// operation (default 2s); each later wait is half as long again, up to
	// MaxPollInterval (default 30s, or PollInterval when that is longer).
	// Some APIs allow only a few reads a minute (Dataplex: 10 a minute per
	// user and region), and an operation can take minutes.
	PollInterval, MaxPollInterval time.Duration
	// LocationHost sends a name in a location other than global to that
	// location's host (Kind.LocationHost, LocationBaseURL).
	LocationHost bool
	// GetQuery are query parameters Get sends (Kind.GetQuery); an
	// operation's read sends none.
	GetQuery map[string]string
}

// Get reads a resource.
func (a *API) Get(ctx context.Context, name string) (map[string]any, error) {
	return a.Do(ctx, http.MethodGet, name, a.GetQuery, nil)
}

// Do sends one request and decodes the answer.
func (a *API) Do(ctx context.Context, method, path string, query map[string]string, body map[string]any) (map[string]any, error) {
	base := a.BaseURL
	if a.LocationHost {
		base = LocationBaseURL(base, path)
	}
	u := base + path
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		u = path
	}
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
// response, or its error as a *googleapi.Error. It reads less often as
// the operation goes on (PollInterval, MaxPollInterval), and a read
// refused for quota (429) does not end it.
func (a *API) Wait(ctx context.Context, op map[string]any) (map[string]any, error) {
	interval := a.PollInterval
	if interval == 0 {
		interval = 2 * time.Second
	}
	maxInterval := a.MaxPollInterval
	if maxInterval == 0 {
		maxInterval = 30 * time.Second
	}
	if maxInterval < interval {
		maxInterval = interval
	}
	// next is the wait before the next read: half as long again each time,
	// and twice as long after a read refused for quota.
	next := func(refused bool) time.Duration {
		d := interval
		if refused {
			interval *= 2
		} else {
			interval += interval / 2
		}
		if interval > maxInterval {
			interval = maxInterval
		}
		return d
	}
	// read reads the operation again at url after a wait; a read the API
	// refuses for quota (429) is tried again after a longer one, as long
	// as ctx allows: the operation goes on in the cloud regardless.
	read := func(url string) (map[string]any, error) {
		refused := false
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(next(refused)):
			}
			got, err := a.Do(ctx, http.MethodGet, url, nil, nil)
			if err != nil && isQuota(err) {
				refused = true
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("reading operation %s: %w", url, err)
			}
			return got, nil
		}
	}
	for {
		// A Compute Engine operation: status DONE, errors in error.errors,
		// read again at its selfLink.
		if st, ok := op["status"].(string); ok {
			if st == "DONE" {
				if e, ok := op["error"].(map[string]any); ok {
					code, _ := op["httpErrorStatusCode"].(float64)
					if code == 0 {
						code = http.StatusBadRequest
					}
					b, _ := json.Marshal(e["errors"])
					return nil, &googleapi.Error{Code: int(code), Message: string(b)}
				}
				return op, nil
			}
			self, _ := op["selfLink"].(string)
			if self == "" {
				return nil, fmt.Errorf("a compute operation without a selfLink: %v", op)
			}
			var err error
			if op, err = read(self); err != nil {
				return nil, err
			}
			continue
		}
		if done, _ := op["done"].(bool); done {
			if e, ok := op["error"].(map[string]any); ok {
				code, _ := e["code"].(float64)
				msg, _ := e["message"].(string)
				// Body as an HTTP error's, with the gRPC status's name, so a
				// caller can tell FAILED_PRECONDITION from another 400 (InUse).
				httpCode := grpcToHTTP(int(code))
				body, _ := json.Marshal(map[string]any{"error": map[string]any{"code": httpCode, "message": msg, "status": grpcName(int(code))}})
				return nil, &googleapi.Error{Code: httpCode, Message: msg, Body: string(body)}
			}
			resp, _ := op["response"].(map[string]any)
			return resp, nil
		}
		name, _ := op["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("an operation without a name: %v", op)
		}
		var err error
		if op, err = read(name); err != nil {
			return nil, err
		}
	}
}

// isQuota reports whether err is an API's refusal for quota (HTTP 429).
func isQuota(err error) bool {
	var ge *googleapi.Error
	return errors.As(err, &ge) && ge.Code == http.StatusTooManyRequests
}

// grpcName is a gRPC status code's name (FAILED_PRECONDITION).
func grpcName(code int) string {
	names := []string{"OK", "CANCELLED", "UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED", "NOT_FOUND",
		"ALREADY_EXISTS", "PERMISSION_DENIED", "RESOURCE_EXHAUSTED", "FAILED_PRECONDITION", "ABORTED",
		"OUT_OF_RANGE", "UNIMPLEMENTED", "INTERNAL", "UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED"}
	if code >= 0 && code < len(names) {
		return names[code]
	}
	return "UNKNOWN"
}

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
