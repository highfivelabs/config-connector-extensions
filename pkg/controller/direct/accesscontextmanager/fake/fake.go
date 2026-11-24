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

// Package fake is the fake cloud for Access Context Manager's GCP user
// access bindings (accesscontextmanager.googleapis.com/v1/organizations/{org}/gcpUserAccessBindings):
// an in-memory store in front of mockgcp, as pkg/discokind/fake is.
// mockgcp serves the collection, but from protos (cloud.google.com/go/accesscontextmanager
// v1.14.0) that have only a binding's group key and access levels, so it
// drops the fields AccessContextManagerGCPUserAccessBinding adds, and its
// patch takes only access_levels. This fake keeps a binding as the JSON it
// was sent: create (the ID the server's, one binding per group key in an
// organization, a done operation with the binding as its response), get,
// list, patch of the fields its update mask names (camelCase or
// snake_case; groupKey refused), and delete; every other request goes on
// to mockgcp.
//
// What it cannot show (docs/fake-cloud.md): organizations, Google Groups
// and access levels (any group key and level is taken), the API's checks
// of a duration or an application, which fields may be set together, and
// permissions.
package fake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// patchable are the fields an update mask may name.
var patchable = map[string]string{
	"access_levels":                  "accessLevels",
	"dry_run_access_levels":          "dryRunAccessLevels",
	"session_settings":               "sessionSettings",
	"restricted_client_applications": "restrictedClientApplications",
	"scoped_access_settings":         "scopedAccessSettings",
}

// Cloud is the fake. It answers the user access binding requests and
// passes every other request to Next.
type Cloud struct {
	Next http.RoundTripper

	mu       sync.Mutex
	bindings map[string]map[string]any // by name
	ops      map[string]map[string]any // by name
	seq      int64
}

// New is the fake in front of next.
func New(next http.RoundTripper) *Cloud {
	return &Cloud{Next: next, bindings: map[string]map[string]any{}, ops: map[string]map[string]any{}}
}

func (c *Cloud) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "accesscontextmanager.googleapis.com" {
		return c.Next.RoundTrip(req)
	}
	path := strings.TrimPrefix(req.URL.Path, "/v1/")
	parts := strings.Split(path, "/")
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(parts) >= 2 && parts[0] == "operations" && req.Method == http.MethodGet {
		if op, ok := c.ops[path]; ok {
			return jsonResponse(req, http.StatusOK, op)
		}
		if strings.HasPrefix(path, "operations/gcpuab-") {
			return notFound(req, path)
		}
	}
	if len(parts) < 3 || parts[0] != "organizations" || parts[2] != "gcpUserAccessBindings" {
		return c.Next.RoundTrip(req)
	}
	switch {
	case len(parts) == 3 && req.Method == http.MethodPost:
		return c.create(req, "organizations/"+parts[1])
	case len(parts) == 3 && req.Method == http.MethodGet:
		var list []map[string]any
		for name, b := range c.bindings {
			if strings.HasPrefix(name, path+"/") {
				list = append(list, b)
			}
		}
		return jsonResponse(req, http.StatusOK, map[string]any{"gcpUserAccessBindings": list})
	case len(parts) == 4 && req.Method == http.MethodGet:
		b, ok := c.bindings[path]
		if !ok {
			return notFound(req, path)
		}
		return jsonResponse(req, http.StatusOK, b)
	case len(parts) == 4 && req.Method == http.MethodPatch:
		return c.patch(req, path)
	case len(parts) == 4 && req.Method == http.MethodDelete:
		if _, ok := c.bindings[path]; !ok {
			return notFound(req, path)
		}
		delete(c.bindings, path)
		return c.done(req, path, "delete", map[string]any{})
	}
	return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("the user access binding fake does not serve %s %s", req.Method, path))
}

func (c *Cloud) create(req *http.Request, parent string) (*http.Response, error) {
	body := map[string]any{}
	if err := readJSON(req, &body); err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	if name, _ := body["name"].(string); name != "" {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "name must not be set in a create; the server assigns it")
	}
	groupKey, _ := body["groupKey"].(string)
	if groupKey == "" {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "group_key is required")
	}
	if _, ok := body["restrictedClientApplications"]; ok {
		if _, ok := body["scopedAccessSettings"]; ok {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "restricted_client_applications and scoped_access_settings cannot both be set")
		}
	}
	for name, b := range c.bindings {
		if strings.HasPrefix(name, parent+"/") && b["groupKey"] == groupKey {
			return errorResponse(req, http.StatusConflict, "ALREADY_EXISTS", fmt.Sprintf("GcpUserAccessBinding with group_key %q already exists in %s", groupKey, parent))
		}
	}
	c.seq++
	name := parent + "/gcpUserAccessBindings/b" + strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.FormatInt(c.seq, 10)
	body["name"] = name
	c.bindings[name] = body
	return c.done(req, name, "create", withType(body))
}

func (c *Cloud) patch(req *http.Request, name string) (*http.Response, error) {
	stored, ok := c.bindings[name]
	if !ok {
		return notFound(req, name)
	}
	body := map[string]any{}
	if err := readJSON(req, &body); err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	mask := req.URL.Query().Get("updateMask")
	if mask == "" {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "update_mask is required")
	}
	next := map[string]any{}
	for k, v := range stored {
		next[k] = v
	}
	for _, path := range strings.Split(mask, ",") {
		field, ok := patchable[snakeCase(strings.TrimSpace(path))]
		if !ok {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Sprintf("update_mask path %q cannot be updated", path))
		}
		if v, set := body[field]; set {
			next[field] = v
		} else {
			delete(next, field)
		}
	}
	if _, ok := next["restrictedClientApplications"]; ok {
		if _, ok := next["scopedAccessSettings"]; ok {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "restricted_client_applications and scoped_access_settings cannot both be set")
		}
	}
	c.bindings[name] = next
	return c.done(req, name, "update", withType(next))
}

// done answers a done operation with response, and keeps it for a get.
func (c *Cloud) done(req *http.Request, name, verb string, response map[string]any) (*http.Response, error) {
	c.seq++
	opName := fmt.Sprintf("operations/gcpuab-%s-%d", verb, c.seq)
	op := map[string]any{"name": opName, "done": true, "response": response,
		"metadata": map[string]any{"@type": "type.googleapis.com/google.identity.accesscontextmanager.v1.GcpUserAccessBindingOperationMetadata"}}
	c.ops[opName] = op
	return jsonResponse(req, http.StatusOK, op)
}

func withType(b map[string]any) map[string]any {
	out := map[string]any{"@type": "type.googleapis.com/google.identity.accesscontextmanager.v1.GcpUserAccessBinding"}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// snakeCase is a mask path in the protos' spelling (sessionSettings and
// session_settings are the same field).
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func readJSON(req *http.Request, v any) error {
	if req.Body == nil {
		return nil
	}
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	return json.Unmarshal(b, v)
}

func notFound(req *http.Request, name string) (*http.Response, error) {
	return errorResponse(req, http.StatusNotFound, "NOT_FOUND", name+" not found")
}

func errorResponse(req *http.Request, code int, status, msg string) (*http.Response, error) {
	return jsonResponse(req, code, map[string]any{"error": map[string]any{"code": code, "status": status, "message": msg}})
}

func jsonResponse(req *http.Request, code int, v any) (*http.Response, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(b)),
		Request:    req,
	}, nil
}
