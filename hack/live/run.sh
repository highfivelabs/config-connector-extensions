#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Run each kind's end-to-end tests twice from the same checkout: against
# the fake cloud (mockgcp), then against a real project, and write a
# summary for docs/live-runs.md. `task test-live PROJECT=...` calls this;
# task check never does.
#
#   hack/live/run.sh --project P [KIND...]
#
# KIND is a package under pkg/controller/direct (default: every built kind,
# in docs/design.md section 6.3's order). LIVE_RUN, when set, is a go test
# -run regexp for both runs of every package (one kind of a package:
# LIVE_RUN=TestIAMOAuthClient). Credentials are Application
# Default Credentials, which must be a service account (it is the IAM
# principal the SQLUser test grants roles to); on an offload VM started
# with --sa they are that service account's.
#
# The run's ID (cce-<UTC month day hour minute>-<random>) prefixes every
# name a test makes and labels what takes labels (cce-e2e=true,
# cce-run=<ID>). Before the run, leftovers older than 2h are swept; after
# it, whatever pass or fail, everything of this run is deleted, old
# leftovers are swept again, and what is left is listed. The Cloud SQL
# kinds share one instance per run (the smallest: Enterprise edition,
# db-f1-micro, zonal, 10 GB HDD, no automated backups), deleted at the end.
#
# Output: offload-artifacts/live/<ID>/{summary.md,<kind>.{mock,live}.log}.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
# shellcheck source=hack/live/projects.sh
source hack/live/projects.sh

# Not by default: logging (LoggingSettings) needs a test folder and a key,
# E2E_LOGGING_FOLDER and E2E_LOGGING_KMS_KEY; serviceusage (ServiceIdentity)
# needs the test folder, E2E_TEST_FOLDER; resourcemanager
# (ProjectDefaultServiceAccounts) needs a test project under it,
# E2E_TEST_FOLDER_PROJECT; logging's LoggingBillingAccountSink skips live
# without a billing account, E2E_BILLING_ACCOUNT; securitycenter
# (SecurityCenterMuteConfig) skips live unless E2E_SECURITY_CENTER=true;
# firestore (FirestoreBackupSchedule) skips live unless
# E2E_FIRESTORE_DATABASE names a Firestore database;
# apigee (ApigeeNATAddress) skips live unless E2E_APIGEE_INSTANCE names an
# Apigee instance;
# spanner (SpannerInstanceConfig) skips live unless E2E_SPANNER=true;
# datacatalog (DataCatalogTag) skips live unless E2E_DATA_CATALOG=true;
# aiplatform (AIPlatformReasoningEngine) skips live unless E2E_VERTEX_AI=true
# (docs/live-runs.md).
DEFAULT_KINDS=(sqladmin secretmanager sql artifactregistry storage compute iam run networksecurity)
ZONE="${LIVE_ZONE:-us-central1-a}"

project=""
kinds=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --project) project="$2"; shift 2 ;;
    -*) echo "live: unknown flag $1" >&2; exit 2 ;;
    *) kinds+=("$1"); shift ;;
  esac
