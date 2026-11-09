// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The denominator: Google API Discovery documents, one per service at its
// main stable version. A resource is a REST resource with an insert or
// create method; its attributes are the writable fields of that method's
// request body.

type dSchema struct {
	Ref                  string              `json:"$ref"`
	Type                 string              `json:"type"`
	Description          string              `json:"description"`
	ReadOnly             bool                `json:"readOnly"`
	Properties           map[string]*dSchema `json:"properties"`
	Items                *dSchema            `json:"items"`
	AdditionalProperties *dSchema            `json:"additionalProperties"`
}

type dMethod struct {
	ID      string `json:"id"`
	Request *struct {
		Ref string `json:"$ref"`
	} `json:"request"`
}

type dResource struct {
	Methods   map[string]*dMethod   `json:"methods"`
	Resources map[string]*dResource `json:"resources"`
}

type dDoc struct {
	Name      string                `json:"name"`
	Version   string                `json:"version"`
	RootURL   string                `json:"rootUrl"`
	Resources map[string]*dResource `json:"resources"`
	Schemas   map[string]*dSchema   `json:"schemas"`
	file      string
}

// Attr is one writable attribute of an API resource.
type Attr struct {
	Path     string
	Type     string
	Required bool
	key      string // normalized path, for matching
}

// APIResource is one creatable resource of a service.
type APIResource struct {
	Service   string // host, run.googleapis.com
	Name      string // Service
	Disco     string // projects.locations.services
	Names     []string
	Attrs     []Attr
	Wrapper   string // request wrapper schema stripped, if any
	Truncated int    // leaves cut by the depth guard
}

func (r *APIResource) ID() string { return r.Service + "/" + r.Name }

// scopeSegments are resource path segments that only say where a resource
// lives; the resource name leaves them out.
var scopeSegments = map[string]bool{
	"projects": true, "locations": true, "organizations": true, "folders": true,
	"billingAccounts": true, "global": true,
}

var stableVersion = regexp.MustCompile(`^v(\d+)(\.\d+)?$`)

// versionRank orders versions: stable above beta above alpha, higher
// numbers first.
func versionRank(v string) (int, int) {
	if m := stableVersion.FindStringSubmatch(v); m != nil {
		n, _ := strconv.Atoi(m[1])
		return 3, n
	}
	n := 0
	if m := regexp.MustCompile(`^v(\d+)`).FindStringSubmatch(v); m != nil {
		n, _ = strconv.Atoi(m[1])
	}
	switch {
	case strings.Contains(v, "alpha"):
		return 1, n
	case strings.Contains(v, "beta") || strings.Contains(v, "b3"):
		return 2, n
	}
	return 0, n
}

func hostOf(d *dDoc) string {
	h := strings.TrimSuffix(strings.TrimPrefix(d.RootURL, "https://"), "/")
	if h == "www.googleapis.com" || !strings.HasSuffix(h, ".googleapis.com") {
		return strings.ToLower(d.Name) + ".googleapis.com"
	}
	return h
}

// loadDocs reads every discovery document in dir, drops the excluded
// services and returns, per service host, the documents to read, best
// first: every stable version when the service has one, else the best
// pre-release.
func loadDocs(dir string, excluded map[string]bool) (map[string][]*dDoc, []string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(files)
	all := map[string][]*dDoc{}
	var skipped []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		var d dDoc
		if err := json.Unmarshal(b, &d); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", f, err)
		}
		d.file = filepath.Base(f)
		if d.Name == "" {
			continue // index.json
		}
		if excluded[d.Name] {
			skipped = append(skipped, d.Name)
			continue
		}
		h := hostOf(&d)
		all[h] = append(all[h], &d)
	}
	out := map[string][]*dDoc{}
	for h, ds := range all {
		sort.SliceStable(ds, func(i, j int) bool {
			ri, ni := versionRank(ds[i].Version)
			rj, nj := versionRank(ds[j].Version)
			if ri != rj {
				return ri > rj
			}
			if ni != nj {
				return ni > nj
			}
			return ds[i].Name < ds[j].Name
		})
		if r, _ := versionRank(ds[0].Version); r == 3 {
			var stable []*dDoc
			for _, d := range ds {
				if r, _ := versionRank(d.Version); r == 3 {
					stable = append(stable, d)
				}
			}
			out[h] = stable
		} else {
			out[h] = ds[:1]
		}
	}
	return out, skipped, nil
}

