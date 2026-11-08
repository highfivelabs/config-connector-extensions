# gaps-gcp plan

Status, 2026-11-07 (evening): the repository exists locally with four
kinds: `SQLUser` with `spec.databaseRoles`, `SQLAdminBackup`, and regional
`SecretManagerSecret` and `SecretManagerSecretVersion` (section 6.6). Each
passes its end-to-end test against mockgcp. An operator runs the managers
as Config Connector's own service accounts, and a kind test proves it in
cluster and namespaced mode against an in-cluster fake cloud (section 9).
Images are built locally with ko; there is no registry, release or remote.
This document says how it is built, why, how it stays in step with Config
Connector, how it is tested, which gaps come next, and what is left.

It is grounded in the builder's analysis "Provider controller internals:
Config Connector, ACK, ASO" (2026-10-31, read at Config Connector 1.160.0)
and the gap scan `krm-gapscan-results-2026-10-19`.

## 1. What the analysis settles

- **One repository per provider.** Each provider's gap kinds are built with
  that provider's generator against that provider's model input, on three
  unrelated cadences. So `gaps-gcp`, `gaps-aws`, `gaps-azure`, each
  versioned on its own and pinned in a Bundt provider bundle beside the
  upstream controller it extends.
- **Our kinds live in our groups.** Config Connector's registration
  controller skips any `managed-by-kcc` CRD missing from its static table,
  and its webhooks and ClusterRoles are generated from the in-tree set. A
  gap kind is therefore `<kind>.<service>.gcp.hfl.dev`, reconciled by our
  process, never by theirs.
- **For GCP, build on the direct family.** Config Connector is three
  reconciler families in one binary: Terraform (vendored fork of
  `terraform-provider-google-beta`, MPL-2.0), DCL, and "direct"
  (`directbase`: a `Model → Adapter{Find, Create, Update, Delete, Export}`
  interface over the Google Cloud Go clients). Upstream says everything is
  migrating to direct; at our pin the static table has 449 direct, 225
  Terraform, 65 DCL and 4 IAM kinds of 743 (the analysis counted 441 of
  735 at 1.160.0). The analysis's recommendation: types, mapper, identity
  and reference from `dev/tools/controllerbuilder`, a hand-written
  controller (~265 lines) on `directbase`, and a mockgcp fixture.
- **GCP is where gaps close fastest upstream**, so `gaps-gcp` should expect
  to retire kinds as often as it adds them; a kind moves from our group to
  theirs when the catalog's per-kind conformance says it can.
- **Hermetic tests exist upstream**: mockgcp (a fake GCP, separate Go
  module, in the Config Connector repository) plus golden `_http.log`
  files. That is the "fake cloud (ideal)" the builder asked for, and it is
  already maintained by Google.
- **The knobs** (deletion policy, adoption, identity per namespace,
  reconcile cadence) are Config Connector's annotations; because we run
  Config Connector's reconciler, our kinds honour the same ones
  (`cnrm.cloud.google.com/deletion-policy: abandon`, `spec.resourceID`
  acquisition, `.../project-id`, `.../reconcile-interval-in-seconds`), so
  Bundt's projection writes one set of annotations for both.

## 2. Architecture chosen

### 2.1 Config Connector as a pinned module, not copied

Options considered for the shared machinery (reconciler, references,
identity, config, mockgcp):

| Option | Verdict |
| --- | --- |
| A. Use Config Connector as a Go module from a checkout at a pinned `master` commit, through `replace` | **Chosen.** The framework stays byte-identical to upstream; syncing is moving the pin. |
| B. Copy `directbase` and what it needs into this repository | Rejected: `go list -deps ./pkg/controller/direct/directbase` pulls 91 Config Connector packages, and through `krmtotf`/`servicemappings` the Terraform fork and DCL. Copying that is a fork, not a copy. |
| C. Copy the Terraform-family `SQLUser` (CRD from the TF schema, `krmtotf`, the provider fork) | Rejected: it needs the whole Terraform family, and the code that would change (`resource_sql_user.go`) is MPL-2.0 in a fork with no patch series, which is the analysis's main asterisk on Config Connector. Upstream is moving off it anyway. |

Option A has a cost: Config Connector's `go.mod` requires the Terraform
fork at `v0.0.0-00010101000000-000000000000` and resolves it, DCL and
mockgcp from its own tree by `replace`. Replace directives do not apply to
dependents, so ours repeats them pointing into the checkout, plus its
`k8s.io/*` version pins (`hack/check-go-mod.sh` fails if any of its pins
is missing from ours). The checkout is about 650 MB and is fetched, not
committed (`hack/upstream.sh`). The binary links what `directbase` links:
`bin/manager` is 119 MB.

### 2.2 Our own manager beside Config Connector

`cmd/manager` is a small controller-runtime manager. For each gap kind,
`pkg/controller` adds Config Connector's direct reconciler
(`directbase.AddController`) and its deletion defender
(`deletiondefender.Add`), both unchanged. The deletion defender is not
optional: the direct reconciler puts the defender's finalizer on every
object and will not delete until the defender removes it; the defender
flips objects to abandon when their CRD is being deleted, so uninstalling
gaps-gcp never deletes cloud resources.

