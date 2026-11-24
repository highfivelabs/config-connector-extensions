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
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	// resource) rather than patch. Outside Compute Engine's shape the PUT
	// may have another name (Binary Authorization's replacePlatformPolicy,
	// kinds.yaml updateMethod); it takes the whole spec with no mask.
	UpdatePUT bool
	// PatchCreates: the API has no create method; patch, sent to the
	// resource's name, makes the resource as well as changing it (Discovery
	// Engine's CMEK configs). Create and update both patch the spec's whole
	// body.
	PatchCreates bool
	// PatchNoMask: the API's patch takes no update mask; the body is every
	// field the spec sets.
	PatchNoMask bool
	// LocationHost: the API serves a location other than global at
	// https://<location>-<host>/ (Discovery Engine's us and eu); BaseURL's
	// own host serves global and names without a location
	// (LocationBaseURL).
	LocationHost bool
	// Query are spec fields that are the write methods' query parameters,
	// not body fields (a CMEK config's setDefault). The API never returns
	// them, so they are sent with every write and never compared.
	Query []string
	// NameInBody: create also sends the resource's full name as the body's
	// name, beside the ID parameter, as Terraform does for the kind; the
	// API refuses a create without it (Cloud Functions v2: "Invalid
	// resource name '' for pattern projects/*/locations/*/functions/*").
	// With no IDParam, the name in the body is the only place the ID goes
	// (Access Context Manager's authorized orgs descriptions, whose create
	// takes no ID parameter).
	NameInBody bool
	// Output are the API's read-only top-level fields that go to
	// status.observedState (name goes to status.externalRef instead).
	Output []string
	// SoftDelete, when set, is a read-only field and the value it takes
	// when the resource is deleted but still answers a get (an OAuth
	// client's state DELETED); such a resource counts as gone.
	SoftDelete *FieldValue
	// MissingIs403: the API answers a get of a resource that does not
	// exist 403 PERMISSION_DENIED, not 404, even to an admin of it (BigLake
	// catalogs, found by the catalog's live runs). Before it has made or
	// acquired the resource (no status.externalRef), the controller reads
	// such a 403 as not found and creates it, as Terraform does (it never
	// reads before a create); a real denial then fails the create. Once the
	// object has a resource, a 403 is a denial, as for any kind, on update;
	// while deleting, a 403 on the read or the delete counts as gone only
	// when a listing proves the resource, or a parent of it, gone
	// (GoneAfter403): a catalog's delete takes its databases and tables
	// along, and they then answer 403 (krm-packages repository run
	// cat-11141549-4cb0).
	MissingIs403 bool
	// Mockgcp: the fake cloud (pkg/discokind/fake) leaves the collection to
	// mockgcp, which already serves it and which other tests' requests for
	// it rely on (a subnetwork, a service attachment): a parallel kind of
	// ours for a resource Config Connector also manages (decision 19).
	Mockgcp bool
	// GetQuery are query parameters every get of the resource sends: API
	// Gateway's view=FULL, without which a config's get leaves out its
	// source files (openapiDocuments), which the spec sets, so the spec
	// and the cloud would always differ.
	GetQuery map[string]string
	// FakeDefaults are values of read-only fields the fake cloud sets on
	// create (state: ACTIVE), where the real API sets them.
	FakeDefaults map[string]any
	// Secrets are the dotted paths of string fields the spec takes as
	// Config Connector's secret.Legacy, `value` or
	// `valueFrom.secretKeyRef` (a Looker instance's
	// oauthConfig.clientSecret): the controller reads the Secret and sends
	// the string. They are input-only too (InputOnly).
	Secrets []string
	// InputOnly are dotted paths of fields the API takes but never
	// returns (a Looker instance's oauthConfig.clientId). They are never
	// a difference. They are sent on create, with any patch of their
	// top-level field, and whenever their version changes: a digest of
	// their values and of each Secret's name, key and resourceVersion
	// (never a Secret's value), kept in status.inputVersion, as
	// ComputeRouter keeps md5AuthenticationKeysVersion.
	InputOnly []string
	// SecretOutputs are top-level read-only string fields that hold a
	// secret the API returns (an OAuth client credential's clientSecret, a
	// SCIM token's securityToken). They never go to status: the controller
	// writes them to a Kubernetes Secret in the object's namespace, owned
	// by the object, named by spec.secretOutput.name (default the object's
	// name), each under the field's name (docs/design.md decision 38).
	SecretOutputs []string
	// CreateOnlyOutputs are SecretOutputs the API returns only in the
	// create's answer, never on a read (a SCIM token's securityToken): if
	// their Secret is lost, the value is gone, and the resource must be
	// deleted and made again.
	CreateOnlyOutputs []string
	// DisableBeforeDelete is a top-level boolean field the API wants true
	// before it deletes the resource (an OAuth client credential's
	// disabled: "Before deleting an OauthClientCredential, it should first
	// be disabled"); the controller patches it to true first.
	DisableBeforeDelete string
}

// SecretOutputSpec is the spec field that names the Secret a kind with
// SecretOutputs writes to.
const SecretOutputSpec = "secretOutput"

// writeOnly are InputOnly and Secrets.
func (k *Kind) writeOnly() []string {
	return append(append([]string(nil), k.InputOnly...), k.Secrets...)
}

// Comparable is body without its input-only fields (InputOnly and
// Secrets), the part a read of the resource can show.
func (k *Kind) Comparable(body map[string]any) map[string]any {
	paths := k.writeOnly()
	if len(paths) == 0 {
		return body
	}
	out := runtime.DeepCopyJSON(body)
	for _, p := range paths {
		unstructured.RemoveNestedField(out, strings.Split(p, ".")...)
	}
	return out
}

// WriteOnlySet are the top-level fields of body that hold an input-only
// field (InputOnly or Secrets) it sets, sorted.
func (k *Kind) WriteOnlySet(body map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range k.writeOnly() {
		parts := strings.Split(p, ".")
		if _, found, _ := unstructured.NestedFieldNoCopy(body, parts...); found && !seen[parts[0]] {
			seen[parts[0]] = true
			out = append(out, parts[0])
		}
	}
	sort.Strings(out)
	return out
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

// LocationOf is the location segment of a resource or operation name
// (".../locations/us/..."), or "".
func LocationOf(name string) string {
	parts := strings.Split(name, "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "locations" {
			return parts[i+1]
		}
	}
	return ""
}

// LocationBaseURL is base with its host prefixed by the name's location
// ("https://discoveryengine.googleapis.com/v1/" for
// projects/p/locations/us/... is "https://us-discoveryengine.googleapis.com/v1/"),
// or base itself for global and for a name without a location.
func LocationBaseURL(base, name string) string {
	loc := LocationOf(name)
	if loc == "" || loc == "global" || !strings.HasPrefix(base, "https://") {
		return base
	}
	return "https://" + loc + "-" + strings.TrimPrefix(base, "https://")
}

// QueryOf is the query parameters a spec sets (Query), as strings.
func (k *Kind) QueryOf(spec map[string]any) map[string]string {
	out := map[string]string{}
	for _, f := range k.Query {
		if v, ok := spec[f]; ok && v != nil {
			out[f] = fmt.Sprint(v)
		}
	}
	return out
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
