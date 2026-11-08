#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# The production manager must not contain the fake-cloud switch: no
# --fake-cloud-url flag and no pkg/fakecloud. Only a build with
# -tags fakecloud (the test images, tests/operator/run.sh) has them.
set -o errexit -o nounset -o pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

if go list -deps ./cmd/manager | grep -qx 'github.com/highfivelabs/gaps-gcp/pkg/fakecloud'; then
  echo "cmd/manager links pkg/fakecloud without -tags fakecloud" >&2
  exit 1
fi
if ! go list -tags fakecloud -deps ./cmd/manager | grep -qx 'github.com/highfivelabs/gaps-gcp/pkg/fakecloud'; then
  echo "cmd/manager with -tags fakecloud no longer links pkg/fakecloud; update this check" >&2
  exit 1
fi
bin="$(mktemp -d)/manager"
trap 'rm -rf "$(dirname "${bin}")"' EXIT
go build -o "${bin}" ./cmd/manager
if "${bin}" --help 2>&1 | grep -q -- '-fake-cloud-url'; then
  echo "the production manager has a --fake-cloud-url flag" >&2
  exit 1
fi
echo "ok: the production manager has no fake-cloud switch"
