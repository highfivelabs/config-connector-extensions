# The fake cloud

A Kubernetes cluster where Config Connector's resources, and ours, become
Ready against a fake Google Cloud, so a tool that applies Config Connector
resources can be tested end to end, reading their status back, without a
real project, credentials or cost. Proven 2026-11-07 on an Apple-silicon Mac (Docker 20.10, kind
0.33).

```sh
make upstream          # once: the pinned Config Connector checkout
make fake-cloud-up     # kind cluster, CRDs, bin/fakecloud on the host
make fake-cloud-test   # SQLInstance, SQLDatabase, IAMServiceAccount, SQLUser: all Ready
make fake-cloud-down   # stop it and delete the cluster
```

`export KUBECONFIG=bin/fake-cloud/kubeconfig` to use the cluster; the
manager's log is `bin/fake-cloud/fakecloud.log`.

## How it works

Config Connector has no switch that points a deployed manager at mockgcp:
mockgcp is an in-process `http.RoundTripper` and gRPC server, not a network
endpoint, and Config Connector's own e2e tests run it in mock mode by
building the manager in the test process
(`config/tests/samples/create/harness.go`). `cmd/fakecloud` does the same
outside a test:

- `kccmanager.New`, Config Connector's real manager with every controller
  (direct, Terraform-based and DCL-based), given mockgcp as its HTTP client
  and gRPC connection, the same hooks the harness sets (including the
  Terraform provider's HTTP client transformers);
- this project's controllers (`SQLUser` with `databaseRoles`) in the
  same manager;
- mockgcp's in-memory store, with the projects named by `--projects`
  (default `mock-project`) created at start.

It runs on the host against the kind cluster's API server. The cluster
holds only CRDs: by default the `sql`, `iam`, `resourcemanager` and
`compute` groups (105 CRDs); `CRD_GROUPS=all` installs all ~800.

## Timings (2026-11-07)

| Step | Time |
| --- | --- |
| `go build ./cmd/fakecloud`, first time (Config Connector compiles with it) | about 3 min of CPU, 25 s wall on 6 cores |
| `make fake-cloud-up`: kind cluster, 105 CRDs, manager started | 41 s |
| `make fake-cloud-test`: 4 resources applied and Ready | 2 s |
| `make fake-cloud-down` | about 2 s |

## What does not work, or differs from the real thing

- **The fake cloud forgets.** Its store is in memory: restarting
  `bin/fakecloud` loses every fake resource, while the Kubernetes objects
  remain and are recreated in the fake cloud on their next reconcile.
- **No Config Connector webhooks.** The admission webhook that copies a
  namespace's `cnrm.cloud.google.com/project-id` annotation onto each
  resource does not run, so every resource must carry the annotation
  itself (`hack/fake-cloud/sample.yaml` does). Defaulting and
  validation webhooks are absent too.
- **mockgcp's limits.** Cloud SQL knows Postgres up to 16, not 17 (the
  sample uses 16), and does not refuse a user on an
  instance that does not exist yet, so our `SQLUser` can be Ready before
  its `SQLInstance` (upstream's ref resolver does not wait on readiness
  either; real Cloud SQL would fail and retry). Services mockgcp does not
  implement fail as they would against an unknown host.
- **Artifact Registry in mockgcp.** It stores a repository as sent, so a
  remote repository's `customRepository` round-trips, but it does not
  reach the upstream: the real API refuses, at creation, an upstream it
  cannot reach ("Failed to validate remote upstream", seen live) unless
  `disableUpstreamValidation` is set. Its update accepts only
  `description`, `labels` and the cleanup policies; changing
  `remoteRepositoryConfig` (for example the custom upstream's URI) is
  refused, and patch 0011 gives the real API's error for it ("Remote
  source configuartion cannot be updated", seen live). Patch 0005 lets
  it change a remote repository's upstream username and password version
  alone, as the real API and Terraform do; it never reads the secret
  version, so a password that does not exist or cannot be read by
  Artifact Registry's service agent passes in the fake and fails in the
  real cloud.
- **Artifact Registry tags in mockgcp** (patch 0009, ours). A tag must be
  in an existing repository; the fake holds no packages, so a tag on a
  version that was never pushed is accepted.
- **Workload identity pool namespaces in mockgcp** (patch 0006, ours). It
  has no workload identity pools, so a namespace is created whether or not
  its pool exists; the real API refuses one in a missing pool. Delete is a
  soft delete, as documented, and the ID cannot be reused; the fake never
  purges it. It never sets `ownerService`.
- **Managed identities in mockgcp** (patch 0007, ours). One must be in an
  existing, active namespace. An attestation rule's `googleCloudResource`
  is only checked to be non-empty; the real API checks its form and
  that the workload exists.
- **Cloud Run services in mockgcp.** Patch 0008 makes `services.patch`
  honour an update mask. The fake fills in fewer server defaults than the
  real API (no ports, startup probe or traffic), does not schedule
  anything, so a GPU `nodeSelector` is stored but never checked against
  the region's GPUs or quota, and `invokerIamDisabled` changes nothing it
  serves.
- **Found by live runs** (`docs/live-runs.md`). Where mockgcp and the
  real API were seen to differ, the fake was patched to match: a deleted
  Cloud SQL backup run is kept with status `DELETED` (patch 0010), and
  the Artifact Registry error above (patch 0011). The same tests run
  against a real project with `make test-live`.
- **Not in the cluster.** The manager runs on the host, not as a pod; a
  Target's agent in the cluster cannot reach it. An in-cluster image is
  the way to make the fake cloud reachable from inside: `cmd/fakegcp` and
  `config/fake-cloud/`, used by `make operator-e2e`.
- **Not the operator.** Config Connector's operator (`ConfigConnector`,
  `ConfigConnectorContext` installation, per-namespace managers) is not
  run; the manager watches the whole cluster. `make operator-e2e` covers
  the operator (`docs/design.md` section 9).

## Found on the way

Google requires a service account ID of 6 to 30 characters, and mockgcp
refuses a shorter one as real Google Cloud would; the sample uses
`app-runtime`.