done
live_project_allowed "${project}" || exit 2
[[ ${#kinds[@]} -gt 0 ]] || kinds=("${DEFAULT_KINDS[@]}")
for k in "${kinds[@]}"; do
  [[ -d "pkg/controller/direct/${k}" ]] || { echo "live: no kind package pkg/controller/direct/${k}" >&2; exit 2; }
done
: "${KUBEBUILDER_ASSETS:?run through task test-live, which installs envtest}"

G=(gcloud --quiet --project "${project}")
if [[ -n "${LIVE_ACCOUNT:-}" ]]; then G+=(--account "${LIVE_ACCOUNT}"); fi
SWEEP=(hack/live/sweep.sh --project "${project}")

sa="${E2E_SERVICE_ACCOUNT:-$(gcloud auth list --filter=status:ACTIVE --format='value(account)' | head -1)}"
if [[ "${sa}" != *.gserviceaccount.com ]]; then
  echo "live: the credentials are ${sa:-none}, not a service account; run on an offload VM with --sa (docs/live-runs.md)" >&2
  exit 2
fi

run_id="cce-$(date -u +%m%d%H%M)-$(od -An -N2 -tx1 /dev/urandom | tr -d ' \n')"
out="offload-artifacts/live/${run_id}"
mkdir -p "${out}"
commit="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
dirty="$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "live: run ${run_id} in ${project} as ${sa}, commit ${commit} (${dirty} changed files), kinds: ${kinds[*]}${LIVE_RUN:+, tests ${LIVE_RUN}}"

finish() {
  local code=$?
  set +o errexit
  echo "== live: cleaning up run ${run_id}"
  "${SWEEP[@]}" --run "${run_id}" || code=1
  "${SWEEP[@]}" || code=1
  "${SWEEP[@]}" --list | tee "${out}/left.txt"
  if ! grep -q '^sweep: 0 live-test resource' "${out}/left.txt"; then
    echo "live: resources are left in ${project} (above); a later sweep deletes them after 2h" >&2
  fi
  echo "== live: summary (${out}/summary.md)"
  cat "${out}/summary.md" 2>/dev/null
  exit "${code}"
}
trap finish EXIT

echo "== live: sweeping leftovers older than 2h"
"${SWEEP[@]}"

needs_sql=0
for k in "${kinds[@]}"; do [[ "${k}" == sql || "${k}" == sqladmin ]] && needs_sql=1; done
sql_instance=""
if [[ ${needs_sql} -eq 1 ]]; then
  sql_instance="${run_id}-pg"
  echo "== live: creating Cloud SQL instance ${sql_instance} (shared by the SQL kinds; minutes)"
  t0=$(date +%s)
  # Through the API: gcloud's GA track has no --labels for Cloud SQL.
  body="$(cat <<JSON
{"name": "${sql_instance}", "databaseVersion": "POSTGRES_16", "region": "${ZONE%-*}",
 "settings": {"edition": "ENTERPRISE", "tier": "db-f1-micro", "availabilityType": "ZONAL",
  "locationPreference": {"zone": "${ZONE}"},
  "dataDiskType": "PD_HDD", "dataDiskSizeGb": "10", "storageAutoResize": false,
  "backupConfiguration": {"enabled": false}, "deletionProtectionEnabled": false,
  "databaseFlags": [{"name": "cloudsql.iam_authentication", "value": "on"}],
  "userLabels": {"cce-e2e": "true", "cce-run": "${run_id}"}}}
JSON
)"
  curl -sS --fail-with-body -X POST -H "Authorization: Bearer $("${G[@]}" auth print-access-token)" \
    -H 'Content-Type: application/json' -d "${body}" \
    "https://sqladmin.googleapis.com/v1/projects/${project}/instances" >"${out}/sql-create.json" \
    || { cat "${out}/sql-create.json" >&2; exit 1; }
  state=""
  for _ in $(seq 1 60); do
    state="$("${G[@]}" sql instances describe "${sql_instance}" --format='value(state)' 2>/dev/null || true)"
    [[ "${state}" == RUNNABLE || "${state}" == FAILED ]] && break
    sleep 20
  done
  [[ "${state}" == RUNNABLE ]] || { echo "live: instance ${sql_instance} is ${state:-missing}, not RUNNABLE" >&2; exit 1; }
  echo "live: instance ${sql_instance} RUNNABLE after $(( $(date +%s) - t0 ))s"
fi

{
  echo "Run \`${run_id}\`, project \`${project}\`, commit \`${commit}\` (${dirty} changed files), started ${started}."
  [[ -z "${sql_instance}" ]] || echo "Shared Cloud SQL instance \`${sql_instance}\` (POSTGRES_16, ENTERPRISE, db-f1-micro, ${ZONE}, 10 GB HDD)."
  echo
  echo "| Kind package | Mock | Live | Live time |"
  echo "| --- | --- | --- | --- |"
} >"${out}/summary.md"

run_filter=()
[[ -z "${LIVE_RUN:-}" ]] || run_filter=(-run "${LIVE_RUN}")
failed=0
for k in "${kinds[@]}"; do
  pkg="./pkg/controller/direct/${k}/"
  echo "== live: ${k}: mock"
  mock=PASS
  E2E_GCP_TARGET=mock go test -count=1 -v -timeout 15m ${run_filter[@]+"${run_filter[@]}"} "${pkg}" >"${out}/${k}.mock.log" 2>&1 || mock=FAIL
  grep -E '^\s*--- (PASS|FAIL|SKIP)|^(ok|FAIL)\s|_test.go:[0-9]+:' "${out}/${k}.mock.log" || true
  echo "== live: ${k}: live"
  live=PASS
  t0=$(date +%s)
  # Streamed as it runs, so a preempted VM's log still has it.
  E2E_GCP_TARGET=real GCP_PROJECT_ID="${project}" E2E_RUN_ID="${run_id}" \
    E2E_SERVICE_ACCOUNT="${sa}" E2E_SQL_INSTANCE="${sql_instance}" \
    go test -count=1 -v -timeout 60m ${run_filter[@]+"${run_filter[@]}"} "${pkg}" 2>&1 | tee "${out}/${k}.live.log" \
    | grep -E --line-buffered '^\s*--- (PASS|FAIL|SKIP)|^(ok|FAIL)\s|_test.go:[0-9]+:|^=== RUN' || live=FAIL
  secs=$(( $(date +%s) - t0 ))
  [[ "${mock}" == PASS && "${live}" == PASS ]] || failed=1
  echo "| ${k} | ${mock} | ${live} | ${secs}s |" >>"${out}/summary.md"
done

if [[ -n "${sql_instance}" ]]; then
  echo "== live: deleting Cloud SQL instance ${sql_instance}"
  "${G[@]}" sql instances delete "${sql_instance}" || failed=1
fi
exit "${failed}"
