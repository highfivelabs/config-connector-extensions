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

// kinddocs checks that every kind has its file in docs/kinds/ and prints
// them as a table (docs/kinds/README.md):
//
//	go run ./hack/kinddocs           the table, to stdout
//	go run ./hack/kinddocs -check    fail if a CRD in config/crds has no
//	                                 docs/kinds file, or a file names a
//	                                 kind with no CRD or the wrong version
//
// A kind's file starts with front matter: `kinds: [A, B]` and
// `apiVersion: group/version`. The table is never committed, so kind pull
// requests do not edit a shared list.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

type front struct {
	Kinds      []string `json:"kinds"`
	APIVersion string   `json:"apiVersion"`
}

type crd struct {
	Spec struct {
		Group string `json:"group"`
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Versions []struct {
			Name string `json:"name"`
		} `json:"versions"`
	} `json:"spec"`
}

type doc struct {
	file     string
	front    front
	gap      string
	lastLive string
}

func main() {
	check := flag.Bool("check", false, "fail on a missing or stray docs/kinds file")
	flag.Parse()

	crds := map[string]string{} // kind -> group/version
	files, _ := filepath.Glob("config/crds/*.yaml")
	for _, f := range files {
		if filepath.Base(f) == "kustomization.yaml" {
			continue
		}
		b, err := os.ReadFile(f)
		must(err)
		var c crd
		must(yaml.Unmarshal(b, &c))
		if c.Spec.Names.Kind == "" || len(c.Spec.Versions) == 0 {
			continue
		}
		crds[c.Spec.Names.Kind] = c.Spec.Group + "/" + c.Spec.Versions[0].Name
	}

	var docs []doc
	documented := map[string]string{} // kind -> file
	mds, _ := filepath.Glob("docs/kinds/*.md")
	var problems []string
	for _, f := range mds {
		if filepath.Base(f) == "README.md" {
			continue
		}
		d, err := read(f)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		for _, k := range d.front.Kinds {
			if prev, ok := documented[k]; ok {
				problems = append(problems, fmt.Sprintf("%s: %s is also in %s", f, k, prev))
			}
			documented[k] = f
			switch v, ok := crds[k]; {
			case !ok:
				problems = append(problems, fmt.Sprintf("%s: no CRD in config/crds for %s", f, k))
			case v != d.front.APIVersion:
				problems = append(problems, fmt.Sprintf("%s: apiVersion %s, but the CRD of %s is %s", f, d.front.APIVersion, k, v))
			}
		}
		docs = append(docs, d)
	}
	for k := range crds {
		if _, ok := documented[k]; !ok {
			problems = append(problems, fmt.Sprintf("%s has a CRD but no docs/kinds/%s.md (docs/kinds/README.md)", k, k))
		}
	}
	sort.Strings(problems)
	if *check {
		if len(problems) > 0 {
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, "kinddocs: "+p)
			}
			os.Exit(1)
		}
		fmt.Printf("ok: %d kinds, each with its docs/kinds file\n", len(crds))
		return
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].front.APIVersion+docs[i].file < docs[j].front.APIVersion+docs[j].file
	})
	fmt.Println("| Kind | API version | Last live run | Gap it fills |")
	fmt.Println("| --- | --- | --- | --- |")
	for _, d := range docs {
		fmt.Printf("| [%s](%s) | `%s` | %s | %s |\n", strings.Join(d.front.Kinds, ", "), d.file, d.front.APIVersion, d.lastLive, d.gap)
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, "kinddocs: "+p)
	}
}

func read(f string) (doc, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return doc{}, err
	}
	d := doc{file: f, lastLive: "before 2026-11-11: docs/live-runs.md"}
	if !bytes.HasPrefix(b, []byte("---\n")) {
		return d, fmt.Errorf("%s: no front matter (--- kinds, apiVersion ---)", f)
	}
	end := bytes.Index(b[4:], []byte("\n---\n"))
	if end < 0 {
		return d, fmt.Errorf("%s: front matter not closed", f)
	}
	if err := yaml.UnmarshalStrict(b[4:4+end], &d.front); err != nil {
		return d, fmt.Errorf("%s: %v", f, err)
	}
	if len(d.front.Kinds) == 0 || d.front.APIVersion == "" {
		return d, fmt.Errorf("%s: front matter needs kinds and apiVersion", f)
	}
	inLive := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "**Gap it fills.** ") {
			d.gap = strings.TrimPrefix(line, "**Gap it fills.** ")
		}
		if strings.HasPrefix(line, "## ") {
			inLive = line == "## Live runs"
		}
		// The last row of the live-runs table: date and the Live cell.
		if inLive && strings.HasPrefix(line, "| 20") {
			cells := strings.Split(strings.Trim(line, "| "), " | ")
			if len(cells) >= 4 {
				d.lastLive = cells[0] + ": " + cells[3]
			}
		}
	}
	return d, nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "kinddocs:", err)
		os.Exit(1)
	}
}
