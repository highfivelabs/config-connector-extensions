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
# firewall policies (their associations are removed first), backend services and health checks, network endpoint groups (their endpoints go with
# them), VPN tunnels, HA and external VPN gateways, addresses, Cloud Routers
# (their route policies go with them) and VPC
# networks, which take no labels and are found by name, Cloud Storage
# buckets (their managed folders go with them), by name, keys of the project's common instance metadata, by
# name and aged by the run ID in them, and workload identity pools (with
# what is in them) and Cloud Run services, by name, and Network Security
# address groups (global and the regions in ADDRESS_GROUP_REGIONS), by
# label or name, after the firewall policies that may use them, and
# security profile groups and security profiles, by name, and IAM deny
# policies on the project, by name, where the account may list them
# (E2E_IAM_DENY_ADMIN=true), log sinks of the billing account in
# E2E_BILLING_ACCOUNT, by name, where it is set, Security Command
# Center mute configs of the project, by name, where it is active
# (E2E_SECURITY_CENTER=true), Cloud DNS response policies, by name,
# where the project has Cloud DNS (E2E_DNS_ADMIN=true), and the discokind
# kinds in hack/live/discokinds.tsv (their global collection in the
# project), by name, through the API, and the project's GKE Hub fleet
# when a test made it, by label, global log buckets (with their links), by
# name, where the account may list them, and BigQuery datasets, by name,
# where the account has BigQuery (E2E_BIGQUERY_ADMIN=true). A failed list
# or delete fails the sweep.
set -o errexit -o nounset -o pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=hack/live/projects.sh
source "${HERE}/projects.sh"

PREFIX="cce-"
SECRET_REGIONS="${SECRET_REGIONS:-us-central1 europe-west1}"
ADDRESS_GROUP_REGIONS="${ADDRESS_GROUP_REGIONS:-us-central1}"
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
# assoc_scope REGION: the policy-location flag of the associations commands.
assoc_scope() {
  if [[ -n "$1" ]]; then echo "--firewall-policy-region=$1"; else echo "--global-firewall-policy"; fi
}
rows="$("${G[@]}" compute network-firewall-policies list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  # A policy with a network attached cannot be deleted: remove its
  # associations first (the describe output lists them).
  if [[ "${mode}" != "list" && ${dry} -eq 0 ]]; then
    attached="$("${G[@]}" compute network-firewall-policies describe "${name}" "$(scope "${region}" "")" --verbosity=error \
      --flatten='associations[]' --format='value(associations.name)' </dev/null)"
    while read -r assoc; do
      [[ -n "${assoc}" ]] || continue
      act "firewall policy association" "${name}/${assoc} ${region:-global}" "${G[@]}" compute network-firewall-policies associations delete \
        --firewall-policy="${name}" --name="${assoc}" "$(assoc_scope "${region}")"
    done <<<"${attached}"
  fi
  act "network firewall policy" "${name} ${region:-global}" "${G[@]}" compute network-firewall-policies delete "${name}" "$(scope "${region}" "")"
done <<<"${rows}"

# Network Security address groups (a project's; an organization's are never
# made live): names are projects/P/locations/L/addressGroups/A.
for loc in global ${ADDRESS_GROUP_REGIONS}; do
  names="$("${G[@]}" network-security address-groups list --location="${loc}" --verbosity=error \
    --filter="$(filter labels /)" --format='value(name)')"
  for name in ${names}; do
    act "address group" "${name}" "${G[@]}" network-security address-groups delete "${name##*/}" --location="${loc}"
  done
done

# by_run NAME [CREATED]: whether this sweep takes NAME: the run's own, or,
# with no run given, any live-test name made before the cutoff, by CREATED
# (an RFC 3339 time) or else by the time in the run ID (cce-MMDDhhmm-...,
# UTC, this year).
by_run() {
  local name="$1" made="${2:-}"
  if [[ -n "${run}" ]]; then [[ "${name}" == "${run}-"* ]]; return; fi
  [[ "${name}" =~ ^${PREFIX}([0-9]{2})([0-9]{2})([0-9]{2})([0-9]{2})-[0-9a-f]+- ]] || return 1
  [[ "${mode}" == "sweep" ]] || return 0
  if [[ -z "${made}" ]]; then
    made="$(date -u +%Y)-${BASH_REMATCH[1]}-${BASH_REMATCH[2]}T${BASH_REMATCH[3]}:${BASH_REMATCH[4]}:00Z"
  fi
  [[ "${made:0:19}Z" < "${cutoff}" ]]
}

