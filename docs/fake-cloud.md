# The fake cloud

A Kubernetes cluster where Config Connector's resources, and ours, become
Ready against a fake Google Cloud, so a tool that applies Config Connector
resources can be tested end to end, reading their status back, without a
real project, credentials or cost. Proven 2026-11-07 on an Apple-silicon Mac (Docker 20.10, kind
0.33).

```sh
make upstream          # once: the pinned Config Connector checkout
make fake-cloud-up     # kind cluster, CRDs, bin/fakecloud on the host
make fake-cloud-test   # SQLInstance, SQLDatabase, IAMServiceAccount, SQLUser: all Ready, then all deleted
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
- Config Connector's deletion defender, which in a real install runs as
  its own process (`cmd/deletiondefender`); without it Config Connector's
  kinds never finish deleting, because their reconcilers wait for it to
  drop its finalizer. Ours have their own defenders in the manager;
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
  reach the upstream (the real API checks a remote upstream with a HEAD or
  GET unless `disableUpstreamValidation` is set), and its update accepts
  only `description`, `labels` and the cleanup policies: changing
  `remoteRepositoryConfig` (for example the custom upstream's URI) is
  refused with "not supported by mockgcp". What the real API does with
  that change is not known (`docs/design.md` section 7). Patch 0005 lets
  it change a remote repository's upstream username and password version
  alone, as the real API and Terraform do; it never reads the secret
  version, so a password that does not exist or cannot be read by
  Artifact Registry's service agent passes in the fake and fails in the
  real cloud.
- **Log Router settings in mockgcp** (patch 0015, ours). Any
  organization or folder ID is accepted, the key is not checked, and the
  service account IDs only follow the documented form.
- **Router route policies in mockgcp** (patch 0012, ours). The CEL in
  terms is stored, not parsed, and the fingerprint is not checked on write.
- **Regional network endpoints in mockgcp** (patch 0011, ours). Any
  endpoint is accepted in any group: the group's type (internet IP, domain
  name, VM) is not checked against the endpoint, nor is the VM.
- **Artifact Registry tags in mockgcp** (patch 0009, ours). A tag must be
  in an existing repository; the fake holds no packages, so a tag on a
  version that was never pushed is accepted.
- **Workload identity pool namespaces in mockgcp** (patch 0006, ours). It
  has no workload identity pools, so a namespace is created whether or not
  its pool exists; the real API refuses one in a missing pool. Delete is a
  soft delete, as documented, and the ID cannot be reused; the fake never
  purges it. It never sets `ownerService`.
- **Workload identity pool provider keys in mockgcp** (patch 0010, ours).
  The public key is a placeholder PEM, not a certificate, and its validity
  window is made up; the provider is not checked.
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
