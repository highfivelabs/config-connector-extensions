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

// The secret guard: every spec field of ours that holds a secret takes
// Config Connector's secret.Legacy (value, or valueFrom.secretKeyRef;
// decision 33) or has a <field>From.secretKeyRef beside it, unless
// kinds.yaml waives it with a reason (secretWaivers). A field holds a
// secret when
//
//   - its name says so (secretNames), or
//   - Crossplane's Upjet provider marks the Terraform field Sensitive, or
//     takes it write-only (<field>_wo, <field>_wo_version), for the
//     kind's Terraform resource (hack/upjet/provider-upjet-gcp.json).
//
// It reads the committed CRDs, so it needs neither Discovery nor the
// network. The discokind kinds' waivers are in kinds.yaml; a hand-written
// kind has none, so its secrets must be secrets.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// secretNames are field names (lower case) that hold a credential or a
// key whatever the API: Terraform marks such fields Sensitive. Not
// "secret" or "token" alone, which name Secret Manager secrets and
// resource IDs as often as values. hack/coverage/upjet.go uses the same
// list.
var secretNames = map[string]bool{
	"password": true, "rawkey": true, "rsaencryptedkey": true, "sharedsecret": true,
	"privatekey": true, "clientsecret": true, "apikey": true, "secretkey": true,
	"accesstoken": true, "refreshtoken": true, "passphrase": true,
}

// crdField is one spec field of a CRD: its dotted path ([] after a list,
// {} after a map) and its schema.
type crdField struct {
	path string
	node map[string]any
}

func asObj(v any) map[string]any { m, _ := v.(map[string]any); return m }

// specFields lists every field under spec, objects included, but not
// inside a secret.Legacy object.
func specFields(spec map[string]any) []crdField {
	var out []crdField
	var walk func(n map[string]any, path string)
	walk = func(n map[string]any, path string) {
		if path != "" {
			out = append(out, crdField{path, n})
		}
		if isLegacySecret(n) {
			return
		}
		if items := asObj(n["items"]); n["type"] == "array" && items != nil {
			walk(items, path+"[]")
			return
		}
		if ap := asObj(n["additionalProperties"]); ap != nil {
			walk(ap, path+"{}")
			return
		}
		props := asObj(n["properties"])
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := k
			if path != "" {
				p = path + "." + k
			}
			walk(asObj(props[k]), p)
		}
	}
	walk(spec, "")
	return out
}

// isLegacySecret: an object of value and valueFrom (Config Connector's
// secret.Legacy).
func isLegacySecret(n map[string]any) bool {
	props := asObj(n["properties"])
	if props["value"] == nil {
		return false
	}
	for k := range props {
		if k != "value" && k != "valueFrom" {
			return false
		}
	}
	return true
}

var markerRE = regexp.MustCompile(`\[\]|\{\}`)

func leaf(path string) string {
	p := markerRE.ReplaceAllString(path, "")
	return p[strings.LastIndex(p, ".")+1:]
}

// normPath compares a CRD path with a Terraform one: lower case, no
// underscores or list markers, a plural's s dropped.
func normPath(p string) string {
	segs := strings.Split(markerRE.ReplaceAllString(p, ""), ".")
	for i, s := range segs {
		s = strings.ToLower(strings.ReplaceAll(s, "_", ""))
		segs[i] = strings.TrimSuffix(s, "s")
	}
	return strings.Join(segs, ".")
}

// upjetSlice is the part of hack/upjet/provider-upjet-gcp.json the guard reads.
type upjetSlice struct {
	Entries []struct {
		Ours struct {
			OursKinds []string `json:"oursKinds"`
		} `json:"ours"`
		Facts struct {
			Terraform       string `json:"terraform"`
			SchemaOverrides []struct {
				Field    string `json:"field"`
				Property string `json:"property"`
				Value    string `json:"value"`
				At       string `json:"at"`
			} `json:"schemaOverrides"`
			MovedToStatus []struct {
				Field string `json:"field"`
				At    string `json:"at"`
			} `json:"movedToStatus"`
		} `json:"facts"`
	} `json:"entries"`
}

// upjetSecrets are the Terraform fields Upjet holds secret for a kind,
// as field -> where, from the entries joined to the kind whose Terraform
// resource is the kind's own: kinds.yaml's terraform, or the same name
// (SQLUser, google_sql_user).
func upjetSecrets(s *upjetSlice, kind, terraform string) map[string]string {
	out := map[string]string{}
	for _, e := range s.Entries {
		if !contains(e.Ours.OursKinds, kind) {
			continue
		}
		tf := e.Facts.Terraform
		if tf != terraform && strings.ReplaceAll(strings.TrimPrefix(tf, "google_"), "_", "") != strings.ToLower(kind) {
			continue
		}
		for _, o := range e.Facts.SchemaOverrides {
			if o.Property == "Sensitive" && o.Value == "true" {
				out[writeOnlyBase(o.Field)] = "Upjet Sensitive, " + o.At
			}
		}
		for _, m := range e.Facts.MovedToStatus {
			if b := writeOnlyBase(m.Field); b != m.Field {
				out[b] = "Upjet write-only " + m.Field + ", " + m.At
			}
		}
	}
	return out
}

