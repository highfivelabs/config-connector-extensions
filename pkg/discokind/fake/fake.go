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
// A kind's input-only fields (Kind.InputOnly and Kind.Secrets, such as a
// Looker instance's OAuth client secret) are kept but never answered, as
// the API does; Stored shows them to a test. A secret the API makes
// (Kind.SecretOutputs) is a random string; one the API returns only at
// create (Kind.CreateOnlyOutputs) is in the create's answer and no read's.
// A kind with Kind.DisableBeforeDelete refuses a delete, FAILED_PRECONDITION,
// until that field is true, as its API's description asks.
//
// BigLake (every kind under biglake.googleapis.com) and any kind with
// Kind.MissingIs403 answer 403 PERMISSION_DENIED, not 404, for a name the
// fake does not hold, as BigLake does for a missing catalog, database or
// table. The fake answers so for BigLake whatever the kind says, as the
// cloud does: a BigLake kind without missingIs403 fails its test. A list
// of a collection whose parent is such a resource the fake does not hold
// answers 403 too. Deleting a BigLake resource deletes the resources under
// its name with it (a catalog's databases and tables, a database's
// tables), as the cloud does (catalog repository run cat-11141549-4cb0);
// they then answer 403. SetDeny makes a name answer 403 while it is held:
// a real denial, which a delete must not take for a resource gone. Two
// switches, off by default, make deletes behave as API Gateway's do
// (SetDeleteDelay, SetRefuseInUse): an operation that takes a while, the
// resource still there until it ends, and a resource another one names
// (a config its gateway serves) or that holds others refused in the
// operation, FAILED_PRECONDITION, as the real API does.
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

	// deleteDelay and refuseInUse: SetDeleteDelay, SetRefuseInUse.
	deleteDelay time.Duration
	refuseInUse bool
	// deleting are the delete operations still running, by the
	// resource's key.
	deleting map[string]*pendingDelete
	// denied are names (BaseURL + name) every request for which answers
	// 403 (SetDeny).
	denied map[string]bool
}

// SetDeny makes every request for the resource name of k (its get, patch
// and delete) answer 403 PERMISSION_DENIED while on is set, whether or
// not the fake holds it: a real denial.
func (c *Cloud) SetDeny(k *discokind.Kind, name string, on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if on {
		c.denied[k.BaseURL+name] = true
	} else {
		delete(c.denied, k.BaseURL+name)
	}
}

// pendingDelete is a delete operation that ends at at: it then removes
// the resource or, when refused is set, fails with it.
type pendingDelete struct {
	at      time.Time
	op      map[string]any
	k       *discokind.Kind
	refused string
}

// Stored is the resource name of k as the fake holds it, input-only
// fields (Kind.InputOnly, Kind.Secrets) included, which a get leaves out;
// nil when it holds none. For tests of what a write sent.
func (c *Cloud) Stored(k *discokind.Kind, name string) map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	obj, ok := c.objects[k.BaseURL+name]
	if !ok {
		return nil
	}
	b, _ := json.Marshal(obj)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

// SetDeleteDelay makes a delete of a kind whose delete answers an
// operation (Kind.DeleteLRO, not Compute Engine's) take d: the operation
// is not done, and the resource is still there, until d has passed.
func (c *Cloud) SetDeleteDelay(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteDelay = d
}

// SetRefuseInUse makes a delete fail FAILED_PRECONDITION while another
// resource names it in a top-level field (a gateway's apiConfig) or holds
// resources under its name: in the delete's operation for a kind whose
// delete answers one (the operation fails, and the resource stays), else
// at once.
func (c *Cloud) SetRefuseInUse(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refuseInUse = on
}

// settle ends the delete operations whose time has come (c.mu held).
func (c *Cloud) settle() {
	now := time.Now()
	for key, p := range c.deleting {
		if now.Before(p.at) {
			continue
		}
		delete(c.deleting, key)
		p.op["done"] = true
		if p.refused != "" {
			p.op["error"] = map[string]any{"code": 9, "message": p.refused} // FAILED_PRECONDITION
			if obj, ok := c.objects[key]; ok && contains(p.k.Output, "state") {
				obj["state"] = "FAILED"
			}
			continue
		}
		p.op["response"] = map[string]any{}
		c.remove(p.k, key)
	}
}

