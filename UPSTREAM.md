# Upstream

config-connector-extensions builds on Config Connector at one commit of `master`.
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
| `apis/sqladmin/v1alpha1/groupversion_info.go` | `apis/sqladmin/v1alpha1/groupversion_info.go` | group `sqladmin.gcp.hfl.dev` |
| `apis/sqladmin/v1alpha1/sqladminbackup_types.go` | `apis/sqladmin/v1alpha1/sqladminbackup_types.go` | group, labels, short names |
| `apis/sqladmin/v1alpha1/sqladminbackup_identity.go` | `apis/sqladmin/v1alpha1/sqladminbackup_identity.go`, plus PR #12553's change to it | the backup ID is Cloud SQL's, so it comes from `status.externalRef` (as in the PR) |
| `pkg/controller/direct/sqladmin/sqladminbackup_controller.go` | PR #12553 (head `9611a6de`) `pkg/controller/direct/sqladmin/sqladminbackup_controller.go` | the sqladmin v1beta4 REST client instead of `cloud.google.com/go/sql` (not in the pin's go.mod; adding it moves ~65 shared modules); Update compares the fields the spec sets; Find takes a run with status `DELETED` as gone (seen live) |
| `pkg/controller/direct/sqladmin/sqladminbackup_mapping.go` | PR #12553 `mapper.generated.go` and `mapper.go` | the same fields, mapped to the REST types |
| `apis/secretmanager/v1beta1/groupversion_info.go` | `apis/secretmanager/v1beta1/groupversion_info.go` | group `secretmanager.gcp.hfl.dev` |
| `apis/secretmanager/v1beta1/secretmanagersecret_types.go` | `apis/secretmanager/v1beta1/secretmanagersecret_types.go` | group, labels, short names; `spec.location` from PR #6651 (head `9aab204d`) |
| `apis/secretmanager/v1beta1/secretmanagersecret_identity.go`, `secretmanagersecret_reference.go` | the same files | PR #6651: a secret's parent carries its location; regional external references parse |
| `apis/secretmanager/v1beta1/secretmanagersecretversion_types.go` | the same file | group, labels, short names; `secretRef` typed `SecretManagerSecretRef`, not its alias (controller-gen v0.16.5 panics on aliases under Go 1.27; same schema) |
| `apis/secretmanager/v1beta1/secretmanagersecretversion_identity.go`, `secretmanagersecretversion_reference.go` | the same files | PR #6651: a version's parent is its secret's identity; regional version references parse |
| `apis/secretmanager/v1beta1/types.generated.go` | the same file | no |
| `pkg/controller/direct/secretmanager/secretmanagersecret_controller.go` | the same file | PR #6651: a regional secret uses `secretmanager.<location>.rep.googleapis.com` |
| `pkg/controller/direct/secretmanager/secretmanagersecretversion_controller.go` | the same file | PR #6651: the same endpoint for its versions; ours: an unset `spec.enabled` means true (upstream dereferences it unchecked; there the direct controller is opt-in) |
| `pkg/controller/direct/secretmanager/mapper.generated.go`, `secret_mapping.go` | the same files | our KRM types |
| `apis/artifactregistry/v1beta1/groupversion_info.go` | `apis/artifactregistry/v1beta1/groupversion_info.go` | group `artifactregistry.gcp.hfl.dev` |
| `apis/artifactregistry/v1beta1/artifactregistryrepository_types.go` | the same file, plus PR #13517 (head `eb2774ce`) | group, labels, short names; `customRepository` under `remoteRepositoryConfig.{docker,maven,npm,python}Repository` from the PR; written here: the `upstreamCredentials` and `disableUpstreamValidation` fields (their types are ours, `upstreamcredentials_types.go`) |
| `apis/artifactregistry/v1beta1/artifactregistryrepository_identity.go` | the same file | no |
| `apis/artifactregistry/v1beta1/artifactregistryrepository_reference.go` | the same file | the reference names our kind (`ArtifactRegistryRepositoryGVK` is in `artifactregistry.gcp.hfl.dev`) |
| `apis/artifactregistry/v1beta1/types.generated.go` | the same file, plus PR #13517 | no (the PR changes only comments) |
| `pkg/controller/direct/artifactregistry/artifactregistryrepository_controller.go` | the same file | our KRM types; written here: resolve the upstream password's secret version, ignore the input-only `disableUpstreamValidation`, narrow the update mask for credentials (`upstreamcredentials_mapping.go`, ours) |
| `pkg/controller/direct/artifactregistry/mapper.go` | the same file | our KRM types |
| `pkg/controller/direct/artifactregistry/mapper.generated.go` | the same file, plus PR #13517 | only the `ArtifactRegistryRepository` (v1beta1) functions; our KRM types; the PR's `customRepository` mapping; written here: the remote config maps `upstreamCredentials` and `disableUpstreamValidation` |
| `pkg/controller/direct/artifactregistry/artifactregistryrepository_fuzzer_test.go` | `pkg/controller/direct/artifactregistry/artifactregistryrepository_fuzzer.go`, plus PR #13517 | a test file, run by its own test function instead of registered with Config Connector's fuzz runner |
| `apis/artifactregistry/v1alpha1/groupversion_info.go` | `apis/artifactregistry/v1alpha1/groupversion_info.go` | group `artifactregistry.gcp.hfl.dev`; the version's only kind, `ArtifactRegistryTag`, is written here |
| `apis/storage/v1alpha1/groupversion_info.go` | `apis/storage/v1alpha1/groupversion_info.go` | group `storage.gcp.hfl.dev`, with the group name marker upstream keeps in `doc.go`; its kind, `StorageBucketObject`, is written here |
| `apis/compute/v1alpha1/groupversion_info.go` | `apis/compute/v1alpha1/groupversion_info.go` | group `compute.gcp.hfl.dev`; its kinds `ComputeRegionNetworkEndpoint`, `ComputeRouterRoutePolicy` and `ComputeProjectMetadataItem` are written here |
| `apis/compute/v1alpha1/computeregionnetworkfirewallpolicy_types.go` | `apis/compute/v1beta1/computenetworkfirewallpolicy_types.go` | `ComputeNetworkFirewallPolicy` becomes `ComputeRegionNetworkFirewallPolicy`; group, labels, short names; projectRef typed as our other kinds'; written here: `spec.region`, `status.regionNetworkFirewallPolicyId` (Terraform's name) |
| `apis/compute/v1alpha1/computeregionnetworkfirewallpolicy_identity.go` | `apis/compute/v1beta1/computenetworkfirewallpolicy_identity.go` | `projects/{project}/regions/{region}/firewallPolicies/{firewallPolicy}`; the region from `spec.region` |
| `pkg/controller/direct/compute/regionnetworkfirewallpolicy_controller.go` | `pkg/controller/direct/compute/computenetworkfirewallpolicy_controller.go`, `computenetworkfirewallpolicy_mapper.go` | the regional client and the region in each request; our KRM types; written here: the spec mapping (description only) and an unset description sent as "" on update |
| `apis/iam/v1alpha1/groupversion_info.go` | `apis/iam/v1alpha1/groupversion_info.go` | group `iam.gcp.hfl.dev` |
| `apis/logging/v1alpha1/groupversion_info.go` | `apis/logging/v1alpha1/groupversion_info.go` | group `logging.gcp.hfl.dev`; its kind, `LoggingSettings`, is written here |
| `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_types.go` | PR #14209 (head `a5e8f5a9`) `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_types.go` | group, labels, short names; written here: `spec.workloadIdentityPoolRef` (the PR has no field for the pool) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_identity.go` | PR #14209 `iamworkloadidentitypoolnamespace_identity.generated.go` | written here: the pool from `workloadIdentityPoolRef` (the PR's is a stub that always fails) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_reference.go` | PR #14209 `iamworkloadidentitypoolnamespace_reference.generated.go` | the reference names our kind |
| `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_types.go` | PR #14418 (head `b080f5dc`) `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_types.go` | group, labels, short names; written here: `spec.workloadIdentityPoolNamespaceRef` and `spec.attestationRules` (the PR has neither) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_identity.go` | PR #14418 `iamworkloadidentitypoolmanagedidentity_identity.generated.go` | written here: the pool and namespace from `workloadIdentityPoolNamespaceRef` (the PR's are stubs that always fail) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_reference.go` | PR #14418 `iamworkloadidentitypoolmanagedidentity_reference.generated.go` | the reference names our kind |
| `apis/iam/v1alpha1/types.generated.go` | PR #14209 `apis/iam/v1alpha1/types.generated.go` | only `OwnerServiceObservedState`, the type the PR adds for this kind |
| `apis/run/v1alpha1/groupversion_info.go` | `apis/run/v1alpha1/groupversion_info.go` | group `run.gcp.hfl.dev` |
| `apis/run/v1alpha1/runservice_types.go` | `apis/run/v1alpha1/runworkerpool_types.go` | `RunWorkerPool` becomes `RunService` (Cloud Run v2 `Service`), its template `RevisionTemplate`; no instance splits; group, labels, short names; secret references typed without their aliases; written here: the service's `ingress`, `invokerIamDisabled`, `defaultUriDisabled`, `scaling`, the template's scaling, timeout, concurrency, execution environment, session affinity, health check and GPU zonal redundancy fields, the URIs in status |
| `apis/run/v1alpha1/servicescaling_types.go` | written here | `ServiceScaling`, `RevisionScaling` from the Cloud Run v2 API |
| `apis/run/v1alpha1/runservice_identity.go`, `runservice_reference.go` | `apis/run/v1alpha1/runworkerpool_identity.go`, `runworkerpool_reference.go` | a service (`.../services/{service}`) instead of a worker pool; the reference names our kind |
| `apis/run/v1alpha1/types.generated.go` | the same file | without `InstanceSplit`, `InstanceSplitStatus`, `WorkerPoolScaling` |
| `pkg/controller/direct/run/runservice_controller.go` | `pkg/controller/direct/run/runworkerpool_controller.go` | the Services client instead of WorkerPools; no instance splits; written here: a service's server defaults taken from what was read |
| `pkg/controller/direct/run/mapper.generated.go` | the same file | only the v1alpha1 functions and three Condition reason helpers; our KRM types; the worker pool's template mapping for a `RevisionTemplate`, plus (written here) the fields a service template has |
| `pkg/controller/direct/run/runservice_mapper.go` | `pkg/controller/direct/run/runworkerpool_mapper.go` | the spec and observed state for a `Service`; written here: the spec's `FromProto`, the service fields, the scaling mappings |
| `pkg/controller/direct/run/resolverefs.go` | the same file | `ResolveRunServiceRefs` only; no v1beta1 cases |
| `apis/iam/v1alpha1/iamrolebinding_types.go`, `iamrolebinding_identity.go`, `iamrolemember_types.go`, `iamrolemember_identity.go`, `iamresourceref_types.go`, `pkg/iampolicy/`, `pkg/controller/direct/iam/rolebinding_controller.go`, `rolemember_controller.go`, `policy_common.go` | written here | `IAMRoleBinding` and `IAMRoleMember`: no Config Connector code (its IAM kinds run on the Terraform provider, which this manager does not load); `membersFrom` reuses `apis/iam/iamrefs.IAMServiceAccountRef`; Terraform's `google_*_iam_binding` read, not copied |
| `hack/boilerplate.go.txt` | `hack/boilerplate.go.txt` | an SPDX line after Google's header, so generated files carry it |
| `hack/generate.sh` | `dev/tasks/generate-crds` (the controller-gen and crd-tools steps) | our paths; no version annotation, no alpha backport |
| `hack/regen-mocksql-users.sh` | `mockgcp/Makefile` (`gen-proto`), `mockgcp/apply-proto-patches.sh`, `mockgcp/fixup-third-party.sh` | one proto file only; our extra fields |

