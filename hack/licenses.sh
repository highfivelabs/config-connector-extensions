#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Write docs/third-party-licenses.csv: every Go module linked into the
# manager and the operator, and its licence, as go-licenses reports it.
# Fails if a module's licence is not one the project accepts. Needs
# go-licenses on PATH (go install github.com/google/go-licenses@latest)
# and the pinned Config Connector checkout (task upstream).
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"
out=docs/third-party-licenses.csv
command -v go-licenses >/dev/null || { echo "go-licenses not on PATH" >&2; exit 1; }

report="$(go-licenses report ./cmd/manager ./cmd/operator 2>/dev/null)"
{
  echo "module,licence"
  # Drop the URL column: for modules resolved from the local checkout it
  # is a guess.
  awk -F, '{print $1 "," $NF}' <<<"${report}" | sort
} >"${out}"

bad="$(awk -F, 'NR > 1 && $2 !~ /^(Apache-2\.0|MIT|BSD-2-Clause|BSD-3-Clause|ISC|MPL-2\.0)$/' "${out}")"
if [[ -n "${bad}" ]]; then
  echo "licences outside the accepted set:" >&2
  echo "${bad}" >&2
  exit 1
fi
echo "ok: $(($(wc -l <"${out}") - 1)) modules, all Apache-2.0, MIT, BSD, ISC or MPL-2.0 (${out})"