The reconciler reads `ConfigConnector` / `ConfigConnectorContext` (actuation
mode, namespaced vs cluster mode), so those CRDs must exist on the cluster,
which they do wherever Config Connector is installed. In namespaced mode it
requires a `ConfigConnectorContext` in the object's namespace, as Config
Connector does.

Identity: the manager authenticates with Application Default Credentials.
In a cluster it runs as Config Connector's own Kubernetes service account
for the scope (section 9), so Workload Identity gives it the Google service
account Config Connector has there, per namespace in namespaced mode.

### 2.3 Per-kind code: copy the nearest upstream code

The rule: where Config Connector has the resource on `master`, copy its
code and change as little as possible. For `SQLUser`:

- On `master` `SQLUser` is a **Terraform-family** kind
  (`static_config.go`: `DefaultController: Terraform`, only Terraform
  supported). There is no direct `SQLUser` controller to copy.
- **Types**: copied from Config Connector's generated Go types for the
  Terraform-based CRD (`pkg/clients/generated/apis/sql/v1beta1/sqluser_types.go`),
  so the schema is the one users of `sql.cnrm.cloud.google.com/SQLUser`
  already write. `instanceRef` and `password` use the shared types Config
  Connector's direct controllers use for the same shapes
  (`refs/v1beta1.SQLInstanceRef`, as `SQLSSLCert` did when it went direct;
  `refs/v1beta1/secret.Legacy`, as `SQLInstance` does). Flattened, the CRD
  has upstream's property paths plus `spec.databaseRoles` and
  `status.appliedDatabaseRoles`, and nothing else differs.
- **Controller and identity**: copied from the direct `SQLSSLCert`
  controller, the sibling Cloud SQL sub-resource that already moved from
  Terraform to direct on the same `sqladmin/v1beta4` client, changed for
  users.
- **Behaviour** of `databaseRoles` follows Terraform's `google_sql_user`
  (read, not copied): roles go in the insert body; when they change, they
  go to `users.update` as `databaseRoles` with `revokeExistingRoles=true`.
- **controllerbuilder is not used for this kind.** Config Connector's
  Cloud SQL direct controllers talk to the REST client
  (`google.golang.org/api/sqladmin/v1beta4`) with hand-written conversion,
  so copying them means not generating. For kinds whose upstream siblings
  are generated from protos, use `controllerbuilder` as the analysis
  recommends (section 3.3).

Upstream PR #7148 (open since 2026-03-24, not approved, last updated
2026-06-04, head `eb5d176a`) migrates `SQLUser` to a direct controller and
writes `databaseRoles` but leaves it commented out (`/*NOTYET*/`) in the
types, the diff and the mapping. The builder said to use it if it does
what we need (2026-11-07); it does not yet, so ours stays, and
`hack/upstream-pr.sh` watches it (UPSTREAM.md, "Watched upstream change").
It is the shape upstream is likely to take, and the kind we would migrate
to (section 6). Two
differences to settle if it merges: it compares `spec.databaseRoles` with
the roles the API returns on `users.get`, where we compare with what we
last applied (Terraform does not read roles back either, and we have not
verified that the API returns them); and its CRD will be in
`sql.cnrm.cloud.google.com`, so moving means re-creating objects in the
other group with `cnrm.cloud.google.com/deletion-policy: abandon` and
acquiring by `resourceID`. 

### Switching to PR #7148

When `hack/upstream-pr.sh` reports `databaseRoles` live:

1. Record the new head in UPSTREAM.md ("PR head", "Read on").
2. Fetch it into the checkout (the script does) and copy the PR's
   `sqluser_types.go`, `sqluser_controller.go`, `sqluser_mappings.go` and
   `sqluser_resolverefs.go` over ours with the group change only, keeping
   Google's headers and a "Modified by" note; record each in "What is
   copied" with the PR head as the source.
3. Re-apply our differences as small edits: revoke on removal
   (`revokeExistingRoles`) if the PR adds without revoking, and
   `status.appliedDatabaseRoles` if the API is shown not to return roles.
4. `make check`; the mockgcp test must still pass unchanged.

Refreshing later is the same steps against the newer head; once the PR
merges, retire the kind instead ("Retiring a gap kind").

## 3. Keeping in sync with upstream

### 3.1 Pin, patches, record

- `UPSTREAM.md` holds the pinned commit (one row the scripts read), the
  nearest release, and a table of every copied file with its source path
  and what changed. Each copied file also says so in its header.
- `hack/upstream.sh` fetches exactly that commit (shallow) into
  `third_party/k8s-config-connector` and applies
  `patches/k8s-config-connector/*.patch` in order;
  `hack/upstream.sh --check` fails unless the checkout is the pin plus
  exactly those patches. This is the patch series the analysis found
  missing from Config Connector's own Terraform fork.
