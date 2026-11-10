// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestSingular(t *testing.T) {
	for in, want := range map[string]string{
		"services": "service", "databases": "database", "addresses": "address", "policies": "policy",
		"zones": "zone", "apis": "api", "status": "status", "analysis": "analysis", "denied": "deny",
		"boxes": "box", "keyrings": "keyring",
	} {
		if got := singular(in); got != want {
			t.Errorf("singular(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormSegs(t *testing.T) {
	got := normSegs("networkInterfaces[].accessConfigs[].natIP")
	want := []string{"networkinterface", "accessconfig", "natip"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normSegs = %v, want %v", got, want)
	}
	if normSegs("labels{}")[0] != "label" {
		t.Errorf("a map marker should vanish")
	}
}

func crdWith(paths ...CPath) *CRD {
	c := &CRD{Kind: "X"}
	for _, p := range paths {
		p.key = crdKey(p)
		c.Paths = append(c.Paths, p)
	}
	return c
}

func res(paths ...string) *APIResource {
	r := &APIResource{}
	for _, p := range paths {
		r.Attrs = append(r.Attrs, Attr{Path: p})
	}
	return r
}

func TestMatchAttrs(t *testing.T) {
	r := res("name", "kmsKeyName", "billing.requesterPays", "disks[].autoDelete", "x.enabled", "labels{}", "unrelated")
	c := crdWith(
		CPath{Path: "kmsKeyRef", Ref: true},
		CPath{Path: "requesterPays"},
		CPath{Path: "bootDisk.autoDelete"},
		CPath{Path: "x"},
		CPath{Path: "labels{}"},
	)
	got := matchAttrs(r, c, false)
	covered := map[string]string{}
	for i, p := range got {
		covered[r.Attrs[i].Path] = p
	}
	want := map[string]string{
		"kmsKeyName": "kmsKeyRef", "billing.requesterPays": "requesterPays",
		"disks[].autoDelete": "bootDisk.autoDelete", "x.enabled": "x", "labels{}": "labels{}",
	}
	if !reflect.DeepEqual(covered, want) {
		t.Errorf("covered = %v, want %v", covered, want)
	}
}

func TestFlattenDiscovery(t *testing.T) {
	d := &dDoc{Schemas: map[string]*dSchema{
		"Thing": {Type: "object", Properties: map[string]*dSchema{
			"name":    {Type: "string", Description: "The name."},
			"id":      {Type: "string"},
			"etag":    {Type: "string"},
			"created": {Type: "string", Description: "Output only. When."},
			"mode":    {Type: "string", Description: "Required. The mode."},
			"labels":  {Type: "object", AdditionalProperties: &dSchema{Type: "string"}},
			"ports":   {Type: "array", Items: &dSchema{Type: "string"}},
			"child":   {Ref: "Child"},
			"loop":    {Ref: "Thing"},
		}},
		"Child": {Type: "object", Properties: map[string]*dSchema{
			"size":  {Type: "integer"},
			"ro":    {Type: "string", ReadOnly: true},
			"items": {Type: "array", Items: &dSchema{Ref: "Child"}},
		}},
	}}
	r := &APIResource{}
	(&flattener{doc: d, res: r}).run("Thing", "thing")
	var got []string
	for _, a := range r.Attrs {
		got = append(got, a.Path)
	}
	want := "child.items[]|child.size|id|labels{}|loop|mode|name|ports[]"
	if strings.Join(got, "|") != want {
		t.Errorf("attributes = %s, want %s", strings.Join(got, "|"), want)
	}
	for _, a := range r.Attrs {
		if a.Path == "mode" && !a.Required {
			t.Errorf("mode should be required")
		}
	}
}

func TestParseTerraform(t *testing.T) {
	in := "service,provider,category,resources_total,attributes_total,attributes_terraform_l1,attributes_terraform_l2,pct_terraform_l2\n" +
		"compute.googleapis.com,google-beta,platform-foundation,3,100,40,60,60.0\n" +
		"compute.googleapis.com,google-beta,application,5,300,90,120,40.0\n" +
		"run.googleapis.com,google-beta,application,2,100,50,50,50.0\n" +
		"run.googleapis.com,google,application,2,100,45,49,49.0\n"
	d, err := parseTerraform(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := d.service("google-beta", "compute.googleapis.com"); got != (tfCounts{400, 130, 180}) {
		t.Errorf("compute summed over categories = %+v", got)
	}
	if got := d.cat["google-beta"]["application"]; got != (tfCounts{400, 140, 170}) {
		t.Errorf("application = %+v", got)
	}
	if got := d.all["google-beta"]; got != (tfCounts{500, 180, 230}) || pct(got.l2, got.attrs) != "46.0" {
		t.Errorf("all = %+v", got)
	}
	if got := d.all["google"]; got != (tfCounts{100, 45, 49}) {
		t.Errorf("google = %+v", got)
	}
	if _, ok := d.service("google-beta", "absent.googleapis.com"); ok {
		t.Errorf("an absent service has no figure")
	}
	if tfPct(tfCounts{}, false) != "—" {
		t.Errorf("no figure should read —")
	}
	if _, err := parseTerraform(strings.NewReader("service,provider\nx,y\n")); err == nil {
		t.Errorf("missing columns should fail")
	}
}
