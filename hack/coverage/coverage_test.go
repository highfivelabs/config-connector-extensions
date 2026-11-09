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
