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

// Package fake is the fake cloud for Service Management
// (servicemanagement.googleapis.com), which mockgcp does not serve: an
// in-memory store in front of mockgcp, as pkg/discokind/fake is, serving
// what EndpointsService's controller calls: services (create, get, list,
// delete, undelete), configs:submit, configs (get, list), rollouts (create) and
// the active configuration (services/{name}/config), with every operation
// finished at once.
//
// What it cannot show (docs/fake-cloud.md): the API's reading of the
// configuration files (it neither parses OpenAPI or gRPC configurations
// nor checks that the document's host is the service's name; a
// configuration's apis are empty and its endpoints are copied from an
// OpenAPI document's x-google-endpoints only), DNS, permissions, and the
// rollout's real propagation time.
package fake

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"sigs.k8s.io/yaml"
)

const host = "servicemanagement.googleapis.com"

// iamVerb matches a resource's IAM methods.
var iamVerb = regexp.MustCompile(`:(getIamPolicy|setIamPolicy|testIamPermissions)$`)

type service struct {
	managed  map[string]any
	deleted  bool
	configs  map[string]map[string]any // by ID
	order    []string
	active   string
	rollouts int
}

// Cloud is the fake. It answers Service Management's requests and passes
// every other request to Next.
type Cloud struct {
	Next http.RoundTripper

	mu       sync.Mutex
	services map[string]*service
	ops      map[string]map[string]any
}

// New is the fake in front of next.
func New(next http.RoundTripper) *Cloud {
	return &Cloud{Next: next, services: map[string]*service{}, ops: map[string]map[string]any{}}
}

func (c *Cloud) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != host || !strings.HasPrefix(req.URL.Path, "/v1/") || iamVerb.MatchString(req.URL.Path) {
		// Not Service Management's, or a service's IAM policy, which
		// mockgcp serves for every resource (patch 0018).
		return c.Next.RoundTrip(req)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	path := strings.TrimPrefix(req.URL.Path, "/v1/")
	parts := strings.Split(path, "/")
	switch {
	case len(parts) == 2 && parts[0] == "operations" && req.Method == http.MethodGet:
		op, ok := c.ops[path]
		if !ok {
			return errorResponse(req, http.StatusNotFound, "NOT_FOUND", "operation "+path+" not found")
		}
		return jsonResponse(req, http.StatusOK, op)
	case path == "services" && req.Method == http.MethodPost:
		return c.create(req)
	case path == "services" && req.Method == http.MethodGet:
		return c.list(req)
	case len(parts) == 2 && parts[0] == "services":
		name, verb, _ := strings.Cut(parts[1], ":")
		switch {
		case verb == "" && req.Method == http.MethodGet:
			s := c.live(name)
			if s == nil {
				return notFound(req, name)
			}
			return jsonResponse(req, http.StatusOK, s.managed)
		case verb == "" && req.Method == http.MethodDelete:
			s := c.live(name)
			if s == nil {
				return notFound(req, name)
			}
			s.deleted = true
			return c.doneOp(req, map[string]any{"@type": "type.googleapis.com/google.protobuf.Empty"})
		case verb == "undelete" && req.Method == http.MethodPost:
			s := c.services[name]
			if s == nil || !s.deleted {
				return errorResponse(req, http.StatusBadRequest, "FAILED_PRECONDITION", fmt.Sprintf("service %s is not deleted", name))
			}
			s.deleted = false
			return c.doneOp(req, map[string]any{"@type": "type.googleapis.com/google.api.servicemanagement.v1.UndeleteServiceResponse", "service": s.managed})
		}
	case len(parts) == 3 && parts[0] == "services" && parts[2] == "config" && req.Method == http.MethodGet:
		s := c.live(parts[1])
		if s == nil {
			return notFound(req, parts[1])
		}
		if s.active == "" {
			return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("service %s has no configuration rolled out", parts[1]))
		}
		return jsonResponse(req, http.StatusOK, s.configs[s.active])
	case len(parts) == 3 && parts[0] == "services" && parts[2] == "configs:submit" && req.Method == http.MethodPost:
		return c.submit(req, parts[1])
	case len(parts) == 3 && parts[0] == "services" && parts[2] == "configs" && req.Method == http.MethodGet:
		s := c.live(parts[1])
		if s == nil {
			return notFound(req, parts[1])
		}
		// Newest first, as the API lists them.
		out := []any{}
		for i := len(s.order) - 1; i >= 0; i-- {
			out = append(out, s.configs[s.order[i]])
		}
		return jsonResponse(req, http.StatusOK, map[string]any{"serviceConfigs": out})
	case len(parts) == 4 && parts[0] == "services" && parts[2] == "configs" && req.Method == http.MethodGet:
		s := c.live(parts[1])
		if s == nil || s.configs[parts[3]] == nil {
			return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("configuration %s of service %s not found", parts[3], parts[1]))
		}
		return jsonResponse(req, http.StatusOK, s.configs[parts[3]])
	case len(parts) == 3 && parts[0] == "services" && parts[2] == "rollouts" && req.Method == http.MethodPost:
		return c.rollout(req, parts[1])
	}
	return errorResponse(req, http.StatusNotImplemented, "UNIMPLEMENTED", fmt.Sprintf("the Service Management fake does not serve %s %s", req.Method, req.URL.Path))
}