- Every patch says whether it should go upstream. Both current patches
  should (they fix mockgcp).

### 3.2 Moving the pin

The procedure is in `UPSTREAM.md`. Cadence: when a gap closes upstream,
when a copied file changes upstream, or before a Bundt provider bundle is
cut, at least monthly while upstream ships every two to three weeks
(v1.156 to v1.160 took two months). A bundle pins the Config Connector
version, the gaps-gcp version and the shared model commit together, and
gaps-gcp should pin a `master` commit at or after the Config Connector
release in the same bundle.

### 3.3 Generating CRDs and code

- `hack/generate.sh` is Config Connector's `dev/tasks/generate-crds` for
  our tree: controller-gen v0.16.5 (`object` and `crd`), then Config
  Connector's own `scripts/add-validation-to-crds` and `scripts/crd-tools`
  run from the pinned checkout. `make check` fails if its output differs
  from what is committed.
- For proto-backed kinds, run Config Connector's `controllerbuilder`
  (`generate-types`, `generate-mapper`) from the checkout against the
  googleapis commit in its `apis/git.versions`, with `--api-version
  <service>.gcp.hfl.dev/v1beta1`, then hand-edit types and controller as
  upstream does. Not exercised yet.

### 3.4 Retiring a gap kind

When upstream `master` gains the kind or field: copy nothing new; mark our
kind deprecated in the catalog; the provider bundle that carries both
versions carries the migration (re-create in their group with
`deletion-policy: abandon` on ours and acquisition by `resourceID` on
theirs; delete ours with abandon); after the support window, remove the
kind here.

## 4. Testing with a fake cloud

- **Fake cloud: mockgcp, from the pinned checkout.** mockgcp is Config
  Connector's in-process fake of GCP's APIs (gRPC services behind
  grpc-gateway, in-memory storage, HTTP served through an
  `http.RoundTripper`). We import it as a module from the checkout, so it
  moves with the pin.
- **It lacked database roles.** Its Cloud SQL `User` proto comes from
  googleapis `731d7f2a` (mockgcp's pin), which predates `database_roles`.
  Patch 0001 adds `User.database_roles` and the update request's
  `database_roles` / `revoke_existing_roles` (googleapis `master` field
  numbers), regenerated with mockgcp's own toolchain (protoc 3.12.4,
  protoc-gen-go v1.28.1, protoc-gen-go-grpc v1.2.0, grpc-gateway v2.11.3)
  by `hack/regen-mocksql-users.sh`; and makes `users.update` apply roles
  as the API documents (body roles ignored; request roles added, or
  replacing the existing ones with `revoke_existing_roles`). With
  `ONLY_UPSTREAM=1` the script reproduces upstream's committed generated
  files byte for byte, which is how we know the patch is only our fields.
- **Patch 0002**: mockgcp listens on `127.0.0.2`, which macOS does not
  have; `MOCKGCP_LISTEN_ADDRESS` overrides it, and the Makefile sets it.
- **The test** (`pkg/controller/direct/sql/sqluser_mockgcp_test.go`) runs
  a real kube-apiserver and etcd (envtest, Kubernetes 1.33), our manager
  code (Config Connector's direct reconciler and deletion defender with
  our adapter), and mockgcp as Cloud SQL. It plays Bundt's case: bntd's
  service account as a `CLOUD_IAM_SERVICE_ACCOUNT` user with
  `cloudsqlsuperuser`, then adds a role, removes one, and deletes, reading
  the fake cloud after each step through the real `sqladmin` client.
- **Results** (2026-11-07, `make check`, macOS arm64, Go 1.27.1):

  ```text
  ok: third_party/k8s-config-connector is 672414b8191fc2d2c12fff53f6faccf5ae291eef plus 2 patch(es)
  ok: go.mod carries every replace pin of the pinned Config Connector
  --- PASS: TestSQLUserDatabaseRoles (6.01s)
      --- PASS: TestSQLUserDatabaseRoles/create_grants_the_roles (0.21s)
      --- PASS: TestSQLUserDatabaseRoles/adding_a_role_grants_it (0.22s)
      --- PASS: TestSQLUserDatabaseRoles/removing_a_role_revokes_it (0.22s)
      --- PASS: TestSQLUserDatabaseRoles/delete_removes_the_user (0.22s)
  ok  github.com/highfivelabs/gaps-gcp/pkg/controller/direct/sql
  ```

  The test was also run with `RevokeExistingRoles(false)` in the
  controller and failed as it should ("cloud roles = [cloudsqlsuperuser
  pg_read_all_data], want [pg_read_all_data]").
- **Next** (section 7): port Config Connector's own `SQLUser` fixtures
  (`pkg/test/resourcefixture/testdata/basic/sql/v1beta1/sqluser/`) to our
  group and run them through its golden harness, which records
  `_http.log`; one run against a real project to check mockgcp against
  the real API, in particular whether `users.get` returns `databaseRoles`.

## 5. The first resource: SQLUser

