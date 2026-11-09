// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sort"
	"strings"
)

// Overrides is the hand-kept part of the mapping (overrides.yaml).
type Overrides struct {
	ExcludeServices []string                     `yaml:"exclude_services"`
	GroupHosts      map[string][]string          `yaml:"group_hosts"`
	Kinds           map[string][]string          `yaml:"kinds"`
	NoResource      map[string]string            `yaml:"no_resource"`
	Prefixes        map[string]string            `yaml:"prefixes"`
	Subresources    map[string]string            `yaml:"subresources"`
	Also            map[string]string            `yaml:"also"`
	Aliases         map[string]map[string]string `yaml:"aliases"`
}

// Match is how one CRD kind covers one API resource.
type Match struct {
	CRD     *CRD
	Res     *APIResource
	Covered map[int]string // attribute index -> CRD path
	Via     string         // how the kind was mapped to the resource
}

type matcher struct {
	ov        *Overrides
	sm        *svcMap
	byHost    map[string][]*APIResource
	hostNames map[string]string // host -> discovery name
	protos    map[string][]string
}

var prefixVariants = []string{"region", "global", "regional", "zone", "zonal"}

func (m *matcher) hostsFor(c *CRD) []string {
	var hosts []string
	seen := map[string]bool{}
	add := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	if hs, ok := m.ov.GroupHosts[c.Service]; ok {
		for _, h := range hs {
			add(h)
		}
		return hosts
	}
	if h := m.sm.GroupHost[c.Service]; m.hostNames[h] != "" {
		add(h)
	}
	g := norm(c.Service)
	for _, h := range sortedKeys(m.hostNames) {
		if norm(m.hostNames[h]) == g || norm(strings.TrimSuffix(h, ".googleapis.com")) == g {
			add(h)
		}
	}
	if len(hosts) > 0 || len(g) < 3 {
		return hosts
	}
	// No exact service: the same name with the usual vendor decoration
	// (kms and cloudkms, sql and sqladmin, bigtable and bigtableadmin).
	for _, h := range sortedKeys(m.hostNames) {
		n := norm(strings.TrimSuffix(h, ".googleapis.com"))
		if n == "cloud"+g || n == g+"admin" || g == "cloud"+n || g == n+"admin" {
			add(h)
		}
	}
	return hosts
}

// candidates returns the names a kind goes by: from its own name, and
// from the proto messages its types file names (used only when the first
// finds nothing).
func (m *matcher) candidates(c *CRD) ([]string, []string) {
	kind := strings.ToLower(c.Kind)
	g := norm(c.Service)
	var out []string
	if p, ok := m.ov.Prefixes[c.Service]; ok {
		g = norm(p)
	}
	if strings.HasPrefix(kind, g) && len(kind) > len(g) {
		out = append(out, kind[len(g):])
	}
	out = append(out, kind)
	var pr []string
	for _, p := range m.protos[kind] {
		if i := strings.LastIndex(p, "."); i >= 0 {
			p = p[i+1:]
		}
		pr = append(pr, singular(norm(p)))
	}
	for i := range out {
		out[i] = singular(norm(out[i]))
	}
	return out, pr
}

// mapKind returns the API resources a CRD kind covers and why.
func (m *matcher) mapKind(c *CRD) ([]*APIResource, string, string) {
	if why, ok := m.ov.NoResource[c.Kind]; ok {
		return nil, "", why
	}
	if sub, ok := m.ov.Subresources[c.Kind]; ok {
		// "host/Resource:path[]": the kind fills one list of its parent's body
		id, prefix, _ := strings.Cut(sub, ":")
		for _, rs := range m.byHost {
			for _, r := range rs {
				if r.ID() == id {
					return []*APIResource{r}, "sub:" + prefix, ""
				}
			}
		}
		return nil, "", "subresource names no resource: " + id
	}
	if ids, ok := m.ov.Kinds[c.Kind]; ok {
		var out []*APIResource
		for _, id := range ids {
			for _, rs := range m.byHost {
				for _, r := range rs {
					if r.ID() == id {
						out = append(out, r)
					}
				}
			}
		}
		if len(out) == 0 {
			return nil, "", "override names no resource: " + strings.Join(ids, ",")
		}
		return out, "override", ""
	}
	hosts := m.hostsFor(c)
	if len(hosts) == 0 {
		return nil, "", "no discovery service for group " + c.Service
	}
	primary, viaProto := m.candidates(c)
	for _, cands := range [][]string{primary, append(append([]string{}, primary...), viaProto...)} {
		if out, why := m.mapNames(c, hosts, cands); len(out) > 0 {
			return out, "name", why
		}
	}
	return nil, "", "no API resource named like " + strings.Join(primary, "/") + " in " + strings.Join(hosts, ",")
}

