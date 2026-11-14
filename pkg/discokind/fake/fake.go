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

// Package fake is the fake cloud for discokind kinds: one in-memory
// store that serves every registered kind's collection as a standard
// resource API (create with an ID, get, list, patch with an update mask,
// delete), in front of mockgcp. It is generated from the same Kind
// descriptors as the controller, so a new kind needs no mockgcp patch.
//
// What it cannot show (docs/testing.md): the API's validation (required
// fields, enum values, cross-field rules), defaults the API fills in
// beyond Kind.FakeDefaults, and quotas. Those are what a live run is for.
package fake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
)

// Cloud is the fake. It answers requests for the kinds' collections and
// operations and passes every other request to Next.
type Cloud struct {
	Next  http.RoundTripper
	Kinds []*discokind.Kind

	mu      sync.Mutex
	objects map[string]map[string]any // by BaseURL + name
	ops     map[string]map[string]any // by BaseURL + operation name
}

// New is a fake for kinds in front of next.
func New(next http.RoundTripper, kinds []*discokind.Kind) *Cloud {
	return &Cloud{Next: next, Kinds: kinds, objects: map[string]map[string]any{}, ops: map[string]map[string]any{}}
}

const opPrefix = "operations/discokind-"

func (c *Cloud) RoundTrip(req *http.Request) (*http.Response, error) {
	full := req.URL.Scheme + "://" + req.URL.Host + req.URL.Path
	if req.Method == http.MethodGet {
		c.mu.Lock()
		_, isOp := c.ops[full]
		c.mu.Unlock()
		if isOp {
			return c.getOp(req, full)
		}
	}
	// A custom method on an object (name:verb, such as :getIamPolicy and
	// :setIamPolicy) is not the standard collection's: the fake does not
	// serve it, and mockgcp serves the IAM verbs for every resource
	// (patch 0018). Without this, a kind's name pattern matched
	// ".../lakes/l:getIamPolicy" as an object called "l:getIamPolicy" and
	// answered 404.
	if last := full[strings.LastIndex(full, "/")+1:]; strings.Contains(last, ":") {
		return c.Next.RoundTrip(req)
	}
	for _, k := range c.Kinds {
		path, ok, wrongHost := kindPath(k, full)
		if !ok {
			continue
		}
		if wrongHost {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Sprintf("Incorrect API endpoint used: %s is served at its location's host", path))
		}
		if strings.HasPrefix(path, opPrefix) && req.Method == http.MethodGet {
			return c.getOp(req, k.BaseURL+path)
		}
		re := k.NameRegexp()
		switch {
		case re.MatchString(path):
			return c.serveObject(req, k, path)
		case req.Method == http.MethodPost && re.MatchString(path+"/x"):
			return c.create(req, k, path)
		case req.Method == http.MethodGet && re.MatchString(path+"/x"):
			return c.list(req, k, path)
		}
	}
	return c.Next.RoundTrip(req)
}

func (c *Cloud) create(req *http.Request, k *discokind.Kind, coll string) (*http.Response, error) {
	body, err := readBody(req)
	if err != nil {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	}
	id := ""
	switch {
	case k.InsertStyle:
		id, _ = body["name"].(string)
	case k.IDParam != "":
		id = req.URL.Query().Get(k.IDParam)
	}
	if id == "" {
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", "the resource's ID is required")
	}
	name := coll + "/" + id
	if got, _ := body["name"].(string); k.NameInBody && got != name {
		// As Cloud Functions v2 answers a create without the name in the
		// body (the catalog's live run in hfl-dev-514205).
		return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Sprintf("Invalid resource name '%s' for pattern %s", got, k.Name))
	}
	key := k.BaseURL + name
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.objects[key]; ok {
		if k.SoftDelete != nil && old[k.SoftDelete.Field] == k.SoftDelete.Value {
			return errorResponse(req, http.StatusConflict, "ALREADY_EXISTS", fmt.Sprintf("%s was deleted and its ID cannot be reused yet", name))
		}
		return errorResponse(req, http.StatusConflict, "ALREADY_EXISTS", fmt.Sprintf("%s already exists", name))
	}
	body = newObject(k, name, id, body)
	c.objects[key] = body
	if k.CreateLRO {
		return c.doneOp(req, k, name, body)
	}
	return jsonResponse(req, http.StatusOK, body)
}

