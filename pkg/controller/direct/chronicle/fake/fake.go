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

// Package fake is the fake cloud for the Chronicle API
// (<location>-chronicle.googleapis.com and chronicle.googleapis.com),
// which mockgcp does not serve: an in-memory store in front of mockgcp, as
// pkg/discokind/fake is, serving what ChronicleRuleDeployment's controller
// and test call: an instance's rules (create, get, delete) and a rule's
// deployment (get, patch with an update mask), which comes with the rule.
//
// What it cannot show (docs/fake-cloud.md): SecOps instances (any
// instance ID is taken), the rule language (a rule's text is kept, never
// parsed), the API's coercion of legacy run frequencies (LIVE and HOURLY
// are kept as sent), whether a run frequency suits the rule, chained
// rules (producerRules and consumerRules stay empty), detections and
// permissions.
package fake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// patchable are the deployment's fields an update mask names;
// scheduleCustomizations goes with runFrequency.
var patchable = map[string]bool{"enabled": true, "alerting": true, "archived": true, "runFrequency": true}

// Cloud is the fake. It answers the Chronicle API's requests and passes
// every other request to Next.
type Cloud struct {
	Next http.RoundTripper

	mu    sync.Mutex
	rules map[string]map[string]any // by name
	deps  map[string]map[string]any // by rule name
}

// New is the fake in front of next.
func New(next http.RoundTripper) *Cloud {
	return &Cloud{Next: next, rules: map[string]map[string]any{}, deps: map[string]map[string]any{}}
}

func isChronicle(host string) bool {
	return host == "chronicle.googleapis.com" || strings.HasSuffix(host, "-chronicle.googleapis.com")
}

func (c *Cloud) RoundTrip(req *http.Request) (*http.Response, error) {
	if !isChronicle(req.URL.Host) || !strings.HasPrefix(req.URL.Path, "/v1/") {
		return c.Next.RoundTrip(req)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	path := strings.TrimPrefix(req.URL.Path, "/v1/")
	parts := strings.Split(path, "/")
	// projects/p/locations/l/instances/i/rules[/r[/deployment]]
	if len(parts) < 7 || parts[0] != "projects" || parts[2] != "locations" || parts[4] != "instances" || parts[6] != "rules" {
		return errorResponse(req, http.StatusNotFound, "NOT_FOUND", "the Chronicle fake does not serve "+path)
	}
	switch {
	case len(parts) == 7 && req.Method == http.MethodPost:
		return c.createRule(req, path)
	case len(parts) == 8 && req.Method == http.MethodGet:
		r, ok := c.rules[path]
		if !ok {
			return notFound(req, path)
		}
		return jsonResponse(req, http.StatusOK, r)
	case len(parts) == 8 && req.Method == http.MethodDelete:
		if _, ok := c.rules[path]; !ok {
			return notFound(req, path)
		}
		delete(c.rules, path)
		delete(c.deps, path)
		return jsonResponse(req, http.StatusOK, map[string]any{})
	case len(parts) == 9 && parts[8] == "deployment" && req.Method == http.MethodGet:
		d, ok := c.deps[strings.TrimSuffix(path, "/deployment")]
		if !ok {
			return notFound(req, path)
		}
		return jsonResponse(req, http.StatusOK, d)
	case len(parts) == 9 && parts[8] == "deployment" && req.Method == http.MethodPatch:
		return c.patchDeployment(req, strings.TrimSuffix(path, "/deployment"))
	}
	return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("the Chronicle fake does not serve %s %s", req.Method, path))
}

func (c *Cloud) createRule(req *http.Request, parent string) (*http.Response, error) {
	body := map[string]any{}
	if err := readJSON(req, &body); err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	if text, _ := body["text"].(string); strings.TrimSpace(text) == "" {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "a rule needs text")
	}
	name := parent + "/ru_" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rule := map[string]any{"name": name, "text": body["text"], "revisionId": "v_" + fmt.Sprint(time.Now().UnixNano()), "revisionCreateTime": now}
	c.rules[name] = rule
	// A new rule's deployment: not running, not alerting, not archived.
	c.deps[name] = map[string]any{
		"name":           name + "/deployment",
		"enabled":        false,
		"alerting":       false,
		"archived":       false,
		"executionState": "DEFAULT",
	}
	return jsonResponse(req, http.StatusOK, rule)
}

func (c *Cloud) patchDeployment(req *http.Request, rule string) (*http.Response, error) {
	cur, ok := c.deps[rule]
	if !ok {
		return notFound(req, rule+"/deployment")
	}
	body := map[string]any{}
	if err := readJSON(req, &body); err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	var mask []string
	if m := req.URL.Query().Get("updateMask"); m != "" {
		mask = strings.Split(m, ",")
	} else {
		for f := range body {
			if patchable[f] {
				mask = append(mask, f)
			}
		}
	}
	next := map[string]any{}
	for k, v := range cur {
		next[k] = v
	}
	for _, f := range mask {
		if !patchable[f] {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Sprintf("invalid update mask field %q", f))
		}
		v, set := body[f]
		if f == "runFrequency" {
			if sc, ok := body["scheduleCustomizations"]; ok {
				next["scheduleCustomizations"] = sc
			} else {
				delete(next, "scheduleCustomizations")
			}
		}
		if !set {
			// The mask names a field the body leaves out: its default.
			delete(next, f)
			if f != "runFrequency" {
				next[f] = false
			}
			continue
		}
		next[f] = v
	}
	wasArchived := cur["archived"] == true
	if wasArchived && next["archived"] == true {
		for _, f := range mask {
			if f == "enabled" || f == "alerting" || f == "runFrequency" {
				return errorResponse(req, http.StatusBadRequest, "FAILED_PRECONDITION", fmt.Sprintf("cannot update %s of an archived rule deployment", f))
			}
		}
	}
	if !wasArchived && next["archived"] == true {
		if next["enabled"] == true {
			return errorResponse(req, http.StatusBadRequest, "FAILED_PRECONDITION", "cannot archive an enabled rule deployment; disable it first")
		}
		next["alerting"] = false
		next["archiveTime"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if wasArchived && next["archived"] != true {
		delete(next, "archiveTime")
	}
	if next["alerting"] != cur["alerting"] {
		next["lastAlertStatusChangeTime"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	c.deps[rule] = next
	return jsonResponse(req, http.StatusOK, next)
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
