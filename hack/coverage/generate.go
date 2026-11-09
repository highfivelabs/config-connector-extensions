// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed overrides.yaml
var overridesYAML []byte

// resRow gathers everything known about one API resource.
type resRow struct {
	r       *APIResource
	off     []*Match
	ours    []*Match
	offPath map[int]string
	ourPath map[int]string
	fromPR  bool
}

type unmappedKind struct {
	crd    *CRD
	reason string
}

type model struct {
	hosts      []string
	byHost     map[string][]*resRow
	docs       map[string][]*dDoc
	excluded   []string
	offUnmap   []unmappedKind
	oursUnmap  []unmappedKind
	offCRDs    []*CRD
	oursCRDs   []*CRD
	offMapped  int
	oursMapped int
	noCreate   []string // services with no creatable resource
	protoHosts []string // services described by protos, not Discovery
}

// covered applies the identity and label rules on top of the field match.
func coverMatch(r *APIResource, c *CRD) map[int]string {
	got := matchAttrs(r, c, c.Labels)
	for i, a := range r.Attrs {
		if _, ok := got[i]; ok {
			continue
		}
		switch attrKey(a.Path) {
		case "name":
			got[i] = "metadata.name"
		case "label":
			if c.Labels && a.Path == "labels{}" {
				got[i] = "metadata.labels"
			}
		}
	}
	return got
}

func buildModel(repo, kccDir, discDir, protoDir string) (*model, *Overrides, error) {
	var ov Overrides
	if err := yaml.Unmarshal(overridesYAML, &ov); err != nil {
		return nil, nil, err
	}
	excl := map[string]bool{}
	for _, s := range ov.ExcludeServices {
		excl[s] = true
	}
	docs, skipped, err := loadDocs(discDir, excl)
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(skipped)
	m := &model{docs: docs, byHost: map[string][]*resRow{}, excluded: skipped}
	byHost := map[string][]*APIResource{}
	hostNames := map[string]string{}
	for h, ds := range docs {
		hostNames[h] = ds[0].Name
		rs := buildResources(h, ds)
		if len(rs) == 0 {
			m.noCreate = append(m.noCreate, h)
			continue
		}
		byHost[h] = rs
		m.hosts = append(m.hosts, h)
		for _, r := range rs {
			m.byHost[h] = append(m.byHost[h], &resRow{r: r, offPath: map[int]string{}, ourPath: map[int]string{}})
		}
	}
	// Services Discovery does not describe: googleapis protos.
	if protoDir != "" {
		skip := map[string]bool{}
		for h := range docs {
			skip[h] = true
		}
		for _, n := range skipped {
			skip[strings.ToLower(n)+".googleapis.com"] = true
		}
		pdocs, failed, err := loadProtos(protoDir, skip)
		if os.Getenv("COVERAGE_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "proto compile errors: %d\n", len(failed))
		}
		if err != nil {
			return nil, nil, err
		}
		for h, ds := range pdocs {
			m.protoHosts = append(m.protoHosts, h)
			hostNames[h] = strings.TrimSuffix(h, ".googleapis.com")
			rs := buildProtoResources(h, ds)
			if len(rs) == 0 {
				m.noCreate = append(m.noCreate, h)
				continue
			}
			byHost[h] = rs
			m.hosts = append(m.hosts, h)
			for _, r := range rs {
				m.byHost[h] = append(m.byHost[h], &resRow{r: r, offPath: map[int]string{}, ourPath: map[int]string{}})
			}
		}
		sort.Strings(m.protoHosts)
	}
	sort.Strings(m.hosts)
	sort.Strings(m.noCreate)

	sm, err := loadServiceMappings(filepath.Join(kccDir, "config", "servicemappings"))
	if err != nil {
		return nil, nil, err
	}
	protos := protoMarkers(filepath.Join(kccDir, "apis"))
	for k, v := range protoMarkers(filepath.Join(repo, "apis")) {
		protos[k] = v
	}
	offAll, err := loadCRDs(filepath.Join(kccDir, "config", "crds", "resources"))
	if err != nil {
		return nil, nil, err
	}
	for _, c := range offAll {
		if strings.HasSuffix(c.Group, ".cnrm.cloud.google.com") && c.Group != "core.cnrm.cloud.google.com" {
			c.Labels = sm.LabelKind[c.Kind]
			m.offCRDs = append(m.offCRDs, c)
		}
	}
	ours, err := loadCRDs(filepath.Join(repo, "config", "crds"))
	if err != nil {
		return nil, nil, err
	}
	for _, c := range ours {
		c.Labels = false
		for _, p := range c.Paths {
			if p.Path == "labels{}" {
				c.Labels = true
			}
		}
		m.oursCRDs = append(m.oursCRDs, c)
	}
	mt := &matcher{ov: &ov, sm: sm, byHost: byHost, hostNames: hostNames, protos: protos}
	prKinds := prKinds(repo)

	index := map[string]*resRow{}
	for _, rows := range m.byHost {
		for _, rr := range rows {
			index[rr.r.ID()] = rr
		}
	}
	apply := func(crds []*CRD, isOurs bool) {
		for _, c := range crds {
			rs, via, why := mt.mapKind(c)
			if len(rs) == 0 {
				if isOurs {
					m.oursUnmap = append(m.oursUnmap, unmappedKind{c, why})
				} else {
					m.offUnmap = append(m.offUnmap, unmappedKind{c, why})
				}
				continue
			}
			if isOurs {
				m.oursMapped++
			} else {
				m.offMapped++
			}
			for _, r := range rs {
				rr := index[r.ID()]
				mm := &Match{CRD: c, Res: r, Covered: coverMatch(r, c), Via: via}
				if isOurs {
					rr.ours = append(rr.ours, mm)
					if prKinds[c.Kind] {
						rr.fromPR = true
					}
				} else {
					rr.off = append(rr.off, mm)
				}
			}
		}
	}
	defer func() {
		if os.Getenv("COVERAGE_DEBUG") != "" {
			for _, u := range m.offUnmap {
				fmt.Fprintf(os.Stderr, "UNMAPPED\t%s\t%s\t%s\n", u.crd.Group, u.crd.Kind, u.reason)
			}
		}
	}()
	apply(m.offCRDs, false)
	apply(m.oursCRDs, true)
	for _, rows := range m.byHost {
		for _, rr := range rows {
			rr.offPath = pathsOf(rr.off)
			rr.ourPath = pathsOf(rr.ours)
		}
	}
	return m, &ov, nil
}