# Network Security security profile groups, then the security profiles
# they hold (a project's; an organization's are never made live), by the
# run's name: gcloud knows only an organization's, so through the API.
# Names are projects/P/locations/global/securityProfile{Group}s/S.
ns_api() { # ns_api METHOD NAME
  curl -sS --fail-with-body -X "$1" -H "Authorization: Bearer $("${G[@]}" auth print-access-token)" \
    "https://networksecurity.googleapis.com/v1/$2"
}
ns_delete() { # ns_delete NAME: delete and wait for the operation (a group
  # must be gone before its profiles can go).
  local op fin
  op="$(ns_api DELETE "$1" | jq -r '.name')" || return 1
  for _ in $(seq 1 60); do
    fin="$(ns_api GET "${op}" | jq -r '.done // false')" || return 1
    if [[ "${fin}" == true ]]; then
      ns_api GET "${op}" | jq -e 'has("error") | not' >/dev/null
      return
    fi
    sleep 5
  done
  return 1
}
for kind in securityProfileGroups securityProfiles; do
  rows="$(ns_api GET "projects/${project}/locations/global/${kind}?pageSize=1000" \
    | jq -r --arg k "${kind}" '.[$k][]? | "\(.name) \(.createTime)"')"
  while read -r name made; do
    [[ -n "${name}" ]] || continue
    by_run "${name##*/}" "${made}" || continue
    act "${kind%s}" "${name}" ns_delete "${name}"
  done <<<"${rows}"
