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

package main

import (
	"strings"
	"testing"
)

// skipSpec writes the spec of a small schema the way a compute instance
// template's is: a list of disks whose items use one key struct ($ref
// Key) at two paths, diskKey and init.sourceKey.
func skipSpec(skip map[string]string) (string, error) {
	str := &Schema{Type: "string"}
	d := &Discovery{Schemas: map[string]*Schema{
		"Key": {Properties: map[string]*Schema{"rawKey": str, "kmsKeyName": str}},
		"Disk": {Properties: map[string]*Schema{
			"diskKey": {Ref: "Key"},
			"init":    {Type: "object", Properties: map[string]*Schema{"sourceKey": {Ref: "Key"}, "size": str}},
		}},
	}}
	top := &Schema{Properties: map[string]*Schema{
		"description": str,
		"etag":        str,
		"properties": {Type: "object", Properties: map[string]*Schema{
			"disks": {Type: "array", Items: &Schema{Ref: "Disk"}},
		}},
	}}
	g := &typeGen{k: &built{c: &Config{Kind: "K", Skip: skip}, d: d}, defs: map[string]string{},
		inStack: map[string]bool{}, secrets: map[string]bool{}, skipped: map[string]bool{}, refPaths: map[string][]string{}}
	out := g.structDef("KSpec", top, true, "")
	for _, n := range g.order {
		out += g.defs[n]
	}
	return out, g.checkSkips()
}

// TestSkipNested: skip takes a nested field's dotted path, through lists
// and shared ($ref) structs, and refuses a path the spec does not have
// or a shared struct skipped at only some of its paths.
func TestSkipNested(t *testing.T) {
	both := map[string]string{
		"etag":                              "x",
		"properties.disks[].diskKey.rawKey": "x",
		"properties.disks[].init.sourceKey.rawKey": "x",
	}
	out, err := skipSpec(both)
	if err != nil {
		t.Fatalf("skip at both paths: %v", err)
	}
	for _, f := range []string{"RawKey", "Etag "} {
		if strings.Contains(out, f) {
			t.Errorf("%s is still in the spec:\n%s", f, out)
		}
	}
	for _, f := range []string{"KmsKeyName", "Size", "Description"} {
		if !strings.Contains(out, f) {
			t.Errorf("%s is missing from the spec:\n%s", f, out)
		}
	}

	if out, err := skipSpec(nil); err != nil || !strings.Contains(out, "RawKey") {
		t.Errorf("no skip: err %v, RawKey in spec %v", err, strings.Contains(out, "RawKey"))
	}

	for name, c := range map[string]struct {
		skip map[string]string
		want string
	}{
		"only where the struct is written": {
			map[string]string{"properties.disks[].diskKey.rawKey": "x"},
			"skip properties.disks[].init.sourceKey.rawKey too",
		},
		"only where it is reused": {
			map[string]string{"properties.disks[].init.sourceKey.rawKey": "x"},
			"skip properties.disks[].diskKey.rawKey too",
		},
		"a path the spec does not have": {
			map[string]string{"properties.disks[].diskKey.sha256": "x"},
			"properties.disks[].diskKey.sha256 is not a writable field of the spec",
		},
		"a list path without its []": {
			map[string]string{"properties.disks.diskKey.rawKey": "x"},
			"properties.disks.diskKey.rawKey is not a writable field of the spec",
		},
	} {
		_, err := skipSpec(c.skip)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, want %q", name, err, c.want)
		}
	}
}
