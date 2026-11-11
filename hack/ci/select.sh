#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# What a pull request must run, from the paths it changes (the builder,
# 2026-11-10: the whole suite nightly, otherwise only what changed).
#
#   hack/ci/select.sh plan BASE HEAD   the plan for git diff BASE...HEAD
#   hack/ci/select.sh plan-files       the plan for the paths on stdin
#   hack/ci/select.sh run GROUP...     check:static, build, the groups' tests
#
# The plan, as GITHUB_OUTPUT lines:
#   mode=none    docs only: nothing heavy runs
#   mode=groups  check:static, build and the tests of the groups only
#   mode=full    task check
#   groups=...   the groups (mode=groups)
#   e2e=true     task operator-e2e runs too (on a Spot runner)
#
# Each path is one of:
#   docs      *.md, docs/, LICENSE, NOTICE; UPSTREAM.md unless a row
#             with a commit (the pin) changed
#   group G   apis/G/, pkg/controller/direct/G/ (kind group G: its unit
#             and envtest tests are in those packages); hack/coverage/
#             (group coverage)
#   adjacent  what a kind's pull request also touches and check:static or
#             the operator e2e covers: config/crds/, pkg/controller/
#             controllers.go, pkg/test/mockenv/, tests/operator/
#   full      everything else: go.mod, patches/, the pin, upstream.yaml,
#             cmd/, pkg/ (the runtime, the operator), hack/, Taskfile.yml,
#             taskfiles/, config/ (the rest), .github/
# A pull request is full if any path is; else groups if any path is a
# group or adjacent; else none. The nightly run (nightly.yml) runs
# everything on main, which catches what a group's tests miss.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# classify PATH: prints "docs", "adjacent", "full" or "group G".
classify() {
  local p="$1" g
  case "$p" in
    # UPSTREAM.md holds the pin (a table row with a commit) and prose that
    # a kind's pull request updates; only a changed pin row is full.
    UPSTREAM.md) if [[ "${PIN_CHANGED:-1}" == 1 ]]; then echo full; else echo docs; fi ;;
    *.md|docs/*|LICENSE|NOTICE) echo docs ;;
    apis/*/*)
      g="${p#apis/}"; echo "group ${g%%/*}" ;;
    pkg/controller/direct/*/*)
      g="${p#pkg/controller/direct/}"; echo "group ${g%%/*}" ;;
    hack/coverage/*) echo "group coverage" ;;
    config/crds/*|pkg/controller/controllers.go|pkg/test/mockenv/*|tests/operator/*) echo adjacent ;;
    *) echo full ;;
  esac
}

# pkgs GROUP: the group's Go packages that exist.
pkgs() {
  local d
  if [[ "$1" == coverage ]]; then echo ./hack/coverage/...; return; fi
  for d in "apis/$1" "pkg/controller/direct/$1"; do
    [[ -d "$d" ]] && echo "./$d/..."
  done
  return 0
}

plan() {
  local p c mode=none groups="" full="" lines=""
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    c="$(classify "$p")"
    lines="$lines$c	$p
"
    case "$c" in
      full) full=1 ;;
      group*) groups="$groups ${c#group }"; [[ "$mode" == none ]] && mode=groups ;;
      adjacent) [[ "$mode" == none ]] && mode=groups ;;
    esac
  done
  [[ -n "$full" ]] && mode=full
  groups="$(echo "$groups" | tr ' ' '\n' | sed '/^$/d' | sort -u | tr '\n' ' ' | sed 's/ $//')"
  [[ "$mode" == groups ]] || groups=""
  local e2e=false
  [[ "$mode" == none ]] || e2e=true
  echo "mode=$mode"
  echo "groups=$groups"
  echo "e2e=$e2e"
  # The why, for the job summary (stderr).
  {
    echo "### What this pull request runs: $mode${groups:+ ($groups)}"
    echo
    echo "| Class | Paths | Example |"
    echo "| --- | --- | --- |"
    printf '%s' "$lines" | awk -F'\t' 'NF == 2 { n[$1]++; if (!($1 in ex)) ex[$1] = $2 } END { for (c in n) printf "| %s | %d | `%s` |\n", c, n[c], ex[c] }' | sort
  } >&2
}

run_groups() {
  [[ $# -gt 0 ]] || { echo "select.sh run: no groups; running check:static and build only"; }
  local g all=""
  for g in "$@"; do all="$all $(pkgs "$g" | tr '\n' ' ')"; done
  all="$(echo "$all" | xargs)"
  task check:static
  task build
  if [[ -n "$all" ]]; then
    echo "select.sh: the tests of $*: $all"
    task test:pkgs PKGS="$all"
  fi
}

case "${1:-}" in
  plan)
    [[ $# -eq 3 ]] || { echo "usage: hack/ci/select.sh plan BASE HEAD" >&2; exit 2; }
    # A diff that fails must fail the plan, never read as "nothing changed".
    files="$(git diff --name-only --no-renames "$2...$3")"
    pin="$(git diff "$2...$3" -- UPSTREAM.md)"
    PIN_CHANGED=0
    if grep -qE '^[-+][|].*[0-9a-f]{40}' <<<"$pin"; then PIN_CHANGED=1; fi
    PIN_CHANGED="$PIN_CHANGED" plan <<<"$files" ;;
  plan-files) plan ;;
  run) shift; run_groups "$@" ;;
  *) echo "usage: hack/ci/select.sh plan BASE HEAD | plan-files | run GROUP..." >&2; exit 2 ;;
esac
