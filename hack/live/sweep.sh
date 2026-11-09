#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Delete what live test runs left in a real project (docs/live-runs.md).
# A live run names everything it makes with its run ID (cce-<time>-<rand>)
# and labels what takes labels cce-e2e=true and cce-run=<run ID>.
#
#   hack/live/sweep.sh --project P              leftovers older than 2h
#   hack/live/sweep.sh --project P --run ID     everything of run ID, any age
#   hack/live/sweep.sh --project P --list       list every live-test resource, delete nothing
#   ... --dry-run                               say what would be deleted
#   ... --max-age-hours N                       instead of 2
#
# It refuses any project not in LIVE_PROJECTS (hack/live/projects.sh).
# Swept: Artifact Registry repositories (every location), Secret Manager
# secrets (global and the regions in SECRET_REGIONS), Cloud SQL instances
# (their users and backups go with them), and Compute Engine network
# firewall policies, network endpoint groups (their endpoints go with
# them), Cloud Routers (their route policies go with them) and VPC
# networks, which take no labels and are found by name, and Cloud Storage
# buckets, by name. A failed list or
# delete fails the sweep.
set -o errexit -o nounset -o pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=hack/live/projects.sh
source "${HERE}/projects.sh"

PREFIX="cce-"
SECRET_REGIONS="${SECRET_REGIONS:-us-central1 europe-west1}"
project="" run="" mode="sweep" dry=0 max_age_hours=2
while [[ $# -gt 0 ]]; do
  case "$1" in
    --project) project="$2"; shift 2 ;;
    --run) run="$2"; shift 2 ;;
    --list) mode="list"; shift ;;
    --dry-run) dry=1; shift ;;
    --max-age-hours) max_age_hours="$2"; shift 2 ;;
    *) echo "sweep: unknown argument $1" >&2; exit 2 ;;
  esac
done
live_project_allowed "${project}" || exit 2
if [[ -n "${run}" && ! "${run}" =~ ^${PREFIX}[a-z0-9-]+$ ]]; then
  echo "sweep: run ID ${run} does not start with ${PREFIX}" >&2
  exit 2
fi

G=(gcloud --quiet --project "${project}")
if [[ -n "${LIVE_ACCOUNT:-}" ]]; then G+=(--account "${LIVE_ACCOUNT}"); fi

cutoff="$(date -u -d "-${max_age_hours} hours" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null \
  || date -u -v "-${max_age_hours}H" +%Y-%m-%dT%H:%M:%SZ)"

# filter LABELS NAME_REGEX_PREFIX: what this sweep matches, by label or
# by name, and in the default mode only what is older than the cutoff.
filter() {
  local labels="$1" anchor="$2" f
  if [[ -n "${run}" ]]; then
    f="${labels}.cce-run=${run} OR name~${anchor}${run}-"
  else
    f="${labels}.cce-e2e=true OR name~${anchor}${PREFIX}"
  fi
  if [[ "${mode}" == "sweep" && -z "${run}" ]]; then
    f="(${f}) AND createTime<${cutoff}"
  fi
  echo "${f}"
}

found=0 failed=0
act() { # act KIND NAME CMD...
  local kind="$1" name="$2"; shift 2
  found=$((found + 1))
  if [[ "${mode}" == "list" ]]; then echo "  ${kind} ${name}"; return; fi
  if [[ ${dry} -eq 1 ]]; then echo "  would delete ${kind} ${name}"; return; fi
  if "$@" </dev/null >/dev/null; then
    echo "  deleted ${kind} ${name}"
  else
    echo "  FAILED to delete ${kind} ${name}" >&2
    failed=$((failed + 1))
  fi
}

case "${mode}" in
  list) echo "sweep: live-test resources in ${project}" ;;
  *)
    if [[ -n "${run}" ]]; then
      echo "sweep: everything of run ${run} in ${project}"
    else
      echo "sweep: leftovers older than ${max_age_hours}h (created before ${cutoff}) in ${project}"
    fi ;;
esac

# Artifact Registry: names are projects/P/locations/L/repositories/R.
names="$("${G[@]}" artifacts repositories list --location=all --verbosity=error \
  --filter="$(filter labels /)" --format='value(name)')"