func (c *Cloud) live(name string) *service {
	if s := c.services[name]; s != nil && !s.deleted {
		return s
	}
	return nil
}

func (c *Cloud) create(req *http.Request) (*http.Response, error) {
	body, err := readBody(req)
	if err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	name, _ := body["serviceName"].(string)
	project, _ := body["producerProjectId"].(string)
	if name == "" || project == "" {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "serviceName and producerProjectId are required")
	}
	if s := c.services[name]; s != nil {
		if s.deleted {
			return errorResponse(req, http.StatusConflict, "ALREADY_EXISTS", fmt.Sprintf("service %s was deleted less than 30 days ago; undelete it", name))
		}
		return errorResponse(req, http.StatusConflict, "ALREADY_EXISTS", fmt.Sprintf("service %s already exists", name))
	}
	managed := map[string]any{"serviceName": name, "producerProjectId": project}
	c.services[name] = &service{managed: managed, configs: map[string]map[string]any{}}
	resp := map[string]any{"@type": "type.googleapis.com/google.api.servicemanagement.v1.ManagedService"}
	for k, v := range managed {
		resp[k] = v
	}
	return c.doneOp(req, resp)
}

func (c *Cloud) list(req *http.Request) (*http.Response, error) {
	project := req.URL.Query().Get("producerProjectId")
	var names []string
	for n, s := range c.services {
		if !s.deleted && (project == "" || s.managed["producerProjectId"] == project) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := []any{}
	for _, n := range names {
		out = append(out, c.services[n].managed)
	}
	return jsonResponse(req, http.StatusOK, map[string]any{"services": out})
}

func (c *Cloud) submit(req *http.Request, name string) (*http.Response, error) {
	s := c.live(name)
	if s == nil {
		return notFound(req, name)
	}
	body, err := readBody(req)
	if err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	src, _ := body["configSource"].(map[string]any)
	files, _ := src["files"].([]any)
	if len(files) == 0 {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "configSource.files is required")
	}
	id := fmt.Sprintf("%sr%d", time.Now().UTC().Format("2006-01-02"), len(s.order))
	config := map[string]any{
		"name":              name,
		"id":                id,
		"producerProjectId": s.managed["producerProjectId"],
	}
	for _, f := range files {
		file, _ := f.(map[string]any)
		ft, _ := file["fileType"].(string)
		raw, _ := file["fileContents"].(string)
		contents, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "fileContents is not base64: "+err.Error())
		}
		if ft != "OPEN_API_YAML" && ft != "OPEN_API_JSON" {
			continue
		}
		doc := map[string]any{}
		if err := yaml.Unmarshal(contents, &doc); err != nil {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "the OpenAPI document does not parse: "+err.Error())
		}
		if info, ok := doc["info"].(map[string]any); ok {
			if t, ok := info["title"].(string); ok {
				config["title"] = t
			}
		}
		if eps, ok := doc["x-google-endpoints"].([]any); ok {
			var out []any
			for _, e := range eps {
				if m, ok := e.(map[string]any); ok {
					out = append(out, map[string]any{"name": m["name"], "target": m["target"]})
				}
			}
			config["endpoints"] = out
		}
	}
	s.configs[id] = config
	s.order = append(s.order, id)
	return c.doneOp(req, map[string]any{
		"@type":         "type.googleapis.com/google.api.servicemanagement.v1.SubmitConfigSourceResponse",
		"serviceConfig": config,
	})
}

func (c *Cloud) rollout(req *http.Request, name string) (*http.Response, error) {
	s := c.live(name)
	if s == nil {
		return notFound(req, name)
	}
	body, err := readBody(req)
	if err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	strategy, _ := body["trafficPercentStrategy"].(map[string]any)
	pct, _ := strategy["percentages"].(map[string]any)
	if len(pct) != 1 {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "the fake rolls out one configuration at 100 percent only")
	}
	var id string
	for k, v := range pct {
		if f, _ := v.(float64); f != 100 {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "the fake rolls out one configuration at 100 percent only")
		}
		id = k
	}
	if s.configs[id] == nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Sprintf("service %s has no configuration %s", name, id))
	}
	s.active = id
	s.rollouts++
	body["@type"] = "type.googleapis.com/google.api.servicemanagement.v1.Rollout"
	body["rolloutId"] = fmt.Sprintf("%sr%d", time.Now().UTC().Format("2006-01-02"), s.rollouts-1)
	body["serviceName"] = name
	body["status"] = "SUCCESS"
	return c.doneOp(req, body)
}

func (c *Cloud) doneOp(req *http.Request, response map[string]any) (*http.Response, error) {
	name := "operations/fake." + uuid.NewString()
	op := map[string]any{"name": name, "done": true, "response": response}
	c.ops[name] = op
	return jsonResponse(req, http.StatusOK, op)
}

func readBody(req *http.Request) (map[string]any, error) {
	if req.Body == nil {
		return map[string]any{}, nil
	}
	defer req.Body.Close()
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if len(bytes.TrimSpace(b)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("reading the request body: %w", err)
	}
	return out, nil
}

func jsonResponse(req *http.Request, code int, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
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

func notFound(req *http.Request, name string) (*http.Response, error) {
	return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("service %s not found", name))
}

func errorResponse(req *http.Request, code int, status, msg string) (*http.Response, error) {
	return jsonResponse(req, code, map[string]any{"error": map[string]any{"code": code, "status": status, "message": msg}})
}
