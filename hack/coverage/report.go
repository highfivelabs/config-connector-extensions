// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	colOfficial = "#2f81f7"
	colOurs     = "#2da44e"
	colNone     = "#d0d7de"
)

// barSVG is a stacked horizontal bar in a self-contained card: official
// (blue), ours beyond official (green), uncovered (grey). The card has its
// own background so the text reads on GitHub's light and dark themes.
func barSVG(name string, c counts, big bool) string {
	w, h, bx, bw, bh := 380, 24, 6, 190, 12
	fs := 11
	if big {
		w, h, bx, bw, bh = 640, 54, 10, 620, 18
		fs = 12
	}
	scale := func(n int) float64 {
		if c.attrs == 0 {
			return 0
		}
		return float64(n) * float64(bw) / float64(c.attrs)
	}
	wo := scale(c.off)
	wb := scale(c.beyond)
	wu := float64(bw) - wo - wb
	by := (h - bh) / 2
	if big {
		by = 8
	}
	var b strings.Builder
	label := fmt.Sprintf("%s: official %s%%, ours beyond official %s%%, uncovered %s%% of %d attributes", name, pct(c.off, c.attrs), pct(c.beyond, c.attrs), pct(c.uncovered, c.attrs), c.attrs)
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\" role=\"img\" aria-label=\"%s\">\n", w, h, w, h, label)
	fmt.Fprintf(&b, "<title>%s</title>\n", label)
	fmt.Fprintf(&b, "<rect x=\"0.5\" y=\"0.5\" width=\"%d\" height=\"%d\" rx=\"4\" fill=\"#ffffff\" stroke=\"#d0d7de\"/>\n", w-1, h-1)
	fmt.Fprintf(&b, "<clipPath id=\"c\"><rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"3\"/></clipPath>\n", bx, by, bw, bh)
	fmt.Fprintf(&b, "<g clip-path=\"url(#c)\">\n")
	fmt.Fprintf(&b, "<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" fill=\"%s\"/>\n", bx, by, bw, bh, colNone)
	x := float64(bx)
	if wo > 0 {
		fmt.Fprintf(&b, "<rect x=\"%.2f\" y=\"%d\" width=\"%.2f\" height=\"%d\" fill=\"%s\"/>\n", x, by, wo, bh, colOfficial)
		x += wo
	}
	if wb > 0 {
		fmt.Fprintf(&b, "<rect x=\"%.2f\" y=\"%d\" width=\"%.2f\" height=\"%d\" fill=\"%s\"/>\n", x, by, wb, bh, colOurs)
	}
	fmt.Fprintf(&b, "</g>\n")
	_ = wu
	font := "font-family=\"-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif\""
	if big {
		fmt.Fprintf(&b, "<text x=\"%d\" y=\"%d\" %s font-size=\"%d\" fill=\"#24292f\">", bx, by+bh+17, font, fs)
		fmt.Fprintf(&b, "<tspan fill=\"%s\">&#9632;</tspan> official %s%%   <tspan fill=\"%s\">&#9632;</tspan> ours beyond %s%%   <tspan fill=\"#8c959f\">&#9632;</tspan> uncovered %s%%   %d attributes, %d resources</text>\n",
			colOfficial, pct(c.off, c.attrs), colOurs, pct(c.beyond, c.attrs), pct(c.uncovered, c.attrs), c.attrs, c.resources)
	} else {
		fmt.Fprintf(&b, "<text x=\"%d\" y=\"%d\" %s font-size=\"%d\" fill=\"#24292f\">", bx+bw+8, by+bh-2, font, fs)
		fmt.Fprintf(&b, "<tspan fill=\"#0969da\">%s%%</tspan> / <tspan fill=\"#1a7f37\">%s%%</tspan> / <tspan fill=\"#57606a\">%s%%</tspan></text>\n",
			pct(c.off, c.attrs), pct(c.beyond, c.attrs), pct(c.uncovered, c.attrs))
	}
	b.WriteString("</svg>\n")
	return b.String()
}