// writeOnlyBase is the field a Terraform write-only field stands for:
// password_wo and password_wo_version are password.
func writeOnlyBase(f string) string {
	for _, suf := range []string{"_wo_version", "_wo"} {
		if strings.HasSuffix(f, suf) {
			return strings.TrimSuffix(f, suf)
		}
	}
	return f
}

// secretFinding is one field the guard holds secret, and what the CRD
// does with it.
type secretFinding struct {
	path, why string
	secret    bool // secret.Legacy, or a <field>From.secretKeyRef beside it
}

// guardKind finds the secret fields of one CRD's spec.
func guardKind(spec map[string]any, upjet map[string]string) []secretFinding {
	fields := specFields(spec)
	byPath := map[string]crdField{}
	for _, f := range fields {
		byPath[f.path] = f
	}
	isSecret := func(f crdField) bool {
		if isLegacySecret(f.node) {
			return true
		}
		_, ok := byPath[f.path+"From.secretKeyRef"]
		return ok
	}
	seen := map[string]bool{}
	var out []secretFinding
	add := func(f crdField, why string) {
		if seen[f.path] {
			return
		}
		seen[f.path] = true
		out = append(out, secretFinding{f.path, why, isSecret(f)})
	}
	for _, f := range fields {
		if secretNames[strings.ToLower(leaf(f.path))] {
			add(f, "named "+leaf(f.path))
		}
	}
	tfFields := make([]string, 0, len(upjet))
	for tf := range upjet {
		tfFields = append(tfFields, tf)
	}
	sort.Strings(tfFields)
	for _, tf := range tfFields {
		for _, f := range fields {
			if normPath(f.path) == normPath(tf) {
				add(f, upjet[tf])
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func repoRoot(t *testing.T) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// loadSpecs reads config/crds: kind -> the first version's spec schema.
func loadSpecs(t *testing.T, root string) map[string]map[string]any {
	files, err := filepath.Glob(filepath.Join(root, "config/crds/*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no CRDs in config/crds: %v", err)
	}
	out := map[string]map[string]any{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var crd map[string]any
		if err := yaml.Unmarshal(b, &crd); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if crd["kind"] != "CustomResourceDefinition" {
			continue
		}
		spec := asObj(crd["spec"])
		kind, _ := asObj(spec["names"])["kind"].(string)
		vs, _ := spec["versions"].([]any)
		if kind == "" || len(vs) == 0 {
			continue
		}
		schema := asObj(asObj(asObj(vs[0])["schema"])["openAPIV3Schema"])
		out[kind] = asObj(asObj(schema["properties"])["spec"])
	}
	return out
}

func loadUpjet(t *testing.T, root string) *upjetSlice {
	b, err := os.ReadFile(filepath.Join(root, "hack/upjet/provider-upjet-gcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &upjetSlice{}
	if err := json.Unmarshal(b, s); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestSecretGuard: every secret field of every kind of ours is a secret
// or waived in kinds.yaml, and every waiver names such a field.
func TestSecretGuard(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "hack/discokind/kinds.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfgs []Config
	if err := yaml.UnmarshalStrict(b, &cfgs); err != nil {
		t.Fatal(err)
	}
	cfgOf := map[string]*Config{}
	for i := range cfgs {
		cfgOf[cfgs[i].Kind] = &cfgs[i]
	}
	specs := loadSpecs(t, root)
	upjet := loadUpjet(t, root)
	kinds := make([]string, 0, len(specs))
	for k := range specs {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		c := cfgOf[kind]
		tf := ""
		var waivers map[string]string
		if c != nil {
			tf, waivers = c.Terraform, c.SecretWaivers
		}
		found := map[string]bool{}
		for _, f := range guardKind(specs[kind], upjetSecrets(upjet, kind, tf)) {
			found[f.path] = true
			why, waived := waivers[f.path]
			switch {
			case f.secret && waived:
				t.Errorf("%s: spec.%s is a secret now; drop its secretWaivers entry (%s)", kind, f.path, why)
			case !f.secret && !waived:
				t.Errorf("%s: spec.%s holds a secret (%s) but is a plain field: list it in kinds.yaml's secrets, or waive it in secretWaivers with why", kind, f.path, f.why)
			case !f.secret && strings.TrimSpace(why) == "":
				t.Errorf("%s: the secretWaivers entry for %s gives no reason", kind, f.path)
			}
		}
		for p := range waivers {
			if !found[p] {
				t.Errorf("%s: secretWaivers names %s, which the guard does not hold secret (or the spec does not have)", kind, p)
			}
		}
	}
	for _, c := range cfgs {
		if specs[c.Kind] == nil && len(c.SecretWaivers) > 0 {
			t.Errorf("%s: secretWaivers on a kind with no CRD", c.Kind)
		}
	}
}

// TestSecretGuardNamedKinds pins what the guard finds for the kinds whose
// Terraform resources hold secrets (HFL-699): the secret fields, and
// whether each is a secret, so a schema change that turns one into a
// plain string, or a guard that stops finding it, fails here by name.
func TestSecretGuardNamedKinds(t *testing.T) {
	root := repoRoot(t)
	specs := loadSpecs(t, root)
	upjet := loadUpjet(t, root)
	for kind, want := range map[string]map[string]bool{
		// password: a secret (google_sql_user.password_wo: Upjet Sensitive, write-only).
		"SQLUser": {"password": true},
		// sharedSecret: a secret (google_compute_vpn_tunnel.shared_secret_wo_version).
		"ComputeVPNTunnel": {"sharedSecret": true},
		// content may be plain or from a Secret; Upjet takes content and
		// customer_encryption as not Sensitive. No customerEncryption field.
		"StorageBucketObject": {},
		// The six CSEK keys are plain strings inside disks[] (waived:
		// secrets cannot reach a list item; docs/design.md section 7).
		"ComputeRegionInstanceTemplate": {
			"properties.disks[].diskEncryptionKey.rawKey":                                     false,
			"properties.disks[].diskEncryptionKey.rsaEncryptedKey":                            false,
			"properties.disks[].initializeParams.sourceImageEncryptionKey.rawKey":             false,
			"properties.disks[].initializeParams.sourceImageEncryptionKey.rsaEncryptedKey":    false,
			"properties.disks[].initializeParams.sourceSnapshotEncryptionKey.rawKey":          false,
			"properties.disks[].initializeParams.sourceSnapshotEncryptionKey.rsaEncryptedKey": false,
		},
		// Config Connector's CloudFunctionsFunction is ours as
		// CloudFunctions2Function: its secret environment variables and
		// volumes name Secret Manager secrets, they hold no value.
		"CloudFunctions2Function": {},
	} {
		spec := specs[kind]
		if spec == nil {
			t.Errorf("%s: no CRD", kind)
			continue
		}
		tf := map[string]string{"ComputeRegionInstanceTemplate": "google_compute_region_instance_template", "CloudFunctions2Function": "google_cloudfunctions2_function"}[kind]
		got := map[string]bool{}
		for _, f := range guardKind(spec, upjetSecrets(upjet, kind, tf)) {
			got[f.path] = f.secret
		}
		for p, w := range want {
			g, ok := got[p]
			if !ok {
				t.Errorf("%s: the guard does not find spec.%s", kind, p)
			} else if g != w {
				t.Errorf("%s: spec.%s secret = %v, want %v", kind, p, g, w)
			}
		}
		for p := range got {
			if _, ok := want[p]; !ok {
				t.Errorf("%s: the guard finds spec.%s, which this test does not expect", kind, p)
			}
		}
		if kind == "StorageBucketObject" {
			if _, ok := asObj(spec["properties"])["contentFrom"]; !ok {
				t.Error("StorageBucketObject: no contentFrom, so content can only be in the manifest")
			}
		}
	}
}

// TestGuardKind: the guard on a small spec.
func TestGuardKind(t *testing.T) {
	str := map[string]any{"type": "string"}
	legacy := map[string]any{"type": "object", "properties": map[string]any{"value": str, "valueFrom": map[string]any{"type": "object"}}}
	spec := map[string]any{"type": "object", "properties": map[string]any{
		"password":    legacy,
		"apiKey":      str,
		"content":     str,
		"contentFrom": map[string]any{"type": "object", "properties": map[string]any{"secretKeyRef": map[string]any{"type": "object"}}},
		"disks": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
			"rawKey": str, "kmsKeyName": str,
		}}},
		"secret": str,
	}}
	got := map[string]bool{}
	for _, f := range guardKind(spec, map[string]string{"content": "Upjet Sensitive, x.go:1"}) {
		got[f.path] = f.secret
	}
	want := map[string]bool{"password": true, "apiKey": false, "content": true, "disks[].rawKey": false}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for p, w := range want {
		if g, ok := got[p]; !ok || g != w {
			t.Errorf("%s: got %v (found %v), want %v", p, g, ok, w)
		}
	}
	if b := writeOnlyBase("shared_secret_wo_version"); b != "shared_secret" {
		t.Errorf("writeOnlyBase = %q", b)
	}
	if normPath("sharedSecret") != normPath("shared_secret") || normPath("disks[].rawKey") != normPath("disk.raw_key") {
		t.Error("normPath does not match a CRD path to its Terraform one")
	}
}