// remove deletes the resource at key, or marks it deleted (SoftDelete)
// (c.mu held).
func (c *Cloud) remove(k *discokind.Kind, key string) {
	if k.SoftDelete != nil {
		if obj, ok := c.objects[key]; ok {
			obj[k.SoftDelete.Field] = k.SoftDelete.Value
		}
		return
	}
	delete(c.objects, key)
	c.cascade(k, key)
}

// cascade deletes the resources under key's name when k's API deletes
// them with their parent (cascadeAPIs) (c.mu held).
func (c *Cloud) cascade(k *discokind.Kind, key string) {
	if !cascades(k) {
		return
	}
	for other := range c.objects {
		if strings.HasPrefix(other, key+"/") {
			delete(c.objects, other)
		}
	}
}

// cascadeAPIs are the APIs whose delete of a resource deletes the
// resources under its name with it: BigLake's catalog delete takes its
// databases and tables (catalog repository run cat-11141549-4cb0).
var cascadeAPIs = []string{"https://biglake.googleapis.com/"}

func cascades(k *discokind.Kind) bool {
	for _, base := range cascadeAPIs {
		if strings.HasPrefix(k.BaseURL, base) {
			return true
		}
	}
	return false
}

// inUse says why the resource at key cannot go: another resource names it
// in a top-level field, or holds resources under its name; "" when
// neither (c.mu held).
func (c *Cloud) inUse(k *discokind.Kind, key, name string) string {
	var keys []string
	for other := range c.objects {
		keys = append(keys, other)
	}
	sort.Strings(keys)
	for _, other := range keys {
		if other == key {
			continue
		}
		if strings.HasPrefix(other, key+"/") {
			return fmt.Sprintf("%s has nested resources", name)
		}
		for _, v := range c.objects[other] {
			if s, ok := v.(string); ok && s == name {
				user, _ := c.objects[other]["name"].(string)
				return fmt.Sprintf("%s is in use by %s", name, user)
			}
		}
	}
	return ""
}

// New is a fake for kinds in front of next.
func New(next http.RoundTripper, kinds []*discokind.Kind) *Cloud {
	return &Cloud{Next: next, Kinds: kinds, objects: map[string]map[string]any{}, ops: map[string]map[string]any{}, deleting: map[string]*pendingDelete{}, denied: map[string]bool{}}
}

const opPrefix = "operations/discokind-"

func (c *Cloud) RoundTrip(req *http.Request) (*http.Response, error) {
	full := req.URL.Scheme + "://" + req.URL.Host + req.URL.Path
	c.mu.Lock()
	c.settle()
	c.mu.Unlock()
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
	case k.NameInBody:
		// No ID parameter: the ID is the last segment of the body's name.
		if got, _ := body["name"].(string); strings.HasPrefix(got, coll+"/") {
			id = got[len(coll)+1:]
		}
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
		return c.doneOp(req, k, name, k.Comparable(body))
	}
	return jsonResponse(req, http.StatusOK, k.Comparable(body))
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
		if contains(k.SecretOutputs, f) {
			// A secret the API makes (an OAuth client credential's
			// clientSecret, a SCIM token's securityToken).
			body[f] = "fake-" + f + "-" + uuid.NewString()
			continue
		}
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

