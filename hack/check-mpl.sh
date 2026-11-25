#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# The rules for code under the Mozilla Public License 2.0 and HashiCorp's
# Business Source License (docs/design.md, decision 41):
#   1. MPL modules are linked unmodified: no patch in patches/ touches
#      Config Connector's copy of terraform-provider-google-beta (or any
#      other path under third_party/github.com/hashicorp).
#   2. No MPL code or text is copied in: no file outside third_party/ carries
#      an MPL licence header.
#   3. No BSL module is required (Terraform core since 1.6, Vault, Consul,
#      Nomad, Boundary, Packer, Waypoint).
#   4. NOTICE names every MPL module that docs/third-party-licenses.csv lists.
# Needs no network and no upstream checkout.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"
fail=0

# 1. Patched paths, from the diff headers of every patch.
while IFS= read -r p; do
  echo "patch modifies an MPL file (link it unmodified instead): ${p}" >&2
  fail=1
done < <(grep -h '^\(+++\|---\) [ab]/' patches/*/*.patch 2>/dev/null |
  awk '{print substr($2, 3)}' | grep '^third_party/github.com/hashicorp/' | sort -u)

# 2. MPL headers in our own files. NOTICE and docs name the licence; they
#    do not carry it.
while IFS= read -r f; do
  [[ -f "${f}" ]] || continue
  if head -n 25 "${f}" | grep -E 'SPDX-License-Identifier: *MPL|Mozilla Public License' >/dev/null; then
    echo "MPL licence header outside third_party/ (MPL code or text copied in?): ${f}" >&2
    fail=1
  fi
done < <(git ls-files --cached --others --exclude-standard -- \
  ':!third_party/' ':!NOTICE' ':!docs/' ':!hack/check-mpl.sh' ':!*.patch')

# 3. BSL modules in go.mod (direct or indirect). terraform-json,
#    terraform-exec and the plugin libraries are MPL and allowed.
#    The JSON is captured first so a failing `go mod edit` stops the script
#    (errexit) instead of feeding the loop nothing. Replace paths count too.
bsl='^github\.com/hashicorp/(terraform|vault|consul|nomad|boundary|packer|waypoint)(/v[0-9]+)?$'
modjson="$(go mod edit -json)"
mods="$(sed -n 's/^[[:space:]]*"Path": "\([^"]*\)".*/\1/p' <<<"${modjson}" | sort -u)"
[[ -n "${mods}" ]] || { echo "no module paths read from go.mod" >&2; exit 1; }
while IFS= read -r m; do
  [[ -n "${m}" ]] || continue
  echo "BSL module required in go.mod: ${m}" >&2
  fail=1
done <<<"$(grep -E "${bsl}" <<<"${mods}" || true)"

# 4. Every MPL module in the licence list is named in NOTICE.
#    A HashiCorp module may be named by its repository (NOTICE says
#    "github.com/hashicorp/terraform-plugin-sdk, ..., hcl, errwrap"); any
#    other module by its full path, without a /vN or /version suffix. Either
#    must appear as a whole word.
[[ -r docs/third-party-licenses.csv ]] || { echo "docs/third-party-licenses.csv missing (task licenses)" >&2; exit 1; }
mpl="$(awk -F, 'NR > 1 && $2 == "MPL-2.0" {print $1}' docs/third-party-licenses.csv)"
while IFS= read -r m; do
  [[ -n "${m}" ]] || continue
  path="$(sed -E 's#/(v[0-9]+|version)$##' <<<"${m}")"
  name="${path}"
  if [[ "${path}" == github.com/hashicorp/* ]]; then
    name="${path#github.com/hashicorp/}"
    name="${name%%/*}"
  fi
  if [[ -z "${name}" ]] || ! grep -wF -- "${name}" NOTICE >/dev/null; then
    echo "MPL module not named in NOTICE: ${m}" >&2
    fail=1
  fi
done <<<"${mpl}"

if [[ ${fail} -ne 0 ]]; then
  echo "MPL/BSL rules broken; see docs/design.md, decision 41" >&2
  exit 1
fi
echo "ok: MPL modules linked unmodified, none copied in, no BSL modules, NOTICE complete"
