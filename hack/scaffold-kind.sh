#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Scaffold a kind with Config Connector's own generator
# (third_party/k8s-config-connector/dev/tools/controllerbuilder) instead of
# writing its types, mapper and controller skeleton by hand. The output goes
# to a scratch directory, never into the tree: copy the generated files in
# beside the hand-written ones (generated files named zz_/*.generated.go,
# hand-written beside them), then record in the PR what share was generated.
#
#   hack/scaffold-kind.sh SERVICE API_VERSION KIND:PROTO_MESSAGE [OUT_DIR]
#
#   hack/scaffold-kind.sh google.cloud.secretmanager.v1 \
#       secretmanager.gcp.hfl.dev/v1alpha1 SecretManagerSecretVersion:SecretVersion
#
# SERVICE is the protobuf package in googleapis; the generator reads the protos
# it fetches (dev/tools/controllerbuilder/generate-proto.sh, needs protoc, so
# run this on an offload VM: offload run --prep 'task upstream' -- hack/scaffold-kind.sh ...).
# It does not fit a resource that has no proto message of its own (a rule or an
# association inside a policy, a project-level setting): say so in the PR.
#
# Not yet exercised by this repository: the first kind that fits (a resource
# with a googleapis proto, such as a folder ServiceIdentity or an IAM role)
# should try it and fix this script.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[[ $# -ge 3 ]] || { sed -n '2,/^set /p' "$0" | sed 's/^# \{0,1\}//' | head -24 >&2; exit 2; }
service="$1" api_version="$2" resource="$3"
out="${4:-${ROOT}/bin/scaffold/${resource%%:*}}"
tool="${ROOT}/third_party/k8s-config-connector/dev/tools/controllerbuilder"
[[ -d "${tool}" ]] || { echo "scaffold-kind: run 'task upstream' first (${tool} is missing)" >&2; exit 1; }
mkdir -p "${out}"

cd "${tool}"
./generate-proto.sh
go run . generate-types --service "${service}" --api-version "${api_version}" --resource "${resource}"
go run . generate-mapper --service "${service}" --output-dir "${out}"
go run . generate-direct-reconciler --service "${service}" --api-version "${api_version}" --resource "${resource}"
echo "scaffold-kind: generated files are under ${tool}'s output dirs and ${out}; copy them in, keep hand-written code beside them"
