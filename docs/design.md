# gaps-gcp design

gaps-gcp has four kinds: `SQLUser` with `spec.databaseRoles`,
`SQLAdminBackup`, and regional `SecretManagerSecret` and
`SecretManagerSecretVersion` (section 6). Each passes its end-to-end test
against mockgcp. An operator runs the managers as Config Connector's own
service accounts, and a kind test proves it in cluster and namespaced mode
against an in-cluster fake cloud (section 9). Images are built locally
with ko; there is no registry or release yet. This document says how it is
built, why, how it stays in step with Config Connector, how it is tested,
how gaps are chosen, and what is left.

## 1. Background

- **One repository per provider.** Each provider's gap kinds are built with
  that provider's generator against that provider's model input, on
  unrelated cadences. So `gaps-gcp` beside Config Connector, and in time
  `gaps-aws` beside ACK and `gaps-azure` beside Azure Service Operator,
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
- **GCP is where gaps close fastest upstream**, so `gaps-gcp` expects to
  retire kinds as often as it adds them; a kind moves from our group to
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
when a copied file changes upstream, or before a gaps-gcp release, at
least monthly while upstream ships every two to three weeks. A gaps-gcp
release names the Config Connector release it is installed beside, and
pins a `master` commit at or after that release.

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
| 7 | `ArtifactRegistryRepository` custom remote repositories (#13517) | PR | pulling through a private upstream | not built |
| 8 | `RunService` `invokerIamDisabled` (#14417), GPUs (#7232) | PRs | Cloud Run services | not built |
| 9 | `IAMWorkloadIdentityPoolNamespace`, `...ManagedIdentity` (#14209, #14418), pool `mode` (#14227) | types only, no controller | workload identity federation | not built |

In the Google API, in no Config Connector code or pull request
(unverified against the live API): Artifact Registry package tags and
repository attachments, Workload Identity pool provider keys, Cloud SQL
`blueGreenDeployments`, Storage HMAC keys. None is built yet.

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

## 7. What is left

Before gaps-gcp can run on a real cluster:

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
6. A Kubernetes service account of our own for the managers (section 9.4).

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

Workload Identity is bound to a namespace and a service-account name, so
the cheapest way to have "the same auth in the same namespace" is to run
in that namespace as that service account: no new Google IAM binding, no
key, nothing extra to grant. `pkg/operator`:

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
`--fake-cloud-url`, which loads no Google credentials and since section
9.4 exists only in builds with `-tags fakecloud`), images built by ko
0.19.1 for linux/arm64, 2026-11-07:

```text
ok: CRDs installed: secretmanagersecrets, secretmanagersecretversions, sqladminbackups, sqlusers (*.gcp.hfl.dev)
== cluster mode: one manager in cnrm-system as cnrm-controller-manager
ok: manager pod runs as cnrm-system/cnrm-controller-manager (Workload Identity maps it to kcc@mock-project.iam.gserviceaccount.com)
ok: cnrm-controller-manager may manage gap kinds in any namespace
== cluster mode: a regional secret and its version become Ready
ok: Ready: projects/mock-project/locations/us-central1/secrets/app-db-password
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

### 9.4 Trade-off: sharing Config Connector's Kubernetes service account

On the Google side, sharing changes nothing: the same Google service
account, through the same Workload Identity binding. On the Kubernetes
side the two directions differ, read from Config Connector's install
bundle at the pin (`config/installbundle/components/clusterroles/` and
`releases/scopes/*`):

- **What our manager gains by running as Config Connector's account.**
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
- **What our manager needs:** our three groups, read Secrets, write
  events, and read the five Config Connector groups ours reference
  (`gaps-gcp-manager` and `gaps-gcp-manager-cluster`,
  `config/operator/rbac.yaml`).
- **What Config Connector's pods gain from our binding:** only full
  control of our three groups (`sql`, `sqladmin` and
  `secretmanager.gcp.hfl.dev`). Everything else in `gaps-gcp-manager` they
  already hold.

So Config Connector's side barely widens, and ours widens a lot: a bug or
a compromise in the gaps-gcp manager acts with Config Connector's full
Kubernetes rights, including writing every Secret in the cluster in
cluster mode. The alternative is our own Kubernetes service account with
only our roles, bound to the same Google service account by one more
`roles/iam.workloadIdentityUser` binding per Google service account
(section 7, item 6).
