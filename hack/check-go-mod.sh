#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# Config Connector's go.mod replace directives do not apply to us, so go.mod
# repeats them. Fail if the version pins in the checkout's go.mod (the
# "=> module version" lines) are not all in ours.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

pins() { awk '/=>/ && $NF ~ /^v[0-9]/ {print $(NF-3), $(NF-1), $NF}' "$1" | sed 's/^replace //' | sort; }
missing="$(comm -23 <(pins third_party/k8s-config-connector/go.mod) <(pins go.mod))"
if [[ -n "${missing}" ]]; then
  echo "go.mod is missing these replace pins from third_party/k8s-config-connector/go.mod:" >&2
  echo "${missing}" >&2
  exit 1
fi
echo "ok: go.mod carries every replace pin of the pinned Config Connector"