func pathsOf(ms []*Match) map[int]string {
	out := map[int]string{}
	multi := len(ms) > 1
	sorted := append([]*Match{}, ms...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CRD.Kind < sorted[j].CRD.Kind })
	parts := map[int][]string{}
	for _, mm := range sorted {
		for i, p := range mm.Covered {
			if !strings.HasPrefix(p, "metadata.") {
				p = "spec." + p
			}
			if multi {
				p = mm.CRD.Kind + ":" + p
			}
			parts[i] = append(parts[i], p)
		}
	}
	for i, ps := range parts {
		out[i] = strings.Join(ps, ";")
	}
	return out
}

// prKinds reads UPSTREAM.md: our kinds with a file copied from an
// unreleased upstream pull request.
func prKinds(repo string) map[string]bool {
	b, err := os.ReadFile(filepath.Join(repo, "UPSTREAM.md"))
	out := map[string]bool{}
	if err != nil {
		return out
	}
	crds, _ := loadCRDs(filepath.Join(repo, "config", "crds"))
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 4 || !strings.Contains(cols[2], "PR #") {
			continue
		}
		here := strings.ToLower(cols[1])
		for _, c := range crds {
			if strings.Contains(here, strings.ToLower(c.Kind)+"_") {
				out[c.Kind] = true
			}
		}
	}
	return out
}

type svcSum struct {
	host string
	c    counts
	tier int
}

type counts struct {
	resources, resOff, resBeyond  int
	attrs, off, beyond, uncovered int
}

func (c *counts) add(o counts) {
	c.resources += o.resources
	c.resOff += o.resOff
	c.resBeyond += o.resBeyond
	c.attrs += o.attrs
	c.off += o.off
	c.beyond += o.beyond
	c.uncovered += o.uncovered
}

func (rr *resRow) counts() counts {
	var c counts
	c.resources = 1
	if len(rr.off) > 0 {
		c.resOff = 1
	} else if len(rr.ours) > 0 {
		c.resBeyond = 1
	}
	c.attrs = len(rr.r.Attrs)
	for i := range rr.r.Attrs {
		_, o := rr.offPath[i]
		_, u := rr.ourPath[i]
		switch {
		case o:
			c.off++
		case u:
			c.beyond++
		default:
			c.uncovered++
		}
	}
	return c
}

func pct(n, d int) string {
	if d == 0 {
		return "0.0"
	}
	return strconv.FormatFloat(float64(n)*100/float64(d), 'f', 1, 64)
}

