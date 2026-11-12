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

package kindtest

// Every discokind fixture against its CRD's OpenAPI schema (required
// fields, types, string lengths, fields the schema does not have), without a
// kube-apiserver: a fixture that misses a field
// the API marks Required fails here in a second rather than in an envtest
// run. CEL rules are not checked here; the end-to-end test checks them.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func repoRoot(*testing.T) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

func TestFixturesMatchTheirCRDs(t *testing.T) {
	root := repoRoot(t)
	schemas := map[string]*apiextensionsv1.JSONSchemaProps{} // kind -> schema
	crds, _ := filepath.Glob(filepath.Join(root, "config/crds/*.yaml"))
	for _, f := range crds {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal(b, crd); err != nil || crd.Spec.Names.Kind == "" || len(crd.Spec.Versions) == 0 {
			continue
		}
		schemas[crd.Spec.Names.Kind] = crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	}
	fixtures, _ := filepath.Glob(filepath.Join(root, "pkg/controller/direct/*/testdata/discokind/*/*.yaml"))
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, f := range fixtures {
		rel, _ := filepath.Rel(root, f)
		t.Run(rel, func(t *testing.T) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			s := strings.NewReplacer("${PROJECT}", "mock-project", "${NAME}", "cce-11111111-abcd-kind", "${ID}", "cce11111111abcdkind", "${RUN_ID}", "cce-11111111-abcd", "${ORGANIZATION}", "123456789").Replace(string(b))
			obj := map[string]any{}
			if err := yaml.Unmarshal([]byte(s), &obj); err != nil {
				t.Fatal(err)
			}
			kind, _ := obj["kind"].(string)
			v1schema := schemas[kind]
			if v1schema == nil {
				t.Fatalf("no CRD for kind %q", kind)
			}
			for _, e := range check("", obj, v1schema) {
				t.Errorf("%s does not match its CRD: %s", rel, e)
			}
		})
	}
}

// check is a small structural validation: required properties present,
// JSON types as the schema says, no properties the schema lacks (where it
// does not preserve unknown fields).
func check(path string, v any, s *apiextensionsv1.JSONSchemaProps) []string {
	if s == nil || (s.XPreserveUnknownFields != nil && *s.XPreserveUnknownFields) {
		return nil
	}
	var errs []string
	switch s.Type {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: want an object, got %T", path, v)}
		}
		for _, r := range s.Required {
			if _, ok := m[r]; !ok {
				errs = append(errs, fmt.Sprintf("%s.%s: required", path, r))
			}
		}
		for k, sub := range m {
			if len(s.Properties) == 0 && s.AdditionalProperties != nil {
				if s.AdditionalProperties.Schema != nil {
					errs = append(errs, check(path+"."+k, sub, s.AdditionalProperties.Schema)...)
				}
				continue
			}
			if path == "" && (k == "apiVersion" || k == "kind" || k == "metadata") {
				continue
			}
			p, ok := s.Properties[k]
			if !ok {
				if len(s.Properties) > 0 {
					errs = append(errs, fmt.Sprintf("%s.%s: not in the schema", path, k))
				}
				continue
			}
			errs = append(errs, check(path+"."+k, sub, &p)...)
		}
	case "array":
		l, ok := v.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: want an array, got %T", path, v)}
		}
		if s.Items != nil {
			for i, e := range l {
				errs = append(errs, check(fmt.Sprintf("%s[%d]", path, i), e, s.Items.Schema)...)
			}
		}
	case "string":
		str, ok := v.(string)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: want a string, got %T", path, v))
		}
		if n := utf8.RuneCountInString(str); s.MaxLength != nil && int64(n) > *s.MaxLength {
			errs = append(errs, fmt.Sprintf("%s: %d characters, more than its maxLength %d", path, n, *s.MaxLength))
		}
	case "integer", "number":
		switch v.(type) {
		case float64, int, int64:
		default:
			errs = append(errs, fmt.Sprintf("%s: want a number, got %T", path, v))
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			errs = append(errs, fmt.Sprintf("%s: want a boolean, got %T", path, v))
		}
	}
	return errs
}