done
# The discokind kinds (hack/live/discokinds.tsv, generated): each
# collection under the project, by the run's name; an ID that takes no
# dashes (cce11091530a3f2...) is read as the run's name with them. A
# soft-deleted resource is already gone.
disco_api() { # disco_api METHOD URL
  curl -sS --fail-with-body -X "$1" -H "Authorization: Bearer $("${G[@]}" auth print-access-token)" "$2"
}
disco_delete() { # disco_delete BASE NAME: delete, and wait if it answers an operation
  local op name self fin
  op="$(disco_api DELETE "$1$2")" || return 1
  # A Compute Engine operation: status until DONE, at its selfLink.
  self="$(jq -r 'if (.kind == "compute#operation" and .status != "DONE") then .selfLink else "" end' <<<"${op}")"
  if [[ -n "${self}" ]]; then
    for _ in $(seq 1 60); do
      fin="$(disco_api GET "${self}" | jq -r '.status')" || return 1
      [[ "${fin}" == DONE ]] && return 0
      sleep 5
    done
    return 1
  fi
  name="$(jq -r 'if (.done == false and (.name // "" | test("/operations/|^operations/"))) then .name else "" end' <<<"${op}")"
  [[ -n "${name}" ]] || return 0
  for _ in $(seq 1 60); do
    fin="$(disco_api GET "$1${name}" | jq -r '.done // false')" || return 1
    [[ "${fin}" == true ]] && return 0
    sleep 5
  done
  return 1
}
while IFS=$'\t' read -r dkind base coll field soft; do
  [[ -n "${dkind}" && "${dkind}" != \#* ]] || continue
  # A kind whose test never runs live (its fixtures' live-skip) makes
  # nothing to sweep, and listing its collection may need a permission the
  # live-test account is not given (a preview API's, BYOIP's): left alone,
  # as the deny policies and mute configs are.
  if compgen -G "${HERE}/../../pkg/controller/direct/*/testdata/discokind/${dkind}/live-skip" >/dev/null; then
    continue
  fi
  coll="${coll//\{project\}/${project}}"
  softf="${soft%%=*}" softv="${soft#*=}"
  rows="$(disco_api GET "${base}${coll}?pageSize=1000" \
    | jq -r --arg f "${field}" --arg s "${softf}" '.[$f][]? | "\(.name) \(.createTime // "-") \(if $s == "-" then "-" else (.[$s] // "-") end)"')"
  while read -r name made state; do
    [[ -n "${name}" ]] || continue
    [[ "${soft}" == "-" || "${state}" != "${softv}" ]] || continue
    [[ "${made}" != "-" ]] || made=""
    id="$(sed -E 's/^cce([0-9]{8})([0-9a-f]{4})/cce-\1-\2-/' <<<"${name##*/}")"
    by_run "${id}" "${made}" || continue
    # A Compute Engine name is the bare ID; the item's path is its collection's.
    # (Its creationTimestamp has a local offset, so its age is the run ID's.)
    [[ "${name}" == */* ]] || name="${coll}/${name}"
    act "${dkind}" "${name}" disco_delete "${base}" "${name}"
  done <<<"${rows}"
done <"${HERE}/discokinds.tsv"
# The project's fleet, when a live GKEHubRolloutSequence test made it (it
# then carries the run's labels; pkg/controller/direct/gkehub/fleet_test.go),
# after the rollout sequences that name it. A fleet without our labels is
# the project's own and is left alone. A project has at most one,
# "default"; none (404), or no GKE Hub for the account (403: then no test
# of ours made one either), is nothing to sweep.
fleet="projects/${project}/locations/global/fleets/default"
if body="$(disco_api GET "https://gkehub.googleapis.com/v1/${fleet}" 2>/dev/null)"; then
  read -r e2e frun made fstate < <(jq -r '"\(.labels["cce-e2e"] // "-") \(.labels["cce-run"] // "-") \(.createTime // "-") \(.state.code // "-")"' <<<"${body}")
  if [[ "${e2e}" == true && "${fstate}" != DELETING ]]; then
    [[ "${made}" != "-" ]] || made=""
    if by_run "${frun}-fleet" "${made}"; then
      act fleet "${fleet}" disco_delete "https://gkehub.googleapis.com/v1/" "${fleet}"
    fi
  fi
else
  code="$(jq -r '.error.code // "-"' <<<"${body}" 2>/dev/null || echo -)"
  if [[ "${code}" != 404 && "${code}" != 403 ]]; then
    echo "  FAILED to read the fleet ${fleet}: ${body}" >&2
    failed=$((failed + 1))
  fi
fi
rows="$("${G[@]}" compute backend-services list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "backend service" "${name} ${region:-global}" "${G[@]}" compute backend-services delete "${name}" "$(scope "${region}" "")"
done <<<"${rows}"
rows="$("${G[@]}" compute health-checks list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "health check" "${name} ${region:-global}" "${G[@]}" compute health-checks delete "${name}" "$(scope "${region}" "")"
done <<<"${rows}"
rows="$("${G[@]}" compute network-endpoint-groups list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename(),zone.basename())')"
while read -r name region zone; do
  [[ -n "${name}" ]] || continue
  act "network endpoint group" "${name} ${region:-${zone:-global}}" "${G[@]}" compute network-endpoint-groups delete "${name}" "$(scope "${region}" "${zone}")"
done <<<"${rows}"
# VPN tunnels before the gateways and routers they use.
rows="$("${G[@]}" compute vpn-tunnels list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "VPN tunnel" "${name} ${region}" "${G[@]}" compute vpn-tunnels delete "${name}" --region="${region}"
done <<<"${rows}"
rows="$("${G[@]}" compute vpn-gateways list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "HA VPN gateway" "${name} ${region}" "${G[@]}" compute vpn-gateways delete "${name}" --region="${region}"
done <<<"${rows}"
# externalVpnGateways.list refuses a name~ filter (a 400, "Invalid list
# filter expression", seen live), so the names and ages are matched here
# as cfilter would.
rows="$("${G[@]}" compute external-vpn-gateways list --verbosity=error --format='value(name,creationTimestamp)')"
while read -r name made; do
  [[ -n "${name}" ]] || continue
  if [[ -n "${run}" ]]; then
    [[ "${name}" == "${run}-"* ]] || continue
  else
    [[ "${name}" == "${PREFIX}"* ]] || continue
    if [[ "${mode}" == "sweep" ]]; then
      [[ "$(date -u -d "${made}" +%s 2>/dev/null || echo 0)" -lt "$(date -u -d "${cutoff}" +%s 2>/dev/null || echo 0)" ]] || continue
    fi
  fi
  act "external VPN gateway" "${name}" "${G[@]}" compute external-vpn-gateways delete "${name}"
done <<<"${rows}"
# Regional addresses (the VPN tunnel test's peer address), by name.
rows="$("${G[@]}" compute addresses list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "address" "${name} ${region:-global}" "${G[@]}" compute addresses delete "${name}" "$(scope "${region}" "")"
done <<<"${rows}"
rows="$("${G[@]}" compute routers list --verbosity=error \
  --filter="$(cfilter)" --format='value(name,region.basename())')"
while read -r name region; do
  [[ -n "${name}" ]] || continue
  act "Cloud Router" "${name} ${region}" "${G[@]}" compute routers delete "${name}" --region="${region}"
done <<<"${rows}"
# Workload identity pools: deleting one deletes what is in it (it is kept,
# deleted, for 30 days, and its ID is not reused).
rows="$("${G[@]}" iam workload-identity-pools list --location=global --verbosity=error --format='value(name.basename())')"
for id in ${rows}; do
  by_run "${id}" || continue
  act "workload identity pool" "${id}" "${G[@]}" iam workload-identity-pools delete "${id}" --location=global
done

# IAM deny policies on the project, by the run's name, only where the
# live-test service account may list them (E2E_IAM_DENY_ADMIN=true: Deny
# Admin, which is granted on an organization); without it the IAMDenyPolicy
# test does not run live and there is nothing to sweep.
if [[ "${E2E_IAM_DENY_ADMIN:-}" == true ]]; then
  attachment="cloudresourcemanager.googleapis.com/projects/${project}"
  rows="$("${G[@]}" iam policies list --attachment-point="${attachment}" --kind=denypolicies \
    --verbosity=error --format='value(name.basename(),createTime)')"
  while read -r id made; do
    [[ -n "${id}" ]] || continue
    by_run "${id}" "${made}" || continue
    act "deny policy" "${id}" "${G[@]}" iam policies delete "${id}" --attachment-point="${attachment}" --kind=denypolicies
  done <<<"${rows}"
fi

# Log sinks of the billing account in E2E_BILLING_ACCOUNT
# (billingAccounts/{id}), by the run's name, only where the live-test
# service account administers logging on it; without it the
# LoggingBillingAccountSink test does not run live and there is nothing to
# sweep.
if [[ -n "${E2E_BILLING_ACCOUNT:-}" ]]; then
  billing="${E2E_BILLING_ACCOUNT#billingAccounts/}"
  rows="$("${G[@]}" logging sinks list --billing-account="${billing}" --verbosity=error --format='value(name,createTime)')"
  while read -r id made; do
    [[ -n "${id}" ]] || continue
    by_run "${id}" "${made}" || continue
    act "billing account log sink" "${id}" "${G[@]}" logging sinks delete "${id}" --billing-account="${billing}"
  done <<<"${rows}"
fi

# Security Command Center mute configs of the project, by the run's name,
# only where Security Command Center is active for the project and the
# live-test service account may list them (E2E_SECURITY_CENTER=true);
# without it the SecurityCenterMuteConfig test does not run live.
if [[ "${E2E_SECURITY_CENTER:-}" == true ]]; then
  rows="$("${G[@]}" scc muteconfigs list --project="${project}" --verbosity=error \
    --format='value(name.basename(),createTime)')"
  while read -r id made; do
    [[ -n "${id}" ]] || continue
    by_run "${id}" "${made}" || continue
    act "mute config" "${id}" "${G[@]}" scc muteconfigs delete "${id}" --project="${project}"
  done <<<"${rows}"
fi

# Cloud DNS response policies, by the run's name, before the networks they
# are bound to (a bound network cannot be deleted), only where the project
# has Cloud DNS for the live-test service account (E2E_DNS_ADMIN=true:
# dns.googleapis.com on and roles/dns.admin); without it the
# DNSResponsePolicy test does not run live and there is nothing to sweep.
# A policy is unbound from its networks before it is deleted, as the
# controller does.
dns_rp_delete() { # dns_rp_delete NAME
  "${G[@]}" dns response-policies update "$1" --networks="" &&
    "${G[@]}" dns response-policies delete "$1"
}
if [[ "${E2E_DNS_ADMIN:-}" == true ]]; then
  rows="$("${G[@]}" dns response-policies list --verbosity=error --format='value(responsePolicyName)')"
  for name in ${rows}; do
    by_run "${name}" || continue
    act "DNS response policy" "${name}" dns_rp_delete "${name}"
  done
fi

# BigQuery datasets (the IAMMember test's), by the run's name with its
# dashes as underscores (a dataset ID takes no dashes), their tables with
# them, only where the live-test account has BigQuery
# (E2E_BIGQUERY_ADMIN=true); without it the test does not run live.
if [[ "${E2E_BIGQUERY_ADMIN:-}" == true ]]; then
  bqbase="https://bigquery.googleapis.com/bigquery/v2/projects/${project}/datasets"
  rows="$(disco_api GET "${bqbase}?maxResults=1000" | jq -r '.datasets[]? | .datasetReference.datasetId')"
  for id in ${rows}; do
    by_run "${id//_/-}" || continue
    act "BigQuery dataset" "${id}" disco_api DELETE "${bqbase}/${id}?deleteContents=true"
  done
fi

# Cloud Run services, every region.
rows="$("${G[@]}" run services list --verbosity=error \
  --format='value(metadata.name,metadata.labels."cloud.googleapis.com/location",metadata.creationTimestamp)')"
while read -r name region made; do
  [[ -n "${name}" ]] || continue
  by_run "${name}" "${made}" || continue
  act "Cloud Run service" "${name} ${region}" "${G[@]}" run services delete "${name}" --region="${region}"
done <<<"${rows}"

# Log buckets (a live LoggingLink test's, with Log Analytics on), global,
# by the run's name, links first (deleting a link is an operation). A
# deleted bucket stays DELETE_REQUESTED for seven days: already gone. An
# account that may not list log buckets (403) made none: nothing to sweep.
# Names are projects/P/locations/global/buckets/B.
lbase="https://logging.googleapis.com/v2/"
if body="$(disco_api GET "${lbase}projects/${project}/locations/global/buckets?pageSize=1000" 2>/dev/null)"; then
  rows="$(jq -r '.buckets[]? | select(.lifecycleState == "ACTIVE") | "\(.name) \(.createTime // "-")"' <<<"${body}")"
  while read -r name made; do
    [[ -n "${name}" ]] || continue
    [[ "${made}" != "-" ]] || made=""
    by_run "${name##*/}" "${made}" || continue
    links="$(disco_api GET "${lbase}${name}/links" | jq -r '.links[]? | select(.lifecycleState != "DELETE_REQUESTED") | .name')"
    for link in ${links}; do
      act "log bucket link" "${link}" disco_delete "${lbase}" "${link}"
    done
    act "log bucket" "${name}" disco_api DELETE "${lbase}${name}"
  done <<<"${rows}"
else
  code="$(jq -r '.error.code // "-"' <<<"${body}" 2>/dev/null || echo -)"
  if [[ "${code}" != 403 ]]; then
    echo "  FAILED to list log buckets: ${body}" >&2
    failed=$((failed + 1))
  fi
fi

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

# Compute Engine project metadata: keys are named <run ID>-<base>; a key
# has no creation time, so its age is the run ID's (cce-MMDDhhmm-xxxx, UTC,
# this year).
keys="$("${G[@]}" compute project-info describe --verbosity=error \
  --flatten=commonInstanceMetadata.items --format='value(commonInstanceMetadata.items.key)')"
for key in ${keys}; do
  if [[ -n "${run}" ]]; then
    [[ "${key}" == "${run}-"* ]] || continue
  else
    [[ "${key}" =~ ^${PREFIX}([0-9]{2})([0-9]{2})([0-9]{2})([0-9]{2})-[0-9a-f]+- ]] || continue
    if [[ "${mode}" == "sweep" ]]; then
      made="$(date -u +%Y)-${BASH_REMATCH[1]}-${BASH_REMATCH[2]}T${BASH_REMATCH[3]}:${BASH_REMATCH[4]}:00Z"
      [[ "${made}" < "${cutoff}" ]] || continue
    fi
  fi
  act "project metadata key" "${key}" "${G[@]}" compute project-info remove-metadata --keys="${key}"
done

if [[ "${mode}" == "list" ]]; then
  echo "sweep: ${found} live-test resource(s) in ${project}"
else
  echo "sweep: ${found} matched, ${failed} failed"
fi
[[ ${failed} -eq 0 ]]