for name in ${names}; do
  loc="$(cut -d/ -f4 <<<"${name}")"
  act "Artifact Registry repository" "${name}" "${G[@]}" artifacts repositories delete "${name##*/}" --location "${loc}"
done

# Secret Manager: a regional secret is listed and deleted only through its
# regional endpoint.
for loc in global ${SECRET_REGIONS}; do
  if [[ "${loc}" == "global" ]]; then
    S=("${G[@]}" secrets)
    L=()
  else
    S=(env "CLOUDSDK_API_ENDPOINT_OVERRIDES_SECRETMANAGER=https://secretmanager.${loc}.rep.googleapis.com/" "${G[@]}" secrets)
    L=(--location "${loc}")
  fi
  names="$("${S[@]}" list ${L[@]+"${L[@]}"} --verbosity=error --filter="$(filter labels /)" --format='value(name)')"
  for name in ${names}; do
    act "Secret Manager secret" "${name}" "${S[@]}" delete "${name##*/}" ${L[@]+"${L[@]}"}
  done
done

# Cloud SQL: names are bare instance names.
names="$("${G[@]}" sql instances list --verbosity=error \
  --filter="$(filter settings.userLabels ^)" --format='value(name)')"
for name in ${names}; do
  act "Cloud SQL instance" "${name}" "${G[@]}" sql instances delete "${name}"
done

# Compute Engine: by name only (these take no labels), and by
# creationTimestamp; what is in a network goes before the network.
cfilter() {
  local f
  if [[ -n "${run}" ]]; then f="name~^${run}-"; else f="name~^${PREFIX}"; fi
  if [[ "${mode}" == "sweep" && -z "${run}" ]]; then
    f="(${f}) AND creationTimestamp<${cutoff}"
  fi
  echo "${f}"
}
# scope REGION ZONE: the location flag for a regional, zonal or global one.
scope() {
  if [[ -n "$1" ]]; then echo "--region=$1"
  elif [[ -n "$2" ]]; then echo "--zone=$2"
  else echo "--global"; fi
}
rows="$("${G[@]}" compute network-firewall-policies list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "network firewall policy" "${name} ${region:-global}" "${G[@]}" compute network-firewall-policies delete "${name}" "$(scope "${region}" "")"
done <<<"${rows}"
rows="$("${G[@]}" compute network-endpoint-groups list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename(),zone.basename())')"
while read -r name region zone; do
  [[ -n "${name}" ]] || continue
  act "network endpoint group" "${name} ${region:-${zone:-global}}" "${G[@]}" compute network-endpoint-groups delete "${name}" "$(scope "${region}" "${zone}")"
done <<<"${rows}"
rows="$("${G[@]}" compute routers list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "Cloud Router" "${name} ${region}" "${G[@]}" compute routers delete "${name}" --region="${region}"
done <<<"${rows}"
# Cloud Storage: buckets by the run's name prefix (bucket names are
# global), aged by their creation time; rm -r takes their objects too.
rows="$("${G[@]}" storage buckets list --verbosity=error --format='value(name,creation_time)')"
while read -r name made; do
  [[ -n "${name}" ]] || continue
  if [[ -n "${run}" ]]; then
    [[ "${name}" == "${run}-"* ]] || continue
  else
    [[ "${name}" == "${PREFIX}"* ]] || continue
    if [[ "${mode}" == "sweep" ]]; then
      [[ "${made:0:19}Z" < "${cutoff}" ]] || continue
    fi
  fi
  act "Cloud Storage bucket" "gs://${name}" "${G[@]}" storage rm -r "gs://${name}"
done <<<"${rows}"

names="$("${G[@]}" compute networks list --verbosity=error --filter="$(cfilter)" --format='value(name)')"
for name in ${names}; do
  act "VPC network" "${name}" "${G[@]}" compute networks delete "${name}"
done

if [[ "${mode}" == "list" ]]; then
  echo "sweep: ${found} live-test resource(s) in ${project}"
else
  echo "sweep: ${found} matched, ${failed} failed"
fi
[[ ${failed} -eq 0 ]]