The CRD schema in `config/crds/` has the same property paths as upstream's
`config/crds/resources/apiextensions.k8s.io_v1_customresourcedefinition_sqlusers.sql.cnrm.cloud.google.com.yaml`
plus `spec.databaseRoles` and `status.appliedDatabaseRoles` (checked by
flattening both).
`config/crds/artifactregistry.gcp.hfl.dev_artifactregistryrepositories.yaml`
has the same property paths as PR #13517's
`apiextensions.k8s.io_v1_customresourcedefinition_artifactregistryrepositories.artifactregistry.cnrm.cloud.google.com.yaml`,
which is the pin's plus the four `customRepository.uri` fields (checked by
flattening all three, 2026-11-08).

## Patches to the checkout

| Patch | What | Upstream candidate |
| --- | --- | --- |
| `0001-mockgcp-sql-users-database-roles.patch` | mockgcp's Cloud SQL `User` gains `database_roles`, and `users.update` gains `database_roles` / `revoke_existing_roles` (googleapis master's field numbers); regenerated with `hack/regen-mocksql-users.sh`, which reproduces upstream's committed files byte for byte when run with `ONLY_UPSTREAM=1` | yes |
| `0002-mockgcp-listen-address-env.patch` | `MOCKGCP_LISTEN_ADDRESS` overrides mockgcp's `127.0.0.2:0` (absent on macOS) | yes |
| `0003-mockgcp-sql-backup-runs.patch` | mockgcp serves Cloud SQL `backupRuns` insert, get and delete: PR #12553's `mocksql/sqlbackup.go` and its registration, unchanged | it is upstream's; drop it when the PR merges and the pin passes it |
| `0004-mockgcp-secretmanager-regional.patch` | mockgcp serves regional secrets (PR #6651's `secrets.go` and `service.go` change, applied by hand) and, ours, versions under a regional secret | yes, with the PR |
| `0005-mockgcp-artifactregistry-upstream-credentials.patch` | written here: mockgcp's Artifact Registry accepts the update mask paths for a remote repository's upstream username and password version, and does not store the input-only `disableUpstreamValidation` | yes |
| `0006-mockgcp-iam-workload-identity-pool-namespaces.patch` | mockgcp serves workload identity pool namespaces: PR #14209's `mockgcp/apis/google/iam/v1/workload_identity_pool_namespace.proto` as package `mockgcp.iam.v1`, its Go generated by `hack/regen-mockiam-workload-identity.sh` with mockgcp's toolchain (reproducible byte for byte), and written here: `mockiam/workloadidentitypoolnamespaces.go` (get, create, patch, soft delete, undelete, list, as long-running operations) and its registration | yes, with the PR |
| `0007-mockgcp-iam-workload-identity-pool-managed-identities.patch` | on top of 0006: mockgcp serves managed identities: PR #14418's proto as package `mockgcp.iam.v1`, without the operation metadata 0006 already defines, and, written here, with `setAttestationRules` and `listAttestationRules`; its Go generated by the same script; and, written here, `mockiam/workloadidentitypoolmanagedidentities.go` (get, create in an active namespace, patch, soft delete, undelete, list, attestation rules) | yes, with the PR |
| `0008-mockgcp-run-service-update-mask.patch` | mockgcp's Cloud Run `services.patch` changes exactly the masked top-level fields (written here), and keeps `invoker_iam_disabled` without a mask (PR #14417's line, head `9297537a`) | yes |
| `0009-mockgcp-artifactregistry-tags.patch` | written here: mockgcp serves Artifact Registry package tags (get, create, patch `version`, delete) in an existing repository | yes |
| `0010-mockgcp-iam-workload-identity-pool-provider-keys.patch` | on top of 0006 and 0007, written here: mockgcp serves workload identity pool provider keys (get, create with a placeholder PEM, soft delete, undelete, list), from a proto written from the IAM v1 discovery document and generated by `hack/regen-mockiam-workload-identity.sh` | yes |
| `0011-mockgcp-compute-region-network-endpoints.patch` | written here: mockgcp attaches, detaches and lists a regional network endpoint group's endpoints, adapted from its zonal ones | yes |
| `0012-mockgcp-compute-router-route-policies.patch` | written here: mockgcp serves a Cloud Router's route policies (get, update, patch, delete, list) | yes |
| `0013-mockgcp-storage-object-media.patch` | written here: mockgcp stores Cloud Storage objects with their content (media and multipart uploads, `alt=media` reads, get, delete), and its request prefilter leaves upload bodies, which are not JSON, alone; its objects were stubs | yes |
| `0014-mockgcp-compute-region-network-firewall-policies.patch` | adapted from mockgcp's global network firewall policies: regional policies (get, list, insert, patch, delete) | yes |
| `0015-mockgcp-sql-backup-runs-deleted.patch` | written here, on 0003: `backupRuns.delete` keeps the run with status `DELETED`, as real Cloud SQL does (a live run, `docs/live-runs.md`) | yes, with PR #12553 |
| `0016-mockgcp-artifactregistry-remote-source-immutable.patch` | written here, on 0005: an update of `remoteRepositoryConfig` fails with the API's own error, "Remote source configuartion cannot be updated" (a live run, `docs/live-runs.md`), not mockgcp's "not supported" | yes |
| `0017-mockgcp-logging-settings.patch` | written here: mockgcp serves Log Router settings (get, update with a mask) for organizations and folders | yes |
| `0018-mockgcp-storage-iam-policy.patch` | written here: mockgcp serves Cloud Storage's `GET`/`PUT /storage/v1/b/{bucket}/iam` (and a managed folder's) from the store it keeps for the `:getIamPolicy` verbs | yes |
| `0019-mockgcp-compute-iam-policy.patch` | written here, on 0018: mockgcp serves Compute Engine's `GET {resource}/getIamPolicy` and `POST {resource}/setIamPolicy` from the store it keeps for the `:getIamPolicy` verbs | yes |

