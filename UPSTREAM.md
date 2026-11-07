# Upstream

gaps-gcp builds on Config Connector at one commit of `master`.
`hack/upstream.sh` reads the commit from the table below (the "Commit"
row), fetches it into `third_party/k8s-config-connector` and applies
`patches/k8s-config-connector/*.patch` in order; `hack/upstream.sh --check`
fails if the checkout is anything else.

| | |
| --- | --- |
| Repository | `github.com/GoogleCloudPlatform/k8s-config-connector` |
| Commit | `672414b8191fc2d2c12fff53f6faccf5ae291eef` |
| Commit date | 2026-11-07T03:04:32Z ("Greenfield: Triage judgement queue and finalize direct KRM types and identity for DataplexGlossaryTerm (#15158)") |
| Branch | `master` |
| Nearest release | `v1.160.0` = `c38b031c7f5e5657b6ac6afc18da95e039066ad8` (2026-10-29); the pin is ahead of it |
| Pinned on | 2026-11-07 |
| How | `git fetch --depth 1 origin <commit>`; used through `replace` in `go.mod`, never vendored or committed |

## What is copied, from where

All paths are in Config Connector at the commit above unless noted.

| Here | From | Changed |
| --- | --- | --- |
| `apis/sql/v1beta1/groupversion_info.go` | `apis/sql/v1beta1/groupversion_info.go` | group `sql.gcp.hfl.dev` |
| `apis/sql/v1beta1/sqluser_types.go` | `pkg/clients/generated/apis/sql/v1beta1/sqluser_types.go` (the Go types of the Terraform-based `SQLUser` CRD) | group, labels and short names; `instanceRef` and `password` use the shared direct-controller types (same schema); new `spec.databaseRoles`, `status.appliedDatabaseRoles` |
| `apis/sql/v1beta1/sqluser_identity.go` | `apis/sql/v1beta1/sqlsslcert_identity.go` | identity is `projects/{project}/instances/{instance}/users/{name}`, name user-chosen, MySQL host carried beside it |
| `pkg/controller/direct/sql/client.go` | `pkg/controller/direct/sql/client.go` | only the users and operations clients |
| `pkg/controller/direct/sql/sqluser_controller.go` | `pkg/controller/direct/sql/sqlsslcert_controller.go` | changed for users; `databaseRoles` as Terraform's `google_sql_user` does it |
| `hack/boilerplate.go.txt` | `hack/boilerplate.go.txt` | no |
| `hack/generate.sh` | `dev/tasks/generate-crds` (the controller-gen and crd-tools steps) | our paths; no version annotation, no alpha backport |
| `hack/regen-mocksql-users.sh` | `mockgcp/Makefile` (`gen-proto`), `mockgcp/apply-proto-patches.sh`, `mockgcp/fixup-third-party.sh` | one proto file only; our extra fields |
| `LICENSE` | `LICENSE` | no |

The CRD schema in `config/crds/` has the same property paths as upstream's
`config/crds/resources/apiextensions.k8s.io_v1_customresourcedefinition_sqlusers.sql.cnrm.cloud.google.com.yaml`
plus `spec.databaseRoles` and `status.appliedDatabaseRoles` (checked by
flattening both).

## Patches to the checkout

| Patch | What | Upstream candidate |
| --- | --- | --- |
| `0001-mockgcp-sql-users-database-roles.patch` | mockgcp's Cloud SQL `User` gains `database_roles`, and `users.update` gains `database_roles` / `revoke_existing_roles` (googleapis master's field numbers); regenerated with `hack/regen-mocksql-users.sh`, which reproduces upstream's committed files byte for byte when run with `ONLY_UPSTREAM=1` | yes |
| `0002-mockgcp-listen-address-env.patch` | `MOCKGCP_LISTEN_ADDRESS` overrides mockgcp's `127.0.0.2:0` (absent on macOS) | yes |

## Other sources consulted, not copied

| Source | Commit | Used for |
| --- | --- | --- |
| `github.com/googleapis/googleapis` | `731d7f2ab6e4e2ea15030c95039e2cb66174d4fb` (mockgcp's pin, `mockgcp/git.versions`) | the protos mockgcp is generated from |
| `github.com/googleapis/googleapis` | `bf54c1031efddde9cdfee0f89254315d0116a5f9` (`master`, 2026-11-07) | field numbers of `User.database_roles` (15) and `SqlUsersUpdateRequest.database_roles` / `revoke_existing_roles` (5, 6) |
| `github.com/hashicorp/terraform-provider-google-beta` | `42eb358f50f54450f5433a359517c42edf10e63f` (`main`, 2026-11-07), `google-beta/services/sql/resource_sql_user.go` | the behaviour of `database_roles` (MPL-2.0; read, not copied) |
| Config Connector PR #7148 "feat(sql): migrate SQLUser from Terraform to Direct controller" | head `eb5d176ab176d4afa5b7d1878d8f035281a7a072`, open, not approved, last updated 2026-06-04 | upstream's own direct `SQLUser`, with `databaseRoles` written but commented out (`/*NOTYET*/`); see `docs/plan.md` |

## Updating the pin

1. Change the "Commit" row (and date, release) above.
2. `hack/upstream.sh`. If a patch no longer applies, rebase it (or drop it
   if upstream took it) and regenerate it from `git diff` in the checkout.
3. `hack/check-go-mod.sh`; copy any changed `replace` pins from the
   checkout's `go.mod` into ours. Then `go mod tidy`.
4. Re-read the upstream files in the table above at the new commit and
   carry over their changes (`git diff <old>..<new> -- <path>` in a full
   clone); keep our changes as small as before.
5. `make check`.
6. If upstream now has a kind or field we fill, follow "Retiring a gap
   kind" in `docs/plan.md`.
