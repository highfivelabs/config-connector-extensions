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
