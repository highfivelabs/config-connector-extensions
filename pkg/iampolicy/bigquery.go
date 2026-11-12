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

// A BigQuery dataset has no getIamPolicy or setIamPolicy: its policy is
// the dataset's access list (datasets.get and datasets.patch of the
// BigQuery API v2), one entry per member and role, as Terraform's
// google_bigquery_dataset_iam_* resources read and write it (read, not
// copied). Here the access list is shown to the IAM kinds as a policy:
//
//   - the dataset's legacy roles READER, WRITER and OWNER are
//     roles/bigquery.dataViewer, dataEditor and dataOwner (the API
//     answers an IAM role it was sent with its legacy name);
//   - userByEmail is user:, or serviceAccount: for a service account's
//     address; groupByEmail is group:; domain is domain:; specialGroup
//     allAuthenticatedUsers is allAuthenticatedUsers; iamMember is the
//     member as written (allUsers, principal://...);
//   - an entry's condition is the binding's;
//   - entries that are not a member (an authorized view, routine or
//     dataset) and the other special groups (projectOwners, projectReaders,
//     projectWriters) are not in the policy and are kept as they are.
//
// A write reads the dataset again, refuses with 412 when its etag is not
// the policy's (Modify then reads again), and patches the access list
// with If-Match, so a concurrent change by another writer is refused by
// the API. Within one manager, Modify also takes a lock per dataset
// (lockDataset): the read, the etag check and the patch are not atomic,
// and two of our objects on one dataset otherwise race between the check
// and the patch (the fake, which ignores If-Match, lost one member that
// way in CI).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/api/googleapi"
	"google.golang.org/genproto/googleapis/type/expr"
)

var bigqueryLegacyRoles = map[string]string{
	"READER": "roles/bigquery.dataViewer",
	"WRITER": "roles/bigquery.dataEditor",
	"OWNER":  "roles/bigquery.dataOwner",
}

type bigqueryDataset struct {
	Etag   string           `json:"etag"`
	Access []map[string]any `json:"access"`
}

// bigqueryMember is the IAM member of an access entry, or false for an
// entry that is not one.
func bigqueryMember(e map[string]any) (string, bool) {
	str := func(k string) string { s, _ := e[k].(string); return s }
	switch {
	case str("userByEmail") != "":
		email := str("userByEmail")
		if strings.HasSuffix(strings.ToLower(email), ".gserviceaccount.com") {
			return "serviceAccount:" + email, true
		}
		return "user:" + email, true
	case str("groupByEmail") != "":
		return "group:" + str("groupByEmail"), true
	case str("domain") != "":
		return "domain:" + str("domain"), true
	case str("specialGroup") == "allAuthenticatedUsers":
		return "allAuthenticatedUsers", true
	case str("iamMember") != "":
		return str("iamMember"), true
	}
	return "", false
}

// bigqueryEntry is the access entry for a member and role.
func bigqueryEntry(member, role string, cond *expr.Expr) map[string]any {
	e := map[string]any{"role": role}
	kind, value, _ := strings.Cut(member, ":")
	switch kind {
	case "user", "serviceAccount":
		e["userByEmail"] = value
	case "group":
		e["groupByEmail"] = value
	case "domain":
		e["domain"] = value
	case "allAuthenticatedUsers":
		e["specialGroup"] = "allAuthenticatedUsers"
	default:
		e["iamMember"] = member
	}
	if cond != nil {
		c := map[string]any{"expression": cond.GetExpression()}
		if cond.GetTitle() != "" {
			c["title"] = cond.GetTitle()
		}
		if cond.GetDescription() != "" {
			c["description"] = cond.GetDescription()
		}
		e["condition"] = c
	}
	return e
}

func bigqueryCondition(e map[string]any) *expr.Expr {
	c, ok := e["condition"].(map[string]any)
	if !ok {
		return nil
	}
	str := func(k string) string { s, _ := c[k].(string); return s }
	return &expr.Expr{Expression: str("expression"), Title: str("title"), Description: str("description")}
}

// bigqueryPolicy is the access list as a policy (version 3: entries may
// carry conditions).
func bigqueryPolicy(d *bigqueryDataset) *iampb.Policy {
	p := &iampb.Policy{Version: 3, Etag: []byte(d.Etag)}
	for _, e := range d.Access {
		member, ok := bigqueryMember(e)
		if !ok {
			continue
		}
		role, _ := e["role"].(string)
		if r, legacy := bigqueryLegacyRoles[role]; legacy {
			role = r
		}
		cond := bigqueryCondition(e)
		var b *iampb.Binding
		for _, have := range p.Bindings {
			if have.Role == role && sameExpr(have.Condition, cond) {
				b = have
				break
			}
		}
		if b == nil {
			b = &iampb.Binding{Role: role, Condition: cond}
			p.Bindings = append(p.Bindings, b)
		}
		b.Members = append(b.Members, member)
	}
	return p
}

func sameExpr(a, b *expr.Expr) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.GetExpression() == b.GetExpression() && a.GetTitle() == b.GetTitle() && a.GetDescription() == b.GetDescription()
}

func (c *Client) bigqueryRead(ctx context.Context, u string) (*bigqueryDataset, error) {
	out, err := c.do(ctx, http.MethodGet, u+"?accessPolicyVersion=3", nil)
	if err != nil {
		return nil, err
	}
	d := &bigqueryDataset{}
	if err := json.Unmarshal(out, d); err != nil {
		return nil, fmt.Errorf("reading the dataset's access list: %w", err)
	}
	return d, nil
}

func (c *Client) bigqueryGet(ctx context.Context, u string) (*iampb.Policy, error) {
	d, err := c.bigqueryRead(ctx, u)
	if err != nil {
		return nil, err
	}
	return bigqueryPolicy(d), nil
}

func (c *Client) bigquerySet(ctx context.Context, u string, p *iampb.Policy) (*iampb.Policy, error) {
	d, err := c.bigqueryRead(ctx, u)
	if err != nil {
		return nil, err
	}
	if d.Etag != string(p.GetEtag()) {
		return nil, &googleapi.Error{Code: http.StatusPreconditionFailed, Message: "the dataset changed since its access list was read"}
	}
	access := []map[string]any{}
	for _, e := range d.Access {
		if _, member := bigqueryMember(e); !member {
			access = append(access, e)
		}
	}
	for _, b := range p.GetBindings() {
		for _, m := range b.GetMembers() {
			access = append(access, bigqueryEntry(m, b.GetRole(), b.GetCondition()))
		}
	}
	body, err := json.Marshal(map[string]any{"access": access})
	if err != nil {
		return nil, err
	}
	out, err := c.doHeader(ctx, http.MethodPatch, u+"?accessPolicyVersion=3", body, map[string]string{"If-Match": d.Etag})
	if err != nil {
		return nil, err
	}
	nd := &bigqueryDataset{}
	if err := json.Unmarshal(out, nd); err != nil {
		return nil, fmt.Errorf("reading the dataset's access list: %w", err)
	}
	return bigqueryPolicy(nd), nil
}

var datasetLocks sync.Map // dataset name -> *sync.Mutex

// lockDataset locks the dataset's access list for one read-modify-write
// and returns the unlock.
func lockDataset(name string) func() {
	m, _ := datasetLocks.LoadOrStore(name, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
