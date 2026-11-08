#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# Fetch Config Connector at the commit pinned in UPSTREAM.md into
# third_party/k8s-config-connector, then apply our patch series from
# patches/k8s-config-connector/ in order.
#
# The checkout is not committed (it is about 650 MB). go.mod points at it
# with replace directives, so nothing builds until this has run.
#
#   hack/upstream.sh          fetch if missing, reset to the pin, apply patches
#   hack/upstream.sh --check  fail if the checkout is not the pin plus patches

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

# The single source of the pin: the "Commit" row of UPSTREAM.md.
KCC_COMMIT="$(sed -n 's/^| Commit | `\([0-9a-f]\{40\}\)`.*/\1/p' UPSTREAM.md | head -1)"
KCC_REPO="https://github.com/GoogleCloudPlatform/k8s-config-connector.git"
DEST="third_party/k8s-config-connector"
PATCHES="${REPO_ROOT}/patches/k8s-config-connector"

if [[ -z "${KCC_COMMIT}" ]]; then
  echo "could not read the Config Connector commit from UPSTREAM.md" >&2
  exit 1
fi

if [[ "${1:-}" == "--check" ]]; then
  if [[ ! -d "${DEST}/.git" ]]; then
    echo "${DEST} is missing; run hack/upstream.sh" >&2
    exit 1
  fi
  head="$(git -C "${DEST}" rev-parse HEAD)"
  if [[ "${head}" != "${KCC_COMMIT}" ]]; then
    echo "${DEST} is at ${head}, UPSTREAM.md pins ${KCC_COMMIT}" >&2
    exit 1
  fi
  # Every patch must be applied, and nothing else changed.
  expected="$(mktemp)"
  actual="$(mktemp)"
  trap 'rm -f "${expected}" "${actual}"' EXIT
  for p in "${PATCHES}"/*.patch; do
    [[ -e "${p}" ]] || continue
    git -C "${DEST}" apply --numstat "${p}" | awk '{print $3}'
  done | sort -u > "${expected}"
  git -C "${DEST}" status --porcelain --untracked-files=all | awk '{print $2}' | sort -u > "${actual}"
  if ! diff -u "${expected}" "${actual}"; then
    echo "${DEST} differs from the pin plus patches/ (above: expected vs actual changed files)" >&2
    exit 1
  fi
  echo "ok: ${DEST} is ${KCC_COMMIT} plus $(ls "${PATCHES}"/*.patch 2>/dev/null | wc -l | tr -d ' ') patch(es)"
  exit 0
fi

mkdir -p "$(dirname "${DEST}")"
if [[ ! -d "${DEST}/.git" ]]; then
  git init --quiet "${DEST}"
  git -C "${DEST}" remote add origin "${KCC_REPO}"
fi
if ! git -C "${DEST}" cat-file -e "${KCC_COMMIT}^{commit}" 2>/dev/null; then
  git -C "${DEST}" fetch --quiet --depth 1 origin "${KCC_COMMIT}"
fi
git -C "${DEST}" -c advice.detachedHead=false checkout --quiet --force "${KCC_COMMIT}"
git -C "${DEST}" reset --quiet --hard "${KCC_COMMIT}"
git -C "${DEST}" clean --quiet -fdx

for p in "${PATCHES}"/*.patch; do
  [[ -e "${p}" ]] || continue
  echo "applying $(basename "${p}")"
  git -C "${DEST}" apply --whitespace=nowarn "${p}"
done

echo "${DEST} is ${KCC_COMMIT} plus our patches"