var tierNames = [3]string{"Tier 1: platform foundation", "Tier 2: security-adjacent and application", "Tier 3: everything else"}

var categoryNames = [4]string{"Platform foundation", "Security-adjacent", "Application", "Other"}

// rollupSVG is one bar per row (tier or category), in the same card style
// as barSVG.
func rollupSVG(title string, names []string, cs []counts, svcs []int) string {
	const w, rowH, bx, bw, bh = 640, 40, 10, 620, 14
	h := rowH*len(cs) + 6
	var b strings.Builder
	font := "font-family=\"-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif\""
	label := title + ":"
	for i, c := range cs {
		label += fmt.Sprintf(" %s official %s%%, ours beyond official %s%%, uncovered %s%%;", names[i], pct(c.off, c.attrs), pct(c.beyond, c.attrs), pct(c.uncovered, c.attrs))
	}
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\" role=\"img\" aria-label=\"%s\">\n", w, h, w, h, label)
	fmt.Fprintf(&b, "<title>%s</title>\n", label)
	fmt.Fprintf(&b, "<rect x=\"0.5\" y=\"0.5\" width=\"%d\" height=\"%d\" rx=\"4\" fill=\"#ffffff\" stroke=\"#d0d7de\"/>\n", w-1, h-1)
	for i, c := range cs {
		y := 3 + i*rowH
		fmt.Fprintf(&b, "<clipPath id=\"c%d\"><rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"3\"/></clipPath>\n", i, bx, y+18, bw, bh)
		fmt.Fprintf(&b, "<text x=\"%d\" y=\"%d\" %s font-size=\"12\" fill=\"#24292f\">%s (%d services, %d attributes): ", bx, y+13, font, names[i], svcs[i], c.attrs)
		fmt.Fprintf(&b, "<tspan fill=\"#0969da\">%s%%</tspan> / <tspan fill=\"#1a7f37\">%s%%</tspan> / <tspan fill=\"#57606a\">%s%%</tspan></text>\n", pct(c.off, c.attrs), pct(c.beyond, c.attrs), pct(c.uncovered, c.attrs))
		scale := func(n int) float64 {
			if c.attrs == 0 {
				return 0
			}
			return float64(n) * float64(bw) / float64(c.attrs)
		}
		wo, wb := scale(c.off), scale(c.beyond)
		fmt.Fprintf(&b, "<g clip-path=\"url(#c%d)\">\n<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" fill=\"%s\"/>\n", i, bx, y+18, bw, bh, colNone)
		if wo > 0 {
			fmt.Fprintf(&b, "<rect x=\"%d\" y=\"%d\" width=\"%.2f\" height=\"%d\" fill=\"%s\"/>\n", bx, y+18, wo, bh, colOfficial)
		}
		if wb > 0 {
			fmt.Fprintf(&b, "<rect x=\"%.2f\" y=\"%d\" width=\"%.2f\" height=\"%d\" fill=\"%s\"/>\n", float64(bx)+wo, y+18, wb, bh, colOurs)
		}
		b.WriteString("</g>\n")
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func rootSection(sums []svcSum, prio *Priority, tierSums [3]counts, tierSvcs [3]int, catSums [4]counts, catN [4]int, total counts, nServices, nExcluded int, src *Sources) string {
	// Tiers 1 and 2 list every service, covered or not (an uncovered
	// priority service is the point); tier 3 only where something is
	// covered. Tier first, then covered attributes.
	var list []svcSum
	for _, s := range sums {
		if s.tier < 3 || s.c.off+s.c.beyond > 0 {
			list = append(list, s)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].tier != list[j].tier {
			return list[i].tier < list[j].tier
		}
		a, b := list[i].c.off+list[i].c.beyond, list[j].c.off+list[j].c.beyond
		if a != b {
			return a > b
		}
		return list[i].host < list[j].host
	})
	var b strings.Builder
	b.WriteString("## Coverage\n\n")
	fmt.Fprintf(&b, "How much of Google Cloud's API can be managed as Kubernetes resources, counted in attributes (writable fields of a resource's create request) across %d services and %d resources. ", nServices, total.resources)
	b.WriteString("**Official** is what Config Connector's CRDs at the pin (`UPSTREAM.md`) cover; **ours beyond official** is what only this repository's kinds cover; **uncovered** is in the API and in neither. ")
	b.WriteString("The counts come from matching CRD fields to API fields by name, so they are estimates: see [docs/coverage](docs/coverage/README.md) for the method, its error and how to regenerate.\n\n")
	b.WriteString("![All services](docs/coverage/bars/_total.svg)\n\n")
	b.WriteString("Services do not matter equally, so the total misleads. Everything up through a managed Kubernetes cluster is table stakes for a platform team; security teams need what sits next to it; the rest is application primitives. Each service has a **category** and a **tier** ([`priority.csv`](docs/coverage/priority.csv), editable): ")
	b.WriteString("**tier 1** is `platform-foundation` (organization, folders, projects, organization policy, IAM, tags, networking, DNS, KMS, log sinks, GKE and Fleet), **tier 2** is `security-adjacent` (Security Command Center, VPC Service Controls, Cloud Asset, Binary Authorization, Secret Manager, CA, Armor) and `application` (Pub/Sub, Storage, Run, SQL, BigQuery, ...), **tier 3** is the rest. ")
	fmt.Fprintf(&b, "The services counted are Google Cloud infrastructure APIs; %d non-cloud APIs in Discovery (Workspace, Ads, Play, Maps, YouTube, ...) are left out ([how](docs/coverage/README.md#tiers-and-categories)).\n\n", nExcluded)
	b.WriteString("By category, counted per resource (Compute Engine's VPC networking is foundation, its machines and load balancers are application):\n\n")
	b.WriteString("![Coverage by category](docs/coverage/bars/_categories.svg)\n\n")
	b.WriteString("By tier, counted per service:\n\n")
	b.WriteString("![Coverage by tier](docs/coverage/bars/_tiers.svg)\n\n")
	b.WriteString("<sub>Blue: official. Green: ours beyond official. Grey: uncovered. Each bar reads official / ours beyond / uncovered. Rollups: [`categories.csv`](docs/coverage/categories.csv), [`tiers.csv`](docs/coverage/tiers.csv); every service: [`services.csv`](docs/coverage/services.csv). What to build first for the foundation: [`foundation.md`](docs/coverage/foundation.md).</sub>\n\n")
	row := func(s svcSum) string {
		return fmt.Sprintf("| %d | %s | `%s` | ![%s](docs/coverage/bars/%s.svg) | %s%% | %s%% | %s%% |\n", s.tier, catShort(prio.Cat[s.host]), s.host, s.host, s.host, pct(s.c.off, s.c.attrs), pct(s.c.beyond, s.c.attrs), pct(s.c.uncovered, s.c.attrs))
	}
	head := "| Tier | Category | Service | Coverage | Official | Ours beyond | Uncovered |\n| ---: | --- | --- | --- | ---: | ---: | ---: |\n"
	for t := 1; t <= 3; t++ {
		var rows []svcSum
		for _, s := range list {
			if s.tier == t {
				rows = append(rows, s)
			}
		}
		if t == 1 {
			fmt.Fprintf(&b, "Tier 1, %d services, most covered attributes first:\n\n", len(rows))
			b.WriteString(head)
			for _, s := range rows {
				b.WriteString(row(s))
			}
			continue
		}
		what := "services other modules use"
		if t == 3 {
			what = "other services where something is covered"
		}
		fmt.Fprintf(&b, "\n<details><summary>Tier %d: %d %s</summary>\n\n", t, len(rows), what)
		b.WriteString(head)
		for _, s := range rows {
			b.WriteString(row(s))
		}
		b.WriteString("\n</details>\n")
	}
	return b.String()
}

