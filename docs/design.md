# config-connector-extensions design

config-connector-extensions has twenty-seven kinds: `SQLUser` with `spec.databaseRoles`,
`SQLAdminBackup`, regional `SecretManagerSecret` and
`SecretManagerSecretVersion`, `ArtifactRegistryRepository` with a custom
remote upstream and its credentials, `ArtifactRegistryTag`,
`IAMWorkloadIdentityPoolNamespace`,
`IAMWorkloadIdentityPoolManagedIdentity` and
`IAMWorkloadIdentityPoolProviderKey`, a narrow direct
`RunService` with `invokerIamDisabled` and GPUs,
`ComputeRegionNetworkEndpoint`, `ComputeRouterRoutePolicy`,
`StorageBucketObject`, `ComputeRegionNetworkFirewallPolicy`,
`LoggingSettings`, `ComputeProjectMetadataItem`, `IAMBinding`,
`IAMMember`, `ComputeRegionNetworkFirewallPolicyRule`,
`ComputeRegionNetworkFirewallPolicyAssociation`, a folder's or
organization's `ServiceIdentity`, `ComputeProjectDefaultNetworkTier`,
`ProjectDefaultServiceAccounts`, and a project's or organization's
`NetworkSecurityAddressGroup`, `NetworkSecuritySecurityProfile` and
`NetworkSecuritySecurityProfileGroup`, and a whole `ComputeRouter`
(section 6). Each
passes its end-to-end test against mockgcp; those in `docs/live-runs.md`
also pass it against a real project. An operator runs the managers beside
Config Connector's, each as its own Kubernetes service account carrying
Config Connector's Google identity, and a kind test proves it in cluster and namespaced mode
against an in-cluster fake cloud (section 9). Images are built locally
with ko; there is no registry or release yet. This document says how it is
built, why, how it stays in step with Config Connector, how it is tested,
how gaps are chosen, and what is left.

## 1. Background

- **One repository per provider.** Each provider's gap kinds are built with
  that provider's generator against that provider's model input, on
  unrelated cadences. So `config-connector-extensions` beside Config
  Connector, and `ack-extensions` beside ACK and `aso-extensions` beside
  Azure Service Operator,
  each versioned on its own and installed beside the upstream controller
  it extends.
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
  Terraform, 65 DCL and 4 IAM kinds of 743. A direct kind is types, mapper,
  identity and reference (from `dev/tools/controllerbuilder` where the
  upstream siblings are generated), a hand-written controller (a few
  hundred lines) on `directbase`, and a mockgcp fixture.
- **GCP is where gaps close fastest upstream**, so
  `config-connector-extensions` expects to retire kinds as often as it
  adds them; a kind moves from our group to
  theirs when upstream ships it (section 3.4).
- **Hermetic tests exist upstream**: mockgcp (a fake GCP, separate Go
  module, in the Config Connector repository) plus golden `_http.log`
  files, maintained by Google.
- **The knobs** (deletion policy, adoption, identity per namespace,
  reconcile cadence) are Config Connector's annotations; because we run
  Config Connector's reconciler, our kinds honour the same ones
  (`cnrm.cloud.google.com/deletion-policy: abandon`, `spec.resourceID`
  acquisition, `.../project-id`, `.../reconcile-interval-in-seconds`), so
  a tool that writes them for Config Connector writes them for us too.

## 2. Architecture chosen

### 2.1 Config Connector as a pinned module, not copied

Options considered for the shared machinery (reconciler, references,
identity, config, mockgcp):

| Option | Verdict |
| --- | --- |
| A. Use Config Connector as a Go module from a checkout at a pinned `master` commit, through `replace` | **Chosen.** The framework stays byte-identical to upstream; syncing is moving the pin. |
| B. Copy `directbase` and what it needs into this repository | Rejected: `go list -deps ./pkg/controller/direct/directbase` pulls 91 Config Connector packages, and through `krmtotf`/`servicemappings` the Terraform fork and DCL. Copying that is a fork, not a copy. |
| C. Copy the Terraform-family `SQLUser` (CRD from the TF schema, `krmtotf`, the provider fork) | Rejected: it needs the whole Terraform family, and the code that would change (`resource_sql_user.go`) is MPL-2.0 in a fork with no patch series. Upstream is moving off it anyway. |

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
config-connector-extensions never deletes cloud resources.

The reconciler reads `ConfigConnector` / `ConfigConnectorContext` (actuation
mode, namespaced vs cluster mode), so those CRDs must exist on the cluster,
which they do wherever Config Connector is installed. In namespaced mode it
requires a `ConfigConnectorContext` in the object's namespace, as Config
Connector does.

Identity: the manager authenticates with Application Default Credentials.
In a cluster it runs as its own Kubernetes service account beside Config
Connector's for the scope, carrying Config Connector's Workload Identity
annotation, so it acts as the Google service account Config Connector has
there, per namespace in namespaced mode (section 9). Sharing Config
Connector's own service account is an opt-in.

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
types, the diff and the mapping. The intent is to use it once it does
what this kind needs; it does not yet, so ours stays, and
`hack/upstream-pr.sh` watches it (UPSTREAM.md, "Watched upstream change").
It is the shape upstream is likely to take, and the kind we would migrate
to. Two
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
4. `task check`; the mockgcp test must still pass unchanged.

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
  exactly those patches.
- Every patch says whether it should go upstream.

### 3.2 Moving the pin

The procedure is in `UPSTREAM.md`. Cadence: when a gap closes upstream,
when a copied file changes upstream, or before a release, at least
monthly while upstream ships every two to three weeks. A release names
the Config Connector release it is installed beside, and pins a `master`
commit at or after that release.

### 3.3 Generating CRDs and code

- `hack/generate.sh` is Config Connector's `dev/tasks/generate-crds` for
  our tree: controller-gen v0.16.5 (`object` and `crd`), then Config
  Connector's own `scripts/add-validation-to-crds` and `scripts/crd-tools`
  run from the pinned checkout. `task check` fails if its output differs
  from what is committed.
- For proto-backed kinds, run Config Connector's `controllerbuilder`
  (`generate-types`, `generate-mapper`) from the checkout against the
  googleapis commit in its `apis/git.versions`, with `--api-version
  <service>.gcp.hfl.dev/v1beta1`, then hand-edit types and controller as
  upstream does. Not exercised yet.

### 3.4 Retiring a gap kind

When a Config Connector release gains the kind or field: copy nothing
new; mark our kind deprecated; document the migration (re-create in their
group with acquisition by `resourceID`, then delete ours with
`deletion-policy: abandon`); after a support window, remove the kind
here.

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
  have; `MOCKGCP_LISTEN_ADDRESS` overrides it, and the Taskfile sets it.
- **The test** (`pkg/controller/direct/sql/sqluser_mockgcp_test.go`) runs
  a real kube-apiserver and etcd (envtest, Kubernetes 1.33), our manager
  code (Config Connector's direct reconciler and deletion defender with
  our adapter), and mockgcp as Cloud SQL. It plays a common case: an
  application's service account as a `CLOUD_IAM_SERVICE_ACCOUNT` user with
  `cloudsqlsuperuser`, then adds a role, removes one, and deletes, reading
  the fake cloud after each step through the real `sqladmin` client.
- **Results** (2026-11-07, `task check`, macOS arm64, Go 1.27.1):

  ```text
  ok: third_party/k8s-config-connector is 672414b8191fc2d2c12fff53f6faccf5ae291eef plus 2 patch(es)
  ok: go.mod carries every replace pin of the pinned Config Connector
  --- PASS: TestSQLUserDatabaseRoles (6.01s)
      --- PASS: TestSQLUserDatabaseRoles/create_grants_the_roles (0.21s)
      --- PASS: TestSQLUserDatabaseRoles/adding_a_role_grants_it (0.22s)
      --- PASS: TestSQLUserDatabaseRoles/removing_a_role_revokes_it (0.22s)
      --- PASS: TestSQLUserDatabaseRoles/delete_removes_the_user (0.22s)
  ok  github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/sql
  ```

  The test was also run with `RevokeExistingRoles(false)` in the
  controller and failed as it should ("cloud roles = [cloudsqlsuperuser
  pg_read_all_data], want [pg_read_all_data]").
- **Live runs** (`docs/live-runs.md`). The same tests run against a real
  project with `E2E_GCP_TARGET=real` (`task test-live`, on an offload VM as
  a least-privilege service account, never from `task check`). The ledger
  there records each run, which kinds have passed live, and what differed
  from mockgcp. Found live on 2026-11-09:
  - `users.get` does return `databaseRoles`, and inserts, adds and
    revokes (`revokeExistingRoles`) behave as patch 0001 fakes them;
  - a deleted Cloud SQL backup run is still returned, with status
    `DELETED`: `SQLAdminBackup` now takes that as gone, and patch 0015
    fakes it;
  - Artifact Registry refuses a remote upstream it cannot reach unless
    `disableUpstreamValidation` is set, and refuses a change of a remote
    repository's source ("Remote source configuartion cannot be
    updated"), which patch 0016 fakes.
- **Next** (section 7): port Config Connector's own `SQLUser` fixtures
  (`pkg/test/resourcefixture/testdata/basic/sql/v1beta1/sqluser/`) to our
  group and run them through its golden harness, which records
  `_http.log`.

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

Decisions (section 8):

- `databaseRoles` is the whole set of the user's roles, declaratively: a
  role removed from the list is revoked (`revokeExistingRoles=true`, as
  Terraform does). Out-of-band grants are
  not detected while the list is unchanged.
- What was applied is kept in `status.appliedDatabaseRoles`, because the
  API is not relied on to return a user's roles.
- A password from a Secret is re-sent when the spec generation changes,
  not when only the Secret changes (Config Connector tracks Secret
  versions with an annotation; not done here).

## 6. Choosing gaps

A gap becomes a kind here only when a real workload needs it and no
existing Config Connector kind covers it. Counting missing fields does not
rank them: Config Connector's existing kinds lack thousands of fields that
Terraform or the API have, and most of them nobody is blocked on.

### 6.1 Upstream first

- **Code on Config Connector `master`, not released.** If the next release
  is near, no kind of ours is needed: install beside that release.
- **An open upstream pull request.** Copy its code under the copy-upstream
  rule, and watch the pull request (`hack/upstream-pr.sh`); when it merges
  and a release ships it, retire ours (section 3.4).
- **A field missing on a large existing kind** (such as
  `SQLInstance.retainBackupsOnDelete`): wait for the Config Connector
  release that carries it. A gap kind for an existing kind, carrying the
  field, is built only when a workload cannot work without the field and
  the release is not expected soon enough. It lives in our group, as
  `SQLUser` does, and retires when the release lands. Config Connector's
  own manager is never rebuilt with a patch.
- **In the Google API, in no Config Connector code or pull request.**
  Written here from the API and Terraform's behaviour, read not copied.

### 6.2 What "missing" means

Comparing Config Connector's CRDs with Terraform's resources and Google's
Discovery documents overcounts gaps:

- Terraform attributes are best read from the provider's schema, not its
  docs; hand-written docs parse unreliably.
- Discovery documents lack field behaviours, so output-only is a guess and
  required or immutable are unknown.