## Other sources consulted, not copied

| Source | Commit | Used for |
| --- | --- | --- |
| `github.com/googleapis/googleapis` | `731d7f2ab6e4e2ea15030c95039e2cb66174d4fb` (mockgcp's pin, `mockgcp/git.versions`) | the protos mockgcp is generated from |
| `github.com/googleapis/googleapis` | `bf54c1031efddde9cdfee0f89254315d0116a5f9` (`master`, 2026-11-07) | field numbers of `User.database_roles` (15) and `SqlUsersUpdateRequest.database_roles` / `revoke_existing_roles` (5, 6) |
| `github.com/hashicorp/terraform-provider-google-beta` | `42eb358f50f54450f5433a359517c42edf10e63f` (`main`, 2026-11-07), `google-beta/services/sql/resource_sql_user.go` | the behaviour of `database_roles` (MPL-2.0; read, not copied) |

## Watched upstream change

Upstream's own direct `SQLUser` is an open pull request. It is not copied:
at the head below it declares `databaseRoles` but leaves the field, its
diff and its mapping commented out (`/*NOTYET*/`), so it does not do what
we need. `hack/upstream-pr.sh` (`task upstream-pr`) fetches the PR's head
by `refs/pull/<n>/head`, says whether it moved, and whether `databaseRoles`
is live yet; when it is, follow "Switching to PR #7148" in `docs/design.md`.

| | |
| --- | --- |
| Watched PR | #7148 "feat(sql): migrate SQLUser from Terraform to Direct controller" |
| PR head | `eb5d176ab176d4afa5b7d1878d8f035281a7a072` |
| Read on | 2026-11-07 (open, review required, last updated 2026-06-04) |
| Verdict | not adequate: `databaseRoles` commented out |

## Updating the pin

1. Change the "Commit" row (and date, release) above.
2. `hack/upstream.sh`. If a patch no longer applies, rebase it (or drop it
   if upstream took it) and regenerate it from `git diff` in the checkout.
3. `hack/check-go-mod.sh`; copy any changed `replace` pins from the
   checkout's `go.mod` into ours. Then `go mod tidy`.
4. Re-read the upstream files in the table above at the new commit and
   carry over their changes (`git diff <old>..<new> -- <path>` in a full
   clone); keep our changes as small as before.
5. `task check`.
6. If upstream now has a kind or field we fill, follow "Retiring a gap
   kind" in `docs/design.md`.

## Copied from open pull requests

Code copied from pull requests that have not merged. `hack/upstream-pr.sh`
reads this table and says, for each, whether its head moved since the row
was written and whether it merged. When one moves, re-read its diff for the
copied files; when it merges and the pin passes it, retire our copy
(`docs/design.md` section 3.4) and drop its mockgcp patch.

| PR | Head | Read on | Copied for |
| --- | --- | --- | --- |
| #12553 | `9611a6de02e08aca9a624affaf59bc104d1cb88f` | 2026-11-07 (open, review required, last updated 2026-09-04) | `SQLAdminBackup`, patch 0003 |
| #6651 | `9aab204dd9d87716863b966c724b7a12664fd85f` | 2026-11-07 (open, review required, last updated 2026-03-17) | regional `SecretManagerSecret` / `SecretManagerSecretVersion`, patch 0004 |
| #14417 | `9297537aa0479a6457f736f96101190cba876fcb` | 2026-11-09 (open, review required, no reviews) | one mockgcp line in patch 0008 (`invokerIamDisabled` kept on update); the field itself is written here on our `RunService` |
| #14418 | `b080f5dcfae5e5be5acd2361a7afd5c60e33568f` | 2026-11-09 (open, review required, no reviews) | `IAMWorkloadIdentityPoolManagedIdentity` types and identity; the mockgcp proto in patch 0007 |
| #14209 | `a5e8f5a9e3648a13d3346cfa8cf07cf26e64ec09` | 2026-11-09 (open, changes requested on its CI files, not on the types) | `IAMWorkloadIdentityPoolNamespace` types and identity; the mockgcp proto in patch 0006 |
| #13517 | `eb2774cef2776e8a1dc2f766d6cf491f0931f8b7` | 2026-11-08 (open, review required, no reviews, paused by upstream's bot for inactivity on 2026-10-13) | `ArtifactRegistryRepository` with `customRepository`; no mockgcp patch needed |