func statsSection(m *model, ov *Overrides, total counts, split bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Generated by `hack/coverage`; do not edit between the markers.\n\n")
	fmt.Fprintf(&b, "| | |\n| --- | --- |\n")
	fmt.Fprintf(&b, "| Services in the denominator (with at least one resource) | %d |\n", len(m.hosts))
	fmt.Fprintf(&b, "| Resources (REST resources with an insert or create method) | %d |\n", total.resources)
	fmt.Fprintf(&b, "| Attributes (writable fields) | %d |\n", total.attrs)
	fmt.Fprintf(&b, "| Resources with an official kind | %d |\n", total.resOff)
	fmt.Fprintf(&b, "| Resources covered only by our kinds | %d |\n", total.resBeyond)
	fmt.Fprintf(&b, "| Attributes: official / ours beyond official / uncovered | %d / %d / %d |\n", total.off, total.beyond, total.uncovered)
	fmt.Fprintf(&b, "| Official kinds (Config Connector CRDs in `*.cnrm.cloud.google.com`) | %d |\n", len(m.offCRDs))
	fmt.Fprintf(&b, "| Official kinds mapped to at least one resource | %d |\n", m.offMapped)
	fmt.Fprintf(&b, "| Official kinds **unmapped** | %d |\n", len(m.offUnmap))
	fmt.Fprintf(&b, "| Our kinds | %d |\n", len(m.oursCRDs))
	fmt.Fprintf(&b, "| Our kinds mapped to at least one resource | %d |\n", m.oursMapped)
	fmt.Fprintf(&b, "| Our kinds outside the denominator | %d |\n", len(m.oursUnmap))
	fmt.Fprintf(&b, "| Discovery APIs excluded as not Google Cloud | %d |\n", len(uniq(m.excluded)))
	fmt.Fprintf(&b, "| Cloud services with no creatable resource (not rows) | %d |\n", len(m.noCreate))
	if split {
		fmt.Fprintf(&b, "| `attributes.csv` | split into `attributes/<service>.csv` (over 25 MB) |\n")
	}
	b.WriteString("\n")

	b.WriteString("### Our kinds outside the denominator\n\n")
	if len(m.oursUnmap) == 0 {
		b.WriteString("None.\n\n")
	}
	for _, u := range m.oursUnmap {
		fmt.Fprintf(&b, "- `%s` (%s): %s\n", u.crd.Kind, u.crd.Group, u.reason)
	}
	b.WriteString("\n### Official kinds that could not be mapped to a resource\n\n")
	b.WriteString("These are in the official column of no row, so the official percentages undercount them. By reason:\n\n")
	reasons := map[string][]string{}
	for _, u := range m.offUnmap {
		r := u.reason
		switch {
		case strings.HasPrefix(r, "no discovery service for group"):
			r = "no Discovery service for the group (gRPC-only API or not in Discovery): " + strings.TrimPrefix(r, "no discovery service for group ")
		case strings.HasPrefix(r, "no API resource named like"):
			r = "service found, no creatable resource with a matching name"
		}
		reasons[r] = append(reasons[r], u.crd.Kind)
	}
	// Collapse the per-group reasons.
	grouped := map[string][]string{}
	for r, ks := range reasons {
		key := r
		if strings.HasPrefix(r, "no Discovery service for the group") {
			key = "no Discovery service for the group (gRPC-only API, not in Discovery, or excluded)"
		}
		grouped[key] = append(grouped[key], ks...)
	}
	for _, r := range sortedKeys(grouped) {
		ks := grouped[r]
		sort.Strings(ks)
		fmt.Fprintf(&b, "- %s (%d): %s\n", r, len(ks), "`"+strings.Join(ks, "`, `")+"`")
	}
	b.WriteString("\n### Discovery APIs excluded as not Google Cloud\n\n")
	b.WriteString("`" + strings.Join(uniq(m.excluded), "`, `") + "`\n\n")
	b.WriteString("### Cloud services with no creatable resource\n\n")
	b.WriteString("`" + strings.Join(m.noCreate, "`, `") + "`\n")
	return b.String()
}

func uniq(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func catShort(c string) string {
	switch c {
	case "platform-foundation":
		return "foundation"
	case "security-adjacent":
		return "security"
	case "application":
		return "application"
	}
	return "other"
}
