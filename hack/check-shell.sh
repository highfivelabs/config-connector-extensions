#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Every shell script in the repository parses (bash -n) and carries no
# merge-conflict marker. Conflict merges twice broke hack/live/sweep.sh
# without a check noticing, since nothing runs the sweeper before a live
# run. A shell script is a tracked or new file named *.sh, or one whose
# first line is a bash or sh shebang (hack/hooks/pre-push). bash -n checks
# syntax only: it does not run the script or expand anything.
#
# No script pipes into a quiet grep (`cmd | grep -q`, `--quiet`,
# `--silent`): grep exits on its first match, cmd can then die of
# SIGPIPE, and under pipefail the pipeline fails although grep matched.
# That made hack/check-no-fake-cloud.sh fail once on CI. Capture the
# output first, or drop -q and send grep's output to /dev/null.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

scripts=()
while IFS= read -r f; do
  [[ -f "${f}" ]] || continue
  case "${f}" in
    third_party/*) continue ;;
    *.sh) scripts+=("${f}") ;;
    *)
      if head -n 1 "${f}" | grep -E '^#!.*[/ ](ba)?sh([[:space:]]|$)' >/dev/null; then
        scripts+=("${f}")
      fi ;;
  esac
done < <(git ls-files --cached --others --exclude-standard)

# The scripts this check was written for must be among those found: a
# check that finds nothing must not pass.
for want in hack/live/sweep.sh hack/live/run.sh hack/ci/gate.sh hack/hooks/pre-push; do
  found=0
  for f in "${scripts[@]}"; do [[ "${f}" == "${want}" ]] && found=1; done
  if [[ ${found} -eq 0 ]]; then
    echo "check-shell: ${want} not found among the shell scripts" >&2
    exit 1
  fi
done

# A single | (not ||) then grep with -q among its short options, or
# --quiet or --silent.
quiet_grep='(^|[^|])[|][[:space:]]*grep([[:space:]]+[^|;&[:space:]]+)*[[:space:]]+(-[A-Za-z]*q[A-Za-z]*|--quiet|--silent)([[:space:]]|$)'

bad=0
for f in "${scripts[@]}"; do
  if ! out="$(bash -n "${f}" 2>&1)"; then
    echo "bash -n failed: ${f}" >&2
    echo "${out}" >&2
    bad=$((bad + 1))
    continue
  fi
  if grep -nE '^(<<<<<<<|>>>>>>>|\|\|\|\|\|\|\|)( |$)' "${f}" >&2; then
    echo "merge-conflict marker: ${f}" >&2
    bad=$((bad + 1))
  fi
  if grep -nE "${quiet_grep}" "${f}" | grep -vE '^[0-9]+:[[:space:]]*#' >&2; then
    echo "a pipe into grep -q (SIGPIPE under pipefail): ${f}" >&2
    bad=$((bad + 1))
  fi
done
if [[ ${bad} -gt 0 ]]; then
  echo "${bad} shell script(s) fail bash -n, carry a conflict marker or pipe into grep -q" >&2
  exit 1
fi
echo "ok: ${#scripts[@]} shell scripts parse (bash -n), no conflict markers, no pipe into grep -q"
