// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

// coverage compares what Config Connector (official) and this repository
// (ours) cover of Google Cloud's REST APIs, and writes docs/coverage and
// the Coverage section of README.md. See docs/coverage/README.md.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Sources struct {
	Discovery struct {
		Repo       string `yaml:"repo"`
		Commit     string `yaml:"commit"`
		CommitDate string `yaml:"commit_date"`
		Fetched    string `yaml:"fetched"`
		Digest     string `yaml:"digest"`
	} `yaml:"discovery"`
	Googleapis struct {
		Repo       string `yaml:"repo"`
		Commit     string `yaml:"commit"`
		CommitDate string `yaml:"commit_date"`
		Fetched    string `yaml:"fetched"`
		Digest     string `yaml:"digest"`
	} `yaml:"googleapis"`
	ConfigConnector struct {
		Repo           string `yaml:"repo"`
		Commit         string `yaml:"commit"`
		CommitDate     string `yaml:"commit_date"`
		NearestRelease string `yaml:"nearest_release"`
		Fetched        string `yaml:"fetched"`
		Digest         string `yaml:"digest"`
	} `yaml:"config_connector"`
}

func main() {
	repo := flag.String("repo", ".", "repository root")
	cache := flag.String("cache", "", "input cache (default <repo>/.cache/coverage)")
	kccDir := flag.String("kcc", "", "Config Connector checkout (default <cache>/kcc)")
	discDir := flag.String("discovery", "", "discovery documents (default <cache>/discovery)")
	protoDir := flag.String("googleapis", "", "googleapis protos (default <cache>/googleapis)")
	check := flag.Bool("check", false, "fail if the committed outputs differ from a regeneration")
	updateSources := flag.Bool("update-sources", false, "rewrite the input digests in docs/coverage/sources.yaml")
	derive := flag.String("derive-priority", "", "write docs/coverage/priority.csv from the catalog checkout at this path (a one-off; the file is hand-edited afterwards)")
	flag.Parse()
	if *derive != "" {
		if err := derivePriority(*repo, *derive); err != nil {
			fmt.Fprintln(os.Stderr, "coverage:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*repo, *cache, *kccDir, *discDir, *protoDir, *check, *updateSources); err != nil {
		fmt.Fprintln(os.Stderr, "coverage:", err)
		os.Exit(1)
	}
}

func digestTree(root string, include func(rel string) bool) (string, error) {
	var rels []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if include(rel) {
			rels = append(rels, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rels)
	h := sha256.New()
	for _, r := range rels {
		b, err := os.ReadFile(filepath.Join(root, r))
		if err != nil {
			return "", err
		}
		s := sha256.Sum256(b)
		fmt.Fprintf(h, "%s %s\n", hex.EncodeToString(s[:]), r)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func run(repo, cache, kccDir, discDir, protoDir string, check, updateSources bool) error {
	repo, _ = filepath.Abs(repo)
	if cache == "" {
		cache = filepath.Join(repo, ".cache", "coverage")
	}
	if kccDir == "" {
		kccDir = filepath.Join(cache, "kcc")
	}
	if discDir == "" {
		discDir = filepath.Join(cache, "discovery")
	}
	if protoDir == "" {
		protoDir = filepath.Join(cache, "googleapis")
	}
	srcPath := filepath.Join(repo, "docs", "coverage", "sources.yaml")
	sb, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	var src Sources
	if err := yaml.Unmarshal(sb, &src); err != nil {
		return err
	}
	// The Config Connector pin is UPSTREAM.md's.
	ub, err := os.ReadFile(filepath.Join(repo, "UPSTREAM.md"))
	if err != nil {
		return err
	}
	if m := regexp.MustCompile("(?m)^\\| Commit \\| `([0-9a-f]{40})`").FindSubmatch(ub); m == nil || string(m[1]) != src.ConfigConnector.Commit {
		return fmt.Errorf("docs/coverage/sources.yaml pins Config Connector at %s, UPSTREAM.md at something else: update sources.yaml and regenerate", src.ConfigConnector.Commit)
	}
	kccDigest, err := digestTree(kccDir, func(rel string) bool {
		return strings.HasPrefix(rel, "config/crds/resources/") || strings.HasPrefix(rel, "config/servicemappings/") ||
			(strings.HasPrefix(rel, "apis/") && strings.HasSuffix(rel, "_types.go"))
	})
	if err != nil {
		return fmt.Errorf("Config Connector inputs: %w (run hack/coverage/run.sh to fetch them)", err)
	}
	discDigest, err := digestTree(discDir, func(rel string) bool { return strings.HasSuffix(rel, ".json") })
	if err != nil {
		return fmt.Errorf("discovery inputs: %w (run hack/coverage/run.sh to fetch them)", err)
	}
	protoDigest, err := digestTree(protoDir, func(rel string) bool { return strings.HasSuffix(rel, ".proto") })
	if err != nil {
		return fmt.Errorf("googleapis inputs: %w (run hack/coverage/run.sh to fetch them)", err)
	}
	if updateSources {
		src.ConfigConnector.Digest = kccDigest
		src.Discovery.Digest = discDigest
		src.Googleapis.Digest = protoDigest
		return writeSources(srcPath, sb, &src)
	}
	if kccDigest != src.ConfigConnector.Digest {
		return fmt.Errorf("Config Connector inputs differ from the pin: digest %s, sources.yaml has %s", kccDigest, src.ConfigConnector.Digest)
	}
	if discDigest != src.Discovery.Digest {
		return fmt.Errorf("discovery inputs differ from the pin: digest %s, sources.yaml has %s", discDigest, src.Discovery.Digest)
	}

	if protoDigest != src.Googleapis.Digest {
		return fmt.Errorf("googleapis inputs differ from the pin: digest %s, sources.yaml has %s", protoDigest, src.Googleapis.Digest)
	}
	files, err := generate(repo, kccDir, discDir, protoDir, &src)
	if err != nil {
		return err
	}
	if check {
		return checkFiles(repo, files)
	}
	return writeFiles(repo, files)
}

func writeSources(path string, old []byte, s *Sources) error {
	// Keep the comments: replace only the digest lines of each section.
	lines := strings.Split(string(old), "\n")
	section := ""
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "discovery:"):
			section = "d"
		case strings.HasPrefix(l, "googleapis:"):
			section = "g"
		case strings.HasPrefix(l, "config_connector:"):
			section = "k"
		case strings.HasPrefix(strings.TrimSpace(l), "digest:"):
			d := s.Discovery.Digest
			if section == "k" {
				d = s.ConfigConnector.Digest
			}
			if section == "g" {
				d = s.Googleapis.Digest
			}
			lines[i] = "  digest: " + d
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// outputs the generator owns under docs/coverage; anything else there is
// stale.
var ownedDirs = []string{"bars", "attributes"}

func writeFiles(repo string, files map[string][]byte) error {
	for _, d := range ownedDirs {
		dir := filepath.Join(repo, "docs", "coverage", d)
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			if _, ok := files[filepath.ToSlash(filepath.Join("docs/coverage", d, e.Name()))]; !ok {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
	paths := sortedKeys(files)
	for _, p := range paths {
		full := filepath.Join(repo, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, files[p], 0o644); err != nil {
			return err
		}
	}
	return nil
}

func checkFiles(repo string, files map[string][]byte) error {
	var bad []string
	for _, p := range sortedKeys(files) {
		got, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(p)))
		if err != nil || !bytes.Equal(got, files[p]) {
			bad = append(bad, p)
		}
	}
	for _, d := range ownedDirs {
		ents, _ := os.ReadDir(filepath.Join(repo, "docs", "coverage", d))
		for _, e := range ents {
			p := filepath.ToSlash(filepath.Join("docs/coverage", d, e.Name()))
			if _, ok := files[p]; !ok {
				bad = append(bad, p+" (stale)")
			}
		}
	}
	if len(bad) > 0 {
		if len(bad) > 20 {
			bad = append(bad[:20], fmt.Sprintf("... and %d more", len(bad)-20))
		}
		return fmt.Errorf("committed coverage outputs are out of date (run hack/coverage/run.sh and commit):\n  %s", strings.Join(bad, "\n  "))
	}
	fmt.Println("ok: docs/coverage and README.md match a regeneration from the pinned inputs")
	return nil
}
