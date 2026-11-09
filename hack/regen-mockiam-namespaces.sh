#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Regenerates mockgcp's Go for workload identity pool namespaces
# (patches/k8s-config-connector/0006-*) from the proto the patch adds,
# mockgcp/apis/mockgcp/iam/v1/workload_identity_pool_namespace.proto (upstream
# pull request #14209's mockgcp/apis/google/iam/v1/workload_identity_pool_namespace.proto,
# moved to package mockgcp.iam.v1). Uses mockgcp's own toolchain, as
# hack/regen-mocksql-users.sh does: protoc 3.12.4, protoc-gen-go v1.28.1,
# protoc-gen-go-grpc v1.2.0, grpc-gateway v2.11.3, and googleapis at
# mockgcp's pin for the imported annotations.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KCC="${REPO_ROOT}/third_party/k8s-config-connector"
MOCKGCP="${KCC}/mockgcp"
BIN="${REPO_ROOT}/bin/protoc-3.12.4"
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

PROTO="mockgcp/iam/v1/workload_identity_pool_namespace.proto"
[[ -f "${MOCKGCP}/apis/${PROTO}" ]] || { echo "missing ${MOCKGCP}/apis/${PROTO}; apply the patches first (hack/upstream.sh)" >&2; exit 1; }

GOOGLEAPIS_COMMIT="$(awk '/googleapis\/googleapis/ {print $2}' "${MOCKGCP}/git.versions")"

mkdir -p "${BIN}"
if [[ ! -x "${BIN}/protoc/bin/protoc" ]]; then
  case "$(uname -s)" in
    Darwin) zip="protoc-3.12.4-osx-x86_64.zip" ;;
    Linux) zip="protoc-3.12.4-linux-x86_64.zip" ;;
    *) echo "unsupported OS" >&2; exit 1 ;;
  esac
  curl -sSfL -o "${BIN}/protoc.zip" "https://github.com/protocolbuffers/protobuf/releases/download/v3.12.4/${zip}"
  rm -rf "${BIN}/protoc" && mkdir -p "${BIN}/protoc"
  (cd "${BIN}/protoc" && unzip -q ../protoc.zip)
fi
GOBIN="${BIN}" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
GOBIN="${BIN}" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
GOBIN="${BIN}" go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.11.3

git init --quiet "${WORK}/googleapis"
git -C "${WORK}/googleapis" remote add origin https://github.com/googleapis/googleapis.git
git -C "${WORK}/googleapis" sparse-checkout set --no-cone /google/api/ /google/rpc/ /google/longrunning/ >/dev/null
git -C "${WORK}/googleapis" fetch --quiet --depth 1 --filter=blob:none origin "${GOOGLEAPIS_COMMIT}"
git -C "${WORK}/googleapis" -c advice.detachedHead=false checkout --quiet FETCH_HEAD

OUT="${WORK}/generated"
mkdir -p "${OUT}"
PATH="${BIN}:${PATH}" "${BIN}/protoc/bin/protoc" \
  -I "${MOCKGCP}/apis" \
  -I "${WORK}/googleapis" \
  --go_out "${OUT}" --go_opt paths=source_relative \
  --go-grpc_out "${OUT}" --go-grpc_opt paths=source_relative \
  --grpc-gateway_out "${OUT}" --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative \
  "${MOCKGCP}/apis/${PROTO}"

DEST="${MOCKGCP}/generated/mockgcp/iam/v1"
cp "${OUT}"/mockgcp/iam/v1/workload_identity_pool_namespace*.go "${DEST}/"
gofmt -w "${DEST}"/workload_identity_pool_namespace*.go
echo "regenerated ${DEST}/workload_identity_pool_namespace*.go"
