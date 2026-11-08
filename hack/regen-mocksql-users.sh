#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# Regenerate mockgcp's Cloud SQL users protos (cloud_sql_users*.pb*.go) in
# third_party/k8s-config-connector with the extra fields our mock needs,
# the same way mockgcp's own Makefile does it, but for this one proto file.
#
# Upstream recipe (mockgcp/Makefile, target gen-proto): clone googleapis at
# the commit in mockgcp/git.versions, run apply-proto-patches.sh, run
# fixup-third-party.sh (google/cloud -> mockgcp/cloud), then protoc 3.12.4
# with protoc-gen-go v1.28.1, protoc-gen-go-grpc v1.2.0 and
# protoc-gen-grpc-gateway v2.11.3, then gofmt. We run the same versions.
# protoc 3.12.4 has no arm64 macOS build, so on macOS we use the x86_64 one
# (Rosetta).
#
#   ONLY_UPSTREAM=1 hack/regen-mocksql-users.sh
#       upstream's proto patches only; on a pristine checkout this must
#       leave `git -C third_party/k8s-config-connector diff` empty, which
#       proves the recipe reproduces upstream's committed files.
#   hack/regen-mocksql-users.sh
#       upstream's patches plus ours (User.database_roles and the update
#       request's database_roles / revoke_existing_roles).
#
# The result, together with the hand edit to mockgcp/mocksql/sqluser.go, is
# patches/k8s-config-connector/0001-mockgcp-sql-users-database-roles.patch.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KCC="${REPO_ROOT}/third_party/k8s-config-connector"
MOCKGCP="${KCC}/mockgcp"
BIN="${REPO_ROOT}/bin/protoc-3.12.4"
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

GOOGLEAPIS_COMMIT="$(awk '/googleapis\/googleapis/ {print $2}' "${MOCKGCP}/git.versions")"

# Tools, pinned as in mockgcp/Makefile.
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

# googleapis at mockgcp's pin, only what the SQL protos import.
git init --quiet "${WORK}/googleapis"
git -C "${WORK}/googleapis" remote add origin https://github.com/googleapis/googleapis.git
git -C "${WORK}/googleapis" sparse-checkout set --no-cone /google/cloud/sql/v1beta4/ /google/api/ /google/rpc/ /google/type/ /google/longrunning/ >/dev/null
git -C "${WORK}/googleapis" fetch --quiet --depth 1 --filter=blob:none origin "${GOOGLEAPIS_COMMIT}"
git -C "${WORK}/googleapis" -c advice.detachedHead=false checkout --quiet FETCH_HEAD
SQL="${WORK}/googleapis/google/cloud/sql/v1beta4"

# 1. Upstream's patch for cloud_sql_users.proto (apply-proto-patches.sh,
#    "SQL patches"), then ours. patch-proto is mockgcp's own tool.
patch() { (cd "${MOCKGCP}/tools/patch-proto" && go run . --file "$1" --message "$2" --mode append); }

patch "${SQL}/cloud_sql_users.proto" User <<'EOF'

  // The status of the user's IAM authentication.

  optional string iam_status = 16;

EOF

if [[ -z "${ONLY_UPSTREAM:-}" ]]; then
# gaps-gcp: User.database_roles, field 15 as in googleapis master.
patch "${SQL}/cloud_sql_users.proto" User <<'EOF'

  // Optional. Role memberships of the user
  repeated string database_roles = 15;

EOF

# gaps-gcp: the update request carries the roles as query parameters, field
# numbers as in googleapis master.
patch "${SQL}/cloud_sql_users.proto" SqlUsersUpdateRequest <<'EOF'

  // Optional. List of database roles to grant to the user. body.database_roles
  // will be ignored for update request.
  repeated string database_roles = 5;

  // Optional. Specifies whether to revoke existing roles that are not present
  // in the `database_roles` field. If `false` or unset, the database roles
  // specified in `database_roles` are added to the user's existing roles.
  optional bool revoke_existing_roles = 6;

EOF
fi

# 2. fixup-third-party.sh, for this directory only.
mkdir -p "${WORK}/googleapis/mockgcp/cloud"
mv "${WORK}/googleapis/google/cloud/sql" "${WORK}/googleapis/mockgcp/cloud/sql"
find "${WORK}/googleapis/mockgcp/cloud/sql" -type f -name '*.proto' -print0 | xargs -0 sed -i.bak \
  -e "s@google/cloud/@mockgcp/cloud/@g" \
  -e "s@google\.cloud@mockgcp.cloud@g"
find "${WORK}/googleapis/mockgcp" -name '*.bak' -delete

# 3. protoc, as in gen-proto-no-fixup, for the users proto only.
OUT="${WORK}/generated"
mkdir -p "${OUT}"
PATH="${BIN}:${PATH}" "${BIN}/protoc/bin/protoc" \
  -I "${MOCKGCP}/apis" \
  -I "${WORK}/googleapis" \
  --go_out "${OUT}" --go_opt paths=source_relative \
  --go-grpc_out "${OUT}" --go-grpc_opt paths=source_relative \
  --grpc-gateway_out "${OUT}" --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative \
  --experimental_allow_proto3_optional \
  "${WORK}/googleapis/mockgcp/cloud/sql/v1beta4/cloud_sql_users.proto"

# 4. Copy into mockgcp/generated and gofmt, as upstream's committed files are.
#    The .pb.go carries the proto's own licence header; upstream's gateway and
#    grpc files carry none either.
DEST="${MOCKGCP}/generated/mockgcp/cloud/sql/v1beta4"
cp "${OUT}"/mockgcp/cloud/sql/v1beta4/cloud_sql_users*.go "${DEST}/"
gofmt -w "${DEST}"/cloud_sql_users*.go
# Upstream's file was formatted by a tool that also puts an empty comment line
# before the indented oneof list; match it so the diff is only our fields.
perl -0pi -e 's@(// Types that are assignable to UserDetails:\n)(\t//\t\*User_SqlserverUserDetails)@$1\t//\n$2@' \
  "${DEST}/cloud_sql_users.pb.go"
echo "regenerated: $(cd "${OUT}/mockgcp/cloud/sql/v1beta4" && ls | tr '\n' ' ')"
