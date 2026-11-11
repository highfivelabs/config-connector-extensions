// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// The Terraform comparison: how much of the same denominator the Terraform
// providers' writable attributes match, from the catalog repository's
// docs/terraform/config-connector-extensions/ (method and error in its
// docs/terraform-leverage.md, "Method and error"). The two files are
// vendored byte for byte under docs/coverage/inputs/terraform/ at the
// catalog commit pinned in sources.yaml, which also holds their sha256.

const (
	tfDir = "docs/coverage/inputs/terraform"
	// tfBarProvider is the provider drawn as the bar; the other one is a
	// table column (they differ by under a point).
	tfBarProvider   = "google-beta"
	tfOtherProvider = "google"
	colTerraform    = "#7b42bc"
	colTerraformL1  = "#24292f"
)

var tfFiles = []string{"terraform.csv", "terraform-resources.csv"}

// tfCounts is one Terraform figure: L1 (strict) and L2 (lenient) matches
// over the attributes the catalog counted.
type tfCounts struct {
	attrs, l1, l2 int
}

func (t *tfCounts) add(o tfCounts) {
	t.attrs += o.attrs
	t.l1 += o.l1
	t.l2 += o.l2
}

// tfData is terraform.csv summed per provider: by service and by category.
type tfData struct {
	svc map[string]map[string]tfCounts // provider -> service -> counts
	cat map[string]map[string]tfCounts // provider -> category -> counts
	all map[string]tfCounts            // provider -> counts
}

func (d *tfData) service(provider, host string) (tfCounts, bool) {
	c, ok := d.svc[provider][host]
	return c, ok
}

// fileSHA256 is the hex sha256 of one file.
func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

// tfDigests are the sha256 of the vendored files, in tfFiles order.
func tfDigests(repo string) ([]string, error) {
	var out []string
	for _, f := range tfFiles {
		d, err := fileSHA256(filepath.Join(repo, tfDir, f))
		if err != nil {
			return nil, fmt.Errorf("Terraform inputs: %w", err)
		}
		out = append(out, d)
	}
	return out, nil
}

func loadTerraform(repo string) (*tfData, error) {
	f, err := os.Open(filepath.Join(repo, tfDir, "terraform.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseTerraform(f)
}

// parseTerraform reads terraform.csv (one row per service, provider and
// category; Compute Engine has a row per category its resources fall in)
// and sums it by service, by category and overall.
func parseTerraform(r io.Reader) (*tfData, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("terraform.csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("terraform.csv: empty")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	for _, h := range []string{"service", "provider", "category", "attributes_total", "attributes_terraform_l1", "attributes_terraform_l2"} {
		if _, ok := col[h]; !ok {
			return nil, fmt.Errorf("terraform.csv: no column %s", h)
		}
	}
	d := &tfData{svc: map[string]map[string]tfCounts{}, cat: map[string]map[string]tfCounts{}, all: map[string]tfCounts{}}
	for n, row := range rows[1:] {
		var c tfCounts
		for _, x := range []struct {
			name string
			p    *int
		}{{"attributes_total", &c.attrs}, {"attributes_terraform_l1", &c.l1}, {"attributes_terraform_l2", &c.l2}} {
			v, err := strconv.Atoi(row[col[x.name]])
			if err != nil {
				return nil, fmt.Errorf("terraform.csv line %d: %s: %w", n+2, x.name, err)
			}
			*x.p = v
		}
		p, s, k := row[col["provider"]], row[col["service"]], row[col["category"]]
		if d.svc[p] == nil {
			d.svc[p] = map[string]tfCounts{}
			d.cat[p] = map[string]tfCounts{}
		}
		sc := d.svc[p][s]
		sc.add(c)
		d.svc[p][s] = sc
		kc := d.cat[p][k]
		kc.add(c)
		d.cat[p][k] = kc
		a := d.all[p]
		a.add(c)
		d.all[p] = a
	}
	return d, nil
}

// tfPct is a Terraform percentage, or "—" when there is no figure.
func tfPct(c tfCounts, ok bool) string {
	if !ok || c.attrs == 0 {
		return "—"
	}
	return pct(c.l2, c.attrs)
}