// readable is what a read of obj answers: obj without its input-only
// fields (Kind.InputOnly, Kind.Secrets) and without the secrets the API
// returns only in a create's answer (Kind.CreateOnlyOutputs).
func readable(k *discokind.Kind, obj map[string]any) map[string]any {
	out := k.Comparable(obj)
	if len(k.CreateOnlyOutputs) == 0 {
		return out
	}
	if len(k.InputOnly)+len(k.Secrets) == 0 {
		out = copyMap(out)
	}
	for _, f := range k.CreateOnlyOutputs {
		delete(out, f)
	}
	return out
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// missing403APIs are the APIs that answer 403, not 404, for a resource
// that does not exist, whichever kind serves them.
var missing403APIs = []string{"https://biglake.googleapis.com/"}

// missingIs403: the fake answers k's missing names 403 (the package doc).
func missingIs403(k *discokind.Kind) bool {
	if k.MissingIs403 {
		return true
	}
	for _, base := range missing403APIs {
		if strings.HasPrefix(k.BaseURL, base) {
			return true
		}
	}
	return false
}

// missing403Resource: name is a resource of one of the fake's kinds at
// k's API that answers 403 when missing (a database's catalog).
func (c *Cloud) missing403Resource(k *discokind.Kind, name string) bool {
	if name == "" {
		return false
	}
	for _, other := range c.Kinds {
		if other.BaseURL == k.BaseURL && missingIs403(other) && other.NameRegexp().MatchString(name) {
			return true
		}
	}
	return false
}

func (c *Cloud) serveObject(req *http.Request, k *discokind.Kind, name string) (*http.Response, error) {
	key := k.BaseURL + name
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.denied[key] {
		return errorResponse(req, http.StatusForbidden, "PERMISSION_DENIED", fmt.Sprintf("Permission denied on resource '%s' (or it may not exist).", name))
	}
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
			return c.doneOp(req, k, name, k.Comparable(obj))
		}
		return jsonResponse(req, http.StatusOK, k.Comparable(obj))
	}
	if !ok && missingIs403(k) {
		// As BigLake answers for a catalog, database or table that does not
		// exist, even to an admin (the catalog's live runs).
		return errorResponse(req, http.StatusForbidden, "PERMISSION_DENIED", fmt.Sprintf("Permission denied on resource '%s' (or it may not exist).", name))
	}
	if !ok {
		return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("%s was not found", name))
	}
	deleted := k.SoftDelete != nil && obj[k.SoftDelete.Field] == k.SoftDelete.Value
	switch req.Method {
	case http.MethodGet:
		// Input-only fields (Kind.InputOnly, Kind.Secrets) are kept but
		// never answered, as the API does (Stored shows them); nor are the
		// secrets only a create answers (Kind.CreateOnlyOutputs).
		return jsonResponse(req, http.StatusOK, readable(k, obj))
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
			return c.doneOp(req, k, name, readable(k, updated))
		}
		return jsonResponse(req, http.StatusOK, readable(k, updated))
	case http.MethodDelete:
		if deleted {
			return errorResponse(req, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("%s was not found", name))
		}
		if p, ok := c.deleting[key]; ok {
			// Already being deleted: the same operation.
			return jsonResponse(req, http.StatusOK, p.op)
		}
		if f := k.DisableBeforeDelete; f != "" && obj[f] != true {
			// As the API's description asks ("Before deleting an
			// OauthClientCredential, it should first be disabled").
			return errorResponse(req, http.StatusBadRequest, "FAILED_PRECONDITION", fmt.Sprintf("%s must be disabled (%s: true) before it is deleted", name, f))
		}
		refused := ""
		if c.refuseInUse {
			refused = c.inUse(k, key, name)
		}
		if k.DeleteLRO && !k.InsertStyle && (c.deleteDelay > 0 || refused != "") {
			// The operation runs for deleteDelay; the resource stays until it
			// ends, and goes then unless the delete was refused (settle).
			op := map[string]any{"name": opPrefix + uuid.NewString(), "done": false}
			c.ops[k.BaseURL+op["name"].(string)] = op
			c.deleting[key] = &pendingDelete{at: time.Now().Add(c.deleteDelay), op: op, k: k, refused: refused}
			return jsonResponse(req, http.StatusOK, op)
		}
		if refused != "" {
			return errorResponse(req, http.StatusBadRequest, "FAILED_PRECONDITION", refused)
		}
		answer := map[string]any{}
		if k.SoftDelete != nil {
			obj[k.SoftDelete.Field] = k.SoftDelete.Value
			answer = readable(k, obj)
		} else {
			delete(c.objects, key)
			c.cascade(k, key)
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
	if parent := discokind.CollectionOf(coll); c.missing403Resource(k, parent) && c.objects[k.BaseURL+parent] == nil {
		// As BigLake answers a list of a deleted catalog's databases.
		return errorResponse(req, http.StatusForbidden, "PERMISSION_DENIED", fmt.Sprintf("Permission denied on resource '%s' (or it may not exist).", parent))
	}
	var items []any
	var keys []string
	for key := range c.objects {
		if strings.HasPrefix(key, k.BaseURL+coll+"/") && !strings.Contains(strings.TrimPrefix(key, k.BaseURL+coll+"/"), "/") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		items = append(items, readable(k, c.objects[key]))
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
	c.settle()
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