func pascal(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// singular is a crude singular, used on both sides of every comparison so
// it only has to be consistent.
func singular(s string) string {
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 4:
		return s[:len(s)-3] + "y"
	case s == "denied":
		return "deny"
	case strings.HasSuffix(s, "sses"), strings.HasSuffix(s, "uses"), strings.HasSuffix(s, "xes"),
		strings.HasSuffix(s, "ches"), strings.HasSuffix(s, "shes"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "ss"), strings.HasSuffix(s, "us"), strings.HasSuffix(s, "is") && !strings.HasSuffix(s, "apis"):
		return s
	case strings.HasSuffix(s, "s") && len(s) > 2:
		return s[:len(s)-1]
	}
	return s
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]`)

func norm(s string) string { return nonAlnum.ReplaceAllString(strings.ToLower(s), "") }

type resNode struct {
	path   []string
	method *dMethod
	doc    *dDoc
}

func collect(d *dDoc, prefix []string, rs map[string]*dResource, out *[]resNode) {
	names := make([]string, 0, len(rs))
	for n := range rs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		r := rs[n]
		p := append(append([]string{}, prefix...), n)
		var m *dMethod
		if r.Methods != nil {
			if r.Methods["insert"] != nil {
				m = r.Methods["insert"]
			} else if r.Methods["create"] != nil {
				m = r.Methods["create"]
			}
		}
		if m != nil {
			*out = append(*out, resNode{p, m, d})
		}
		collect(d, p, r.Resources, out)
	}
}

// shortSegs drops the scope segments.
func shortSegs(p []string) []string {
	var out []string
	for _, s := range p {
		if !scopeSegments[s] {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return p
	}
	return out
}

// buildResources makes the API resources of one document.
func buildResources(host string, docs []*dDoc) []*APIResource {
	var nodes []resNode
	// An older stable version adds only resources the newer ones do not
	// have: same path, or same short name (run v1 namespaces.services and
	// run v2 projects.locations.services are one resource).
	seen := map[string]bool{}
	seenName := map[string]bool{}
	for _, d := range docs {
		var ns []resNode
		collect(d, nil, d.Resources, &ns)
		var add []resNode
		for _, n := range ns {
			k := strings.Join(n.path, ".")
			if seen[k] || seenName[norm(singular(n.path[len(n.path)-1]))] {
				continue
			}
			add = append(add, n)
		}
		for _, n := range add {
			seen[strings.Join(n.path, ".")] = true
			seenName[norm(singular(n.path[len(n.path)-1]))] = true
			nodes = append(nodes, n)
		}
	}
	type cand struct {
		node resNode
		name string
	}
	var cs []cand
	for _, n := range nodes {
		if n.method.Request == nil || n.method.Request.Ref == "" {
			continue // no request body: nothing to count
		}
		ss := shortSegs(n.path)
		var b strings.Builder
		for i, s := range ss {
			if i == len(ss)-1 {
				s = singular(s)
			} else {
				s = singular(s)
			}
			b.WriteString(pascal(s))
		}
		cs = append(cs, cand{n, b.String()})
	}
	// The legacy zonal surface (container v1 projects.zones.clusters)
	// duplicates the location-based one: drop it where both exist.
	hasLoc := map[string]bool{}
	for _, c := range cs {
		for _, s := range c.node.path {
			if s == "locations" {
				hasLoc[norm(singular(c.node.path[len(c.node.path)-1]))] = true
			}
		}
	}
	var kept []cand
	for _, c := range cs {
		legacy := len(c.node.path) > 2 && c.node.path[0] == "projects" && c.node.path[1] == "zones"
		if legacy && hasLoc[norm(singular(c.node.path[len(c.node.path)-1]))] {
			continue
		}
		kept = append(kept, c)
	}
	cs = kept
	count := map[string]int{}
	for _, c := range cs {
		count[c.name]++
	}
	var out []*APIResource
	// Of resources with the same short name, the one with the fewest path
	// segments keeps it; the others carry their scope (LocationSecret).
	shortest := map[string]int{}
	for _, c := range cs {
		if l, ok := shortest[c.name]; !ok || len(c.node.path) < l {
			shortest[c.name] = len(c.node.path)
		}
	}
	used := map[string]int{}
	for _, c := range cs {
		name := c.name
		if count[name] > 1 && len(c.node.path) > shortest[name] {
			var pre strings.Builder
			for _, s := range c.node.path {
				if scopeSegments[s] && s != "projects" {
					pre.WriteString(pascal(singular(s)))
				}
			}
			name = pre.String() + name
		}
		if used[name]++; used[name] > 1 {
			name = strings.Join(c.node.path, ".")
		}
		r := &APIResource{Service: host, Name: name, Disco: c.node.doc.Version + ":" + strings.Join(c.node.path, ".")}
		last := singular(c.node.path[len(c.node.path)-1])
		// The name by its last segment, then with one more parent
		// segment, and so on up to the whole short path.
		ss := shortSegs(c.node.path)
		acc := last
		r.Names = []string{norm(last)}
		for i := len(ss) - 2; i >= 0; i-- {
			acc = singular(ss[i]) + acc
			r.Names = append(r.Names, norm(acc))
		}
		fl := &flattener{doc: c.node.doc, res: r}
		fl.run(c.node.method.Request.Ref, last)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

type flattener struct {
	doc   *dDoc
	res   *APIResource
	stack []string
}

var outputOnly = regexp.MustCompile(`(?i)^\s*(?:[A-Za-z ,]{0,30}\.\s*)?(?:\[?output[ -]only\]?|read[- ]only)\b`)

// serverNames are fields every API assigns itself but some documents do not
// mark read-only.
var serverNames = map[string]bool{
	"kind": true, "etag": true, "selfLink": true, "selfLinkWithId": true, "creationTimestamp": true,
	"timeCreated": true, "updated": true, "generation": true, "metageneration": true,
	"fingerprint": true, "labelFingerprint": true, "projectNumber": true,
}
var requiredDesc = regexp.MustCompile(`(?i)^\s*\[?required\]?[.:\s]`)

func isOutputOnly(s *dSchema) bool {
	return s.ReadOnly || outputOnly.MatchString(s.Description)
}

func isServerName(name string) bool { return serverNames[name] }

func (f *flattener) run(ref, last string) {
	sc := f.doc.Schemas[ref]
	if sc == nil {
		return
	}
	f.stack = []string{ref}
	// A request wrapper (CreateInstanceRequest{instanceId, instance}):
	// the resource body is its one message-typed property; paths are
	// relative to it.
	if strings.HasSuffix(ref, "Request") && len(sc.Properties) > 0 {
		var main string
		var objs []string
		names := sortedKeys(sc.Properties)
		for _, n := range names {
			p := sc.Properties[n]
			if p.Ref != "" && !isOutputOnly(p) {
				objs = append(objs, n)
				if norm(n) == norm(last) {
					main = n
				}
			}
		}
		if main == "" && len(objs) == 1 {
			main = objs[0]
		}
		if main != "" {
			f.res.Wrapper = ref
			for _, n := range names {
				p := sc.Properties[n]
				if isOutputOnly(p) {
					continue
				}
				if n == main {
					f.prop(p, "", nil, 1)
				} else {
					f.prop(p, n, p, 1)
				}
			}
			return
		}
	}
	f.object(sc, "", 0)
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// prop walks one property. name is the path of the property itself; with
// an empty name the property's own properties are walked at the root.
func (f *flattener) prop(s *dSchema, path string, req *dSchema, depth int) {
	if path == "" {
		f.schema(s, "", depth)
		return
	}
	f.schema(s, path, depth)
}

func (f *flattener) object(sc *dSchema, path string, depth int) {
	for _, n := range sortedKeys(sc.Properties) {
		p := sc.Properties[n]
		if isOutputOnly(p) || isServerName(n) {
			continue
		}
		f.leafOrWalk(p, join(path, n), depth+1)
	}
}

func (f *flattener) leafOrWalk(p *dSchema, path string, depth int) {
	f.schema(p, path, depth)
}

func (f *flattener) emit(path, typ string, desc string) {
	if path == "" {
		return
	}
	f.res.Attrs = append(f.res.Attrs, Attr{Path: path, Type: typ, Required: requiredDesc.MatchString(desc)})
}

// schema walks a property schema at path, resolving references.
func (f *flattener) schema(p *dSchema, path string, depth int) {
	desc := p.Description
	cur := p
	if p.Ref != "" {
		for _, s := range f.stack {
			if s == p.Ref {
				f.res.Truncated++
				f.emit(path, "object", desc)
				return
			}
		}
		def := f.doc.Schemas[p.Ref]
		if def == nil {
			f.emit(path, "object", desc)
			return
		}
		f.stack = append(f.stack, p.Ref)
		defer func() { f.stack = f.stack[:len(f.stack)-1] }()
		cur = def
	}
	if depth > 24 {
		f.res.Truncated++
		f.emit(path, "object", desc)
		return
	}
	switch {
	case cur.Type == "array" && cur.Items != nil:
		if cur.Items.Ref != "" || len(cur.Items.Properties) > 0 || cur.Items.Type == "array" || cur.Items.AdditionalProperties != nil {
			f.schema(cur.Items, path+"[]", depth+1)
		} else {
			f.emit(path+"[]", orStr(cur.Items.Type), desc)
		}
	case len(cur.Properties) > 0:
		if path == "" {
			f.object(cur, path, depth)
			return
		}
		before := len(f.res.Attrs)
		f.object(cur, path, depth)
		_ = before
	case cur.AdditionalProperties != nil:
		ap := cur.AdditionalProperties
		if ap.Ref != "" || len(ap.Properties) > 0 || ap.Type == "array" || ap.AdditionalProperties != nil {
			f.schema(ap, path+"{}", depth+1)
		} else {
			f.emit(path+"{}", orStr(ap.Type), desc)
		}
	default:
		f.emit(path, orStr(cur.Type), desc)
	}
}

func orStr(t string) string {
	if t == "" {
		return "object"
	}
	return t
}
