#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# Stop bin/fakecloud and delete the kind cluster. The fake cloud is in
# memory, so this forgets every fake Google resource.
set -o errexit -o nounset -o pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE="${ROOT}/bin/fake-cloud"
if [[ -f "${STATE}/fakecloud.pid" ]]; then
  kill "$(cat "${STATE}/fakecloud.pid")" 2>/dev/null || true
  rm -f "${STATE}/fakecloud.pid"
fi
kind delete cluster --name "${FAKE_CLOUD_CLUSTER:-cce-fake-cloud}" --kubeconfig "${STATE}/kubeconfig" 2>/dev/null || true
echo "fake cloud down"