- Only APIs vendored in google-api-go-client are visible.
- Terraform resources and API types are not one-to-one (IAM add-ons,
  singletons, virtual resources); many IAM add-ons are covered by Config
  Connector's generic `IAMPolicyMember`.
- Config Connector's Terraform kinds run a pinned fork that lags upstream
  Terraform, so "missing" mixes "never modelled" with "not yet
  re-vendored".
- Config Connector sometimes names a field differently from Terraform, so
  a name match is only a heuristic.

### 6.3 Candidates (read 2026-11-07)

From the open pull requests on Config Connector (read-only) and the pinned
checkout (`672414b8`). `StorageManagedFolder` and `ArtifactRegistryRule`
already have direct controllers on master: no gap kind, install beside
the release that ships them.

Upstream code exists, not released:

| Rank | Candidate | Upstream | Why | Done |
| --- | --- | --- | --- | --- |
| 1 | `SQLAdminBackup`, an on-demand Cloud SQL backup | types on master; controller and mockgcp in PR #12553 | a backup before a change that can lose data (a major upgrade, a destructive migration) | built |
| 2 | `SecretManagerSecret.spec.location`, regional secrets and their versions | PR #6651 (and its duplicate #5730), on the direct controller | secrets kept in one region (data residency) | built |
| 3 | `SQLUser` on the direct reconciler | PR #7148 | ours exists; switch when `databaseRoles` is live (section 2) | watched |
| 4 | `SQLInstance`: `retainBackupsOnDelete` (#14854), `diskEncryptionConfiguration` (#7227), PSC auto-connections (#14419), performance capture (#14431) | PRs on a large direct kind | deleting an instance must not take its backups | wait for the release (6.1) |
| 5 | `ContainerNodePool` `autoscaling.enabled` (#14182), kubelet sysctls (#14187) | PRs | clusters made from manifests | wait for the release |
| 6 | `StorageBucket` `rpo` (#13713), encryption enforcement (#15145), direct controller (#11533) | PRs; the kind is Terraform-based on master | buckets | wait for the release |
| 7 | `ArtifactRegistryRepository` custom remote repositories (#13517) | PR | pulling through a private upstream | built |
| 8 | `RunService` `invokerIamDisabled` (#14417), GPUs (#7232) | PRs, Terraform only; no direct `RunService` upstream | Cloud Run services | built, as a narrow direct `RunService` (decision 11) |
| 9 | `IAMWorkloadIdentityPoolNamespace`, `...ManagedIdentity` (#14209, #14418), pool `mode` (#14227) | types only, no controller, no mockgcp | workload identity federation | namespace and managed identity built; `mode` needs the DCL fork |

In the Google API, in no Config Connector code or pull request
(unverified against the live API), in the order decision 10 takes them:

| Candidate | Done |
| --- | --- |
| Artifact Registry package tags | built, `ArtifactRegistryTag` (2026-11-09) |
| a remote repository's `upstreamCredentials` | built (decision 8) |
| Workload Identity pool provider keys | built, `IAMWorkloadIdentityPoolProviderKey` (2026-11-09) |
| Cloud SQL `blueGreenDeployments` | skipped: not in the API. Neither `sqladmin/v1` nor `sqladmin/v1beta4` in google-api-go-client v0.287.1 (the pin's) has it; the entry was wrong |

Two entries were dropped from this list on 2026-11-09: Artifact Registry
repository attachments have a direct controller on `master`
(`ArtifactRegistryAttachment`, not in `v1.160.0`; install beside the
release that ships it), and Storage HMAC keys are Config Connector's
`StorageHMACKey`, already in `v1.160.0`.

Rechecked 2026-11-08 (read-only, `gh` and `git fetch` of `master` at
`c8208a77`), before building rank 7. No candidate above merged or was
released: the latest release is still `v1.160.0` (2026-10-29), and every
pull request in the table is open with review required or changes
requested, none approved. Ranks 4 to 6 stay "wait for the release" (6.1).
Rank 7, PR #13517 (head `eb2774ce`, opened 2026-09-29 by upstream's
coding bot), has no human review (only bots fixing its CI), and was
paused by upstream's bot for inactivity on 2026-10-13: not close to
merging. `master`'s Artifact Registry code is unchanged since our pin.

Rechecked 2026-11-09 (read-only, `master` at `7822e4bd`), for ranks 8
and 9 after rank 7 was built. Still no release after `v1.160.0`; none of
their pull requests merged or was approved. Neither can be built by
copying upstream code, so both waited on a question, since answered
(decisions 9 and 10):

- Rank 8: `RunService` is Terraform-only on `master` (`static_config.go`
  lists Terraform as its only controller), and #14417 (opened 2026-10-25,
  review required, no reviews) and #7232 (opened 2026-03-27, review
  required) change only the Terraform fork and its generated types.
  Upstream has no direct `RunService` types or controller, and no pull
  request for one; `apis/run/v1beta1` has only its identity and reference.
- Rank 9: #14209 (changes requested) and #14418 (review required) add
  v1alpha1 types and identities only, no controller. mockgcp serves no
  workload identity pools at all (the pool is DCL-based), and #14227
  (`mode`, changes requested) changes the DCL fork, which we do not run.

### 6.4 Notes on what was built

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
- Tests (`task test`, macOS arm64, Go 1.27.1, 2026-11-07):

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
- `ArtifactRegistryRepository` is master's direct controller (on `master`
  it is opt-in; Terraform is the default there) with PR #13517's change:
  `remoteRepositoryConfig.{docker,maven,npm,python}Repository.customRepository.uri`,
  a remote repository's upstream at a URI of the user's choosing instead
  of one of the public upstreams Artifact Registry names. The PR changes
  only the types, the mapper and the fuzzer for the direct controller (and
  Terraform's fork, which we do not use); the controller itself is
  unchanged. mockgcp already stores the field (it keeps the client
  library's protos as sent), so no mockgcp patch is needed. The CRD has
  exactly the PR's property paths.
  - A virtual repository's `upstreamPolicies[].repositoryRef.name` names a
    repository of ours, as the Secret Manager version's `secretRef` does;
    a Config Connector repository is named by `repositoryRef.external`.
  - What the PR does not map, ours does not either: Apt and Yum remote
    repositories. `upstreamCredentials` and `disableUpstreamValidation`
    were added later, written here (below).
  - Changing `customRepository.uri` on an existing repository: Terraform
    marks it ForceNew; Config Connector's direct controller sends
    `remoteRepositoryConfig` in an update mask, which mockgcp refuses
    ("not supported by mockgcp"), so the object goes not Ready and the
    fake cloud keeps the old URI. What the real API does is not known.
  - A resync (Config Connector's force-reconcile annotation) reads the
    repository back and sends no update, which shows the mapper
    round-trips `customRepository`: a dropped field would be a diff.
- Tests (`go test ./pkg/controller/direct/artifactregistry/`, envtest
  Kubernetes 1.33, macOS arm64, Go 1.27.1, 2026-11-08):

  ```text
  --- PASS: TestArtifactRegistryRepositoryFuzz (1.45s)
      1000 seeds through the spec and status mappers, customRepository included
  --- PASS: TestArtifactRegistryRemoteRepository (39.69s)
      create sets the custom upstream; a resync sends no update; a
      description change keeps the custom upstream; a changed upstream URI
      is not applied in the fake cloud; delete removes the repository;
      abandon keeps the repository
  --- PASS: TestArtifactRegistryCustomUpstreamFormats (18.49s)
      MAVEN, NPM, PYTHON
  ```

  With the mapper's read-back of the Docker `customRepository` removed,
  the fuzz test and "a resync sends no update" fail (the controller sends
  an update that mockgcp refuses), so the round trip is what they check.
- `task check` and `task operator-e2e` on a Linux amd64 VM (16 CPUs, Go
  1.27.3, kind 0.27.0, ko 0.19.1, kustomize 5.7.1), 2026-11-09: both pass.
  Every package's tests pass, the artifactregistry package's included; the
  operator installs `artifactregistryrepositories.artifactregistry.gcp.hfl.dev`
  with the other gap CRDs and its managers' roles cover the new group. The
  operator test applies no Artifact Registry object; the kind's behaviour
  is proven by the envtest test above. `tests/operator/run.sh` needs
  `kustomize` on the path.
- `upstreamCredentials` and `disableUpstreamValidation` on
  `ArtifactRegistryRepository` (decision 8) are **written here**, from the
  Artifact Registry API and Terraform's `google_artifact_registry_repository`
  (provider `main` at `42eb358f`, read, not copied). No Config Connector
  code or pull request has them.
  - `remoteRepositoryConfig.upstreamCredentials.usernamePasswordCredentials`
    takes `username` and `passwordSecretVersionRef`, Config Connector's
    `refs/v1beta1.SecretManagerSecretVersionRef` (as `CloudBuildConnection`
    and `DataformRepository` use it): a Config Connector
    `SecretManagerSecretVersion` by name, resolved to
    `projects/{p}/secrets/{s}/versions/{v}`, or that string as `external`.
  - Credentials change in place, as Terraform does: when only they differ,
    the update mask names
    `remoteRepositoryConfig.upstreamCredentials.usernamePasswordCredentials.{username,passwordSecretVersion}`
    instead of the whole `remoteRepositoryConfig`.
  - `disableUpstreamValidation` is input only (the API does not return
    it), so it is copied from the spec onto what was read before
    comparing, and never causes an update by itself. It is sent with
    every create and with any update of the remote config.
  - Patch 0005: mockgcp accepts those two mask paths and does not store
    `disableUpstreamValidation`, as the API documents.
  - Written-here code is marked "Written here" in the copied files and
    lives in `upstreamcredentials_types.go` and
    `upstreamcredentials_mapping.go`. `pkg/test/mockenv` now installs
    Config Connector's two Secret Manager CRDs, so a reference by name
    resolves in tests.
- `IAMWorkloadIdentityPoolNamespace` (`iam.gcp.hfl.dev/v1alpha1`, decision
  9). Rechecked 2026-11-09: PR #14209 (head `a5e8f5a9`) is open; changes
  were requested on its CI files, not on the types. It is the first kind
  here whose controller is **written here** rather than copied: upstream
  has only the types.
  - Copied from the PR: the types, identity and reference, and the
    mockgcp proto. Written here: `spec.workloadIdentityPoolRef` (the PR's
    identity leaves the pool empty and always fails), the direct
    controller on `google.golang.org/api/iam/v1`, and the mockgcp service.
  - `workloadIdentityPoolRef` names Config Connector's `IAMWorkloadIdentityPool`
    (DCL-based, no reference type upstream) by name, or a pool as
    `external`. A pool in another project or location than the namespace's
    `projectRef` and `location` is refused.
  - Behaviour follows Terraform's
    `google_iam_workload_identity_pool_namespace` (read, not copied):
    `description` and `disabled` change in place (`updateMask`); a namespace
    in state `DELETED` (soft-deleted, purged after about 30 days) is absent;
    its ID cannot be reused until purged, so re-creating it fails with the
    API's error, as in Terraform. Changes are long-running operations, polled
    until done.
  - Patch 0006: mockgcp gains the namespace service, with soft delete. Its
    Go is generated by `hack/regen-mockiam-workload-identity.sh` with mockgcp's
    toolchain and reproduces byte for byte.
  - `pkg/test/mockenv` also installs Config Connector's `IAMWorkloadIdentityPool`
    CRD, so the pool resolves by name in tests.
- `IAMWorkloadIdentityPoolManagedIdentity` (`iam.gcp.hfl.dev/v1alpha1`,
  decision 9). Rechecked 2026-11-09: PR #14418 (head `b080f5dc`) is open,
  review required, no reviews; types only, like #14209.
  - Copied from the PR: the types, identity and reference, and the
    mockgcp proto. Written here: `spec.workloadIdentityPoolNamespaceRef`
    (our `IAMWorkloadIdentityPoolNamespace` by name, once it has a
    `status.externalRef`, or a namespace as `external`; one in another
    project or location is refused), `spec.attestationRules`, the
    controller, and the mockgcp service.
  - `attestationRules` follow Terraform's `attestation_rules` (read, not
    copied): set with `setAttestationRules` after the identity is made and
    whenever they differ, read with `listAttestationRules`, the list being
    the whole set. Unset leaves the rules alone; an empty list removes them
    all. The PR leaves them out, but a managed identity without them can be
    given to no workload.
  - Patch 0007, on top of 0006: the managed identity service and its
    attestation rules. The PR's proto has no attestation-rule methods; they
    are added from the IAM v1 API, and the proto's duplicate operation
    metadata is dropped.
- `RunService` (`run.gcp.hfl.dev/v1alpha1`, decisions 10 and 11). Rechecked
  2026-11-09: PR #14417 (`invokerIamDisabled`, head `9297537a`) and PR
  #7232 (GPUs) are open, review required, and change only Config
  Connector's Terraform fork; no direct `RunService` exists upstream.
  - Copied from Config Connector's direct `RunWorkerPool` (master): the
    types (template, containers, env, volumes, VPC access, node selector),
    identity, reference, mappers, reference resolution and controller,
    renamed for a service, without instance splits.
  - Written here, from the Cloud Run v2 API: the service's `ingress`,
    `invokerIamDisabled`, `defaultUriDisabled` and `scaling`; the
    template's `scaling`, `timeout`, `maxInstanceRequestConcurrency`,
    `executionEnvironment`, `sessionAffinity`, `healthCheckDisabled` and
    `gpuZonalRedundancyDisabled`; the URIs in status; and the server
    defaults a service has that a worker pool does not (ingress, timeout,
    concurrency, execution environment, template scaling), taken from what
    was read when the spec leaves them unset.
  - Not modelled: `traffic` (a service sends all traffic to its latest
    revision), IAP, multi-region settings, the build config.
  - Secret references are typed `SecretManagerSecretRef` and
    `SecretManagerSecretVersionRef`, not their aliases (controller-gen
    v0.16.5 panics on aliases, as for the Secret Manager kinds).
  - Patch 0008: mockgcp's `services.patch` honours an update mask (exact
    top-level fields), and keeps `invokerIamDisabled` without one (PR
    #14417's line).
  - The managers' role also reads the `compute`, `vpcaccess`, `storage`
    and `networkservices` Config Connector groups the template references.
- `ArtifactRegistryTag` (`artifactregistry.gcp.hfl.dev/v1alpha1`, decision
  10, the first unranked candidate). **Written here** in full: no Config
  Connector code or pull request, and no Terraform resource, has package
  tags; the kind follows the Artifact Registry API (`tags.create`, `get`,
  `patch` with update mask `version`, `delete`) on the client the
  repository controller already uses.
  - `spec.repositoryRef` is Config Connector's `ArtifactRegistryRepositoryRef`
    (a Config Connector repository by name, or `external`), `spec.package`
    the package ID (a "/" in it is sent as `%2F`, as resource names encode
    it), `spec.resourceID` the tag, `spec.version` the version's ID (for a
    Docker image its digest). Moving the tag is changing `version`.
  - Patch 0009: mockgcp serves tags in an existing repository; it holds no
    packages, so neither the package nor the version is checked.
  - The managers' role also reads `artifactregistry.cnrm.cloud.google.com`;
    `pkg/test/mockenv` installs Config Connector's repository CRD.
- `IAMWorkloadIdentityPoolProviderKey` (`iam.gcp.hfl.dev/v1alpha1`, decision
  10, the last unranked candidate). **Written here** in full: no Config
  Connector code or pull request, and no Terraform resource (Terraform has
  only `google_iam_workforce_pool_provider_key`, the workforce analogue),
  has workload identity pool provider keys.
  - `spec.workloadIdentityPoolProviderRef` names Config Connector's
    (DCL-based) `IAMWorkloadIdentityPoolProvider` by name, reading its pool
    from its own `workloadIdentityPoolRef`, or a provider as `external`;
    `spec.use` (`ENCRYPTION`) and `spec.keyData.keySpec` (`RSA_2048`,
    `RSA_3072`, `RSA_4096`). Every field is immutable, as in Terraform's
    workforce analogue: a change leaves the object not Ready with the
    field named, and is not applied.
  - `status.observedState.keyData` carries the public key (an X.509 PEM)
    and its validity window, to configure in the identity provider.
  - A deleted key is soft-deleted (state `DELETED`) and counts as absent,
    as for namespaces.
  - Patch 0010: mockgcp serves provider keys. The proto is written here
    from the API's discovery document (googleapis has none), generated by
    the same script, and the mock returns a placeholder PEM, not a
    certificate.
  - With this, every ranked and unranked candidate in section 6.3 is built,
    watched, waiting for a release, or skipped with its reason.

### 6.5 From the seed catalog (read 2026-11-09)

The catalog agent's inventory of Cloud Foundation Fabric v60.0.0 against
Config Connector v1.160.0 (`highfivelabs/catalog`, `docs/gcp-inventory.md`
at `a5a451a`, "Gaps for config-connector-extensions") ranks what fabric
modules need and no Config Connector kind covers, by how many modules need
it. Rechecked here read-only on 2026-11-09 against Config Connector
`v1.160.0`, `master` at `b795cda8` and its open pull requests; rows follow
this repository's rules (6.1, decision 5, section 10).

| Rank | Gap | Modules | Upstream (2026-11-09) | Here |
| ---: | --- | ---: | --- | --- |
| 1 | Role-authoritative IAM (`google_*_iam_binding`): the members of one role on one resource, removing any others | 43 | none. PR #14133 (open, review required) adds an `IAMPolicyBinding` that is IAM v3's `PolicyBinding` (principal access boundary), a different resource with the name the catalog suggests | built (2026-11-09), as `IAMRoleBinding`, now `IAMBinding` (decision 12) |
| 2 | IAM on the 32 resource types `IAMPolicyMember` cannot reference | 24 | none | built (2026-11-09), as `IAMRoleMember`, now `IAMMember` (decision 12), over the same resource table as `IAMBinding`, which accepts the same 32 resource types |
| 3 | `ComputeRegionNetworkEndpoint` | 9 | no kind or PR (the global and zonal endpoint kinds are Terraform-based alphas) | built (2026-11-09) |
| 4 | `ComputeRouterRoutePolicy` | 5 | no kind or PR | built (2026-11-09) |
| 5 | `StorageBucketObject` | 4 | no kind or PR | built (2026-11-09) |
| 6 | Regional network firewall policies: the policy, its rules, its association | 2 | no kind or PR (the global `ComputeNetworkFirewallPolicy` is Terraform-based by default, with an opt-in direct controller; its rule and association kinds are Terraform-only) | the policy, its rules and its association built (2026-11-09) |
| 7 | Hierarchy settings: folder and organization service identities, logging settings, a billing account's log sink | 3 | `ServiceIdentity` exists (Terraform-based) for projects only; `LoggingLogSink` takes project, folder and organization parents, not a billing account; no `LoggingSettings` | `LoggingSettings` built (2026-11-09; Config Connector's PR #3730, closed, was a scaffold with no fields); folder and organization `ServiceIdentity` built (2026-11-09, `docs/coverage/foundation.md` item 1); the rest are field gaps on existing kinds: wait for the release (decision 5) |
| 8 | Values known only after apply, as references (`TagsTagBinding.parentRef` to an `IAMServiceAccount`, a project's service agents in `memberFrom`, `StorageBucket.encryption.kmsKeyHandleRef`) | 3+ | none found | field gaps on existing kinds: wait for the release (decision 5) |
| 9 | Field gaps on foundational kinds: `StorageBucket` `rpo`, `hierarchicalNamespace`, `enableObjectRetention`; `ComputeNetwork` BGP settings and `status.networkId`; `ComputeSubnetwork` `ipCollection`, `allowSubnetCidrRoutesOverlap` | 4 | `rpo` is PR #13713 (open, review required); none found for the rest | wait for the release (decision 5) |
| 10 | Project settings: default service accounts, default network tier, per-key compute metadata | 1 | no kinds or PRs (`ComputeProjectMetadata`, whole-project, is Terraform-based) | per-key metadata built (2026-11-09, `ComputeProjectMetadataItem`); the default network tier built (2026-11-09, `ComputeProjectDefaultNetworkTier`); default service accounts built (2026-11-09, `ProjectDefaultServiceAccounts`) |

Notes on what was built from this table:

- `ComputeRegionNetworkEndpoint` (`compute.gcp.hfl.dev/v1alpha1`, rank 3).
  **Written here**, after Config Connector's zonal `ComputeNetworkEndpoint`
  (whose types have only the group, `ipAddress` and `instanceRef`), with
  the fields Terraform's `google_compute_region_network_endpoint` has:
  `regionNetworkEndpointGroupRef` (Config Connector's
  `ComputeRegionNetworkEndpointGroup` by name, or `external`), `port`,
  `ipAddress`, `fqdn`, `clientDestinationPort`, `instance`. The controller
  follows Terraform (read, not copied): `attachNetworkEndpoints`, found by
  listing the group's endpoints and matching address and port,
  `detachNetworkEndpoints`; every field is immutable (a change leaves the
  object not Ready and attaches nothing). An endpoint has no resource name
  in the API, so `status.externalRef` is the group's name plus
  `/networkEndpoints/{address}:{port}`, ours. Patch 0011: mockgcp attaches,
  detaches and lists a regional group's endpoints, adapted from its zonal
  ones.
- `ComputeRouterRoutePolicy` (`compute.gcp.hfl.dev/v1alpha1`, rank 4).
  **Written here** from the Compute Engine API's `RoutePolicy`:
  `routerRef` (Config Connector's `ComputeRouter` by name, or `external`),
  `resourceID`, `type`, `description`, and `terms` (priority, a CEL
  `match`, CEL `actions`). The controller follows Terraform's
  `google_compute_router_route_policy` (read, not copied): `getRoutePolicy`
  reads it, `updateRoutePolicy` creates it and replaces it whole (with the
  fingerprint it read), `deleteRoutePolicy` deletes it; type, description
  and terms change in place. A route policy has no API resource name, so
  `status.externalRef` is the router's name plus `/routePolicies/{name}`,
  ours. Patch 0012: mockgcp serves a router's route policies with a new
  fingerprint on each write.
- `StorageBucketObject` (`storage.gcp.hfl.dev/v1alpha1`, rank 5). **Written
  here**: `bucketRef` (Config Connector's `StorageBucketRef`), `resourceID`
  (the object name; "/" allowed), the content inline or `contentFrom` a
  Secret's or ConfigMap's key, `contentType`, `cacheControl`,
  `contentDisposition`, `contentEncoding`, `contentLanguage`, `metadata`;
  status carries the generation, MD5, CRC32C, size and media link. The
  controller follows Terraform's `google_storage_bucket_object` (read, not
  copied): `objects.insert` uploads it; a changed content (by MD5) or any
  other changed field uploads it again, a new generation, since Terraform
  replaces the object for each; `objects.delete`. It is for small objects
  (a configuration file, a script): the content lives in a Kubernetes
  object, so about 1 MiB at most. A changed Secret or ConfigMap is uploaded
  at the next reconcile (`reconcile-interval-in-seconds`), not when it
  changes. Patch 0013: mockgcp's objects were stubs; it now serves media
  and multipart uploads, content reads, get and delete. The managers' role
  now reads ConfigMaps.
- `ComputeRegionNetworkFirewallPolicy` (`compute.gcp.hfl.dev/v1alpha1`,
  rank 6, the policy). **Copied** from Config Connector's opt-in direct
  `ComputeNetworkFirewallPolicy` (types, identity, controller, the status
  mapping) for a region: `spec.region`, the regional client,
  `projects/{project}/regions/{region}/firewallPolicies/{name}` as the
  identity, and Terraform's `regionNetworkFirewallPolicyId` in status.
  Written here: the spec mapping (`description`, the one field that
  changes in place), and an unset description is sent as "" so that
  removing it clears it, as Terraform's
  `google_compute_region_network_firewall_policy` does. A changed region,
  project or name leaves the object not Ready (the identity is checked
  against `status.selfLink`, as upstream does). `policyType` (VPC or RDMA)
  is not a field yet: the API has it, Terraform's vendored resource does
  not. Patch 0014: mockgcp serves regional policies, adapted from its
  global ones. The rules and the association are separate kinds, next.
  Live: passes (`docs/live-runs.md`).
- `ComputeRegionNetworkFirewallPolicyRule` (`compute.gcp.hfl.dev/v1alpha1`,
  rank 6, the rules). **Written here**: no Config Connector code has a
  regional policy's rule (`ComputeFirewallPolicyRule` is the rule of an
  organization or folder policy; the global network policy's rule is
  Terraform-only). Fields: `firewallPolicyRef`
  (`ComputeRegionNetworkFirewallPolicyRef`: a `ComputeRegionNetworkFirewallPolicy`
  by `name`, or `external`; the association kind reuses it), `priority`,
  `action`, `direction`, `match` (`layer4Configs`, source and destination
  ranges, address groups, FQDNs, region codes and threat intelligence
  lists, source secure tags), `ruleName`, `description`, `disabled`,
  `enableLogging`, `targetServiceAccounts`, `targetSecureTags`. Not here
  yet: `securityProfileGroup`, `tlsInspect` and `targetResources`
  (the API has them; Terraform's rule has them; add on demand). The
  controller follows Terraform's
  `google_compute_region_network_firewall_policy_rule` (read, not copied):
  a rule has no resource name, only its priority in the policy, so
  `getRule`, `addRule`, `patchRule` and `removeRule` all take the priority,
  and `status.externalRef` is ours,
  `projects/{p}/regions/{r}/firewallPolicies/{policy}/rules/{priority}`.
  The priority and the policy are immutable (CEL, and the identity is
  checked against `status.externalRef`). A rule already at the priority
  is taken over (Find finds it, Update brings it to the spec). `patchRule`
  is sent only when the fields the spec manages differ from what was read,
  so a resync writes nothing. What the real API does (live run, 2026-11-09)
  and the fake cannot show: a priority with no rule is answered with 400,
  "The firewall policy does not contain a rule at priority N", not 404
  (Find and Delete take that as "gone"); `patchRule` **keeps a list the
  body leaves out** (a rule patched without its `srcIpRanges` kept them),
  so `patchRule` is sent through the Compute Engine REST client (the
  generated gRPC-style client leaves an empty value out of the body) with
  a list the cloud's rule has and the spec has not in `ForceSendFields`
  (sent as `[]`), and the optional strings and booleans always sent, `""`
  and false when unset; and a rule just changed is "not ready" to the API
  for a while (a patch then answers 400 "The rule at priority N is not
  ready"), which the reconciler's retry absorbs.
  Patch 0020: mockgcp serves a regional policy's rules, adapted from the
  global ones (its `patchRule` replaces the whole rule).
  Live: passes (`docs/live-runs.md`).
- `ComputeRegionNetworkFirewallPolicyAssociation` (`compute.gcp.hfl.dev/v1alpha1`,
  rank 6, the association). **Written here**: attaches a regional policy
  to a VPC network. `firewallPolicyRef` (the
  `ComputeRegionNetworkFirewallPolicyRef` the rule kind added),
  `networkRef` (Config Connector's `ComputeNetworkRef`: a `ComputeNetwork`
  by `name` or `external`) and `resourceID` (the association's name in
  the policy; `metadata.name` by default), all immutable (CEL).
  `getAssociation`, `addAssociation` and `removeAssociation` address it by
  name; `status.externalRef` is ours,
  `projects/{p}/regions/{r}/firewallPolicies/{policy}/associations/{name}`.
  Nothing about an association changes in place, so Update only checks
  that the cloud's association is attached to the network the spec
  names: one that exists with the name but another network is an error,
  never a silent move. `replaceExistingAssociation` is not sent: a
  network that already has a policy attached is the API's error on the
  object. The real API does not return the policy's ID in an association
  (an earlier draft had a status field for it). A missing association is
  a 400 ("Invalid value for field 'name': ... An association with that
  name does not exist."), which Find and Delete take as "gone". The
  sweeper removes a policy's associations before the policy
  (`hack/live/sweep.sh`).
  Patch 0021: mockgcp serves a regional policy's associations (the global
  mock has none).
  Generated versus written: all written by hand. Config Connector's
  controllerbuilder (`hack/scaffold-kind.sh`) generates from a proto
  resource; an association is a list inside the policy's proto with no
  resource of its own.
  Live: passes (`docs/live-runs.md`).
- `LoggingSettings` (`logging.gcp.hfl.dev/v1alpha1`, rank 7). **Written
  here** from the Cloud Logging API's `Settings`: `organizationRef` or
  `folderRef` (exactly one, checked by the CRD), `kmsKeyRef` (Config
  Connector's `KMSCryptoKeyRef`), `storageLocation`, `disableDefaultSink`;
  status carries the Log Router's service accounts. The controller
  follows Terraform's `google_logging_organization_settings` and
  `google_logging_folder_settings` (read, not copied): the settings
  always exist, so create and update are both `settings.patch` with a
  mask of the fields the spec sets and the cloud does not match; a field
  left unset (or taken off the spec) is not managed; delete leaves the
  settings as they are. Project settings are read-only in the API and
  not a field here; `defaultSinkConfig` waits for mockgcp's logging
  protos to have it. Patch 0017: mockgcp serves `getSettings` and
  `updateSettings`.
  Live: pending, needs a test folder with `roles/logging.admin` and a
  KMS key (`docs/live-runs.md`).

- `ServiceIdentity` (`serviceusage.gcp.hfl.dev/v1alpha1`, rank 7, and
  `docs/coverage/foundation.md` item 1). A folder's or an organization's
  service agent: `folderRef` or `organizationRef` (exactly one, checked
  by the CRD), `resourceID` (the service; `metadata.name` by default);
  status `email` (as upstream's), `externalRef` and `observedState`
  (`email`, `uniqueID`). Upstream (read 2026-11-09): Config Connector's
  direct `ServiceIdentity` takes a project only, and no pull request adds
  a folder or an organization. **The first kind built from
  `hack/scaffold-kind.sh`**: Config Connector's `controllerbuilder`
  generated the types scaffold, `types.generated.go` and
  `mapper.generated.go` from `google.api.serviceusage.v1beta1`, with
  `hack/scaffold/serviceusage/overlay.proto` marking `email` and
  `unique_id` output-only (the proto marks nothing, so without it they
  landed in the spec). The scaffold's `projectRef` and `location` were
  replaced by the two parents; the controller is upstream's, copied
  (`generate-controller` writes nothing for a service upstream already
  has a controller for), with the parent and status changed. The API has
  only `services.generateServiceIdentity`, so, as upstream's, Find trusts
  `status.email`, create generates, update rewrites status, and delete
  leaves the identity; a changed parent or service is refused (the
  identity against `status.externalRef`). Terraform's
  `google_folder_service_identity` and
  `google_organization_service_identity` were read for the request's
  path, not copied (Magic Modules hand-writes them; there is no product
  YAML to take behaviour from). Patch 0022: mockgcp serves the folder and
  organization parents. Live: pending, needs the test folder
  (`E2E_TEST_FOLDER`, `mockenv.TestFolder`); a live run never uses the
  organization.

- `ComputeProjectDefaultNetworkTier` (`compute.gcp.hfl.dev/v1alpha1`,
  rank 10, and `docs/coverage/foundation.md` item 1). **Written here**
  after Terraform's `google_compute_project_default_network_tier` (read,
  not copied), in the shape of `ComputeProjectMetadataItem`: `projectRef`
  and `networkTier` (`PREMIUM` or `STANDARD`, checked by the CRD). The
  setting always exists, so Find reads the project, create and update
  call `projects.setDefaultNetworkTier` only when the tier differs (a
  tier changed by hand is put back at the next reconcile), and delete
  leaves the tier as it is, as Terraform's does. Upstream (read
  2026-11-09): no kind or pull request. Not generated: Compute Engine has
  no googleapis proto resource for it, so `hack/scaffold-kind.sh` does
  not apply; the Terraform route (catalog `docs/terraform-leverage.md`,
  spike 2) was not taken, since it would load Config Connector's
  Terraform family into our manager (decision 2.1). Patch 0023: mockgcp
  serves `setDefaultNetworkTier`. Live: pending, needs a test project
  under the test folder (`E2E_TEST_FOLDER_PROJECT`,
  `mockenv.FolderProject`; never `hfl-dev-514205`, whose tier the offload
  VMs rely on) and `compute.projects.setDefaultNetworkTier` there, which
  `cce-live-tests` lacks (the spike's 403).

- `ProjectDefaultServiceAccounts` (`resourcemanager.gcp.hfl.dev/v1alpha1`,
  rank 10, and `docs/coverage/foundation.md` item 1). **Written here**
  after Terraform's `google_project_default_service_accounts` (google-beta
  `24c6f5fd`, read, not copied; Magic Modules hand-writes it, so there is
  no product YAML): `projectRef`, `action` (`DELETE`, `DISABLE` or
  `DEPRIVILEGE`, immutable by CEL, as Terraform's is ForceNew) and
  `restorePolicy` (`REVERT` by default, `REVERT_AND_IGNORE_FAILURE`,
  `NONE`); status `serviceAccounts`, unique ID to email, as Terraform's
  state. The defaults are the accounts whose display name is "Compute
  Engine default service account" or "App Engine default service
  account" (Terraform's test; their emails may change). `DELETE` and
  `DISABLE` go through IAM v1 by unique ID; `DEPRIVILEGE` removes
  `serviceAccount:{email}` from every binding of the project's policy
  through `pkg/iampolicy` (version 3, etag retry), dropping a binding
  left empty and keeping conditions. On delete, unless `NONE`, `DELETE`
  is undone with `undelete` (by unique ID; an account that exists is
  left alone) and `DISABLE` with `enable`; `REVERT_AND_IGNORE_FAILURE`
  logs a failure and lets the object go; `DEPRIVILEGE` is never undone;
  abandon leaves everything. Two choices of ours, not Terraform's:
  Terraform acts once, at create (its read is a no-op), where this acts
  on every reconcile, so an account re-enabled by hand, or one that
  appears when App Engine is turned on later, is acted on, and status
  only grows; and Terraform reads the policy as version 1, which loses
  conditional bindings. Upstream (read 2026-11-09): no kind or pull
  request. Not generated: there is no proto message for it, so
  `hack/scaffold-kind.sh` does not apply (0% generated beyond the CRD and
  deepcopy; the group file is copied). Patch 0024: mockgcp lists and
  undeletes service accounts and takes unique IDs. Live: pending, needs a
  test project under the test folder (`E2E_TEST_FOLDER_PROJECT`,
  `mockenv.FolderProject`), never `hfl-dev-514205`, whose compute default
  account the offload VMs and `RunService` use.

- `NetworkSecurityAddressGroup` (`networksecurity.gcp.hfl.dev/v1alpha1`,
  `docs/coverage/foundation.md` item 6). An address group, the ranges
  that firewall policy rules name as `srcAddressGroups` or
  `destAddressGroups`, owned by an organization (for its hierarchical
  firewall policies) or by a project. Named by decision 18: Config
  Connector's name in our group, with the parent a field, as Terraform's
  `google_network_security_address_group` takes `parent`
  (`organizations/{id}` or `projects/{id}`). Upstream (read 2026-11-09,
  pin `672414b8`): Config Connector's direct `NetworkSecurityAddressGroup`
  is `v1alpha1` and project-only, and no pull request adds the
  organization; the API's `OrganizationAddressGroupService` has the same
  methods and messages as the project's `AddressGroupService`.
  **Copied**: the types, identity (and its test), controller, fuzzer and
  the address group's functions of `mapper.generated.go` (which upstream's
  `controllerbuilder` generated); `hack/scaffold-kind.sh` was not used,
  since upstream already has the kind and the copy is nearer than a fresh
  scaffold. Changed: `organizationRef` beside `projectRef`, exactly one
  (CEL); the identity takes either parent; an organization's group goes
  through the `OrganizationAddressGroupService` client (its operations
  are the same Go types); `capacity` is immutable (CEL), as Magic
  Modules' `AddressGroup.yaml` (`aad42de6`) marks it, where upstream sends
  the change and lets the API answer. A changed parent is refused (the
  identity against `status.externalRef`). Share of the 1,238 non-test
  lines: 42% generated (controller-gen's CRD and deepcopy here, 36%;
  upstream's `controllerbuilder` mapper, 6%), 49% copied unchanged, 9%
  written or changed here. Patch 0025: mockgcp serves the organization
  service. Live: a project's address group passes (run
`cce-11092303-4b16`, no difference from the fake); an
  organization's is "live: pending, needs test folder" (it needs
  organization access, and a live run never touches the organization).

- `NetworkSecuritySecurityProfile` (`networksecurity.gcp.hfl.dev/v1alpha1`,
  `docs/coverage/foundation.md` item 6). A security profile: what a
  firewall policy rule with `apply_security_profile_group` does with
  threats (severity, threat and antivirus overrides), URLs (filters), or
  where it mirrors or intercepts packets, owned by an organization or a
  project. Named by decision 18, as the address group is; Terraform's
  `google_network_security_security_profile` takes `parent`
  (`organizations/{id}` or `projects/{id}`). Upstream (read 2026-11-09,
  pin `672414b8`): Config Connector's direct kind is `v1alpha1` and
  project-only, no pull request adds the organization, and its
  controller dials gRPC (`GRPCClientOptions`), which our manager does
  not configure. **Copied**: the types, identity (and its test),
  reference, controller, fuzzer and the profile's functions of
  `mapper.generated.go` (upstream's `controllerbuilder` output); the
  intercept and mirroring endpoint group references stay Config
  Connector's kinds. Changed: `organizationRef` beside `projectRef`,
  exactly one (CEL); the REST clients, the organization service's for an
  organization parent; an unchanged object still gets its status. From
  Magic Modules' `SecurityProfile.yaml` (`aad42de6`): `type` is
  required, an enum and immutable (CEL), at most one profile is set
  (CEL; the proto's `oneof`), the override and filter lists compare as
  sets, and the URL filter of priority 2147483647 that the API adds is
  not a difference unless the spec names it (its `url_filters`
  flattener). Not carried: `wildfireAnalysisProfile` and
  `customMirroringProfile.mirroringDeploymentGroups`, which the pinned
  `networksecurity` v1 client (v0.17.0) does not have (decision 5). Share
  of the 2,009 non-test lines: 55% generated (controller-gen's CRD and
  deepcopy here, 42%; upstream's `controllerbuilder` mapper, 13%), 33%
  copied unchanged, 12% written or changed here. Patch 0026: mockgcp
  serves the organization service for profiles and profile groups with
  its own server (the project service's parses only project names).
  Live: a project's profile passes (run `cce-11092339-c43e`, no
  difference from the fake: create, a resync with no patch for a threat
  prevention and a URL filtering profile, one patch for an override and
  description change, delete and abandon; the account's
  `roles/compute.networkAdmin` was enough); an organization's is "live:
  pending, needs test folder" (it needs organization access, and a live
  run never touches the organization).

- `NetworkSecuritySecurityProfileGroup` (`networksecurity.gcp.hfl.dev/v1alpha1`,
  `docs/coverage/foundation.md` item 6, its last kind). The group a
  firewall policy rule applies (`securityProfileGroup` with action
  `apply_security_profile_group`): one profile of each type, by
  reference to our `NetworkSecuritySecurityProfile`. Owned by an
  organization or a project, as Terraform's
  `google_network_security_security_profile_group` takes `parent`
  (decision 18). Upstream (read 2026-11-09, pin `672414b8`): `v1alpha1`,
  project-only, gRPC, as the profile. **Copied**: the types, identity
  (and its test), controller, fuzzer and the group's functions of
  `mapper.generated.go`; the controller in the shape of our profile's
  (the two upstream controllers differ only in names). Changed:
  `organizationRef` beside `projectRef`, exactly one (CEL); the REST
  clients, the organization service's for an organization parent; an
  unchanged object still gets its status. Magic Modules'
  `SecurityProfileGroup.yaml` (`aad42de6`) marks only the parent,
  location and name immutable, as the identity check already does. Not
  carried: `wildfireAnalysisProfile` (decision 5, as for the profile).
  Share of the 1,232 non-test lines: 48% generated (controller-gen's CRD
  and deepcopy here, 42%; upstream's `controllerbuilder` mapper, 6%),
  38% copied unchanged, 14% written or changed here (the sweeper's
  groups-then-profiles pass among them). Patch 0026, from the profile's
  PR, serves an organization's groups. Live: a project's group passes
  (run `cce-11092345-9b77`, no difference from the fake: the API
  returns a profile reference as it was sent, so a resync writes
  nothing); an organization's is "live: pending, needs test folder".

- `ComputeRouter` (`compute.gcp.hfl.dev/v1alpha1`, decision 14,
  `docs/coverage/foundation.md` item 9). A whole Cloud Router in one
  object: BGP (with `identifierRange`), interfaces, BGP peers (BFD,
  custom learned ranges, IPv4 and IPv6 next hops, import and export
  route policies, MD5 key names), Cloud NATs (with `autoNetworkTier` and
  NAT64) and the MD5 authentication keys. Named by decision 18:
  Config Connector's name in our group. Upstream (read 2026-11-09, pin
  `672414b8`): the direct `ComputeRouter` has only the description, the
  BGP basics, `encryptedInterconnectRouter`, the network and region; PR
  #13960 (head `9b916158`, open, review required) changes only its
  migration diff. The interfaces, peers and NATs are upstream's
  Terraform-based (and, for NAT and interface, direct) sibling kinds,
  without the fields listed in item 9. **Generated**:
  `hack/scaffold-kind.sh google.cloud.compute.v1
  compute.gcp.hfl.dev/v1alpha2 ComputeRouter:Router` (v1alpha2 because
  upstream has compute v1alpha1; the compute proto has no field
  behaviour or resource annotations, so no overlay). Kept unedited: the
  BFD, custom learned range and NAT64 subnetwork types and their mapper
  functions; the generated `RouterBgpPeer` field set and mapper are the
  base of ours, renamed to Config Connector's `ComputeRouterPeer` field
  names. **Copied**: the direct `ComputeRouter` types, identity and
  controller, and upstream's router, NAT and interface mappers (for our
  inline NAT and interface types, which reuse upstream's NAT rule, log
  config, subnetwork and reference types). **Changed or written**:
  - the router is written with `routers.update` (PUT): the body is the
    router as read with the owned fields replaced, because
    `routers.patch` keeps a list the body leaves out and the client
    leaves an empty list out (the firewall rule's lesson), so a removed
    peer, NAT, interface or key would stay. Peers and interfaces the API
    made for an interconnect attachment (`MANAGED_BY_ATTACHMENT`) are
    kept unless the spec names them. Live runs showed the API takes one
    feature per write ("Creating a router with multiple features is
    currently unsupported", then "Updating multiple Router features is
    currently unsupported"), so the router is inserted bare and then
    written one feature at a time, as Terraform's router, interface,
    peer and NAT resources each patch only their own: changed key
    values, the base fields, the interfaces (keeping those a remaining
    peer is on), the peers with their keys, the interfaces as the spec
    has them, the NATs; a step with nothing to change is skipped;
  - the comparison is written here: what the spec sets must match, what
    it leaves out is the API's default (Magic Modules: `keepaliveInterval`
    20, `identifierRange` from the API; Terraform's peer: BFD, `enable`,
    `ipAddress` computed), the four lists must have the same members in
    any order, route policy lists in order, links without their URL
    prefix;
  - MD5 key values come only from Secrets (`keyRef`) and are never
    logged or kept; they are sent only in the steps that write the keys
    (a live run: the API takes a key sent with its value as a change of
    the keys, so a NAT-only update carrying them was refused), and the
    other steps send the keys as read, by name, which keeps their values; the API never returns them
    (Terraform's peer flattener; patch 0027 makes the fake do the same),
    so `status.md5AuthenticationKeysVersion`, a digest of each key's
    Secret name, key and `resourceVersion` (not of the value), shows when
    a Secret changed and the keys are written again;
  - `privateIpAddress` and `redundantInterface` are the API's strings
    (upstream maps an address reference into a field that takes an IP);
    VPN tunnel and attachment references by name read the Config
    Connector object's `status.selfLink`; the network and
    `encryptedInterconnectRouter` are immutable (an error, not a write).
  Share of the ~3,450 non-test lines: about 52% generated (controller-gen's
  CRD and deepcopy, 48%; `controllerbuilder`, 4%), 27% copied unchanged,
  21% written or changed here. `ComputeRouterRoutePolicy`'s Config
  Connector router GVK variable is renamed `ConfigConnectorComputeRouterGVK`
  (the name is now our kind's). Not carried: `nccGateway` (a router for
  an NCC gateway spoke) and the input-only `params.resourceManagerTags`;
  `task coverage` counts 76 of the Router's 81 fields covered with ours,
  the MD5 key's value (ours is `keyRef`) and the NAT subnetworks' names
  (ours are references) among the five it does not match. Live: passes (run
  `cce-11100336-0ecc`) after four live runs that each found what the fake
  did not show: the one-feature rules for insert and update, and the
  key value counted as a change; each is now in patch 0027. The rotated
  key's value and the `MANAGED_BY_ATTACHMENT` path cannot be seen live
  either (the API returns no key values; the test makes no
  attachment).

- `ComputeProjectMetadataItem` (`compute.gcp.hfl.dev/v1alpha1`, rank 10,
  per-key metadata). **Written here** after Terraform's
  `google_compute_project_metadata_item` (read, not copied): `projectRef`,
  `resourceID` (the key; `metadata.name` by default), `value`. The
  controller reads the project's common instance metadata, changes the
  one key, writes it all back with the fingerprint it read, and reads
  again and retries on a 412 from another writer; delete removes only its
  key. Unlike Terraform, which refuses a key that is already there, a key
  already there is taken over, as Config Connector takes over existing
  resources. The API has no resource name for one key, so
  `status.externalRef` is ours: `projects/{project}/commonInstanceMetadata/{key}`.
  mockgcp already serves `projects.get` and `setCommonInstanceMetadata`;
  no patch.
  Live: passes, with `roles/iam.serviceAccountUser` on the project
  (`setCommonInstanceMetadata` checks `actAs` there; `docs/live-runs.md`).
- `IAMBinding` (`iam.gcp.hfl.dev/v1alpha1`, rank 1; built as
  `IAMRoleBinding`, renamed by decision 12). **Written here**:
  `resourceRef` (`kind`, `name` or `external`, shaped like
  `IAMPolicyMember`'s), `role`, `condition` (title, description,
  expression), `members` (strings as the API writes them) and
  `membersFrom[].serviceAccountRef`. It owns one role under one condition
  on one resource: members added out of band are removed at the next
  reconcile, every other role (and the same role under another condition),
  the audit configs and the etag are carried through unchanged, and delete
  removes only that binding (`deletion-policy: abandon` leaves it).
  `resourceRef`, `role` and `condition` are immutable (CEL). Two objects
  for the same role, condition and resource would overwrite each other, as
  two Terraform `google_*_iam_binding`s do.
  The controller does not use Config Connector's IAM client, which needs
  the Terraform provider and service mappings this manager does not load
  (section 2.1). `pkg/iampolicy` reads and writes a policy over HTTP: one
  table of resource types (`registry.go`) says, for each `kind`, the API
  host and version, whether the policy is read with GET or POST, the
  pattern of the resource's full name, how a bare ID becomes one
  (`external`) and how a referenced object becomes one (`name`: its
  `status.externalRef` when it sets it, else the project, `spec.location`
  and `spec.resourceID` or the object's name). Two protocols: the
  `google.iam.v1` mixin (`{resource}:getIamPolicy`, `:setIamPolicy`) and
  Cloud Storage's `GET`/`PUT .../iam`. The policy is read at version 3,
  changed, and written back with its etag; a 409 or 412 reads and tries
  again (five times). A member is compared as the API writes it (the
  address of a `user:`, `group:`, `serviceAccount:` or `domain:` member is
  lower-cased), in any order. A referenced object that is missing is "not
  found", one whose Ready condition is not True is "not ready", and the
  reconciler waits and retries; while the binding is being deleted, a
  missing object means there is nothing to remove. A kind that is not in
  the table is refused with the list of kinds.
  Kinds in the table (2026-11-09): `Project`, `Folder`, `Organization`,
  `IAMServiceAccount`, `StorageBucket`, `SecretManagerSecret` (global and
  regional; not in `IAMPolicyMember`), `ArtifactRegistryRepository`,
  `RunService`, `PubSubTopic`, `PubSubSubscription`, `KMSKeyRing`,
  `KMSCryptoKey`. A `Folder`, `Organization` or `KMSCryptoKey` is named by
  `external`; the others also by `name`. The table grows by one row per
  resource type, with no change to the kind. `logSinkRef`,
  `sqlInstanceRef` and the other `memberFrom` sources of `IAMPolicyMember`
  are not offered; `members` takes the address.
  Patch 0018: mockgcp serves Cloud Storage's `.../iam` paths from the
  store it already keeps for the `:getIamPolicy` verbs.
  Live: passes, on a secret and a bucket (`docs/live-runs.md`).
- `IAMMember` (`iam.gcp.hfl.dev/v1alpha1`, rank 2; built as
  `IAMRoleMember`, renamed by decision 12). **Written here**,
  the additive sibling of `IAMBinding`, shaped like Config Connector's
  `IAMPolicyMember`: `resourceRef` (the same type as `IAMBinding`'s),
  `role`, `condition`, and one of `member` or `memberFrom.serviceAccountRef`
  (CEL: exactly one). Every field is immutable, as in `IAMPolicyMember`.
  It adds its member to the role's binding (`AddMembers`) and removes only
  that member on delete (`RemoveMembers`, and the binding if it was the
  last): other members of the role, other roles and the audit configs are
  carried through unchanged, so several objects can give one role to
  several members and anyone else's grants stay. `Find` is "the member is
  in the binding"; a member taken off out of band is put back at the next
  reconcile, a member somebody else added is left. `status.externalRef` is
  ours: `{resource}#{role}#{member}`, plus `#{condition title}`.
  It shares everything else with `IAMBinding` (`pkg/iampolicy`,
  `policy_common.go`), and the resource table grew by the rows
  `IAMPolicyMember` cannot reference, which both kinds accept (marked
  `NotInIAMPolicyMember` in `registry_more.go`: 30 rows, and the regional
  `SecretManagerSecret` that was already there; the catalog counts 32 by
  its own grouping, and a type missing is a row to add): `StorageManagedFolder`
  (Cloud Storage's own paths with the folder's name escaped whole and
  ending in a slash), `LoggingLogView`, `RunWorkerPool`, `PrivateCACAPool`,
  `CloudDeployDeliveryPipeline`, `CloudDeployTarget`, `DataformRepository`,
  `DataplexLake`, `DataplexZone`, `DataplexDataScan`, `DataplexAspectType`,
  `WorkstationConfig`, `Workstation`, `BigQueryAnalyticsHubDataExchange`,
  `BigQueryAnalyticsHubListing`, `APIGatewayAPI`, `APIGatewayAPIConfig`,
  `APIGatewayGateway`, `BinaryAuthorizationAttestor`,
  `BigQueryConnectionConnection`, `SecureSourceManagerInstance`,
  `SecureSourceManagerRepository`, `DataCatalogTagTemplate`,
  `CloudBuildConnection`, `WorkbenchInstance`, `EndpointsService`,
  `IAPTunnelInstance`, `IAPWebCloudRunService`, and, with the third
  protocol, `ComputeBackendService` (global and regional) and
  `ComputeMachineImage`. The kinds Config Connector has can be named by
  `name` (the project, `spec.location` and the ID make the full name, or
  `status.externalRef` when it already is one); the others by `external`.
  **Compute Engine's protocol** is `GET {resource}/getIamPolicy?optionsRequestedPolicyVersion=3`
  and `POST {resource}/setIamPolicy` with `{"policy": ...}`, as Terraform
  sends it (the handoff's `{bindings, etag, policy}` body is the same
  request, whose `policy` field is the one used). Patch 0019: mockgcp
  serves those paths from the same store as the verbs.
  **Which rows are proven.** Three are exercised end to end, on the fake
  cloud and on a real project: a regional `SecretManagerSecret`, a
  `StorageManagedFolder` and a `ComputeBackendService`. The others are
  written from each service's REST reference and Terraform's matching
  `google_*_iam_*` resources (read, not copied) and checked only for their
  names and URLs (`registry_test.go`); none has been called against the
  real API. A wrong path, verb (GET or POST for the read) or version shows
  as the API's own error on the object's Ready condition, and is a
  one-row fix. They are listed as such so that nobody relies on them
  before a live run.
  Live: passes, on a regional secret, a managed folder and a backend
  service (`docs/live-runs.md`).
Below the top ten, each needed by one module (Looker instances, Endpoints
services, Cloud Build v2 repositories, regional instance templates, and
others in the inventory): not ranked here yet. Container Registry is
deprecated upstream and is not built.

## 7. What is left

Before config-connector-extensions can run on a real cluster:

1. Images are built locally by ko (`config/default` uses `ko://`
   references). A registry, signing and a release version scheme are not
   decided.
2. Done: install manifests (`config/default`) and namespaced identity
   matching Config Connector's (section 9).
3. Proven only on kind: Workload Identity itself (section 9.3).
4. Secret change tracking for `spec.password` (Config Connector's
   observed-secret-versions), and watching referenced Secrets.
5. Validation webhook parity: Config Connector's webhooks enforce
   immutability (`host`, `type`, `resourceID`); ours relies on the
   controller erroring. Either CEL `x-kubernetes-validations` in the CRD
   or a webhook.
6. Done: the managers' own Kubernetes service account, the default, with
   sharing Config Connector's as an opt-in (section 9.2).

To raise confidence in the kinds:

7. Port Config Connector's `SQLUser` fixtures (`sqluser`, `sqluser-full`)
   to our group and run its golden harness, recording `_http.log`.
8. Done (2026-11-09, `docs/live-runs.md`): against a real Cloud SQL
   Postgres 16 instance, `databaseRoles` on insert, adding a role and
   revoking one all match mockgcp, and `users.get` returns the roles. Not
   yet run live: an empty list (revoke everything).
9. Unit tests for `passwordPolicyEqual`, `sortedRoles` and identity
   parsing (Config Connector has an identity conformance helper,
   `apis/common/identity`).
10. Offer patches 0001 and 0002 upstream; comment on PR #7148 with the
    revoke semantics and the mockgcp change.

Housekeeping:

11. Done: the SQLUser test runs on `pkg/test/mockenv`, so it runs live too.
12. CI: `task check` on Linux (the 127.0.0.2 default then works without
    patch 0002, but the patch stays harmless).
13. `tests/operator/run.sh` does not clean the fake cloud on a reused kind
    cluster; run it on a fresh one.

To raise confidence in `ArtifactRegistryRepository`:

14. Done (2026-11-09, `docs/live-runs.md`): the API refuses a change of a
    custom upstream's URI ("Remote source configuartion cannot be
    updated"; patch 0016 makes mockgcp answer the same), so the object goes
    not Ready and the repository keeps its URI; Terraform replaces the
    repository instead. Whether ours should replace it too is not decided.
    The API also refuses, at creation, a remote upstream it cannot reach,
    unless `disableUpstreamValidation` is set; mockgcp does not check.
15. Watch PR #13517 (UPSTREAM.md, "Copied from open pull requests"); when
    it merges and a release ships it, retire ours (section 3.4).
16. The workload identity kinds against a real project: the API's
    errors for a missing pool, an ID still soft-deleted, and a namespace
    owned by a Google service (`ownerService`), none of which mockgcp
    models, and which `googleCloudResource` forms the API accepts in an
    attestation rule; then correct mockgcp.
17. `RunService` against a real project: which server defaults the real
    API fills in beyond mockgcp's (ports, startup probe, traffic), each of
    which must be taken from what was read or it shows as a difference on
    every resync; that the API accepts a top-level update mask for
    `template` and `invokerIamDisabled`; and a GPU service actually
    scheduling with `nodeSelector` and `gpuZonalRedundancyDisabled`.
18. `ArtifactRegistryTag` against a real project: a tag on a version that
    exists, the API's error for one that does not, and a package whose ID
    has a "/" (`%2F` in the name), which the fake test leaves out.
19. `IAMWorkloadIdentityPoolProviderKey` against a real project: the key's
    real certificate and validity window, its use from a SAML provider, the
    API's error for a provider that does not exist or is not SAML, and the
    soft-deleted ID's reuse; then correct mockgcp.
20. `ComputeRegionNetworkEndpoint` against a real project: the API's checks
    by group type (an IP address in an `INTERNET_FQDN_PORT` group, a VM in
    another zone), which mockgcp does not make, and the endpoint's identity
    when the group holds several with the same address and port.
21. `ComputeRouterRoutePolicy` against a real project: the API's checks of
    the CEL expressions and priorities, which mockgcp does not make, and a
    write with a stale fingerprint.
22. `StorageBucketObject`: watch the referenced Secret or ConfigMap so a
    change is uploaded at once, not at the next reconcile; and, against a
    real project, a resumable upload for content near the size limit
    (mockgcp serves media and multipart only).
23. `ComputeRegionNetworkFirewallPolicy` against a real project: whether a
    patch with a stale fingerprint is refused, whether `description: ""`
    clears the description, and deleting a policy that is still
    associated with a network (mockgcp checks none of these).
24. `LoggingSettings` against a real organization and folder: the
    service accounts' real names, the API's checks of the key (it must
    exist and the Log Router's account must be able to use it) and of the
    location, and `defaultSinkConfig` (newer than mockgcp's protos).
25. `ComputeProjectMetadataItem`: the 412 retry is not exercised by the
    fake (its writers do not race the way real ones can); the default
    network tier and default service accounts, rank 10's other parts.
26. `IAMBinding` and `IAMMember`: live runs of the rows in
    `pkg/iampolicy/registry_more.go` that have only been checked for
    their names and URLs (each service's quirks: GET or POST for the read,
    the API version, the policy version it accepts; a BigQuery dataset has
    no IAM policy, only `access`); `memberFrom` sources beyond a service
    account; and a check that the policy's etag conflict is seen live (the
    fake serves 409; the API's real answer is 409 for most services and
    412 for Compute Engine).
27. `ComputeRegionNetworkFirewallPolicyRule` against a real project: the
    API's checks of a rule (`goto_next` with logging, secure tags together
    with target service accounts, FQDNs or region codes on the wrong
    direction, the policy's tuple limit), which mockgcp does not make;
    `securityProfileGroup`, `tlsInspect` and `targetResources`, the
    fields this kind lacks; how long a changed rule stays "not ready".
28. `ComputeRouter`: a rotated MD5 key Secret is noticed at the next
    reconcile (Config Connector's resync interval), not when the Secret
    changes (no watch, as item 22); peers and interfaces the API manages
    for an interconnect attachment are kept on update but no test makes
    one (mockgcp never does); `ComputeRouterRoutePolicy.routerRef.name`
    still names Config Connector's `ComputeRouter`, so a policy on a
    router that moved to ours names it by `external`; no CEL rule yet for
    "at most one of `vpnTunnelRef`, `interconnectAttachmentRef` and
    `subnetworkRef`" (the API refuses it); the Terraform-based
    `ComputeRouterInterface`, `ComputeRouterPeer` and `ComputeRouterNAT`
    on the same router would fight ours, so moving a router means
    abandoning all four official objects.

## 8. Decisions

1. **Upstream's unmerged `SQLUser` (PR #7148)**: use it once it does what
   this kind needs, and keep it easy to update from upstream. It does not
   yet (`databaseRoles` is `/*NOTYET*/`), so ours stays;
   `hack/upstream-pr.sh` follows the PR's head and section 2 ("Switching
   to PR #7148") says how to switch.
2. **Roles replace.** The list is the whole set; a role taken off it is
   revoked.
3. **Status field name**: `status.appliedDatabaseRoles` stays until a run
   against real Cloud SQL shows whether `users.get` returns roles
   (section 7, item 8).
4. **Managers follow Config Connector's mode.** In cluster mode one manager
   watches the whole cluster, because Config Connector has one Google
   identity there. In namespaced mode there is one manager per
   `ConfigConnectorContext`, watching only its namespace (section 9).
5. **Field gaps on large existing kinds** wait for Config Connector's
   release, with the exception in section 6.1. Never a fork. Exceptions
   the builder has made: decisions 11 and 14 (and 13, 16 for whole
   kinds). A parallel kind copied from an upstream migration pull
   request, owning the whole object, is now the accepted pattern when a
   foundation chart is blocked; a deprecation path for our kinds, for
   when upstream ships theirs, will be designed separately (the builder's
   sidebar, 2026-11-09) and is not built.
6. **The fake-cloud switch exists only in test builds.**
   - `--fake-cloud-url` and `pkg/fakecloud` are compiled into the manager
     only with `-tags fakecloud` (`cmd/manager/fakecloud.go`).
   - A production build has no such flag and refuses to start with it
     ("flag provided but not defined").
   - `hack/check-no-fake-cloud.sh`, run by `task check`, fails if the
     default build links `pkg/fakecloud` or shows the flag.
   - The kind test builds its images with `GOFLAGS=-tags=fakecloud`.
7. **Licence**: Apache-2.0 (`LICENSE`, `NOTICE`).
8. **`upstreamCredentials`: build it now** (the builder, 2026-11-09,
   answering section 10's question 1 of 2026-11-08). The remote
   repository's `upstreamCredentials` and `disableUpstreamValidation` are
   written here from the API and Terraform's behaviour (section 6.4).
9. **Rank 9: build it now** (the builder, 2026-11-09, answering section
   10's question 2): `IAMWorkloadIdentityPoolNamespace` first, copying PR
   #14209's types, a direct controller written here on
   `google.golang.org/api/iam/v1`, and a minimal mockgcp patch; then
   managed identities (#14418) if they follow cleanly.
10. **Then rank 8, and on down the list** (the builder, 2026-11-09:
    "carry it on in rank order as far as you can go … Chew through as
    many as you can do and try to get some confidence that we've built a
    working operator"). This overrides decision 5's "wait for the release"
    for rank 8 (`RunService` `invokerIamDisabled` and GPUs), to be built
    the smallest workable way rather than as all of `RunService`; then the
    unranked candidates in section 6.3, top to bottom, skipping any that
    Config Connector already has.
11. **Rank 8 as a narrow direct `RunService` that owns the whole service**
    (2026-11-09; the builder's coordinator agreed). Two ways were weighed:
    - a field patch beside Config Connector's `RunService`, re-applying
      `invokerIamDisabled` (and the GPU fields) after Config Connector's
      updates. Rejected: Config Connector's `RunService` is Terraform-based,
      and Terraform's update sends the whole service with no update mask,
      so every change to it (every deploy) resets the fields until the
      patch runs again; for GPUs the template would flip between revisions
      with and without the accelerator, and a service whose GPU limit has
      no node selector may not be creatable at all;
    - **chosen:** our own `run.gcp.hfl.dev/v1alpha1` `RunService`, copied
      from Config Connector's direct `RunWorkerPool` (the nearest upstream
      code: same API family, a revision template with containers,
      volumes, VPC access, service account and `nodeSelector`), owning the
      whole service. It updates with an update mask of the top-level fields
      that differ, so fields it does not model are left alone. A workload
      that needs `invokerIamDisabled` or a GPU moves its service to this
      kind (acquire by `resourceID`, delete the Config Connector object with
      `deletion-policy: abandon`); others stay on Config Connector's.
12. **IAM kinds are named as Terraform's Google provider names them:
    `IAMBinding` and `IAMMember`** (the builder, 2026-11-09, through the
    coordinator: "can we use Terraform as an example of how this should be
    named? I think it does rather well in the GCP providers"). Terraform's
    `google_<resource>_iam_binding` is authoritative for one role and
    `google_<resource>_iam_member` is additive, and ours follow that.
    Each is one generic kind in `iam.gcp.hfl.dev/v1alpha1` with a resource
    reference. `IAMBinding` is one role, an optional condition and a list
    of members on one resource, authoritative for that role and
    condition. `IAMMember` is one member of one role on one resource,
    additive. The two were first built as `IAMRoleBinding` (the builder
    first said it "sounds like the right name") and `IAMRoleMember` (a
    working name, section 10 question 3). They were renamed before
    anything used them, so nothing needed migrating.
    `IAMPolicyBinding`, the catalog's suggestion, is not used: Config
    Connector's open PR #14133 gives that name to IAM v3's principal
    access boundary `PolicyBinding`. The builder prefers the
    non-authoritative form ("I think I would prefer the non-authoritative,
    but that's fine if authoritative is what you've built"), so Config
    Connector's `IAMPolicyMember` stays the default in everything written
    here, and rank 2 is built additive (section 6.5).
13. **Alpha-only foundation kinds: ship our own stable kinds** (the
    builder, 2026-11-09, answering `docs/coverage/foundation.md`: "Ship
    our own stable kinds."). IAM deny policies, principal access
    boundary, hierarchical security policy, network firewall rules and
    DNS response policies: copy upstream's alpha types and controllers
    into our group as stable kinds. When upstream promotes, users move
    over by acquiring the same resource (as with `RunService`, decision
    11).
14. **Cloud Router: copy upstream's PR as our kind** (the builder,
    2026-11-09: "Copy upstream's PR as our kind."). Copy PR #13960's
    direct `ComputeRouter` into our group as a complete kind that owns
    the whole router (BGP peer fields, NAT, interfaces, MD5 keys). A
    workload that needs these fields moves its router to ours (acquire by
    name, abandon the official object); ours is retired when upstream
    releases.
15. **Billing account log sink: build `LoggingBillingAccountSink` now**
    (the builder, 2026-11-09). Its live test needs billing-account
    access the builder has not granted: mark it "live: pending, needs
    billing account access" and prove it on the fake.
16. **Security Command Center at folder and project level: build ours
    with all three parents** (the builder, 2026-11-09: "Build ours with
    all three parents"), organization, folder and project, accepting that
    it duplicates what upstream is building and widens the kinds to
    retire later.
17. **`ProjectDefaultServiceAccounts` acts on every reconcile and keeps
    conditional bindings** (the orchestrator, 2026-11-09, accepting the
    two choices section 6.5 records for the kind). Terraform's
    `google_project_default_service_accounts` acts once, at create, and
    its read does nothing. Ours acts on every reconcile, because a
    Kubernetes object is desired state held continuously, not a one-off
    command: a default account re-enabled by hand, or one that appears
    when App Engine is turned on later, is disabled, deleted or
    deprivileged again, and status only grows. `DEPRIVILEGE` reads and
    writes the project's IAM policy as version 3, where Terraform reads
    version 1; a version 1 read drops the conditions of conditional
    bindings, and writing that back would lose them, so ours keeps them.
18. **A new kind is named as Config Connector names it, else as
    Terraform's Google provider does** (the builder, 2026-11-09, on
    naming: "can we use Terraform as an example of how this should be
    named? I think it does rather well in the GCP providers"). A new kind
    takes Config Connector's name for the resource where Config Connector
    has one, otherwise the Terraform google provider's resource name in
    CamelCase, in our group. Parents (organization, folder, project) are
    reference fields on one kind, not separate kinds per parent, as
    Terraform's `parent` argument is. So organization address groups are
    `NetworkSecurityAddressGroup` in `networksecurity.gcp.hfl.dev`, taking
    an organization or a project as its parent as
    `google_network_security_address_group`'s `parent` does, not
    `NetworkSecurityOrganizationAddressGroup`, and the security profile
    kinds follow the same rule. This generalises decision 12 (the IAM
    kinds) and the folder and organization `ServiceIdentity`, which kept
    upstream's name with new parents.

## 9. The operator: Config Connector's identity, per namespace

The goal: install the CRDs and run the managers so that a gap kind acts
with the same Google identity Config Connector uses in the same
namespace.

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

Workload Identity is bound to a namespace and a service-account name. Two
ways give "the same auth in the same namespace":

- **Its own service account (the default, `--identity=own`).** The manager
  runs in Config Connector's namespace as its own Kubernetes service
  account, which the operator creates with the same
  `iam.gke.io/gcp-service-account` annotation as Config Connector's
  (copied from it), and binds only to this project's roles. The Google
  service account is the same; the Kubernetes rights are ours alone. It
  costs one more Workload Identity binding per Google service account
  (below). With a key Secret it costs nothing: the same Secret is mounted
  the same way.
- **Config Connector's service account (opt-in, `config-connector`).** The
  manager runs as Config Connector's own service account. No extra
  binding, but the manager then holds every Kubernetes right Config
  Connector's account has (section 9.4). Chosen by `--identity=config-connector`
  on the operator, or per scope by the annotation
  `gcp.hfl.dev/identity: config-connector` on the `ConfigConnector` object
  (every manager) or on one `ConfigConnectorContext` (that namespace's
  manager). The annotation `gcp.hfl.dev/identity: own` opts a scope back. A
  context's annotation wins over the `ConfigConnector` object's, which wins
  over the flag; any other value is refused.

`pkg/operator`:

- watches `ConfigConnector`, `ConfigConnectorContext`, Config Connector's
  service accounts, its own service accounts and its own Deployments, and
  in one pass applies what should run and deletes what it made that
  should not;
- cluster mode: Deployment and service account
  `cnrm-system/config-connector-extensions-manager`; with a key, the same
  Secret mounted the same way;
- namespaced mode: Deployment and service account
  `config-connector-extensions-manager-N` with `--scoped-namespace=N` and
  the context's project policy flags, in `cnrm-system` or (dedicated) the
  context's manager namespace, annotated with the context's
  `googleServiceAccount` (copied from `cnrm-controller-manager-N`);
- without a `ConfigConnector` object (Config Connector from its install
  bundle): cluster mode, Workload Identity if `cnrm-controller-manager`
  carries the annotation, else the bundle's `gcp-key` Secret;
- waits for Config Connector's service account for the scope before it
  makes anything: until it exists Config Connector is not set up there,
  and there is no identity to copy;
- binds the manager's service account to the managers' roles
  (`config/operator/rbac.yaml`): `config-connector-extensions-manager` (our
  kinds, read Secrets, events, read the Config Connector kinds ours
  reference) cluster-wide in cluster mode or by RoleBinding in N alone,
  and `config-connector-extensions-manager-cluster` (CRDs,
  `ConfigConnector`, namespaces) cluster-wide;
- itself runs in `config-connector-extensions-system` as its own service
  account, holds no Google credentials, never changes Config Connector's
  service accounts, writes only the service accounts it labels as its own
  (RBAC cannot limit `create` by name, so its role allows service-account
  writes in every namespace), and may bind only those two roles (RBAC
  `bind` by name).

**The one step outside the cluster, for Workload Identity.** Allow the
manager's service account to act as the Google service account Config
Connector uses for the scope. `<cluster-project>` is the project that owns
the cluster's workload identity pool, `<gsa>` the Google service account:

```sh
# cluster mode
gcloud iam service-accounts add-iam-policy-binding <gsa> \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:<cluster-project>.svc.id.goog[cnrm-system/config-connector-extensions-manager]"

# namespaced mode, for the context in namespace N (its googleServiceAccount)
gcloud iam service-accounts add-iam-policy-binding <gsa> \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:<cluster-project>.svc.id.goog[cnrm-system/config-connector-extensions-manager-N]"
```

Under dedicated isolation the namespace in the member is the context's
`spec.managerNamespace`. The operator logs the member it needs when it
first creates a manager's service account.

**The symptom when it is missing.** The operator cannot see it (it holds
no Google credentials), and the manager starts normally. Each gap object
then fails to reconcile: its `Ready` condition is `False` with a message
from the GKE metadata server that it could not get an access token for
the Google service account (`iam.serviceAccounts.getAccessToken`
permission denied, HTTP 403). Config Connector's own objects in the same
namespace keep working, which is the tell. Add the binding; the next
reconcile succeeds.

A static bundle was the alternative to an operator: cheaper, but it can
only cover cluster mode, because namespaced mode's managers depend on which
contexts exist.

### 9.3 What was proven (2026-11-07)

`pkg/operator` against envtest (`task test`): cluster mode with Workload
Identity (its own service account with Config Connector's annotation,
bound to our roles; Config Connector's service account left unchanged)
and with a key (the same Secret, no annotation); the switch to namespaced
mode (the cluster manager, its service account and bindings go, one
manager per context comes, with its own service account carrying the
context's Google service account and its RoleBinding only in its
namespace, and the billing-project flags); a context removed takes its
manager, service account and bindings; the opt-in to share Config
Connector's service account, on the `ConfigConnector` object and back; the
`--identity=config-connector` flag with one context opting back to its
own; an unknown identity refused; no manager before Config Connector's
service account exists; Config Connector without its operator.

`tests/operator/run.sh` (`task operator-e2e`) on kind 0.33 (Kubernetes
1.37) with the in-cluster fake cloud (`cmd/fakegcp`; managers get
`--fake-cloud-url`, which loads no Google credentials and since section
9.4 exists only in builds with `-tags fakecloud`), images built by ko
0.19.1 for linux/arm64, on a fresh cluster, 2026-11-08:

```text
ok: CRDs installed: secretmanagersecrets, secretmanagersecretversions, sqladminbackups, sqlusers (*.gcp.hfl.dev)
== cluster mode: one manager in cnrm-system as its own service account with Config Connector's identity
ok: manager pod runs as cnrm-system/config-connector-extensions-manager, mapped like Config Connector's to kcc@mock-project.iam.gserviceaccount.com
ok: the manager may manage gap kinds, and may not create Secrets, change ConfigConnector or create webhooks
ok: Config Connector's service account has no rights on gap kinds
== cluster mode: a regional secret and its version become Ready
ok: Ready: projects/mock-project/locations/us-central1/secrets/app-db-password
== the opt-in: share Config Connector's service account, then back
ok: shared: the manager runs as cnrm-controller-manager, its own service account is gone
ok: back to its own service account; Config Connector's has no rights on gap kinds again
== namespaced mode: Config Connector's operator moves to one manager per context
ok: team-a's manager runs as cnrm-system/config-connector-extensions-manager-team-a, mapped like Config Connector's to cnrm-system-team-a@mock-project.iam.gserviceaccount.com, with --scoped-namespace=team-a
ok: team-a's manager may manage gap kinds in team-a and not in team-b; Config Connector's team-a account has no rights on them
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
manager pod runs as its own service account in Config Connector's
namespace, carrying exactly Config Connector's Google service account for
the scope, with only this project's Kubernetes rights, so on GKE one more
binding (section 9.2) gives it Config Connector's identity. With the
opt-in it runs as Config Connector's service account itself, which needs
no binding.
Config Connector's own operator was not installed; its footprint (the
namespace, its service accounts with the annotation, `ConfigConnector`,
`ConfigConnectorContext`) was made by hand as its code says it makes them.

### 9.4 Why not share Config Connector's Kubernetes service account by default

On the Google side, sharing changes nothing: the same Google service
account, through the same Workload Identity binding. On the Kubernetes
side the two directions differ, read from Config Connector's install
bundle at the pin (`config/installbundle/components/clusterroles/` and
`releases/scopes/*`):

- **What our manager would gain by running as Config Connector's account.**
  Cluster mode (`cnrm-controller-manager`, all by ClusterRoleBinding):
  - `cnrm-admin`: create, update and delete on every
    `*.cnrm.cloud.google.com` group (180 groups), cluster-wide;
  - `cnrm-manager-cluster-role`: read CRDs and namespaces; full control of
    `validatingwebhookconfigurations` and of every
    `core.cnrm.cloud.google.com` resource;
  - `cnrm-manager-ns-role`: full control of Secrets, ConfigMaps, Services
    and events in every namespace.

  Namespaced mode (`cnrm-controller-manager-N`): the same roles, but
  `cnrm-admin` and `cnrm-manager-ns-role` only in N (and
  `cnrm-manager-ns-role` in `cnrm-system`); `cnrm-manager-cluster-role`
  stays cluster-wide.
- **What our manager needs:** our six groups, read Secrets, write
  events, and read the ten Config Connector groups ours reference
  (`config-connector-extensions-manager` and `config-connector-extensions-manager-cluster`,
  `config/operator/rbac.yaml`).
- **What Config Connector's pods would gain from our binding, when
  shared:** only full control of our six groups (`sql`, `sqladmin`,
  `secretmanager`, `artifactregistry`, `iam` and `run.gcp.hfl.dev`). Everything else in
  `config-connector-extensions-manager` they already hold.

So sharing barely widens Config Connector's side, and widens ours a lot:
a bug or a compromise in the manager would act with Config Connector's
full Kubernetes rights, including writing every Secret in the cluster in
cluster mode. That is why the default is the manager's own service
account (section 9.2), with only our roles, bound to the same Google
service account by one more `roles/iam.workloadIdentityUser` binding per
Google service account. Sharing stays as an opt-in for a quick trial where
nobody can change IAM. With the default, Config Connector's pods gain
nothing from this project.

## 10. Open questions

A question for the builder goes here, with a recommendation; the answer
moves to section 8.

1. Answered 2026-11-09: the kind is `IAMBinding`, first built as
   `IAMRoleBinding` (decision 12).

2. **`status.appliedDatabaseRoles` after the live run** (2026-11-09).
   Decision 3 kept the field "until a run against real Cloud SQL shows
   whether `users.get` returns roles". It does (`docs/live-runs.md`).
   Upstream PR #7148 compares the spec with the roles `users.get` returns.
   Recommendation: keep the field and the comparison with what was
   applied until we switch to PR #7148 (section 2), since changing it now
   changes the CRD for no user-visible gain; then follow the PR.

3. Answered 2026-11-09: the additive kind is `IAMMember`, and the
   authoritative one is renamed with it to `IAMBinding`. Both mirror
   Terraform's `google_<resource>_iam_member` and `_iam_binding`
   (decision 12). The question had recommended keeping `IAMRoleMember`.
