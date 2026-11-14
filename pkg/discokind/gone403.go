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
	"fmt"
	"net/http"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
)

// deleteGoneOn403 turns on what GoneAfter403 is for. It is off only in a
// test build with the tag cce_delete403_off (gone403_off.go), which shows
// the tests that need it fail without it (docs/design.md, "A 403 while
// deleting").
var deleteGoneOn403 = true

// goneOn403: err is a 403 that, for this kind, may mean the resource is
// gone (Kind.MissingIs403), so a delete looks further (GoneAfter403).
func (a *adapter) goneOn403(err error) bool {
	return deleteGoneOn403 && a.kind.MissingIs403 && direct.HasHTTPCode(err, http.StatusForbidden)
}

// maxListPages bounds the pages GoneAfter403 reads of one collection.
const maxListPages = 50

// GoneAfter403 says whether the resource a delete was answered 403 for
// (Kind.MissingIs403: BigLake answers 403 for a catalog, database or table
// that does not exist, even to an admin) is proven gone, and why.
//
// A 403 alone cannot tell a missing resource from a real denial, so the
// proof is a listing that succeeds: the resource's collection is listed,
// and if the resource is not in it, it is gone. When the listing answers
// 403 too, the parent may be gone (a catalog's delete takes its databases
// and tables along, catalog repository run cat-11141549-4cb0), so the
// parent's own collection is listed in the same way, and so on up while
// the parent is a resource of a MissingIs403 kind of the same API. The
// resource is gone when an ancestor's collection lists without that
// ancestor (or without the resource itself). It is not gone, and the 403
// stays a denial, when a listing shows the resource or the ancestor still
// there, or when every listing up to the top answers 403. A listing that
// fails otherwise is returned as an error. An item with deleteTime set,
// or soft-deleted (Kind.SoftDelete), counts as gone.
func (a *adapter) GoneAfter403(ctx context.Context) (bool, string, error) {
	name := a.name
	for {
		coll := CollectionOf(name)
		if coll == "" {
			return false, "no collection to list for " + name, nil
		}
		present, err := a.listed(ctx, coll, name)
		switch {
		case err == nil && present:
			if name == a.name {
				return false, "listing " + coll + " shows " + name, nil
			}
			return false, "listing " + coll + " shows " + name + ", a parent of " + a.name, nil
		case err == nil:
			if name == a.name {
				return true, "listing " + coll + " does not show it", nil
			}
			return true, "listing " + coll + " does not show " + name + ", its parent", nil
		case !direct.HasHTTPCode(err, http.StatusForbidden):
			return false, "", fmt.Errorf("listing %s: %w", coll, err)
		}
		parent := CollectionOf(coll)
		if !a.isMissing403Resource(parent) {
			return false, "listing " + coll + " answers 403 too, and " + parent + " is not a resource that answers 403 when missing", nil
		}
		name = parent
	}
}

// isMissing403Resource: name is a resource of a MissingIs403 kind served
// at this kind's API (a database's catalog, a table's database).
func (a *adapter) isMissing403Resource(name string) bool {
	if name == "" {
		return false
	}
	for _, k := range Kinds() {
		if k.MissingIs403 && k.BaseURL == a.kind.BaseURL && k.NameRegexp().MatchString(name) {
			return true
		}
	}
	return false
}

// listed lists coll, every page, and says whether name is in it and not
// deleted.
func (a *adapter) listed(ctx context.Context, coll, name string) (bool, error) {
	field := coll[strings.LastIndex(coll, "/")+1:]
	token := ""
	for page := 0; page < maxListPages; page++ {
		q := map[string]string{"pageSize": "1000"}
		if token != "" {
			q["pageToken"] = token
		}
		out, err := a.api.Do(ctx, http.MethodGet, coll, q, nil)
		if err != nil {
			return false, err
		}
		items, _ := out[field].([]any)
		for _, it := range items {
			obj, _ := it.(map[string]any)
			if n, _ := obj["name"].(string); n != name {
				continue
			}
			if dt, _ := obj["deleteTime"].(string); dt != "" {
				return false, nil
			}
			return !a.kind.softDeleted(obj), nil
		}
		token, _ = out["nextPageToken"].(string)
		if token == "" {
			return false, nil
		}
	}
	return false, fmt.Errorf("listing %s: more than %d pages", coll, maxListPages)
}
