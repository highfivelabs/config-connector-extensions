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

// Package discokind reconciles kinds whose Google Cloud API is a standard
// resource collection (AIP-121 to AIP-135: create with an ID, get, patch
// with an update mask, delete), described by the API's Discovery document.
// One generic controller serves every such kind: the kind's spec, minus its
// parent reference and the fields that make up its name, is the request
// body as it is (the CRD's field names are the API's), and the API's
// read-only fields go to status.observedState. hack/discokind generates a
// kind's Go types and its Kind descriptor from the Discovery document;
// docs/throughput.md says why and docs/design.md section 2.4 what it covers.
package discokind

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Kind describes one kind and the API collection behind it.
type Kind struct {
	GVK     schema.GroupVersionKind
	CRDName string
	// BaseURL is the API's root with its version and a trailing slash,
	// https://logging.googleapis.com/v2/.
	BaseURL string
	// Name is the resource name template, for example
	// "{parent}/locations/{location}/savedQueries/{id}". {parent} is the
	// parent reference's name (projects/P, folders/F, organizations/O or
	// billingAccounts/B), {id} is spec.resourceID or metadata.name, and any
	// other {field} is the spec string field of that name.
	Name string
	// Parents are the parent kinds the spec may reference, each by its own
	// field: project (projectRef), folder (folderRef), organization
	// (organizationRef), billingAccount (billingAccountRef). Exactly one is
	// set. Empty when Name has no {parent}.
	Parents []string
	// IDParam is the create request's query parameter that carries the ID
	// (savedQueryId).
	IDParam string
	// CreateLRO, UpdateLRO and DeleteLRO say which methods answer a
	// long-running operation rather than the resource.
	CreateLRO, UpdateLRO, DeleteLRO bool
	// Updatable is whether the API has a patch (or, with UpdatePUT, an
	// update) method; without one every field is immutable.
	Updatable bool
	// InsertStyle is Compute Engine's shape: create (insert) takes the ID as
	// the body's name, not a query parameter; a patch is a merge of the
	// fields sent (no update mask) with the resource's fingerprint; and
	// operations are compute operations, polled at their selfLink.
	InsertStyle bool
	// UpdatePUT: the API changes a resource with update (PUT, the whole
	// resource) rather than patch.
	UpdatePUT bool
	// NameInBody: create also sends the resource's full name as the body's
	// name, beside the ID parameter, as Terraform does for the kind; the
	// API refuses a create without it (Cloud Functions v2: "Invalid
	// resource name '' for pattern projects/*/locations/*/functions/*").
	NameInBody bool
	// Output are the API's read-only top-level fields that go to
	// status.observedState (name goes to status.externalRef instead).
	Output []string
	// SoftDelete, when set, is a read-only field and the value it takes
	// when the resource is deleted but still answers a get (an OAuth
	// client's state DELETED); such a resource counts as gone.
	SoftDelete *FieldValue
	// FakeDefaults are values of read-only fields the fake cloud sets on
	// create (state: ACTIVE), where the real API sets them.
	FakeDefaults map[string]any
}

// FieldValue is a top-level field and a value of it.
type FieldValue struct {
	Field string
	Value string
}

var paramRE = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

// PathParams are the spec fields that make up the name: every {field} of
// Name but {parent} and {id}.
func (k *Kind) PathParams() []string {
	var out []string
	for _, m := range paramRE.FindAllStringSubmatch(k.Name, -1) {
		if m[1] != "parent" && m[1] != "id" {
			out = append(out, m[1])
		}
	}
	return out
}

// CollectionOf is the collection a resource name is in: the name without
// its last two segments' ID ("projects/p/locations/global/savedQueries").
func CollectionOf(name string) string {
	i := strings.LastIndex(name, "/")
	if i < 0 {
		return ""
	}
	return name[:i]
}

// ParentOf is the name of the collection's parent.
func ParentOf(name string) string {
	return CollectionOf(CollectionOf(name))
}

// RefField is the spec field of a parent kind.
func RefField(parent string) string { return parent + "Ref" }

// parentPrefixes are the collections the parents' names start with.
var parentPrefixes = map[string]string{
	"project":        "projects",
	"folder":         "folders",
	"organization":   "organizations",
	"billingAccount": "billingAccounts",
}

// NameRegexp matches the names of this kind's resources (not a
// collection, not an operation).
func (k *Kind) NameRegexp() *regexp.Regexp {
	var alts []string
	for _, p := range k.Parents {
		alts = append(alts, parentPrefixes[p])
	}
	var segs []string
	for _, s := range strings.Split(k.Name, "/") {
		switch {
		case s == "{parent}":
			segs = append(segs, "(?:"+strings.Join(alts, "|")+")/[^/]+")
		case paramRE.MatchString(s):
			segs = append(segs, "[^/]+")
		default:
			segs = append(segs, regexp.QuoteMeta(s))
		}
	}
	return regexp.MustCompile("^" + strings.Join(segs, "/") + "$")
}

// Render fills Name's template.
func (k *Kind) Render(parent, id string, params map[string]string) (string, error) {
	var missing []string
	out := paramRE.ReplaceAllStringFunc(k.Name, func(s string) string {
		p := s[1 : len(s)-1]
		var v string
		switch p {
		case "parent":
			v = parent
		case "id":
			v = id
		default:
			v = params[p]
		}
		if v == "" {
			missing = append(missing, p)
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("%s: no value for %s in the name %s", k.GVK.Kind, strings.Join(missing, ", "), k.Name)
	}
	return out, nil
}

var registered []*Kind

// Register adds a kind; the generated registry calls it.
func Register(k *Kind) { registered = append(registered, k) }

// Kinds are the registered kinds.
func Kinds() []*Kind { return registered }

// ByKind is the registered kind of that name, or nil.
func ByKind(kind string) *Kind {
	for _, k := range registered {
		if k.GVK.Kind == kind {
			return k
		}
	}
	return nil
}
