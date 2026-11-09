# Live runs

Every kind is built and tested against the fake cloud (Config Connector's
mockgcp, `docs/fake-cloud.md`). The fake is only as good as its likeness to
Google Cloud, so the same tests are also run against a real project: at
least once for every kind before it is called done, and again when a
kind or its mockgcp patch changes in a way the fake might get wrong. This
page says how, and keeps the ledger of every live run.

## The switch

The end-to-end tests are built on `pkg/test/mockenv`, which reads
`E2E_GCP_TARGET`, the variable Config Connector's own harness uses
(`config/tests/samples/create/harness.go`):

| `E2E_GCP_TARGET` | Google Cloud is | Set by |
| --- | --- | --- |
| unset or `mock` | mockgcp, in process, with our patches | `make test`, `make test-mock`, `make check` (which force `mock`) |
| `real` | the project in `GCP_PROJECT_ID`, with Application Default Credentials | `make test-live` only |

The test code is the same for both. What differs is data, all in
`pkg/test/mockenv/target.go`:

- `mockenv.Project`: `mock-project`, or the real project.
- `mockenv.Name(base)`: `base`, or `<run ID>-base`, so runs never collide
  and every resource a run makes can be found by its name.
- `mockenv.Labels()`: none, or `cce-e2e=true` and `cce-run=<run ID>` on every
  object (Config Connector copies `metadata.labels` to the resource).
- `mockenv.Timeout()`: 60 s, or 15 min (a real Cloud SQL backup takes minutes).
- `env.SQLInstance()`: an instance made in mockgcp, or the run's shared one.
- `mockenv.ServiceAccount(t)`: a made-up email, or the run's own service
  account (a Cloud SQL IAM user must be a real principal).

Requests are recorded in both modes, so checks on what the controller sent
(the regional endpoint, the update mask) hold against the real API too.

## Running it

```sh
make test-mock                                  # the default; what make check runs
make test-live PROJECT=hfl-dev-514205           # every built kind, mock then live
make test-live PROJECT=hfl-dev-514205 KINDS=sql # one kind (a package under pkg/controller/direct)
make live-sweep PROJECT=hfl-dev-514205          # delete leftovers older than 2h
make live-sweep PROJECT=hfl-dev-514205 RUN=cce-11091530-a3f2   # everything of one run
make live-list PROJECT=hfl-dev-514205           # list every live-test resource
```

Live runs happen on a short-lived Spot VM that runs as a dedicated
service account, never on a laptop and never with a downloaded key:

```sh
../offload/offload run --sa cce-live-tests@hfl-dev-514205.iam.gserviceaccount.com --hours 3 -- \
  bash -c 'make upstream && make test-live PROJECT=hfl-dev-514205'
```

`hack/live/run.sh` (what `make test-live` runs):

1. refuses any project not in `hack/live/projects.sh` and any credentials
   that are not a service account;
2. sweeps leftovers older than 2h;
3. makes a run ID, `cce-<UTC month day hour minute>-<4 hex>`;
4. if a SQL kind is in the run, creates one Cloud SQL instance for the run
   (POSTGRES_16, Enterprise edition, `db-f1-micro`, zonal, 10 GB HDD, no
   automated backups, `cloudsql.iam_authentication=on`), shared by the SQL
   kinds because a real one takes minutes to make;
5. for each kind, in `docs/design.md` section 6.3's order, runs its package's
   tests with `E2E_GCP_TARGET=mock` and then `real`;
6. deletes the instance; then, pass or fail, sweeps everything of the run
   and leftovers older than 2h, and lists what is left;
