#!/usr/bin/env bash
# Apply a Cloud SQL Postgres instance, a database, a Google service account
# and an IAM database user with database roles (as bundt's charts/infra/gcp
# would) to the fake cloud, and wait for each to be Ready.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export KUBECONFIG="${ROOT}/bin/fake-cloud/kubeconfig"
start=$(date +%s)

kubectl apply -n bundt -f "${ROOT}/hack/fake-cloud/sample.yaml"
ok=0
for r in sqlinstance.sql.cnrm.cloud.google.com/bundt \
         sqldatabase.sql.cnrm.cloud.google.com/bundt \
         iamserviceaccount.iam.cnrm.cloud.google.com/bntd-app \
         sqluser.sql.gcp.hfl.dev/bntd; do
  if kubectl wait -n bundt --for=condition=Ready "$r" --timeout=180s; then
    ok=$((ok + 1))
  else
    kubectl get -n bundt "$r" -o jsonpath='{.status.conditions}' >&2; echo >&2
  fi
done
kubectl get -n bundt sqluser.sql.gcp.hfl.dev/bntd -o jsonpath='appliedDatabaseRoles: {.status.appliedDatabaseRoles}{"\n"}'
echo "${ok} of 4 Ready in $(( $(date +%s) - start ))s"
[[ ${ok} -eq 4 ]]