| | |
| --- | --- |
| Group, version, kind | `sql.gcp.hfl.dev/v1beta1` `SQLUser`, short names `hflsqluser`, `hflsqlusers` (not Config Connector's `gcpsqluser`, so `kubectl` is never ambiguous), categories `gcp`, `hfl` |
| CRD | `config/crds/sql.gcp.hfl.dev_sqlusers.yaml`, labelled `gcp.hfl.dev/gap: "true"`, annotated `gcp.hfl.dev/upstream-kind: sql.cnrm.cloud.google.com/SQLUser`; no `cnrm.cloud.google.com/*` labels, so Config Connector never claims it |
| Identity | `projects/{project}/instances/{instance}/users/{name}`; name from `spec.resourceID`, else `metadata.name`; project from `instanceRef` (external, or the referenced `SQLInstance` and its project) |
| Create | `users.insert` with name, host, type, password (from `spec.password` value or Secret), password policy, `databaseRoles` |
| Update | type immutable (error); password sent when `metadata.generation` is not yet observed; password policy when it differs; roles, with `revokeExistingRoles`, when `spec.databaseRoles` differs from `status.appliedDatabaseRoles` |
| Delete | `users.delete`, unless `deletion-policy: abandon` |
| Status | `conditions`, `observedGeneration`, `sqlServerUserDetails` (as upstream), `appliedDatabaseRoles` (new) |

Decisions (section 8 records the builder's answers):

- `databaseRoles` is the whole set of the user's roles, declaratively: a
  role removed from the list is revoked (`revokeExistingRoles=true`, as
  Terraform does). Kept by the builder, 2026-11-07. Out-of-band grants are
  not detected while the list is unchanged.
- What was applied is kept in `status.appliedDatabaseRoles`, because the
  API is not relied on to return a user's roles.
- A password from a Secret is re-sent when the spec generation changes,
  not when only the Secret changes (Config Connector tracks Secret
  versions with an annotation; not done here).

## 6. The gap scan, ranked for Bundt

Input: `krm-gapscan-results-2026-10-19`, GCP part, machine-generated from
Config Connector `cf12e119` (master, 2026-10-17), google-api-go-client
Discovery documents `abd2ce12` and `terraform-provider-google` docs
`bb3d14f7`. Read as data.

### 6.1 Headline numbers

| | Count |
| --- | --- |
| Config Connector kinds | 651 (262 GA, 5 beta, 384 alpha; 351 direct, 235 Terraform, 65 DCL) |
| Terraform resources | 1,499; 715 with a Config Connector kind, **784 without** |
| API resource types (create + get collections) | 1,197; **718 without** a Config Connector kind, 572 without Terraform either |
| Attribute rows compared (kind, Terraform resource) | 20,100 counted: 13,531 in KRM, 728 partial, **5,841 missing** |
| KRM-only spec fields (no Terraform counterpart) | 3,275 (not counted as gaps) |

So the gaps are of two kinds: whole resources (784 Terraform resources,
many of them IAM add-ons and singletons Config Connector covers
differently) and missing fields on kinds Config Connector already has
(5,841). For Bundt the second matters more: the kinds Bundt uses exist,
and the field gaps sit on them.

**SQLUser.databaseRoles is in the scan** as an attribute gap:
`google_sql_user.database_roles`, `in_krm=no`, `in_api=yes`
(`databaseRoles`). The other `SQLUser` gaps are `iam_email` (output),
`password_wo` and `password_wo_version` (Terraform write-only arguments).

### 6.2 By Bundt's first slices

Counts from `attributes_gcp.csv` / `resources_gcp.csv` (my filter excludes
rows noted "tf-only field", so totals differ slightly from the summary's).

| Slice | Missing fields on existing kinds (top) | Missing resources |
| --- | --- | --- |
| Cloud SQL | `SQLInstance` 123 (read-pool autoscale, performance capture, PSC options, final backup, Entra ID, server CA mode), `SQLUser` 4, `SQLDatabase` 0 | 4: `google_sql_source_representation_instance`, `google_sql_provision_script`, API-only `backupRuns`, `blueGreenDeployments` |
| GCS | `StorageBucket` 21 (`hierarchical_namespace`, `rpo`, encryption enforcement configs, lifecycle size conditions) | 18, mostly ACL and object resources (`google_storage_bucket_object`, `*_acl`, managed folder IAM) |
| IAM | `IAMWorkforcePoolProvider` 29, `IAMWorkloadIdentityPool` 27, `IAMWorkloadIdentityPoolProvider` 10; `IAMServiceAccount` none; `IAMPolicyMember` only Terraform's computed `etag` on each `*_iam_member` resource (58) | 21 (`google_iam_workload_identity_pool_*_iam_*`, OAuth clients, org-policy legacy resources) |
| Artifact Registry | `ArtifactRegistryRepository` 27 (remote repository upstream credentials and custom repositories, vulnerability scanning, `registry_uri`) | 5 (`google_artifact_registry_rule`, project config) |
| Networking | `ComputeURLMap` 180, `ComputeBackendService` 76+63, `ComputeSecurityPolicy` 72; for a basic VPC: `ComputeSubnetwork` 12, `ComputeNetwork` 9, `ComputeRouterNAT` 6, `ComputeAddress` 3, `ComputeFirewall` 3, `ServiceNetworkingConnection` 1 | 160, mostly regional variants, IAM add-ons and newer network-security products |
| GKE | `ContainerCluster` 219 (most of any kind), `ContainerNodePool` 30 | 26, mostly GKE on AWS/Azure/on-prem and hub IAM add-ons |
| Secrets / KMS | `KMSCryptoKey` 6, `SecretManagerSecret` 4 | 14 (`google_secret_manager_regional_secret` and its IAM) |

Proposed order for gaps-gcp, by what Bundt's slices need, not by count:

1. `SQLUser.databaseRoles` (done here).
2. Whatever the next Bundt slice blocks on, found by trying it: the counts
   above say where to look (`StorageBucket.hierarchicalNamespace`,
   `ArtifactRegistryRepository` remote-repository credentials,
   `ContainerCluster` / `ContainerNodePool` fields) but a field only
   becomes a gap kind when a Package needs it.
3. Missing resources only when a Package needs one and no existing kind
   covers it (many of the 784 are IAM add-ons Config Connector covers with
   its generic `IAMPolicyMember`, which the scan counts correctly only
   when the parent's reference doc says so).

### 6.3 Copy upstream master, or truly new

Per the builder's rule, a gap that already exists on Config Connector
`master` but is not released is copied, not written. Checked by comparing
the CRDs at our pin (`672414b8`, master) with release `v1.160.0`
(`c38b031c`), and each missing attribute's camelCase path against both
(a name heuristic):

- **Resources**: master has 19 kinds that v1.160.0 does not (AI Platform
  jobs and RAG, API Hub, App Hub services and workloads, Bigtable memory
  layers, Config deployments, NetApp KMS configs, a network-security rule,
  SaaS unit kinds, Web Security Scanner). None is in Bundt's slices.
- **Fields**: of the scan's missing attributes, 7 now exist on master but
  not in v1.160.0 (in Bundt's slices only
  `SQLInstance.settings.ipConfiguration.customSubjectAlternativeNames`);
  46 already exist in the release under the same name (the scan's matcher
  missed them, so they are not gaps); the rest, about 5,880, are new
  everywhere. "New" is an upper bound: Config Connector sometimes names a
  field differently from Terraform.
- **SQLUser.databaseRoles is truly new**: not in v1.160.0, not on master.
  `SQLUser` itself is on master (Terraform-based), so its schema is
  copied; the field is written by us, following Terraform's behaviour, and
  an unmerged upstream PR (#7148) already contains the same field.

A "master-only" gap needs no kind of ours at all if the next Config
Connector release is near: pin a bundle to that release instead.

### 6.4 Caveats from the scan's method

- Terraform attributes come from the provider's markdown docs, not its
  schema; hand-written docs (133) parse less reliably, and 34 nested
  blocks could not be attached.
- Discovery documents lack field behaviours, so output-only is a guess
  and required/immutable are unknown.
- Only APIs vendored in google-api-go-client are visible (no
  `memorystore`, `edgecontainer`, `modelarmor` and a few more).
- Terraform resources and API types are not one-to-one (IAM add-ons,
  singletons, virtual resources).
- Config Connector's Terraform kinds run a pinned fork that lags upstream
  Terraform, so "missing" mixes "never modelled" with "not yet
  re-vendored".
- Regional compute variants merged into one direct kind show as uncovered.
- Maturity for 169 unlabeled direct CRDs is inferred from the served
  version.

### 6.5 AWS and Azure, briefly (from their summaries)

- **AWS** (ACK controllers, terraform-provider-aws and awscc,
  aws-sdk-go-v2 models): 1,732 Terraform resources, 470 with an ACK kind
  and 1,262 without; 281 ACK kinds (66 GA); of 9,077 counted attributes,
  6,322 in KRM, 1,051 partial, 1,704 missing. Resource coverage is the
  big gap on AWS.
- **Azure** (ASO, azurerm, azure-rest-api-specs): 1,106 Terraform
  resources, 308 with an ASO kind and 798 without; 302 ASO kinds (ARM
  types known to ASO's config: 297); of 7,964 attribute rows, 2,570 in
  ASO, 2,141 partial, 3,253 missing. Field coverage is the weakest of the
  three on Azure.

### 6.6 Second batch: candidates and what was built (2026-11-07)

The builder asked to start from open pull requests and code on Config
Connector's `master`, then move to what the Google API has that Config
Connector does not. Read with read-only `gh` (843 open pull requests) and
the pinned checkout (`672414b8`); the scan above is three weeks older than
the pin and is now stale in places (`StorageManagedFolder` and
`ArtifactRegistryRule` have direct controllers on master: no gap kind,
pin a bundle on the release that ships them).

**(a) Upstream code exists, not released.** Ranked for Bundt:

| Rank | Candidate | Upstream | Why for Bundt | Done |
| --- | --- | --- | --- | --- |
| 1 | `SQLAdminBackup`, an on-demand Cloud SQL backup | types on master; controller and mockgcp in PR #12553 (open, review required) | a backup before a plan that can lose data (a major upgrade, a destructive migration); the rollback story for databases | built |
| 2 | `SecretManagerSecret.spec.location`, regional secrets and their versions | PR #6651 (and its duplicate #5730), on the direct controller | secrets in values at L2, kept in the Environment's region | built |
| 3 | `SQLUser` on the direct reconciler | PR #7148 | ours exists; switch when `databaseRoles` is live (section 2) | watched |
| 4 | `SQLInstance`: `retainBackupsOnDelete` (#14854), `diskEncryptionConfiguration` (#7227), PSC auto-connections (#14419), performance capture (#14431) | PRs on a direct kind of several thousand lines | an Instance's time limit or deletion must not take its backups (`retainBackupsOnDelete` first) | not built: open question 1 |
| 5 | `ContainerNodePool` `autoscaling.enabled` (#14182), kubelet sysctls (#14187) | PRs | self-service clusters (L2) | not built: open question 1 |
| 6 | `StorageBucket` `rpo` (#13713), encryption enforcement (#15145), direct controller (#11533) | PRs; the kind is Terraform-based on master | buckets in stacks | not built: open question 1 |
| 7 | `ArtifactRegistryRepository` custom remote repositories (#13517) | PR | pulling through a private upstream | not built |
| 8 | `RunService` `invokerIamDisabled` (#14417), GPUs (#7232) | PRs | Cloud Run stacks | not built |
| 9 | `IAMWorkloadIdentityPoolNamespace`, `...ManagedIdentity` (#14209, #14418), pool `mode` (#14227) | types only, no controller | federation for agents, later | not built |

**(b) In the Google API, in no Config Connector code or PR.** From the scan
(Bundt's services, `covered_by_krm=no`, `covered_by_tf=no`), each
unverified against the live API: Artifact Registry package tags and
repository attachments, Workload Identity pool provider keys, Cloud SQL
`blueGreenDeployments`, Storage HMAC keys (Terraform has
`google_storage_hmac_key`; the scan's match missed it), Cloud Run v1
`namespaces.*` (superseded by v2). On fields, the scan's ~5,880 missing
attributes on existing kinds (sections 6.2, 6.3). None was built in this
batch: section 6.2's rule is that a gap becomes a kind when a Package needs
it, and no Bundt Package needs one of these yet.

What was built is described in "What is copied" in `UPSTREAM.md`, and:

- `SQLAdminBackup` uses the sqladmin v1beta4 REST client instead of the
  PR's `cloud.google.com/go/sql`, which is not in the pin's `go.mod`:
  adding it moved about 65 shared modules (gRPC, genproto, otel and the
  `cloud.google.com/go/*` clients) off the versions Config Connector is
  built and tested with. Patch 0003 is the PR's mockgcp.
- The Secret Manager kinds are master's direct controllers with the PR's
  change. Two fixes of ours: an unset `spec.enabled` on a version means
  `true` (upstream dereferences it unchecked; there the direct controller
  is opt-in behind Terraform's, which defaults it), and the version's
  `secretRef` is typed without its alias (controller-gen v0.16.5 panics on
  aliases under Go 1.27). Patch 0004 is the PR's mockgcp plus versions
  under a regional secret, which the PR's mock never names. The PR's mock
  serves only `us-central1`'s regional endpoint; other regions need their
  hosts added.
- Tests (`make test`, macOS arm64, Go 1.27.1, 2026-11-07):

  ```text
  --- PASS: TestSQLAdminBackup (10.10s)
      create takes an on-demand backup; a resync takes no second backup;
      a backup cannot be changed; delete removes the backup; abandon keeps it
  --- PASS: TestRegionalSecret (7.20s)
      created in its region (every request at the regional endpoint);
      a version holds the Kubernetes Secret's value; cannot move region;
      delete removes the version and the secret
  --- PASS: TestGlobalSecretUnchanged (5.86s)
  ```

  The regional-endpoint check was confirmed to fail with the endpoint
  removed from the controller.

## 7. What is left

Before gaps-gcp can run on a real cluster:

1. Images are built locally by ko (`config/default` uses `ko://`
   references). A registry, signing and a release version scheme are not
   decided; neither is the Bundt provider entry (a Bundt `install`
   Package wrapping `config/default`).
2. Done: install manifests (`config/default`) and namespaced identity
   matching Config Connector's (section 9).
3. Proven only on kind: Workload Identity itself (section 9, "What was
   proven").
4. Secret change tracking for `spec.password` (Config Connector's
   observed-secret-versions), and watching referenced Secrets.
5. Validation webhook parity: Config Connector's webhooks enforce
   immutability (`host`, `type`, `resourceID`); ours relies on the
   controller erroring. Either CEL `x-kubernetes-validations` in the CRD
   or a webhook.

To raise confidence in the kind:

6. Port Config Connector's `SQLUser` fixtures (`sqluser`, `sqluser-full`)
   to our group and run its golden harness, recording `_http.log`.
7. One run against a real Cloud SQL Postgres instance, to confirm
   `databaseRoles` on insert and update, revoke with an empty list, and
   what `users.get` returns; then correct mockgcp if it differs.
8. Unit tests for `passwordPolicyEqual`, `sortedRoles` and identity
   parsing (Config Connector has an identity conformance helper,
   `apis/common/identity`).
9. Offer patches 0001 and 0002 upstream; comment on PR #7148 with the
   revoke semantics and the mockgcp change.

Housekeeping:

10. Done: `hack/upstream-pr.sh` also follows the pull requests copied
    from (`UPSTREAM.md`, "Copied from open pull requests").
11. The SQLUser test predates `pkg/test/mockenv` and keeps its own harness.
12. CI: `make check` on Linux (the 127.0.0.2 default then works without
    patch 0002, but the patch stays harmless).
13. A real GitHub repository, when the builder says so (none exists; no
    remote is configured).

## 8. Builder's answers (2026-11-07)

1. **Upstream's unmerged `SQLUser` (PR #7148)**: "If the unmerged change
   does what we need, sure use it. Find a way to ref it so we can update
   that part from upstream easily." It does not yet (`databaseRoles` is
   `/*NOTYET*/`), so ours stays; `hack/upstream-pr.sh` follows the PR's
   head and section 2 ("Switching to PR #7148") says how to switch.
2. **Roles**: replace. The list is the whole set; a role taken off it is
   revoked.
3. **Status field name**: `status.appliedDatabaseRoles` stays until a run
   against real Cloud SQL shows whether `users.get` returns roles
   (section 7, item 7).
4. **Managers**: "One controller manager per target/cluster for now.
   We'll try to go more granular later." The manager watches the whole
   cluster by default (`--scoped-namespace` unset); namespace scoping
   stays available for later.
5. **Licence**: none; the repository stays private (README, NOTICE).

## 9. The operator: Config Connector's identity, per namespace

The builder: "We need to package that as an operator so the crds are avail
and then controller works. Ideally it can use the same auth as the actual
kcc cnrm operator when working in the same namespace."

### 9.1 How Config Connector's operator does it (read at the pin)

- **Cluster mode, Workload Identity.** One controller manager in
  `cnrm-system` runs as the Kubernetes service account
  `cnrm-controller-manager`, which the operator annotates with
  `iam.gke.io/gcp-service-account: <ConfigConnector.spec.googleServiceAccount>`
  (`operator/pkg/controllers/configconnector`,
  `objectTransformForWorkloadIdentity`). The Google side is an IAM binding
  of `roles/iam.workloadIdentityUser` for
  `serviceAccount:<project>.svc.id.goog[cnrm-system/cnrm-controller-manager]`,
  which the user makes when installing Config Connector.
- **Cluster mode, a key.** `spec.credentialSecretName` names a Secret in
  `cnrm-system`, mounted as volume `gcp-service-account` at
  `/var/secrets/google`, with `GOOGLE_APPLICATION_CREDENTIALS` pointing at
  `key.json` (`config/installbundle/components/manager/authmodes/withsecretvolume`).
- **Namespaced mode.** For each `ConfigConnectorContext` in namespace N, a
  StatefulSet with `--scoped-namespace=N`, running as
  `cnrm-controller-manager-N`, annotated with that context's
  `googleServiceAccount`, in `cnrm-system`, or in the context's
  `spec.managerNamespace` under `--manager-namespace-isolation=dedicated`
  (`operator/channels/packages/configconnector/<v>/namespaced/per-namespace-components.yaml`,
  `pkg/controllers/configconnectorcontext/namespaced.go`). A context's
  `requestProjectPolicy` becomes `--user-project-override` and
  `--billing-project`.

### 9.2 What ours does

Workload Identity is bound to a namespace and a service-account name, so
the cheapest way to have "the same auth in the same namespace" is to run
in that namespace as that service account: no new Google IAM binding, no
key, nothing for Bundt to grant. `pkg/operator`:

- watches `ConfigConnector`, `ConfigConnectorContext`, Config Connector's
  service accounts and its own Deployments, and in one pass applies what
  should run and deletes what it made that should not;
- cluster mode: Deployment `cnrm-system/gaps-gcp-manager` as
  `cnrm-controller-manager`; with a key, the same Secret mounted the same
  way;
- namespaced mode: Deployment `gaps-gcp-manager-N` as
  `cnrm-controller-manager-N` with `--scoped-namespace=N` and the
  context's project policy flags, in `cnrm-system` or (dedicated) the
  context's manager namespace;
- without a `ConfigConnector` object (Config Connector from its install
  bundle): cluster mode, Workload Identity if `cnrm-controller-manager`
  carries the annotation, else the bundle's `gcp-key` Secret;
- waits for Config Connector's service account rather than run as none;
- binds Config Connector's service account to the managers' roles
  (`config/operator/rbac.yaml`): `gaps-gcp-manager` (our kinds, Secrets,
  events, the Config Connector kinds ours reference) cluster-wide in
  cluster mode or by RoleBinding in N alone, and `gaps-gcp-manager-cluster`
  (CRDs, `ConfigConnector`, namespaces) cluster-wide;
- itself runs in `gaps-gcp-system` as its own service account, holds no
  Google credentials, may read but never create or annotate service
  accounts, and may bind only those two roles (RBAC `bind` by name).

A static bundle was the alternative: cheaper, but it can only cover
cluster mode, because namespaced mode's managers depend on which contexts
exist. The operator is about 400 lines and covers both.

### 9.3 What was proven (2026-11-07)

`pkg/operator` against envtest (`make test`): cluster mode with Workload
Identity and with a key; the switch to namespaced mode (the cluster
manager and its bindings go, one manager per context comes, with its
RoleBinding only in its namespace and the billing-project flags); a context
removed takes its manager and bindings; no manager before Config
Connector's service account exists; Config Connector without its
operator.

`tests/operator/run.sh` (`make operator-e2e`) on kind 0.33 (Kubernetes
1.37) with the in-cluster fake cloud (`cmd/fakegcp`; managers get
`--fake-cloud-url`, which loads no Google credentials), images built by ko
0.19.1 for linux/arm64, 2026-11-07:

```text
ok: CRDs installed: secretmanagersecrets, secretmanagersecretversions, sqladminbackups, sqlusers (*.gcp.hfl.dev)
== cluster mode: one manager in cnrm-system as cnrm-controller-manager
ok: manager pod runs as cnrm-system/cnrm-controller-manager (Workload Identity maps it to kcc@mock-project.iam.gserviceaccount.com)
ok: cnrm-controller-manager may manage gap kinds in any namespace
== cluster mode: a regional secret and its version become Ready
ok: Ready: projects/mock-project/locations/us-central1/secrets/bundt-db-password
== namespaced mode: Config Connector's operator moves to one manager per context
ok: team-a's manager runs as cnrm-system/cnrm-controller-manager-team-a with --scoped-namespace=team-a
ok: team-a's service account may manage gap kinds in team-a and not in team-b
ok: team-a/app-config Ready through team-a's manager
ok: team-b/app-config untouched: no context, no manager
all checks passed
```

Found on the way: the manager had never been started as a binary (only
its controllers, in tests), and it panicked at start ("flag redefined:
alsologtostderr"): Config Connector's dependencies already register klog's
flags, so `klog.InitFlags` must not be called again. Fixed in
`cmd/manager` and `cmd/fakegcp`. mockgcp also needs a writable `/tmp`
(it links controller-runtime's envtest helpers), and ko must run with an
empty Docker config on a Mac (Docker Desktop's credential helper blocked
on the keychain).

Not proven, and cannot be on kind: Workload Identity itself (the GKE
metadata server exchanging the pod's token for the Google service
account's). What is proven instead is everything on our side of it: the
manager pod's service account is exactly Config Connector's for the scope,
in the same namespace, so on GKE the same IAM binding applies to it.
Config Connector's own operator was not installed; its footprint (the
namespace, its service accounts with the annotation, `ConfigConnector`,
`ConfigConnectorContext`) was made by hand as its code says it makes them.

### 9.4 Open questions for the builder

1. **Field gaps on large existing kinds** (section 6.6, ranks 4 to 6:
   `SQLInstance.retainBackupsOnDelete` and others). The gap-kind model
   copies a whole kind into our group; for `SQLInstance` or
   `ContainerCluster` that is thousands of lines and a second kind for one
   resource. The options: (a) wait for the release and pin the bundle on
   it; (b) pin the bundle on a Config Connector `master` commit after the
   PR merges; (c) build Config Connector's manager ourselves with the PR as
   a patch, which is a fork. Recommended: (a), or (b) when it cannot wait,
   never (c).
2. **Sharing Config Connector's service account cuts both ways.** Our
   manager also holds Config Connector's Kubernetes rights, and Config
   Connector's account gains rights to our kinds. The alternative is our
   own service account plus one more `workloadIdentityUser` binding per
   Google service account, which someone must grant. Recommended: share,
   as asked, and revisit with the L3 split of agents by role.
3. **"One manager per target/cluster"** (section 8, answer 4) and
   namespaced mode. The operator follows Config Connector's mode, so a
   namespaced-mode cluster gets one gaps-gcp manager per namespace, as
   Config Connector has. Confirm that this overrides answer 4 for
   namespaced mode.
4. **`--fake-cloud-url` is in the production manager**, as a flag for
   tests. It loads no credentials, so it cannot leak them; anyone able to
   set it can also change the image. Keep it, or move it behind a build
   tag?
5. **Deleting a `SQLAdminBackup` deletes the backup** (upstream's
   behaviour; `deletion-policy: abandon` keeps it). For Bundt's "backup
   before a risky plan", should Bundt write `abandon` on the backups it
   makes, so deleting the Instance's live component keeps them?
