#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# The production manager must not contain the fake-cloud switch: no
# --fake-cloud-url flag and no pkg/fakecloud. Only a build with
# -tags fakecloud (the test images, tests/operator/run.sh) has them.
set -o errexit -o nounset -o pipefail
#
# Each command's output is captured first and grep reads it afterwards:
# `cmd | grep -q` under pipefail fails when grep exits on its first match
# and cmd then dies of SIGPIPE, which made the second test below fail on
# CI ("no longer links pkg/fakecloud") while the package was linked. A
# failing go list or --help now fails the check (errexit) instead of
# reading as "not linked" or "no flag".
cd "$(dirname "${BASH_SOURCE[0]}")/.."
fakecloud='github.com/highfivelabs/config-connector-extensions/pkg/fakecloud'

deps="$(go list -deps ./cmd/manager)"
if grep -qx "${fakecloud}" <<<"${deps}"; then
  echo "cmd/manager links pkg/fakecloud without -tags fakecloud" >&2
  exit 1
fi
deps="$(go list -tags fakecloud -deps ./cmd/manager)"
if ! grep -qx "${fakecloud}" <<<"${deps}"; then
  echo "cmd/manager with -tags fakecloud no longer links pkg/fakecloud; update this check" >&2
  exit 1
fi
bin="$(mktemp -d)/manager"
trap 'rm -rf "$(dirname "${bin}")"' EXIT
go build -o "${bin}" ./cmd/manager
help="$("${bin}" --help 2>&1)"
if grep -q -- '-fake-cloud-url' <<<"${help}"; then
  echo "the production manager has a --fake-cloud-url flag" >&2
  exit 1
fi
echo "ok: the production manager has no fake-cloud switch"