func (m *matcher) mapNames(c *CRD, hosts, cands []string) ([]*APIResource, string) {
	var set0, set1, set2, set3 []*APIResource
	for _, h := range hosts {
		for _, r := range m.byHost[h] {
			hit0, hit1, hit2, hit3 := false, false, false, false
			for _, cd := range cands {
				if cd == r.Names[0] {
					hit0 = true
				}
				for _, n := range r.Names[1:] {
					if cd == n {
						hit1 = true
					}
				}
				for _, p := range prefixVariants {
					if r.Names[0] == p+cd {
						hit2 = true
					}
				}
				// one name ends with the other (logsink and sink)
				for _, n := range r.Names {
					a, b := cd, n
					if len(a) < len(b) {
						a, b = b, a
					}
					if len(b) >= 4 && strings.HasSuffix(a, b) {
						hit3 = true
					}
				}
			}
			switch {
			case hit0:
				set0 = append(set0, r)
			case hit1:
				set1 = append(set1, r)
			case hit2:
				set2 = append(set2, r)
			case hit3:
				set3 = append(set3, r)
			}
		}
	}
	base := set0
	if len(base) == 0 {
		base = set1
	}
	if len(base) == 0 && len(set2) == 0 {
		base = set3
	}
	// Several resources of the same name (projects.foos and
	// projects.locations.foos). A kind with a location field covers the
	// location-scoped one as well; any kind covers the one it fills most
	// (ties: the shorter path).
	sameName := append([]*APIResource{}, base...)
	if len(base) > 1 {
		hasLoc := false
		for _, p := range c.Paths {
			if p.Path == "location" {
				hasLoc = true
			}
		}
		var global, local []*APIResource
		for _, r := range base {
			if strings.Contains(r.Disco, "locations.") {
				local = append(local, r)
			} else {
				global = append(global, r)
			}
		}
		pick := func(rs []*APIResource) *APIResource {
			best, bestN := rs[0], -1
			for _, r := range rs {
				n := len(matchAttrs(r, c, false))
				if n > bestN || (n == bestN && len(r.Disco) < len(best.Disco)) {
					best, bestN = r, n
				}
			}
			return best
		}
		switch {
		case len(global) > 0 && len(local) > 0 && hasLoc:
			g, l := pick(global), pick(local)
			ng, nl := len(matchAttrs(g, c, false)), len(matchAttrs(l, c, false))
			switch {
			case 2*nl < ng:
				base = []*APIResource{g}
			case 2*ng < nl:
				base = []*APIResource{l}
			default:
				base = []*APIResource{g, l}
			}
		case len(global) > 0:
			base = []*APIResource{pick(global)}
		default:
			base = []*APIResource{pick(local)}
		}
	}
	// Compute's global and regional twins (address and globalAddress): a
	// kind with a location, region or zone field covers both, as Config
	// Connector's do.
	scoped := false
	for _, p := range c.Paths {
		if p.Path == "location" || p.Path == "region" || p.Path == "zone" {
			scoped = true
		}
	}
	if !scoped {
		set2 = nil
	}
	out := append(base, set2...)
	// A kind with folderRef, organizationRef or billingAccountRef covers
	// the folder, organization and billing account variants of the
	// resource (Config Connector's LoggingLogSink, OrgPolicyPolicy).
	have := map[string]bool{}
	for _, r := range out {
		have[r.ID()] = true
	}
	for _, r := range sameName {
		if have[r.ID()] {
			continue
		}
		path := r.Disco[strings.Index(r.Disco, ":")+1:]
		if strings.Contains(path, "locations.") && !hasRef(c, "location") {
			continue
		}
		ref := "projectRef" // projects.* and the unscoped legacy paths
		for scope, rf := range map[string]string{"folders.": "folderRef", "organizations.": "organizationRef", "billingAccounts.": "billingAccountRef"} {
			if strings.HasPrefix(path, scope) {
				ref = rf
			}
		}
		if hasRef(c, ref) {
			out = append(out, r)
			have[r.ID()] = true
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out, ""
}

// matchAttrs maps each API attribute the CRD covers to the CRD path.
func matchAttrs(r *APIResource, c *CRD, labelsInMeta bool) map[int]string {
	got := map[int]string{}
	byKey := map[string][]int{}
	for i, p := range c.Paths {
		byKey[p.key] = append(byKey[p.key], i)
	}
	usedC := map[int]bool{}
	// 1. the same path, up to case, plurals and array/map markers
	for i, a := range r.Attrs {
		if cs, ok := byKey[attrKey(r.alias(a.Path))]; ok {
			got[i] = c.Paths[cs[0]].Path
			for _, ci := range cs {
				usedC[ci] = true
			}
		}
	}
	// 1b. a reference to a thing the API names by a string
	// (kmsKeyRef and kmsKeyName, serviceAccountRef and serviceAccountEmail)
	refSuffix := []string{"name", "id", "email", "uri", "link", "url", "selflink"}
	for i, a := range r.Attrs {
		if _, ok := got[i]; ok {
			continue
		}
		segs := normSegs(r.alias(a.Path))
		last := segs[len(segs)-1]
		for _, suf := range refSuffix {
			if len(last) > len(suf)+2 && strings.HasSuffix(last, suf) {
				alt := append(append([]string{}, segs[:len(segs)-1]...), singular(strings.TrimSuffix(last, suf)))
				for ci, p := range c.Paths {
					if p.Ref && p.key == strings.Join(alt, ".") {
						got[i] = p.Path
						usedC[ci] = true
					}
				}
			}
			if _, ok := got[i]; ok {
				break
			}
		}
	}
	// 1c. a flag the CRD flattens (x.enabled and x)
	for i, a := range r.Attrs {
		if _, ok := got[i]; ok {
			continue
		}
		segs := normSegs(r.alias(a.Path))
		if len(segs) > 1 && segs[len(segs)-1] == "enabled" {
			if cs, ok := byKey[strings.Join(segs[:len(segs)-1], ".")]; ok {
				got[i] = c.Paths[cs[0]].Path
				usedC[cs[0]] = true
			}
		}
	}
	// 2. a free-form object covers everything under it
	for ci, p := range c.Paths {
		if !p.Free {
			continue
		}
		for i, a := range r.Attrs {
			if _, ok := got[i]; ok {
				continue
			}
			if strings.HasPrefix(attrKey(r.alias(a.Path))+".", p.key+".") {
				got[i] = p.Path
				usedC[ci] = true
			}
		}
	}
	// 3. the CRD is more detailed than the API leaf (password.value).
	for i, a := range r.Attrs {
		if _, ok := got[i]; ok {
			continue
		}
		k := attrKey(r.alias(a.Path)) + "."
		for ci, p := range c.Paths {
			if strings.HasPrefix(p.key, k) {
				got[i] = p.Path
				usedC[ci] = true
				break
			}
		}
	}
	// 4. the same tail under another head (properties.machineType and
	// machineType): only when the tail has two segments or more, or the
	// API head is a wrapper, and only against a CRD path nothing used.
	for i, a := range r.Attrs {
		if _, ok := got[i]; ok {
			continue
		}
		as := normSegs(r.alias(a.Path))
		for ci, p := range c.Paths {
			if usedC[ci] {
				continue
			}
			cs := strings.Split(p.key, ".")
			if suffixMatch(as, cs) {
				got[i] = p.Path
				usedC[ci] = true
				break
			}
		}
	}
	// 5. the same leaf under a similar parent chain (disks[].autoDelete
	// and bootDisk.autoDelete; acceleratorCount and count).
	for i, a := range r.Attrs {
		if _, ok := got[i]; ok {
			continue
		}
		as := dropItems(normSegs(r.alias(a.Path)))
		for ci, p := range c.Paths {
			if usedC[ci] {
				continue
			}
			if similarChain(as, dropItems(strings.Split(p.key, "."))) {
				got[i] = p.Path
				usedC[ci] = true
				break
			}
		}
	}
	// 6. a long leaf name that only one unmatched API attribute and one
	// unused CRD path have (requesterPays and billing.requesterPays).
	apiLeaf := map[string][]int{}
	for i, a := range r.Attrs {
		if _, ok := got[i]; ok {
			continue
		}
		s := normSegs(r.alias(a.Path))
		l := s[len(s)-1]
		if l == "enabled" && len(s) > 1 {
			l = s[len(s)-2]
		}
		apiLeaf[l] = append(apiLeaf[l], i)
	}
	crdLeaf := map[string][]int{}
	for ci, p := range c.Paths {
		if usedC[ci] {
			continue
		}
		s := strings.Split(p.key, ".")
		crdLeaf[s[len(s)-1]] = append(crdLeaf[s[len(s)-1]], ci)
	}
	for l, is := range apiLeaf {
		if cs := crdLeaf[l]; len(l) >= 8 && len(is) == 1 && len(cs) == 1 {
			got[is[0]] = c.Paths[cs[0]].Path
			usedC[cs[0]] = true
		}
	}
	return got
}

// dropItems removes list wrapper segments (metadata.items[].key).
func dropItems(s []string) []string {
	out := make([]string, 0, len(s))
	for i, x := range s {
		if (x == "item" || x == "value") && i > 0 && len(s) > 2 {
			continue
		}
		out = append(out, x)
	}
	return out
}

func looseEq(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	return len(b) >= 4 && (strings.HasSuffix(a, b) || strings.HasPrefix(a, b))
}

// similarChain: same depth or one level apart, leaf names loosely equal,
// each parent pair loosely equal from the leaf up, the shorter exhausted.
func similarChain(a, c []string) bool {
	if len(a) < 2 || len(c) < 2 {
		return false
	}
	if d := len(a) - len(c); d > 1 || d < -1 {
		return false
	}
	if !looseEq(a[len(a)-1], c[len(c)-1]) {
		return false
	}
	i, j := len(a)-2, len(c)-2
	for i >= 0 && j >= 0 {
		if !looseEq(a[i], c[j]) {
			return false
		}
		i--
		j--
	}
	return true
}

func suffixMatch(a, c []string) bool {
	short, long := a, c
	if len(c) < len(a) {
		short, long = c, a
	}
	if len(short) == len(long) {
		return false
	}
	if len(short) < 2 {
		// one segment: only when the longer path is a wrapper plus it
		if !(len(long) == 2 && (long[0] == "property" || long[0] == "config" || long[0] == "setting")) {
			return false
		}
	}
	for i := 1; i <= len(short); i++ {
		if short[len(short)-i] != long[len(long)-i] {
			return false
		}
	}
	return true
}

func hasRef(c *CRD, ref string) bool {
	for _, p := range c.Paths {
		if p.Path == ref || strings.HasSuffix(p.Path, "."+ref) {
			return true
		}
	}
	return false
}

// subMatch matches a CRD against the attributes of r under prefix
// (nats[]): the paths are compared with the prefix cut off, and the result
// is keyed by the attributes' indexes in r.
func subMatch(r *APIResource, c *CRD, prefix string) map[int]string {
	sub := &APIResource{}
	var idx []int
	for i, a := range r.Attrs {
		if p := r.alias(a.Path); strings.HasPrefix(p, prefix+".") {
			sub.Attrs = append(sub.Attrs, Attr{Path: strings.TrimPrefix(p, prefix+"."), Type: a.Type})
			idx = append(idx, i)
		}
	}
	out := map[int]string{}
	for i, p := range matchAttrs(sub, c, false) {
		out[idx[i]] = p
	}
	return out
}

// alias rewrites an attribute path to the name the Config Connector kinds
// give it, by the longest matching prefix of the resource's aliases
// (overrides.yaml): the API's autoscaling is a cluster's clusterAutoscaling.
func (r *APIResource) alias(path string) string {
	best := ""
	for p := range r.Aliases {
		if len(p) > len(best) && strings.HasPrefix(path, p) {
			if rest := path[len(p):]; rest == "" || rest[0] == '.' || rest[0] == '[' || rest[0] == '{' {
				best = p
			}
		}
	}
	if best == "" {
		return path
	}
	return r.Aliases[best] + path[len(best):]
}
