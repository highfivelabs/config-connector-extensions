#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# Start the fake cloud: a kind cluster with Config Connector's CRDs and
# ours, and bin/fakecloud (Config Connector's manager plus this project's
# controllers, every Google API answered by mockgcp) running on the host
# against it. See docs/fake-cloud.md.
#
#   CRD_GROUPS="sql iam resourcemanager" hack/fake-cloud/up.sh
#   CRD_GROUPS=all hack/fake-cloud/up.sh     # all ~800 Config Connector CRDs
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER="${FAKE_CLOUD_CLUSTER:-cce-fake-cloud}"
STATE="${ROOT}/bin/fake-cloud"
KCC="${ROOT}/third_party/k8s-config-connector"
CRD_GROUPS="${CRD_GROUPS:-sql iam resourcemanager compute}"
PROJECT="${FAKE_CLOUD_PROJECT:-mock-project}"
mkdir -p "${STATE}"
export KUBECONFIG="${STATE}/kubeconfig"

command -v docker >/dev/null && docker info >/dev/null 2>&1 || { echo "docker is not running" >&2; exit 1; }
command -v kind >/dev/null || { echo "kind is not installed" >&2; exit 1; }

start=$(date +%s)
# grep without -q reads all its input: no SIGPIPE for kind under pipefail.
if ! kind get clusters 2>/dev/null | grep -x "${CLUSTER}" >/dev/null; then
  kind create cluster --name "${CLUSTER}" --kubeconfig "${KUBECONFIG}" --wait 120s
fi

# Our CRDs only: config/crds/kustomization.yaml is not an object kubectl can apply.
crds=("${KCC}"/operator/config/crd/bases/core.cnrm.cloud.google.com_*.yaml "${ROOT}"/config/crds/*.gcp.hfl.dev_*.yaml)
if [[ "${CRD_GROUPS}" == all ]]; then
  crds+=("${KCC}"/config/crds/resources/*.yaml)
else
  for g in ${CRD_GROUPS}; do
    crds+=("${KCC}"/config/crds/resources/*."${g}".cnrm.cloud.google.com.yaml)
  done
fi
echo "installing ${#crds[@]} CRDs"
for f in "${crds[@]}"; do printf -- '---\n'; cat "$f"; done \
  | kubectl apply --server-side --force-conflicts -f - >/dev/null
kubectl wait --for=condition=Established crd --all --timeout=180s >/dev/null

kubectl create namespace example --dry-run=client -o yaml \
  | kubectl annotate --local -f - "cnrm.cloud.google.com/project-id=${PROJECT}" -o yaml \
  | kubectl apply -f - >/dev/null

if [[ -f "${STATE}/fakecloud.pid" ]] && kill -0 "$(cat "${STATE}/fakecloud.pid")" 2>/dev/null; then
  echo "fakecloud already running (pid $(cat "${STATE}/fakecloud.pid"))"
else
  [[ -x "${ROOT}/bin/fakecloud" ]] || (cd "${ROOT}" && go build -o bin/fakecloud ./cmd/fakecloud)
  # mockgcp listens on 127.0.0.2 by default, which macOS lacks (patches 0002).
  MOCKGCP_LISTEN_ADDRESS="${MOCKGCP_LISTEN_ADDRESS:-127.0.0.1:0}" nohup "${ROOT}/bin/fakecloud" --projects "${PROJECT}" >"${STATE}/fakecloud.log" 2>&1 &
  echo $! >"${STATE}/fakecloud.pid"
  for _ in $(seq 1 60); do
    grep -q "Starting workers" "${STATE}/fakecloud.log" && break
    kill -0 "$(cat "${STATE}/fakecloud.pid")" 2>/dev/null || { tail -40 "${STATE}/fakecloud.log" >&2; exit 1; }
    sleep 1
  done
fi
echo "fake cloud up in $(( $(date +%s) - start ))s; KUBECONFIG=${KUBECONFIG}; log ${STATE}/fakecloud.log"