func kindList(ms []*Match) string {
	seen := map[string]bool{}
	var ks []string
	for _, m := range ms {
		if !seen[m.CRD.Kind] {
			seen[m.CRD.Kind] = true
			ks = append(ks, m.CRD.Kind)
		}
	}
	sort.Strings(ks)
	return strings.Join(ks, ";")
}

func csvBytes(rows [][]string) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.WriteAll(rows)
	return b.Bytes()
}

func b2s(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// maxAttrCSV: above this size attributes.csv is split by service.
const maxAttrCSV = 25 << 20

func generate(repo, kccDir, discDir, protoDir string, src *Sources) (map[string][]byte, error) {
	m, ov, err := buildModel(repo, kccDir, discDir, protoDir)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}

	svcHead := []string{"service", "resources_total", "resources_official", "resources_ours_beyond", "attributes_total", "attributes_official", "attributes_ours_beyond", "attributes_uncovered", "pct_official", "pct_ours_beyond", "pct_uncovered"}
	svcRows := [][]string{svcHead}
	resRows := [][]string{{"service", "resource", "attributes_total", "official_kinds", "attributes_official", "ours_kinds", "attributes_ours_beyond", "attributes_uncovered", "ours_from_upstream_pr", "notes"}}
	attrHead := []string{"service", "resource", "attribute", "type", "required", "covered_by", "official_path", "ours_path"}
	attrBySvc := map[string][][]string{}

	prio, err := loadPriority(repo)
	if err != nil {
		return nil, err
	}
	tierOf := func(h string) int {
		if t, ok := prio[h]; ok {
			return t
		}
		return 3
	}
	var sums []svcSum
	var total counts
	for _, h := range m.hosts {
		var sc counts
		for _, rr := range m.byHost[h] {
			c := rr.counts()
			sc.add(c)
			notes := []string{"api=" + rr.r.Disco}
			if rr.r.Wrapper != "" {
				notes = append(notes, "request-wrapper="+rr.r.Wrapper)
			}
			if rr.r.Truncated > 0 {
				notes = append(notes, fmt.Sprintf("recursive-cut=%d", rr.r.Truncated))
			}
			for _, side := range []struct {
				name string
				ms   []*Match
			}{{"official", rr.off}, {"ours", rr.ours}} {
				if len(side.ms) == 1 {
					mm := side.ms[0]
					used := map[string]bool{}
					for _, p := range mm.Covered {
						used[p] = true
					}
					n := 0
					for _, p := range mm.CRD.Paths {
						if !used[p.Path] {
							n++
						}
					}
					notes = append(notes, fmt.Sprintf("%s-spec-fields-unmatched=%d/%d", side.name, n, len(mm.CRD.Paths)))
				}
			}
			resRows = append(resRows, []string{h, rr.r.ID(), strconv.Itoa(c.attrs), kindList(rr.off), strconv.Itoa(c.off), kindList(rr.ours), strconv.Itoa(c.beyond), strconv.Itoa(c.uncovered), b2s(rr.fromPR), strings.Join(notes, "; ")})
			for i, a := range rr.r.Attrs {
				_, o := rr.offPath[i]
				_, u := rr.ourPath[i]
				by := "none"
				switch {
				case o && u:
					by = "both"
				case o:
					by = "official"
				case u:
					by = "ours"
				}
				attrBySvc[h] = append(attrBySvc[h], []string{h, rr.r.ID(), a.Path, a.Type, b2s(a.Required), by, rr.offPath[i], rr.ourPath[i]})
			}
		}
		total.add(sc)
		sums = append(sums, svcSum{h, sc, tierOf(h)})
		svcRows = append(svcRows, []string{h, strconv.Itoa(sc.resources), strconv.Itoa(sc.resOff), strconv.Itoa(sc.resBeyond), strconv.Itoa(sc.attrs), strconv.Itoa(sc.off), strconv.Itoa(sc.beyond), strconv.Itoa(sc.uncovered), pct(sc.off, sc.attrs), pct(sc.beyond, sc.attrs), pct(sc.uncovered, sc.attrs)})
	}
	// Our kinds with no API resource to count them against.
	for _, u := range m.oursUnmap {
		host := ""
		if hs := (&matcher{ov: ov, sm: &svcMap{GroupHost: map[string]string{}}, hostNames: hostNamesOf(m)}).hostsFor(u.crd); len(hs) > 0 {
			host = hs[0]
		}
		if host == "" {
			host = u.crd.Service + ".googleapis.com"
		}
		resRows = append(resRows, []string{host, host + "/" + u.crd.Kind, "0", "", "0", u.crd.Kind, "0", "0", b2s(prKinds(repo)[u.crd.Kind]), "outside-denominator; " + u.reason})
	}
	// Order resources.csv and keep services.csv in service order.
	sortRows(resRows[1:])

	files["docs/coverage/services.csv"] = csvBytes(svcRows)
	files["docs/coverage/resources.csv"] = csvBytes(resRows)

	// attributes.csv, or one file per service when it would be too big.
	var all [][]string
	all = append(all, attrHead)
	size := 0
	for _, h := range m.hosts {
		for _, r := range attrBySvc[h] {
			for _, f := range r {
				size += len(f) + 1
			}
		}
	}
	if size > maxAttrCSV {
		for _, h := range m.hosts {
			files["docs/coverage/attributes/"+h+".csv"] = csvBytes(append([][]string{attrHead}, attrBySvc[h]...))
		}
	} else {
		for _, h := range m.hosts {
			all = append(all, attrBySvc[h]...)
		}
		files["docs/coverage/attributes.csv"] = csvBytes(all)
	}

	// Bars.
	for _, s := range sums {
		files["docs/coverage/bars/"+s.host+".svg"] = []byte(barSVG(s.host, s.c, false))
	}
	files["docs/coverage/bars/_total.svg"] = []byte(barSVG("Google Cloud, all services", total, true))

	// Tiers: the rollup by how much a service matters (priority.csv).
	var tierSums [3]counts
	var tierSvcs [3]int
	for _, s := range sums {
		tierSums[s.tier-1].add(s.c)
		tierSvcs[s.tier-1]++
	}
	tierRows := [][]string{{"tier", "services", "resources_total", "resources_official", "resources_ours_beyond", "attributes_total", "attributes_official", "attributes_ours_beyond", "attributes_uncovered", "pct_official", "pct_ours_beyond", "pct_uncovered"}}
	for i, c := range tierSums {
		tierRows = append(tierRows, []string{strconv.Itoa(i + 1), strconv.Itoa(tierSvcs[i]), strconv.Itoa(c.resources), strconv.Itoa(c.resOff), strconv.Itoa(c.resBeyond), strconv.Itoa(c.attrs), strconv.Itoa(c.off), strconv.Itoa(c.beyond), strconv.Itoa(c.uncovered), pct(c.off, c.attrs), pct(c.beyond, c.attrs), pct(c.uncovered, c.attrs)})
	}
	files["docs/coverage/tiers.csv"] = csvBytes(tierRows)
	files["docs/coverage/bars/_tiers.svg"] = []byte(tiersSVG(tierSums, tierSvcs))

	// Root README section and the docs README statistics.
	root, err := os.ReadFile(filepath.Join(repo, "README.md"))
	if err != nil {
		return nil, err
	}
	nr, err := replaceBlock(root, "<!-- coverage:start -->", "<!-- coverage:end -->", rootSection(sums, tierSums, tierSvcs, total, len(m.hosts), len(uniq(m.excluded)), src))
	if err != nil {
		return nil, fmt.Errorf("README.md: %w", err)
	}
	files["README.md"] = nr
	dr, err := os.ReadFile(filepath.Join(repo, "docs", "coverage", "README.md"))
	if err != nil {
		return nil, err
	}
	nd, err := replaceBlock(dr, "<!-- stats:start -->", "<!-- stats:end -->", statsSection(m, ov, total, size > maxAttrCSV))
	if err != nil {
		return nil, fmt.Errorf("docs/coverage/README.md: %w", err)
	}
	files["docs/coverage/README.md"] = nd
	return files, nil
}

func hostNamesOf(m *model) map[string]string {
	out := map[string]string{}
	for h, ds := range m.docs {
		out[h] = ds[0].Name
	}
	return out
}

func sortRows(rows [][]string) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i][0] != rows[j][0] {
			return rows[i][0] < rows[j][0]
		}
		return rows[i][1] < rows[j][1]
	})
}

func replaceBlock(doc []byte, start, end, body string) ([]byte, error) {
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(start) + `.*?` + regexp.QuoteMeta(end))
	if !re.Match(doc) {
		return nil, fmt.Errorf("markers %s ... %s not found", start, end)
	}
	return re.ReplaceAllLiteral(doc, []byte(start+"\n"+strings.TrimRight(body, "\n")+"\n"+end)), nil
}
