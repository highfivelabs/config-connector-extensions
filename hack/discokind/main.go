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
	Location string `json:"location"`
	Type     string `json:"type"`
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
	c          *Config
	d          *Discovery
	svc        string // logging
	baseURL    string
	name       string // the name template
	pathParams []string
	idParam    string
	createLRO  bool
	updateLRO  bool
	deleteLRO  bool
	updatable  bool
	schema     *Schema
	output     []string
	sweep      string // the collection under a project, for the sweeper
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

func isLRO(m *Method) bool {
	return m != nil && m.Response != nil && (m.Response.Ref == "Operation" || m.Response.Ref == "GoogleLongrunningOperation")
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
	if create == nil || get == nil || del == nil {
		return nil, fmt.Errorf("%s lacks create, get or delete", c.Resource)
	}
	patch := r.Methods["patch"]
	k.updatable = patch != nil
	k.createLRO, k.updateLRO, k.deleteLRO = isLRO(create), isLRO(patch), isLRO(del)
	for n, p := range create.Parameters {
		if p.Location == "query" && strings.HasSuffix(n, "Id") {
			k.idParam = n
		}
	}
	if k.idParam == "" {
		return nil, fmt.Errorf("%s.create has no ID query parameter", c.Resource)
	}
	// The version prefix: what is before {+name} in get's path.
	i := strings.Index(get.Path, "{+name}")
	if i < 0 {
		return nil, fmt.Errorf("%s.get path %s has no {+name}", c.Resource, get.Path)
	}
	prefix := get.Path[:i]
	k.baseURL = d.RootURL + d.ServicePath + prefix
	segs := strings.Split(strings.TrimPrefix(get.FlatPath, prefix), "/")
	var out []string
	for j := 0; j < len(segs); j += 2 {
		coll := segs[j]
		if j+1 >= len(segs) || !flatParamRE.MatchString(segs[j+1]) {
			return nil, fmt.Errorf("%s.get flat path %s is not collection/{id} pairs", c.Resource, get.FlatPath)
		}
		switch {
		case j == 0 && len(c.Parents) > 0:
			if parentCollections[c.Parents[0]] == "" {
				return nil, fmt.Errorf("unknown parent %s", c.Parents[0])
			}
			out = append(out, "{parent}")
		case j == len(segs)-2:
			out = append(out, coll, "{id}")
		default:
			p := singular(coll)
			k.pathParams = append(k.pathParams, p)
			out = append(out, coll, "{"+p+"}")
		}
	}
	k.name = strings.Join(out, "/")
	// Every parent has the same collection.
	first := strings.Split(c.Resource, ".")[0]
	for _, p := range c.Parents {
		alt := parentCollections[p] + strings.TrimPrefix(c.Resource, first)
		if findRes(d, alt) == nil {
			return nil, fmt.Errorf("parent %s: no resource %s", p, alt)
		}
	}
	if len(c.Parents) > 0 && parentCollections[c.Parents[0]] != first {
		return nil, fmt.Errorf("resource %s does not start with the first parent's collection %s", c.Resource, parentCollections[c.Parents[0]])
	}
	if create.Request == nil || create.Request.Ref == "" {
		return nil, fmt.Errorf("%s.create has no request schema", c.Resource)
	}
	k.schema = d.Schemas[create.Request.Ref]
	if k.schema == nil {
		return nil, fmt.Errorf("no schema %s", create.Request.Ref)
	}
	for f, s := range k.schema.Properties {
		if isOutput(s) {
			k.output = append(k.output, f)
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
	return s.ReadOnly || strings.HasPrefix(strings.TrimSpace(s.Description), "Output only")
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
