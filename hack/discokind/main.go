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

// discokind generates kinds for standard Google Cloud resource collections
// from the APIs' Discovery documents (pkg/discokind, docs/throughput.md):
//
//	go run ./hack/discokind [-discovery DIR] [-check]
//
// It reads hack/discokind/kinds.yaml and, for each kind, the Discovery
// document named there (DIR/<api>.<version>.json; by default the coverage
// inputs in .cache/coverage/discovery, else fetched from the pinned
// discovery-artifact-manager commit in docs/coverage/sources.yaml), and
// writes:
//
//   - apis/<service>/<version>/<kind>_types.go: the CRD's Go types, the
//     spec from the create request's writable fields, observedState from
//     the read-only ones;
//   - pkg/controller/zz_discokinds.go: each kind's descriptor, its
//     controller and its scheme;
//   - pkg/controller/direct/<service>/zz_discokind_test.go: each kind's
//     end-to-end test (pkg/discokind/kindtest), which reads the fixtures in
//     testdata/discokind/<Kind>/ beside it;
//   - hack/live/discokinds.tsv: what the live sweeper lists;
//   - docs/kinds/<Kind>.md, when it is missing (docs/kinds/README.md).
//
// Then hack/generate.sh makes the CRDs and deepcopy code. -check fails if
// any output would change.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Config is one kind in kinds.yaml.
type Config struct {
	Kind    string `json:"kind"`
	Group   string `json:"group"`   // logging.gcp.hfl.dev
	Version string `json:"version"` // v1alpha1
	// Discovery is the API and version, logging.v2.
	Discovery string `json:"discovery"`
	// Resource is the Discovery resource path of the collection under one
	// of its parents: projects.locations.savedQueries.
	Resource string `json:"resource"`
	// Parents the kind takes (project, folder, organization,
	// billingAccount); the same collection under each must exist.
	Parents []string `json:"parents,omitempty"`
	// Terraform is the Terraform resource the kind matches, for the docs.
	Terraform string `json:"terraform,omitempty"`
	// SoftDelete: a deleted resource still answers get with this field
	// and value.
	SoftDelete *struct {
		Field string `json:"field"`
		Value string `json:"value"`
	} `json:"softDelete,omitempty"`
	// FakeDefaults are read-only values the fake cloud sets on create.
	FakeDefaults map[string]any `json:"fakeDefaults,omitempty"`
	// Immutable are top-level fields the API cannot patch beyond those
	// its descriptions call Immutable.
	Immutable []string `json:"immutable,omitempty"`
	// Skip are writable fields left out of the spec, with why.
	Skip map[string]string `json:"skip,omitempty"`
	// Note is a sentence for the kind's doc comment.
	Note string `json:"note,omitempty"`
	// SweepLocation is the location the live sweeper lists the kind's
	// collection in, for a collection that has no global location
	// (default global); the kind's live fixtures use it.
	SweepLocation string `json:"sweepLocation,omitempty"`
	// LocationHost: the API serves a location other than global only at
	// https://<location>-<host>/ (Discovery Engine's us and eu), which
	// the Discovery document does not say.
	LocationHost bool `json:"locationHost,omitempty"`
	// Query are the patch method's query parameters that are spec fields
	// (a CMEK config's setDefault), sent with every write.
	Query []string `json:"query,omitempty"`
	// NameInBody: the API's create also needs the resource's full name in
	// the request body, beside the ID parameter (Cloud Functions v2
	// refuses one without it: "Invalid resource name ''").
	NameInBody bool `json:"nameInBody,omitempty"`
	// MissingIs403: the API answers a get of a resource that does not
	// exist 403 PERMISSION_DENIED rather than 404 (BigLake catalogs), which
	// the Discovery document does not say (Kind.MissingIs403).
	MissingIs403 bool `json:"missingIs403,omitempty"`
	// GetQuery are query parameters every get of the resource sends
	// (Kind.GetQuery): API Gateway's view FULL, without which a config's
	// source files are left out.
	GetQuery map[string]string `json:"getQuery,omitempty"`
}