// kindPath is full's path under k's BaseURL, and whether full is k's. A
// LocationHost kind's location other than global is served only at
// https://<location>-<host>/ (discokind.LocationBaseURL): at another host
// the real API refuses it, and so does the fake (wrongHost).
func kindPath(k *discokind.Kind, full string) (path string, ok, wrongHost bool) {
	if strings.HasPrefix(full, k.BaseURL) {
		path = strings.TrimPrefix(full, k.BaseURL)
		return path, true, k.LocationHost && discokind.LocationBaseURL(k.BaseURL, path) != k.BaseURL
	}
	if !k.LocationHost || !strings.HasPrefix(k.BaseURL, "https://") {
		return "", false, false
	}
	rest := strings.TrimPrefix(full, "https://")
	loc, hostRest, found := strings.Cut(rest, "-")
	if !found || !strings.HasPrefix("https://"+hostRest, k.BaseURL) {
		return "", false, false
	}
	path = strings.TrimPrefix("https://"+hostRest, k.BaseURL)
	return path, true, discokind.LocationOf(path) != loc
}

// newObject is the stored resource a create (or, for a PatchCreates kind,
// a patch of a missing name) makes from its body.
func newObject(k *discokind.Kind, name, id string, body map[string]any) map[string]any {
	for _, f := range k.Output {
		delete(body, f)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for f, v := range k.FakeDefaults {
		body[f] = v
	}
	body["name"] = name
	if k.InsertStyle {
		// Compute Engine: name is the ID; the URL is selfLink.
		body["name"] = id
	}
	for _, f := range k.Output {
		switch f {
		case "createTime", "updateTime":
			body[f] = now
		case "creationTimestamp":
			body[f] = now
		case "uid":
			body[f] = uuid.NewString()
		case "id":
			body[f] = fmt.Sprintf("%d", uuid.New().ID())
		case "selfLink":
			body[f] = k.BaseURL + name
		case "etag", "fingerprint":
			body[f] = etag(body)
		}
	}
	return body
}

func (c *Cloud) serveObject(req *http.Request, k *discokind.Kind, name string) (*http.Response, error) {
	key := k.BaseURL + name
	c.mu.Lock()
	defer c.mu.Unlock()
	obj, ok := c.objects[key]
	if !ok && k.PatchCreates && req.Method == http.MethodPatch {
		// No create method: patch makes the resource.
		body, err := readBody(req)
		if err != nil {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		}
		obj = newObject(k, name, name[strings.LastIndex(name, "/")+1:], body)
		c.objects[key] = obj
		if k.UpdateLRO {
			return c.doneOp(req, k, name, obj)
		}
		return jsonResponse(req, http.StatusOK, obj)
	}
	if !ok {
		return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("%s was not found", name))
	}
	deleted := k.SoftDelete != nil && obj[k.SoftDelete.Field] == k.SoftDelete.Value
	switch req.Method {
	case http.MethodGet:
		return jsonResponse(req, http.StatusOK, obj)
	case http.MethodPatch, http.MethodPut:
		if !k.Updatable || (req.Method == http.MethodPut) != k.UpdatePUT {
			return errorResponse(req, http.StatusMethodNotAllowed, "UNIMPLEMENTED", "no patch method")
		}
		if deleted {
			return errorResponse(req, http.StatusBadRequest, "FAILED_PRECONDITION", fmt.Sprintf("%s is deleted", name))
		}
		body, err := readBody(req)
		if err != nil {
			return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		}
		if k.InsertStyle {
			// name and fingerprint come back as read; a stale
			// fingerprint is refused, as Compute Engine does.
			if fp, ok := obj["fingerprint"]; ok && body["fingerprint"] != nil && body["fingerprint"] != fp {
				return errorResponse(req, http.StatusPreconditionFailed, "FAILED_PRECONDITION", "the fingerprint is stale")
			}
			delete(body, "fingerprint")
			delete(body, "name")
			for _, f := range k.Output {
				delete(body, f)
			}
		}
		var mask []string
		if m := req.URL.Query().Get("updateMask"); m != "" {
			mask = strings.Split(m, ",")
		} else {
			for f := range body {
				mask = append(mask, f)
			}
		}
		updated := map[string]any{}
		for f, v := range obj {
			// update (PUT) replaces every writable field.
			if req.Method == http.MethodPut && !contains(k.Output, f) && f != "name" {
				continue
			}
			updated[f] = v
		}
		for _, f := range mask {
			f = strings.SplitN(f, ".", 2)[0]
			if contains(k.Output, f) || f == "name" {
				return errorResponse(req, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Sprintf("%s is output only", f))
			}
			if v, ok := body[f]; ok {
				updated[f] = v
			} else {
				delete(updated, f)
			}
		}
		if contains(k.Output, "updateTime") {
			updated["updateTime"] = time.Now().UTC().Format(time.RFC3339Nano)
		}
		for _, f := range []string{"etag", "fingerprint"} {
			if contains(k.Output, f) {
				updated[f] = etag(updated)
			}
		}
		c.objects[key] = updated
		if k.UpdateLRO {
			return c.doneOp(req, k, name, updated)
		}
		return jsonResponse(req, http.StatusOK, updated)
	case http.MethodDelete:
		if deleted {
			return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("%s was not found", name))
		}
		answer := map[string]any{}
		if k.SoftDelete != nil {
			obj[k.SoftDelete.Field] = k.SoftDelete.Value
			answer = obj
		} else {
			delete(c.objects, key)
		}
		if k.DeleteLRO {
			return c.doneOp(req, k, name, map[string]any{})
		}
		return jsonResponse(req, http.StatusOK, answer)
	}
	return errorResponse(req, http.StatusMethodNotAllowed, "UNIMPLEMENTED", req.Method+" is not served")
}

