#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Regenerate docs/coverage/ and the Coverage section of README.md from the
# pinned inputs in docs/coverage/sources.yaml and this repository's CRDs.
#
#   hack/coverage/run.sh                  fetch the inputs if missing, regenerate
#   hack/coverage/run.sh --check          fail if the committed outputs differ from a regeneration
#   hack/coverage/run.sh --update-sources after moving a pin in sources.yaml: refetch, record the input digests
#
# Inputs (Discovery documents, googleapis protos, Config Connector) are cached under .cache/coverage/ (gitignored). Config Connector is
# taken from third_party/k8s-config-connector when that checkout is at the
# pin, else fetched sparsely (CRDs, service mappings, API types: ~50 MB).
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"

MODE="generate"
case "${1:-}" in
  "") ;;
  --check) MODE="check" ;;
  --update-sources) MODE="update-sources" ;;
  *) echo "usage: $0 [--check|--update-sources]" >&2; exit 2 ;;
esac

SOURCES=docs/coverage/sources.yaml
CACHE="${ROOT}/.cache/coverage"
# yaml_get <section> <key>: a scalar from sources.yaml's two flat sections.
yaml_get() {
  awk -v sec="$1:" -v key="$2:" '
    /^[a-z_]+:/ { cur = $1 }
    cur == sec && $1 == key { gsub(/"/, "", $2); print $2; exit }' "${SOURCES}"
}
DISC_REPO="$(yaml_get discovery repo)"
DISC_COMMIT="$(yaml_get discovery commit)"
PROTO_REPO="$(yaml_get googleapis repo)"
PROTO_COMMIT="$(yaml_get googleapis commit)"
KCC_REPO="$(yaml_get config_connector repo)"
KCC_COMMIT="$(yaml_get config_connector commit)"
[[ -n "${DISC_COMMIT}" && -n "${KCC_COMMIT}" ]] || { echo "cannot read the pins from ${SOURCES}" >&2; exit 1; }

fetch_discovery() {
  local dir="${CACHE}/discovery"
  if [[ "$(cat "${dir}/.commit" 2>/dev/null || true)" == "${DISC_COMMIT}" && "${MODE}" != "update-sources" ]]; then
    return
  fi
  echo "fetching discovery documents at ${DISC_COMMIT}" >&2
  local tmp
  tmp="$(mktemp -d)"
  curl -fsSL "${DISC_REPO}/archive/${DISC_COMMIT}.tar.gz" -o "${tmp}/d.tgz"
  mkdir -p "${tmp}/x"
  tar xzf "${tmp}/d.tgz" -C "${tmp}/x" --strip-components=1
  rm -rf "${dir}"
  mkdir -p "${dir}"
  cp "${tmp}"/x/discoveries/*.json "${dir}/"
  echo "${DISC_COMMIT}" > "${dir}/.commit"
  rm -rf "${tmp}"
}

fetch_protos() {
  local dir="${CACHE}/googleapis"
  if [[ "$(cat "${dir}/.commit" 2>/dev/null || true)" == "${PROTO_COMMIT}" && "${MODE}" != "update-sources" ]]; then
    return
  fi
  echo "fetching googleapis protos at ${PROTO_COMMIT}" >&2
  local tmp
  tmp="$(mktemp -d)"
  curl -fsSL "${PROTO_REPO}/archive/${PROTO_COMMIT}.tar.gz" -o "${tmp}/p.tgz"
  mkdir -p "${tmp}/x"
  tar xzf "${tmp}/p.tgz" -C "${tmp}/x" --strip-components=1
  rm -rf "${dir}"
  mkdir -p "${dir}"
  (cd "${tmp}/x" && find google -name '*.proto' | tar cf - -T -) | tar xf - -C "${dir}"
  echo "${PROTO_COMMIT}" > "${dir}/.commit"
  rm -rf "${tmp}"
}

KCC_DIR=""
fetch_kcc() {
  if [[ "$(git -C third_party/k8s-config-connector rev-parse HEAD 2>/dev/null || true)" == "${KCC_COMMIT}" ]]; then
    KCC_DIR="${ROOT}/third_party/k8s-config-connector"
    return
  fi
  KCC_DIR="${CACHE}/kcc"
  if [[ "$(git -C "${KCC_DIR}" rev-parse HEAD 2>/dev/null || true)" == "${KCC_COMMIT}" ]]; then
    return
  fi
  echo "fetching Config Connector at ${KCC_COMMIT} (sparse)" >&2
  rm -rf "${KCC_DIR}"
  mkdir -p "${KCC_DIR}"
  git -C "${KCC_DIR}" init -q
  git -C "${KCC_DIR}" remote add origin "${KCC_REPO}.git"
  git -C "${KCC_DIR}" sparse-checkout set config/crds/resources config/servicemappings apis
  git -C "${KCC_DIR}" fetch -q --depth 1 --filter=blob:none origin "${KCC_COMMIT}"
  git -C "${KCC_DIR}" checkout -q FETCH_HEAD
}

fetch_discovery
fetch_protos
fetch_kcc

mkdir -p "${CACHE}/bin"
(cd hack/coverage && go build -o "${CACHE}/bin/coverage" .)
ARGS=(-repo "${ROOT}" -kcc "${KCC_DIR}" -discovery "${CACHE}/discovery" -googleapis "${CACHE}/googleapis")
case "${MODE}" in
  check) exec "${CACHE}/bin/coverage" "${ARGS[@]}" -check ;;
  update-sources) exec "${CACHE}/bin/coverage" "${ARGS[@]}" -update-sources ;;
  *) exec "${CACHE}/bin/coverage" "${ARGS[@]}" ;;
esac