// Discovery is the part of a Discovery document used here.
type Discovery struct {
	RootURL     string             `json:"rootUrl"`
	ServicePath string             `json:"servicePath"`
	Resources   map[string]*DRes   `json:"resources"`
	Schemas     map[string]*Schema `json:"schemas"`
}

type DRes struct {
	Methods   map[string]*Method `json:"methods"`
	Resources map[string]*DRes   `json:"resources"`
}

type Method struct {
	HTTPMethod string            `json:"httpMethod"`
	Path       string            `json:"path"`
	FlatPath   string            `json:"flatPath"`
	Parameters map[string]*Param `json:"parameters"`
	Request    *Schema           `json:"request"`
	Response   *Schema           `json:"response"`
}

type Param struct {
	Location    string `json:"location"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Schema struct {
	ID                   string             `json:"id"`
	Ref                  string             `json:"$ref"`
	Type                 string             `json:"type"`
	Format               string             `json:"format"`
	Description          string             `json:"description"`
	ReadOnly             bool               `json:"readOnly"`
	Properties           map[string]*Schema `json:"properties"`
	Items                *Schema            `json:"items"`
	AdditionalProperties *Schema            `json:"additionalProperties"`
	Enum                 []string           `json:"enum"`
}

var parentCollections = map[string]string{
	"project":        "projects",
	"folder":         "folders",
	"organization":   "organizations",
	"billingAccount": "billingAccounts",
}

func main() {
	root, _ := os.Getwd()
	discoDir := flag.String("discovery", "", "directory of Discovery documents")
	check := flag.Bool("check", false, "fail if an output would change")
	flag.Parse()

	b, err := os.ReadFile(filepath.Join(root, "hack/discokind/kinds.yaml"))
	must(err)
	var cfgs []Config
	must(yaml.UnmarshalStrict(b, &cfgs))

	outputs := map[string][]byte{}
	var kinds []*built
	for i := range cfgs {
		c := &cfgs[i]
		d, err := loadDiscovery(root, *discoDir, c.Discovery)
		if err != nil {
			fatalf("%s: %v", c.Kind, err)
		}
		k, err := build(c, d)
		if err != nil {
			fatalf("%s: %v", c.Kind, err)
		}
		kinds = append(kinds, k)
		path := filepath.Join("apis", k.svc, c.Version, strings.ToLower(c.Kind)+"_types.go")
		outputs[path] = gofmt(path, k.typesFile())
		kd := filepath.Join("docs/kinds", c.Kind+".md")
		if _, err := os.Stat(filepath.Join(root, kd)); err != nil {
			outputs[kd] = kindDoc(k)
		}
		gv := filepath.Join("apis", k.svc, c.Version, "groupversion_info.go")
		if _, err := os.Stat(filepath.Join(root, gv)); err != nil {
			outputs[gv] = gofmt(gv, groupVersionFile(c))
		}
	}
	outputs["pkg/controller/zz_discokinds.go"] = gofmt("zz_discokinds.go", registryFile(kinds))
	bySvc := map[string][]*built{}
	for _, k := range kinds {
		bySvc[k.svc] = append(bySvc[k.svc], k)
	}
	for svc, ks := range bySvc {
		p := filepath.Join("pkg/controller/direct", svc, "zz_discokind_test.go")
		outputs[p] = gofmt(p, testFile(svc, ks))
	}
	outputs["hack/live/discokinds.tsv"] = sweepFile(kinds)

	var paths []string
	for p := range outputs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	changed := 0
	for _, p := range paths {
		full := filepath.Join(root, p)
		old, _ := os.ReadFile(full)
		if bytes.Equal(old, outputs[p]) {
			continue
		}
		changed++
		if *check {
			fmt.Fprintf(os.Stderr, "discokind: %s is not current\n", p)
			continue
		}
		must(os.MkdirAll(filepath.Dir(full), 0o755))
		must(os.WriteFile(full, outputs[p], 0o644))
		fmt.Printf("discokind: wrote %s\n", p)
	}
	if *check && changed > 0 {
		fatalf("%d file(s) are not current; run go run ./hack/discokind", changed)
	}
}

func loadDiscovery(root, dir, name string) (*Discovery, error) {
	var b []byte
	var err error
	candidates := []string{}
	if dir != "" {
		candidates = append(candidates, filepath.Join(dir, name+".json"))
	}
	candidates = append(candidates,
		filepath.Join(root, ".cache/coverage/discovery", name+".json"),
		filepath.Join(root, ".cache/discokind", name+".json"))
	for _, c := range candidates {
		if b, err = os.ReadFile(c); err == nil {
			break
		}
	}
	if b == nil {
		commit, cerr := discoveryCommit(root)
		if cerr != nil {
			return nil, cerr
		}
		url := "https://raw.githubusercontent.com/googleapis/discovery-artifact-manager/" + commit + "/discoveries/" + name + ".json"
		resp, herr := http.Get(url)
		if herr != nil {
			return nil, herr
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("fetching %s: %s", url, resp.Status)
		}
		if b, err = io.ReadAll(resp.Body); err != nil {
			return nil, err
		}
		cache := filepath.Join(root, ".cache/discokind", name+".json")
		_ = os.MkdirAll(filepath.Dir(cache), 0o755)
		_ = os.WriteFile(cache, b, 0o644)
	}
	d := &Discovery{}
	return d, json.Unmarshal(b, d)
}

// discoveryCommit is the pinned discovery-artifact-manager commit.
func discoveryCommit(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "docs/coverage/sources.yaml"))
	if err != nil {
		return "", err
	}
	var s struct {
		Discovery struct {
			Commit string `json:"commit"`
		} `json:"discovery"`
	}
	if err := yaml.Unmarshal(b, &s); err != nil {
		return "", err
	}
	if s.Discovery.Commit == "" {
		return "", fmt.Errorf("no discovery commit in docs/coverage/sources.yaml")
	}
	return s.Discovery.Commit, nil
}

// built is a kind, worked out from its config and Discovery document.
type built struct {
	c           *Config
	d           *Discovery
	svc         string // logging
	baseURL     string
	name        string // the name template
	pathParams  []string
	idParam     string
	createLRO   bool
	updateLRO   bool
	deleteLRO   bool
	updatable   bool
	insertStyle bool
	updatePUT   bool
	// patchCreates: no create method; patch makes the resource
	// (discokind.Kind.PatchCreates), without an update mask when the
	// method takes none (patchNoMask).
	patchCreates bool
	patchNoMask  bool
	query        map[string]*Param // c.Query's parameters
	schema       *Schema
	output       []string
	sweep        string // the collection under a project, for the sweeper
}

func findRes(d *Discovery, path string) *DRes {
	parts := strings.Split(path, ".")
	res := d.Resources
	var r *DRes
	for _, p := range parts {
		if res == nil {
			return nil
		}
		r = res[p]
		if r == nil {
			return nil
		}
		res = r.Resources
	}
	return r
}

var flatParamRE = regexp.MustCompile(`^\{[A-Za-z0-9_]+\}$`)

func singular(s string) string {
	switch {
	case strings.HasSuffix(s, "ies"):
		return strings.TrimSuffix(s, "ies") + "y"
	case strings.HasSuffix(s, "s"):
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// isLRO reports whether a method answers a long-running operation: the
// schema Operation or GoogleLongrunningOperation, or an API's own copy of
// google.longrunning.Operation under another name (API Gateway's
// ApigatewayOperation), known by its fields done, name, error, metadata
// and response. Missing one made API Gateway's deletes return before the
// operation ended: the catalog's live run deleted a gateway and its config
// together, the config's delete failed "in use" while the gateway was
// still going, and the config stayed in the cloud, FAILED.
func isLRO(d *Discovery, m *Method) bool {
	if m == nil || m.Response == nil {
		return false
	}
	ref := m.Response.Ref
	if ref == "Operation" || ref == "GoogleLongrunningOperation" {
		return true
	}
	s := d.Schemas[ref]
	if s == nil || !strings.HasSuffix(ref, "Operation") {
		return false
	}
	for _, f := range []string{"done", "name", "error", "metadata", "response"} {
		if s.Properties[f] == nil {
			return false
		}
	}
	return s.Properties["done"].Type == "boolean"
}

func build(c *Config, d *Discovery) (*built, error) {
	if !strings.HasSuffix(c.Group, ".gcp.hfl.dev") {
		return nil, fmt.Errorf("group %s is not <service>.gcp.hfl.dev", c.Group)
	}
	k := &built{c: c, d: d, svc: strings.TrimSuffix(c.Group, ".gcp.hfl.dev")}
	r := findRes(d, c.Resource)
	if r == nil {
		return nil, fmt.Errorf("no resource %s in %s", c.Resource, c.Discovery)
	}
	create, get, del := r.Methods["create"], r.Methods["get"], r.Methods["delete"]
	if create == nil && r.Methods["insert"] != nil {
		// Compute Engine's shape: insert with the name in the body.
		create = r.Methods["insert"]
		k.insertStyle = true
	}
	patch := r.Methods["patch"]
	if create == nil && patch != nil {
		// No create: patch makes the resource (Discovery Engine's CMEK
		// configs), and takes the ID in the name.
		create = patch
		k.patchCreates = true
		k.patchNoMask = patch.Parameters["updateMask"] == nil
	}
	if create == nil || get == nil || del == nil {
		return nil, fmt.Errorf("%s lacks create (or insert, or a patch that makes it), get or delete", c.Resource)
	}
	k.query = map[string]*Param{}
	for _, q := range c.Query {
		p := (*Param)(nil)
		if patch != nil {
			p = patch.Parameters[q]
		}
		if p == nil || p.Location != "query" || (p.Type != "boolean" && p.Type != "string") {
			return nil, fmt.Errorf("query %s: not a boolean or string query parameter of %s.patch", q, c.Resource)
		}
		if create != patch && create.Parameters[q] == nil {
			return nil, fmt.Errorf("query %s: %s.create does not take it", q, c.Resource)
		}
		k.query[q] = p
	}
	if patch == nil && k.insertStyle && r.Methods["update"] != nil {
		patch = r.Methods["update"]
		k.updatePUT = true
	}
	k.updatable = patch != nil
	k.createLRO, k.updateLRO, k.deleteLRO = isLRO(d, create), isLRO(d, patch), isLRO(d, del)
	if !k.insertStyle && !k.patchCreates {
		// The ID parameter: a query parameter named ...Id, but not
		// requestId (an idempotency key some APIs take beside the ID).
		// Sorted, so a run does not depend on map order.
		names := make([]string, 0, len(create.Parameters))
		for n := range create.Parameters {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if p := create.Parameters[n]; p.Location == "query" && strings.HasSuffix(n, "Id") && n != "requestId" {
				if k.idParam != "" {
					return nil, fmt.Errorf("%s.create has two ID query parameters, %s and %s", c.Resource, k.idParam, n)
				}
				k.idParam = n
			}
		}
		if k.idParam == "" {
			return nil, fmt.Errorf("%s.create has no ID query parameter", c.Resource)
		}
	}
	prefix, flat := "", get.FlatPath
	if k.insertStyle {
		// Compute's paths are already flat, relative to servicePath.
		flat = get.Path
	} else {
		// The version prefix: what is before {+name} in get's path.
		i := strings.Index(get.Path, "{+name}")
		if i < 0 {
			return nil, fmt.Errorf("%s.get path %s has no {+name}", c.Resource, get.Path)
		}
		prefix = get.Path[:i]
	}
	k.baseURL = d.RootURL + d.ServicePath + prefix
	segs := strings.Split(strings.TrimPrefix(flat, prefix), "/")
	var out []string
	for j := 0; j < len(segs); {
		coll := segs[j]
		if j+1 >= len(segs) || !flatParamRE.MatchString(segs[j+1]) {
			// A literal segment (compute's global).
			out = append(out, coll)
			j++
			continue
		}
		switch {
		case j == 0 && len(c.Parents) > 0:
			if parentCollections[c.Parents[0]] != coll {
				return nil, fmt.Errorf("%s's name starts with %s, not the first parent's %s", c.Resource, coll, parentCollections[c.Parents[0]])
			}
			out = append(out, "{parent}")
		case j == len(segs)-2:
			out = append(out, coll, "{id}")
		default:
			p := singular(coll)
			k.pathParams = append(k.pathParams, p)
			out = append(out, coll, "{"+p+"}")
		}
		j += 2
	}
	k.name = strings.Join(out, "/")
	// Every parent has the same collection.
	first := strings.Split(c.Resource, ".")[0]
	for _, p := range c.Parents[min(1, len(c.Parents)):] {
		alt := parentCollections[p] + strings.TrimPrefix(c.Resource, first)
		if findRes(d, alt) == nil {
			return nil, fmt.Errorf("parent %s: no resource %s", p, alt)
		}
	}
	if create.Request == nil || create.Request.Ref == "" {
		return nil, fmt.Errorf("%s.create has no request schema", c.Resource)
	}
	k.schema = d.Schemas[create.Request.Ref]
	if k.schema == nil {
		return nil, fmt.Errorf("no schema %s", create.Request.Ref)
	}
	for f, s := range k.schema.Properties {
		if isOutput(s) || (k.insertStyle && f == "fingerprint") {
			// A compute fingerprint is read and sent back by the
			// controller, never set by the spec.
			k.output = append(k.output, f)
		} else if contains(k.pathParams, f) || (len(c.Parents) > 0 && f == c.Parents[0]) {
			// The name's parts are the spec's own fields.
			if c.Skip == nil {
				c.Skip = map[string]string{}
			}
			c.Skip[f] = "part of the name"
		}
	}
	sort.Strings(k.output)
	if contains(c.Parents, "project") {
		k.sweep = strings.Replace(collectionTemplate(k.name), "{parent}", "projects/{project}", 1)
	}
	return k, nil
}

func collectionTemplate(name string) string {
	return name[:strings.LastIndex(name, "/")]
}

func isOutput(s *Schema) bool {
	d := strings.TrimSpace(s.Description)
	return s.ReadOnly || strings.HasPrefix(d, "Output only") || strings.HasPrefix(d, "[Output Only]")
}

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func must(err error) {
	if err != nil {
		fatalf("%v", err)
	}
}

func fatalf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "discokind: "+f+"\n", a...)
	os.Exit(1)
}

func gofmt(path string, b []byte) []byte {
	out, err := format.Source(b)
	if err != nil {
		fatalf("formatting %s: %v\n%s", path, err, b)
	}
	return out
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// goName is a Go field name for a JSON field: initialisms as Go writes them.
func goName(s string) string {
	n := upperFirst(s)
	for _, r := range []struct{ from, to string }{{"Id", "ID"}, {"Uri", "URI"}, {"Url", "URL"}, {"Ip", "IP"}} {
		if strings.HasSuffix(n, r.from) {
			n = strings.TrimSuffix(n, r.from) + r.to
		}
		n = strings.ReplaceAll(n, r.from+"s", r.to+"s")
	}
	return n
}
