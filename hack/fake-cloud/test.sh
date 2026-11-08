#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# Apply a Cloud SQL Postgres instance, a database, a Google service account
# and an IAM database user with database roles (hack/fake-cloud/sample.yaml)
# to the fake cloud, and wait for each to be Ready.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export KUBECONFIG="${ROOT}/bin/fake-cloud/kubeconfig"
start=$(date +%s)

kubectl apply -n example -f "${ROOT}/hack/fake-cloud/sample.yaml"
ok=0
for r in sqlinstance.sql.cnrm.cloud.google.com/example \
         sqldatabase.sql.cnrm.cloud.google.com/example \
         iamserviceaccount.iam.cnrm.cloud.google.com/app-runtime \
         sqluser.sql.gcp.hfl.dev/app-runtime; do
  if kubectl wait -n example --for=condition=Ready "$r" --timeout=180s; then
    ok=$((ok + 1))
  else
    kubectl get -n example "$r" -o jsonpath='{.status.conditions}' >&2; echo >&2
  fi
done
kubectl get -n example sqluser.sql.gcp.hfl.dev/app-runtime -o jsonpath='appliedDatabaseRoles: {.status.appliedDatabaseRoles}{"\n"}'
echo "${ok} of 4 Ready in $(( $(date +%s) - start ))s"
[[ ${ok} -eq 4 ]]
