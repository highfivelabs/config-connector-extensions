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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/api/googleapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// Client reads and writes IAM policies over HTTP, with whatever
// authentication its http.Client carries (Application Default Credentials
// in a cluster or on a VM, the fake cloud's round tripper in a test).
type Client struct {
	HTTP *http.Client
	// Retries is how many times a read-modify-write is repeated when
	// another writer changed the policy under it (default 5).
	Retries int
}

func (c *Client) url(t *ResourceType, name string, verb string) (string, error) {
	host := t.Endpoint(name)
	switch t.Protocol {
	case IAMv1:
		return "https://" + host + "/" + t.Version + "/" + name + ":" + verb, nil
	case Storage:
		// projects/_/buckets/{bucket}, or
		// projects/_/buckets/{bucket}/managedFolders/{folder}, whose name
		// ends with a slash and is escaped whole.
		rest := strings.TrimPrefix(name, "projects/_/buckets/")
		bucket, folder, isFolder := strings.Cut(rest, "/managedFolders/")
		u := "https://" + host + "/storage/v1/b/" + url.PathEscape(bucket)
		if isFolder {
			u += "/managedFolders/" + url.PathEscape(strings.TrimSuffix(folder, "/")+"/")
		}
		return u + "/iam", nil
	case Compute:
		return "https://" + host + "/compute/" + t.Version + "/" + name + "/" + verb, nil
	}
	return "", fmt.Errorf("unknown protocol %d for %s", t.Protocol, t.Kind)
}

func (c *Client) do(ctx context.Context, method, u string, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := googleapi.CheckResponse(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// Get reads the resource's IAM policy, with conditions (version 3).
func (c *Client) Get(ctx context.Context, t *ResourceType, name string) (*iampb.Policy, error) {
	u, err := c.url(t, name, "getIamPolicy")
	if err != nil {
		return nil, err
	}
	var out []byte
	switch {
	case t.Protocol == Storage || t.Protocol == Compute:
		out, err = c.do(ctx, http.MethodGet, u+"?optionsRequestedPolicyVersion=3", nil)
	case t.GetWithPost:
		out, err = c.do(ctx, http.MethodPost, u, []byte(`{"options":{"requestedPolicyVersion":3}}`))
	default:
		out, err = c.do(ctx, http.MethodGet, u+"?options.requestedPolicyVersion=3", nil)
	}
	if err != nil {
		return nil, err
	}
	return decode(out)
}

// Set writes the policy. Its etag makes the write fail with a conflict if
// another writer got there first.
func (c *Client) Set(ctx context.Context, t *ResourceType, name string, p *iampb.Policy) (*iampb.Policy, error) {
	u, err := c.url(t, name, "setIamPolicy")
	if err != nil {
		return nil, err
	}
	pj, err := protojson.Marshal(p)
	if err != nil {
		return nil, err
	}
	var out []byte
	if t.Protocol == Storage {
		out, err = c.do(ctx, http.MethodPut, u, pj)
	} else {
		out, err = c.do(ctx, http.MethodPost, u, []byte(`{"policy":`+string(pj)+`}`))
	}
	if err != nil {
		return nil, err
	}
	return decode(out)
}

func decode(b []byte) (*iampb.Policy, error) {
	p := &iampb.Policy{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, p); err != nil {
		return nil, fmt.Errorf("reading the IAM policy: %w", err)
	}
	return p, nil
}

// IsNotFound reports whether the error is the API saying the resource does
// not exist.
func IsNotFound(err error) bool { return hasCode(err, http.StatusNotFound) }

func isConflict(err error) bool {
	return hasCode(err, http.StatusConflict) || hasCode(err, http.StatusPreconditionFailed)
}

func hasCode(err error, code int) bool {
	var ge *googleapi.Error
	return errors.As(err, &ge) && ge.Code == code
}

// Modify reads the policy, lets change edit it, and writes it back with the
// etag it read, only if change reports a change. When another writer
// changed the policy meanwhile, it reads again and repeats. It reports
// whether it wrote. A resource that does not exist is IsNotFound.
func (c *Client) Modify(ctx context.Context, t *ResourceType, name string, change func(*iampb.Policy) bool) (bool, error) {
	attempts := c.Retries
	if attempts <= 0 {
		attempts = 5
	}
	for i := 1; ; i++ {
		p, err := c.Get(ctx, t, name)
		if err != nil {
			return false, fmt.Errorf("reading the IAM policy of %s: %w", name, err)
		}
		if !change(p) {
			return false, nil
		}
		_, err = c.Set(ctx, t, name, p)
		if err == nil {
			return true, nil
		}
		if i < attempts && isConflict(err) {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(time.Duration(i) * 200 * time.Millisecond):
			}
			continue
		}
		return false, fmt.Errorf("setting the IAM policy of %s: %w", name, err)
	}
}
