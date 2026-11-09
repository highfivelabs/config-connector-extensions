#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Scaffold a kind with Config Connector's own generator
# (third_party/k8s-config-connector/dev/tools/controllerbuilder) instead of
# writing its types, mapper and controller skeleton by hand.
#
#   hack/scaffold-kind.sh SERVICE API_VERSION KIND:PROTO_MESSAGE [OUT_DIR] [-- GENERATE_TYPES_FLAGS...]
#
#   hack/scaffold-kind.sh google.api.serviceusage.v1beta1 \
#       serviceusage.gcp.hfl.dev/v1alpha1 ServiceIdentity:ServiceIdentity \
#       offload-artifacts/scaffold -- --allow-non-ga-proto --allow-non-declarative
#
# SERVICE is the protobuf package in googleapis. The generator only writes into
# the Config Connector checkout, under upstream's group, so this runs it there
# as <service>.cnrm.cloud.google.com, takes every file it wrote or changed
# (apis/<service>/<version>/ and pkg/controller/direct/<service>/), rewrites
# the group and the import paths to ours, and puts them under OUT_DIR
# (default bin/scaffold/KIND) with the same layout as this repository.
# Nothing is copied into the tree: copy the files you keep, keep the
# generated ones (types.generated.go, mapper.generated.go) unedited, write the
# rest beside them, record in the PR what share was generated, and run
# hack/upstream.sh to reset the checkout.
#
# generate-proto.sh needs protoc and clones googleapis, so run this on an
# offload VM:
#   ../offload/offload run --prep 'task upstream' --step 'hack/scaffold-kind.sh ... offload-artifacts/scaffold'
# The files come back with the run's artifacts.
#
# Field behaviour the protos leave out (OUTPUT_ONLY, REQUIRED) goes in
# hack/scaffold/<service>/overlay.proto, which every step reads (--overlay);
# take it from Magic Modules' product YAML or the API reference, and say which
# beside each field. The mapper is generated from the new version's types
# alone, so a service upstream already has (other versions, other kinds) does
# not mix them in. generate-controller writes nothing when upstream already
# has a controller package for the service: the controller is then copied
# from upstream's nearest one, by hand.
#
# It does not fit a resource that has no proto message of its own (a rule or
# an association inside a policy, a project-level setting): say so in the PR.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
usage() { sed -n '2,/^set /p' "$0" | sed 's/^# \{0,1\}//' | grep -v '^set ' >&2; exit 2; }
[[ $# -ge 3 ]] || usage
service="$1" api_version="$2" resource="$3"
shift 3
out="${ROOT}/bin/scaffold/${resource%%:*}"
if [[ $# -gt 0 && "$1" != "--" ]]; then out="$1"; shift; fi
[[ $# -eq 0 || "$1" == "--" ]] || usage
[[ $# -gt 0 ]] && shift
extra=("$@")
[[ "${out}" == /* ]] || out="${ROOT}/${out}"

group="${api_version%/*}" version="${api_version#*/}"
[[ "${group}" == *.gcp.hfl.dev && -n "${version}" && "${version}" != "${api_version}" ]] ||
  { echo "scaffold-kind: API_VERSION must be <service>.gcp.hfl.dev/<version>, not ${api_version}" >&2; exit 2; }
svc="${group%.gcp.hfl.dev}"
upstream_api_version="${svc}.cnrm.cloud.google.com/${version}"

kcc="${ROOT}/third_party/k8s-config-connector"
tool="${kcc}/dev/tools/controllerbuilder"
[[ -d "${tool}" ]] || { echo "scaffold-kind: run 'task upstream' first (${tool} is missing)" >&2; exit 1; }
if [[ -d "${kcc}/apis/${svc}/${version}" ]]; then
  echo "scaffold-kind: ${kcc}/apis/${svc}/${version} exists upstream; pick another version so the generator scaffolds fresh files" >&2
  exit 1
fi

marker="$(mktemp)"
trap 'rm -f "${marker}"' EXIT
sleep 1 # mtimes have a resolution of a second on some filesystems

cd "${tool}"
# It clones googleapis from GitHub; a VM's connection sometimes fails.
for attempt in 1 2 3; do
  if ./generate-proto.sh; then break; fi
  [[ ${attempt} -lt 3 ]] || { echo "scaffold-kind: generate-proto.sh failed 3 times" >&2; exit 1; }
  rm -rf "${kcc}/.build/third_party/googleapis"
  sleep 20
done
# Each step runs even if an earlier one failed, so one run shows every
# failure; what was written is still copied out, and the script fails at the end.
failed=()
overlay=()
if [[ -f "${ROOT}/hack/scaffold/${svc}/overlay.proto" ]]; then overlay=(--overlay "${ROOT}/hack/scaffold/${svc}/overlay.proto"); fi
go run . generate-types --service "${service}" --api-version "${upstream_api_version}" \
  --resource "${resource}" --prepopulate-resource "${overlay[@]+"${overlay[@]}"}" \
  "${extra[@]+"${extra[@]}"}" || failed+=(generate-types)
# The mapper from the new version's types only: upstream's other versions of
# the service would otherwise be mapped into the same file.
mapper_apis="$(mktemp -d)"
mkdir -p "${mapper_apis}/apis/${svc}" "${mapper_apis}/direct"
cp -R "${kcc}/apis/${svc}/${version}" "${mapper_apis}/apis/${svc}/" 2>/dev/null || true
go run . generate-mapper --service "${service}" --api-version "${upstream_api_version}" \
  --api-dir "${mapper_apis}/apis/" --output-dir "${mapper_apis}/direct/" \
  --prepopulate-resource "${overlay[@]+"${overlay[@]}"}" || failed+=(generate-mapper)
mkdir -p "${kcc}/pkg/controller/direct/${svc}"
cp "${mapper_apis}/direct/${svc}/"*.go "${kcc}/pkg/controller/direct/${svc}/" 2>/dev/null || failed+=(generate-mapper-output)
go run . generate-controller --service "${service}" --api-version "${upstream_api_version}" \
  --resource "${resource}" --prepopulate-resource "${overlay[@]+"${overlay[@]}"}" || failed+=(generate-controller)

rm -rf "${out}"
mkdir -p "${out}"
cd "${kcc}"
written=()
while IFS= read -r f; do written+=("${f#./}"); done < <(
  find ./apis/"${svc}" ./pkg/controller/direct/"${svc}" -type f -newer "${marker}" 2>/dev/null | sort)
[[ ${#written[@]} -gt 0 ]] || { echo "scaffold-kind: the generator wrote nothing (failed: ${failed[*]-none})" >&2; exit 1; }
for f in "${written[@]}"; do
  mkdir -p "${out}/$(dirname "${f}")"
  sed -e "s#${svc}\.cnrm\.cloud\.google\.com#${svc}.gcp.hfl.dev#g" \
    -e "s#github.com/GoogleCloudPlatform/k8s-config-connector/apis/${svc}/${version}#github.com/highfivelabs/config-connector-extensions/apis/${svc}/${version}#g" \
    -e "s#github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/${svc}\"#github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/${svc}\"#g" \
    "${f}" >"${out}/${f}"
  # hack/check-spdx.sh: an SPDX line after Google's header, as hack/boilerplate.go.txt has.
  if [[ "${f}" == *.go ]] && ! grep -q 'SPDX-License-Identifier' "${out}/${f}"; then
    awk '{ print } !done && /^\/\/ limitations under the License\.$/ { print ""; print "// SPDX-License-Identifier: Apache-2.0"; done = 1 }' \
      "${out}/${f}" >"${out}/${f}.tmp" && mv "${out}/${f}.tmp" "${out}/${f}"
  fi
done
# Upstream runs goimports after generating; the rewritten import paths sort differently.
find "${out}" -name '*.go' -exec gofmt -w {} +
printf 'scaffold-kind: %s file(s) written under %s:\n' "${#written[@]}" "${out}"
printf '  %s\n' "${written[@]}"
echo "scaffold-kind: run hack/upstream.sh to reset ${kcc}"
[[ ${#failed[@]} -eq 0 ]] || { echo "scaffold-kind: failed: ${failed[*]}" >&2; exit 1; }