func (c *Cloud) list(req *http.Request, k *discokind.Kind, coll string) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var items []any
	var keys []string
	for key := range c.objects {
		if strings.HasPrefix(key, k.BaseURL+coll+"/") && !strings.Contains(strings.TrimPrefix(key, k.BaseURL+coll+"/"), "/") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		items = append(items, c.objects[key])
	}
	field := coll[strings.LastIndex(coll, "/")+1:]
	if k.InsertStyle {
		field = "items"
	}
	out := map[string]any{}
	if len(items) > 0 {
		out[field] = items
	}
	return jsonResponse(req, http.StatusOK, out)
}

// doneOp answers a finished operation on the resource name (c.mu held):
// a google.longrunning one, or for an InsertStyle kind a Compute Engine
// one in the resource's scope (projects/p/regions/r/operations/...).
func (c *Cloud) doneOp(req *http.Request, k *discokind.Kind, name string, response map[string]any) (*http.Response, error) {
	if k.InsertStyle {
		opName := "operation-discokind-" + uuid.NewString()
		self := k.BaseURL + discokind.ParentOf(name) + "/operations/" + opName
		op := map[string]any{"kind": "compute#operation", "name": opName, "status": "DONE", "progress": 100,
			"selfLink": self, "targetLink": k.BaseURL + name}
		c.ops[self] = op
		return jsonResponse(req, http.StatusOK, op)
	}
	opName := opPrefix + uuid.NewString()
	op := map[string]any{"name": opName, "done": true, "response": response}
	c.ops[k.BaseURL+opName] = op
	return jsonResponse(req, http.StatusOK, op)
}

func (c *Cloud) getOp(req *http.Request, key string) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	op, ok := c.ops[key]
	if !ok {
		return errorResponse(req, http.StatusNotFound, "NOT_FOUND", "operation not found")
	}
	return jsonResponse(req, http.StatusOK, op)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func etag(obj map[string]any) string {
	c := map[string]any{}
	for k, v := range obj {
		if k != "etag" {
			c[k] = v
		}
	}
	b, _ := json.Marshal(c)
	return "W/\"" + uuid.NewSHA1(uuid.Nil, b).String() + "\""
}

func readBody(req *http.Request) (map[string]any, error) {
	out := map[string]any{}
	if req.Body == nil {
		return out, nil
	}
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("the body is not a JSON object: %w", err)
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
		Proto:      "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
	}, nil
}

func errorResponse(req *http.Request, code int, status, msg string) (*http.Response, error) {
	return jsonResponse(req, code, map[string]any{"error": map[string]any{"code": code, "message": msg, "status": status}})
}