7. writes `offload-artifacts/live/<run ID>/summary.md` and each kind's mock
   and live log (uploaded with the offload run's log).

Each test deletes what it makes, including what it abandons on purpose
(`deletion-policy: abandon`). The sweeper (`hack/live/sweep.sh`) is the
backstop for a test that fails midway or a VM that is preempted, and also
refuses any project but the ones listed.

### The service account

`cce-live-tests@hfl-dev-514205.iam.gserviceaccount.com`, made 2026-11-09:

| Role | On | For |
| --- | --- | --- |
| `roles/artifactregistry.admin` | project | `ArtifactRegistryRepository` |
| `roles/secretmanager.admin` | project | `SecretManagerSecret`, `SecretManagerSecretVersion` |
| `roles/cloudsql.admin` | project | `SQLUser`, `SQLAdminBackup`, the run's instance |
| `roles/compute.instanceAdmin.v1` | project | the offload VM (deletes itself) |
| `roles/logging.logWriter` | project | the offload VM |
| `roles/storage.objectAdmin` | bucket `gs://hfl-dev-514205-offload` | the offload VM (checkout in, log out) |
| `roles/iam.serviceAccountUser` | itself | the offload VM |
| `roles/storage.admin` | project | `StorageBucketObject` (the test's bucket and its objects) |

APIs enabled for it: `artifactregistry`, `secretmanager`, `sqladmin`,
`cloudresourcemanager`. A new kind that needs another role or API adds it
here, and to the account, in the same change.

## Adding a kind

A kind is not done until it has one live run in the ledger below:

1. Its end-to-end test uses `pkg/test/mockenv` and takes every name from
   `mockenv.Name`, labels from `mockenv.Labels()`, the project from
   `mockenv.Project`, and deletes everything it makes, abandoned
   resources included.
2. The sweeper knows its resource type (`hack/live/sweep.sh`), and the
   service account has the role it needs.
3. `make test-live PROJECT=hfl-dev-514205 KINDS=<package>` on an offload VM.
4. A row per kind below. A difference between mock and live is fixed where
   it belongs: in our controller (with a test), or in mockgcp (a patch in
   `patches/k8s-config-connector/`), or written down in `docs/fake-cloud.md`
   when the fake cannot show it.

## Ledger

One row per kind per live run. Mock and live ran from the same checkout,
on the same VM, minutes apart.

| Date | Commit | Kind | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-11-09 | `8a13722`+ | SQLAdminBackup | PASS | FAIL | after `backupRuns.delete` completes, `backupRuns.get` still returns the run, status `DELETED`; mockgcp answered 404 | instance `cce-11090729-0d21-pg` (db-f1-micro, ~30 min; swept by hand after the VM was preempted), 2 on-demand backups (deleted) |
| 2026-11-09 | `8a13722`+ | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | 1 regional and 1 global secret, 1 version (deleted) |
| 2026-11-09 | `8a13722`+ | SQLUser | PASS | PASS | none: `users.get` returns `databaseRoles`, and insert, add and revoke match patch 0001 | 1 IAM user in the shared instance (deleted) |
| 2026-11-09 | `8a13722`+ | ArtifactRegistryRepository | PASS | none | the VM was preempted during the live test | none left (checked) |
| 2026-11-09 | `8a13722`+ | SQLAdminBackup | PASS | FAIL | the same as above, now with the status printed: `DELETED` | instance `cce-11090817-870d-pg` (~40 min; swept by hand), 2 backups |
| 2026-11-09 | `8a13722`+ | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | SQLUser | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | ArtifactRegistryRepository | PASS | FAIL | the API refuses a remote upstream it cannot reach ("Failed to validate remote upstream", `registry.example.com/v2/`); mockgcp never checks. Run stopped by hand | none created |
| 2026-11-09 | `8a13722`+ | SQLAdminBackup | PASS | PASS | none, with the fix (Find takes `DELETED` as gone) and patch 0015 (numbered 0010 then) | instance `cce-11090854-9525-pg` (~25 min), 3 backups (deleted) |
| 2026-11-09 | `8a13722`+ | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | SQLUser | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | ArtifactRegistryRepository | PASS | FAIL | a changed custom upstream URI: the API refuses it with "Remote source configuartion cannot be updated", mockgcp with "not supported by mockgcp" (fixed by patch 0016, numbered 0011 then); the upstreamCredentials test passed live | 6 remote repositories, 1 secret with 2 versions (deleted) |
| 2026-11-09 | `f2b89cd` | SQLAdminBackup | PASS | PASS | none | instance `cce-11090920-6dcf-pg` (~25 min), 3 backups (deleted) |
| 2026-11-09 | `f2b89cd` | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | 2 secrets, 1 version (deleted) |
| 2026-11-09 | `f2b89cd` | SQLUser | PASS | PASS | none | 1 IAM user (deleted with the instance) |
| 2026-11-09 | `f2b89cd` | ArtifactRegistryRepository | PASS | PASS | none | 6 repositories, 1 secret (deleted) |
| 2026-11-09 | `01afd2d`+ | ArtifactRegistryTag | PASS | PASS | none: tags on two pushed images, moved, deleted, abandoned | 1 Docker repository with 2 minimal images (deleted) |
| 2026-11-09 | `01afd2d`+ | ArtifactRegistryRepository | PASS | PASS | none | 6 repositories, 1 secret (deleted) |
| 2026-11-09 | `08fcc66` | SQLAdminBackup | PASS | PASS | none | instance `cce-11091348-ef59-pg` (db-f1-micro, ~25 min), 3 on-demand backups (deleted) |
| 2026-11-09 | `08fcc66` | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | 2 secrets, 1 version (deleted) |
| 2026-11-09 | `08fcc66` | SQLUser | PASS | PASS | none | 1 IAM user (deleted with the instance) |
| 2026-11-09 | `08fcc66` | ArtifactRegistryRepository, ArtifactRegistryTag | PASS | PASS | none | 7 repositories, 2 minimal images, 1 secret (deleted) |
| 2026-11-09 | pending | ComputeRegionNetworkEndpoint | PASS | pending, needs `roles/compute.networkAdmin` | not run | none |
| 2026-11-09 | pending | ComputeRouterRoutePolicy | PASS | pending, needs `roles/compute.networkAdmin` | not run | none |
| 2026-11-09 | pending | ComputeRegionNetworkFirewallPolicy | PASS | pending, needs `roles/compute.securityAdmin` | not run | none |
| 2026-11-09 | pending | LoggingSettings | PASS | pending, needs a test folder with `roles/logging.admin` and a KMS key its Log Router can use | not run | none |
| 2026-11-09 | `756c69f` | StorageBucketObject | PASS | PASS | none: media and multipart uploads, a new generation for new content, content from a ConfigMap and a Secret, delete and abandon all match patch 0013 | 1 bucket (US) with 4 small objects (deleted with the bucket) |

`+` means the commit plus the uncommitted changes that became the next
commit. Runs `cce-11090718-8cc1` (gcloud's GA track has no `--labels` for
Cloud SQL; nothing made) and `cce-11090806-613a` (preempted while making
the instance; swept by hand) reached no test.

Not yet run live: `IAMWorkloadIdentityPoolNamespace`,
`IAMWorkloadIdentityPoolManagedIdentity`,
`IAMWorkloadIdentityPoolProviderKey` and `RunService` (packages `iam`
and `run`). Granting the live-test service
account `roles/iam.workloadIdentityPoolAdmin`, `roles/run.admin` and
`roles/iam.serviceAccountUser` on the runtime service account is with
the builder; then their tests need run names and cleanup, and the
sweeper needs workload identity pools and Cloud Run services.

Package `compute` (`ComputeRegionNetworkEndpoint`,
`ComputeRouterRoutePolicy`, `ComputeRegionNetworkFirewallPolicy`) is
ready to run live: its tests take every cloud name from `mockenv.Name`
(a network, a regional network endpoint group, a router, the firewall
policies), delete what they make, abandoned resources included, and the
sweeper finds these by name (they take no labels). It waits for two
roles on the live-test service account, with the builder:
`roles/compute.networkAdmin` (the networks, the group and its
endpoints, the router and its route policies) and
`roles/compute.securityAdmin` (network firewall policies, which
`compute.networkAdmin` leaves out). Then:
`make test-live PROJECT=hfl-dev-514205 KINDS=compute`. The managed identity test's attestation rules name
made-up Compute Engine instances, and the RunService test asks for an
NVIDIA L4 GPU, which needs Cloud Run GPU quota in the project.

`LoggingSettings` (package `logging`) is ready to run live but waits on
the builder for a test folder and a key; it never changes an
organization's settings live (that case runs in the fake cloud only). It
needs: a test folder, passed as `E2E_LOGGING_FOLDER=folders/<id>`, with
`roles/logging.admin` on it for the live-test service account; and a
Cloud KMS key, passed as `E2E_LOGGING_KMS_KEY`, on which the folder's
Log Router service account (`kmsServiceAccountId` in the folder's
settings) has `roles/cloudkms.cryptoKeyEncrypterDecrypter`. Settings
cannot be deleted, so the test puts the folder's back as it found them;
there is nothing for the sweeper. Then:
`E2E_LOGGING_FOLDER=... E2E_LOGGING_KMS_KEY=... make test-live PROJECT=hfl-dev-514205 KINDS=logging`.

Every run ended with `make live-list` empty (run `cce-11091348-ef59`:
"0 live-test resource(s) in hfl-dev-514205"). Cost of all the runs above,
roughly: five db-f1-micro instances for 25 to 40 minutes each (well under
$0.50 together), Spot VMs of 4 to 8 vCPUs for about 4 VM-hours (about
$0.30), and Artifact Registry and Secret Manager use of a few cents.
