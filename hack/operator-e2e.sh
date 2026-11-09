#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Runs tests/operator/run.sh and refuses a pass that proves nothing: the
# script must be a real one (more than 200 lines, with a failure path), exit
# 0, and end with the line "all checks passed". A script truncated to
# nothing, or one that exits early, fails here rather than passing.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"
script=tests/operator/run.sh
min_lines=200

fail() { echo "operator-e2e: $*" >&2; exit 1; }

[[ -s "${script}" ]] || fail "${script} is missing or empty"
lines="$(wc -l <"${script}")"
(( lines > min_lines )) || fail "${script} has ${lines} lines, expected more than ${min_lines}: truncated?"
grep -Eq '^set (-[a-z]*e[a-z]*|-o errexit)' "${script}" || grep -Eq '^set .*-o errexit' "${script}" \
  || fail "${script} has no 'set -e' (errexit) line: nothing makes a failed step fail"
grep -Fxq 'echo "all checks passed"' "${script}" || fail "${script} never prints 'all checks passed'"

out="$(mktemp)"
trap 'rm -f "${out}"' EXIT
# The output is captured to a file (and shown as it comes), then tested:
# no grep -q in the pipe, which can stop the writer early under pipefail.
status=0
"${script}" 2>&1 | tee "${out}" || status=$?
[[ ${status} -eq 0 ]] || fail "${script} exited ${status}"
[[ "$(tail -n 1 "${out}")" == "all checks passed" ]] || fail "${script} exited 0 but its last line is not 'all checks passed'"
echo "operator-e2e: ${script} ran and ended with 'all checks passed'"
