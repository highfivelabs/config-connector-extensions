# config-connector-extensions design

config-connector-extensions has five kinds: `SQLUser` with `spec.databaseRoles`,
`SQLAdminBackup`, regional `SecretManagerSecret` and
`SecretManagerSecretVersion`, and `ArtifactRegistryRepository` with a custom
remote upstream (section 6). Each passes its end-to-end test
against mockgcp. An operator runs the managers beside Config Connector's,
each as its own Kubernetes service account carrying Config Connector's
Google identity, and a kind test proves it in cluster and namespaced mode
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
  run from the pinned checkout. `make check` fails if its output differs
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
  have; `MOCKGCP_LISTEN_ADDRESS` overrides it, and the Makefile sets it.
- **The test** (`pkg/controller/direct/sql/sqluser_mockgcp_test.go`) runs
  a real kube-apiserver and etcd (envtest, Kubernetes 1.33), our manager
  code (Config Connector's direct reconciler and deletion defender with
  our adapter), and mockgcp as Cloud SQL. It plays a common case: an
  application's service account as a `CLOUD_IAM_SERVICE_ACCOUNT` user with
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
  ok  github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/sql
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
| 8 | `RunService` `invokerIamDisabled` (#14417), GPUs (#7232) | PRs, Terraform only; no direct `RunService` upstream | Cloud Run services | to build (decision 10) |
| 9 | `IAMWorkloadIdentityPoolNamespace`, `...ManagedIdentity` (#14209, #14418), pool `mode` (#14227) | types only, no controller, no mockgcp | workload identity federation | to build (decision 9) |

In the Google API, in no Config Connector code or pull request
(unverified against the live API): Artifact Registry package tags, a
remote repository's `upstreamCredentials` (built, decision 8), Workload Identity
pool provider keys, Cloud SQL `blueGreenDeployments`. None is built yet.
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
- `make check` and `make operator-e2e` on a Linux amd64 VM (16 CPUs, Go
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
8. One run against a real Cloud SQL Postgres instance, to confirm
   `databaseRoles` on insert and update, revoke with an empty list, and
   what `users.get` returns; then correct mockgcp if it differs.
9. Unit tests for `passwordPolicyEqual`, `sortedRoles` and identity
   parsing (Config Connector has an identity conformance helper,
   `apis/common/identity`).
10. Offer patches 0001 and 0002 upstream; comment on PR #7148 with the
    revoke semantics and the mockgcp change.

Housekeeping:

11. The SQLUser test predates `pkg/test/mockenv` and keeps its own harness.
12. CI: `make check` on Linux (the 127.0.0.2 default then works without
    patch 0002, but the patch stays harmless).
13. `tests/operator/run.sh` does not clean the fake cloud on a reused kind
    cluster; run it on a fresh one.

To raise confidence in `ArtifactRegistryRepository`:

14. One run against a real project: create a remote repository with a
    custom upstream, then change its URI, to learn whether the API updates
    it in place, refuses, or needs a new repository; correct mockgcp to
    match. Real Artifact Registry also checks that a remote upstream is
    reachable when the repository is made (hence `disableUpstreamValidation`
    in the API); mockgcp does not.
15. Watch PR #13517 (UPSTREAM.md, "Copied from open pull requests"); when
    it merges and a release ships it, retire ours (section 3.4).

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
   release, with the exception in section 6.1. Never a fork.
6. **The fake-cloud switch exists only in test builds.**
   - `--fake-cloud-url` and `pkg/fakecloud` are compiled into the manager
     only with `-tags fakecloud` (`cmd/manager/fakecloud.go`).
   - A production build has no such flag and refuses to start with it
     ("flag provided but not defined").
   - `hack/check-no-fake-cloud.sh`, run by `make check`, fails if the
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

`pkg/operator` against envtest (`make test`): cluster mode with Workload
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

`tests/operator/run.sh` (`make operator-e2e`) on kind 0.33 (Kubernetes
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
- **What our manager needs:** our four groups, read Secrets, write
  events, and read the five Config Connector groups ours reference
  (`config-connector-extensions-manager` and `config-connector-extensions-manager-cluster`,
  `config/operator/rbac.yaml`).
- **What Config Connector's pods would gain from our binding, when
  shared:** only full control of our four groups (`sql`, `sqladmin`,
  `secretmanager` and `artifactregistry.gcp.hfl.dev`). Everything else in
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

None open. A question for the builder goes here, with a recommendation;
the answer moves to section 8.
