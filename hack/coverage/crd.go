// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// CPath is one input field path of a CRD's spec.
type CPath struct {
	Path string
	Type string
	Free bool // a free-form object: covers whatever is under it
	Ref  bool // a reference object (name/external): one field
	key  string
}

// CRD is one custom resource definition reduced to what the matching needs.
type CRD struct {
	Kind    string
	Group   string // run.cnrm.cloud.google.com
	Service string // run
	Plural  string
	Tier    string // tf, dcl, direct
	File    string
	Paths   []CPath
	Labels  bool // labels come from metadata.labels (Terraform service mapping)
	Protos  []string
	Alpha   bool // served only as v1alpha1
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// loadCRDs reads CRD files; dir holds apiextensions CRD YAML.
func loadCRDs(dir string) ([]*CRD, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []*CRD
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc map[string]any
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if doc["kind"] != "CustomResourceDefinition" {
			continue
		}
		spec := asMap(doc["spec"])
		names := asMap(spec["names"])
		c := &CRD{File: filepath.Base(f)}
		c.Kind, _ = names["kind"].(string)
		c.Plural, _ = names["plural"].(string)
		c.Group, _ = spec["group"].(string)
		c.Service = strings.SplitN(c.Group, ".", 2)[0]
		lbl := asMap(asMap(doc["metadata"])["labels"])
		switch {
		case lbl["cnrm.cloud.google.com/tf2crd"] == "true":
			c.Tier = "tf"
		case lbl["cnrm.cloud.google.com/dcl2crd"] == "true":
			c.Tier = "dcl"
		default:
			c.Tier = "direct"
		}
		// The version to read: the storage version, else the last served.
		var chosen map[string]any
		vs, _ := spec["versions"].([]any)
		c.Alpha = len(vs) > 0
		for _, v := range vs {
			vm := asMap(v)
			if vm["storage"] == true {
				chosen = vm
			}
			if n, _ := vm["name"].(string); n != "v1alpha1" {
				c.Alpha = false
			}
		}
		if chosen == nil && len(vs) > 0 {
			chosen = asMap(vs[len(vs)-1])
		}
		if chosen == nil {
			continue
		}
		root := asMap(asMap(asMap(chosen["schema"])["openAPIV3Schema"])["properties"])
		sp := asMap(root["spec"])
		if sp == nil {
			continue
		}
		flatCRD(sp, "", &c.Paths)
		for i := range c.Paths {
			c.Paths[i].key = crdKey(c.Paths[i])
		}
		out = append(out, c)
	}
	return out, nil
}

func isRefProps(props map[string]any) bool {
	if len(props) == 0 {
		return false
	}
	has := false
	for k := range props {
		switch k {
		case "name", "external", "namespace", "kind", "apiVersion":
			if k == "name" || k == "external" {
				has = true
			}
		default:
			return false
		}
	}
	return has
}

func isValueProps(props map[string]any) bool {
	if len(props) == 0 {
		return false
	}
	has := false
	for k := range props {
		switch k {
		case "value", "valueFrom":
			has = true
		default:
			return false
		}
	}
	return has
}

func flatCRD(n map[string]any, path string, out *[]CPath) {
	typ, _ := n["type"].(string)
	props := asMap(n["properties"])
	preserve := n["x-kubernetes-preserve-unknown-fields"] == true
	switch {
	case typ == "array":
		items := asMap(n["items"])
		if items == nil {
			*out = append(*out, CPath{Path: path + "[]", Type: "any"})
			return
		}
		flatCRD(items, path+"[]", out)
	case len(props) > 0:
		if path != "" && isRefProps(props) {
			*out = append(*out, CPath{Path: path, Type: "reference", Ref: true})
			return
		}
		if path != "" && isValueProps(props) {
			*out = append(*out, CPath{Path: path, Type: "string"})
			return
		}
		for _, k := range sortedKeys(props) {
			child := asMap(props[k])
			if child == nil {
				continue
			}
			p := k
			if path != "" {
				p = path + "." + k
			}
			flatCRD(child, p, out)
		}
	case asMap(n["additionalProperties"]) != nil:
		flatCRD(asMap(n["additionalProperties"]), path+"{}", out)
	default:
		if path == "" {
			return
		}
		t := typ
		if t == "" {
			t = "any"
		}
		*out = append(*out, CPath{Path: path, Type: t, Free: typ == "object" && (preserve || len(props) == 0)})
	}
}

var markers = regexp.MustCompile(`[\[\]{}]`)

// normSegs normalizes a dotted path to lowercase alphanumeric, singular
// segments, without the array and map markers.
func normSegs(p string) []string {
	parts := strings.Split(markers.ReplaceAllString(p, ""), ".")
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		s = norm(s)
		if s != "" {
			out = append(out, singular(s))
		}
	}
	return out
}

func crdKey(p CPath) string {
	segs := strings.Split(markers.ReplaceAllString(p.Path, ""), ".")
	last := len(segs) - 1
	if p.Ref {
		l := strings.ToLower(segs[last])
		l = strings.TrimSuffix(l, "refs")
		l = strings.TrimSuffix(l, "ref")
		segs[last] = l
	}
	var out []string
	for _, s := range segs {
		s = norm(s)
		if s != "" {
			out = append(out, singular(s))
		}
	}
	return strings.Join(out, ".")
}

func attrKey(p string) string { return strings.Join(normSegs(p), ".") }

// protoMarkers reads the +kcc:proto messages each direct kind's types
// file names, keyed by the lower-cased kind.
func protoMarkers(apisDir string) map[string][]string {
	out := map[string][]string{}
	files, _ := filepath.Glob(filepath.Join(apisDir, "*", "*", "*_types.go"))
	sort.Strings(files)
	re := regexp.MustCompile(`^//\s*\+kcc:(?:spec:)?proto=([A-Za-z0-9_.]+)`)
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), "_types.go")
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if m := re.FindStringSubmatch(sc.Text()); m != nil && strings.Contains(sc.Text(), "spec:proto") {
				out[base] = append(out[base], m[1])
			}
		}
		fh.Close()
	}
	return out
}

// serviceMappings reads Config Connector's service mappings: group ->
// host, and (group, kind) -> labels live in metadata.labels.
type svcMap struct {
	GroupHost map[string]string
	LabelKind map[string]bool // kind -> labels in metadata
}

func loadServiceMappings(dir string) (*svcMap, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
	sort.Strings(files)
	sm := &svcMap{GroupHost: map[string]string{}, LabelKind: map[string]bool{}}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Spec struct {
				ServiceHostName string `yaml:"serviceHostName"`
				Resources       []struct {
					Kind            string `yaml:"kind"`
					MetadataMapping struct {
						Labels string `yaml:"labels"`
					} `yaml:"metadataMapping"`
				} `yaml:"resources"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			continue
		}
		if doc.Metadata.Name == "" {
			continue
		}
		g := strings.SplitN(doc.Metadata.Name, ".", 2)[0]
		if doc.Spec.ServiceHostName != "" {
			sm.GroupHost[g] = doc.Spec.ServiceHostName
		}
		for _, r := range doc.Spec.Resources {
			if r.MetadataMapping.Labels != "" {
				sm.LabelKind[r.Kind] = true
			}
		}
	}
	return sm, nil
}
