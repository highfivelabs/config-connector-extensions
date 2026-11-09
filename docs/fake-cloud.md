# The fake cloud

A Kubernetes cluster where Config Connector's resources, and ours, become
Ready against a fake Google Cloud, so a tool that applies Config Connector
resources can be tested end to end, reading their status back, without a
real project, credentials or cost. Proven 2026-11-07 on an Apple-silicon Mac (Docker 20.10, kind
0.33).

```sh
task upstream          # once: the pinned Config Connector checkout
task fake-cloud-up     # kind cluster, CRDs, bin/fakecloud on the host
task fake-cloud-test   # SQLInstance, SQLDatabase, IAMServiceAccount, SQLUser: all Ready, then all deleted
task fake-cloud-down   # stop it and delete the cluster
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
| `task fake-cloud-up`: kind cluster, 105 CRDs, manager started | 41 s |
| `task fake-cloud-test`: 4 resources applied and Ready | 2 s |
| `task fake-cloud-down` | about 2 s |

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
  refused, and patch 0016 gives the real API's error for it ("Remote
  source configuartion cannot be updated", seen live). Patch 0005 lets
  it change a remote repository's upstream username and password version
  alone, as the real API and Terraform do; it never reads the secret
  version, so a password that does not exist or cannot be read by
  Artifact Registry's service agent passes in the fake and fails in the
  real cloud.
- **IAM policies in mockgcp** (patch 0018, ours, for `IAMRoleBinding`).
  Every `:getIamPolicy` and `:setIamPolicy` is served from one store keyed
  by host and path, for any resource name, existing or not: a policy on a
  secret that does not exist reads as empty and can be set. The etag is
  checked on write (409); permissions are not, nor are the members' forms
  (`user:` with no address is accepted) or whether a role exists.
  Cloud Storage's own `.../iam` paths are served from the same store.
  Compute Engine's `GET .../getIamPolicy` and `POST .../setIamPolicy`
  (patch 0019, ours, for `IAMRoleBinding` and `IAMRoleMember`) are served
  from it too. The store's etag check and write are atomic (patch 0019; before it, two
  writers that read the same policy both passed the check and the later one
  replaced the earlier). What the store cannot show: a role that does not apply to
  the resource (`roles/viewer` on a managed folder), a member the
  resource's organization refuses, a managed folder that does not exist,
  and each service's own choice of GET or POST for the read, its API
  version and its etag error (409 or 412), which is why the rows of the
  resource table beyond the three with a live run are unproven.
- **Regional network firewall policies in mockgcp** (patch 0014, adapted
  from its global ones; their rules are patch 0020). The fingerprint is
  not checked on patch, a policy is deleted even when associated, and
  associations are patch 0021. A rule is stored as sent: mockgcp
  checks none of the API's rules about a rule (a `goto_next` rule with
  logging, secure tags together with target service accounts, a layer 4
  protocol with ports it cannot have, FQDNs and region codes on the wrong
  direction, the policy's limit on rule tuples), and it reports a rule
  tuple count of 2 for every rule, whatever it matches. Its `patchRule`
  replaces the whole rule; the real API keeps a list the body leaves out
  (the controller sends `[]` for it), and answers 400 "The rule at
  priority N is not ready" to a patch soon after another change, which
  the fake never does.
  An association (patch 0021) is stored as sent: mockgcp does not check
  that the network exists, nor that a network has no policy attached
  from another policy (it refuses only a repeated name or network inside
  one policy), nor what the API fills in itself.
- **Log Router settings in mockgcp** (patch 0017, ours). Any
  organization or folder ID is accepted, the key is not checked, and the
  service account IDs only follow the documented form.
- **Cloud Storage objects in mockgcp** (patch 0013, ours). Before it,
  objects were stubs. Uploads are media or multipart, not resumable; an
  object's ACLs, holds, retention and storage class are not modelled.
- **Router route policies in mockgcp** (patch 0012, ours). The CEL in
  terms is stored, not parsed, and the fingerprint is not checked on write.
  A missing policy is answered with the real API's 400 ("The policy does
  not exist"), not a 404, as a live run showed.
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
  refuses one naming a project outside the pool's organization (a live
  run), so against a real project the test names the test project.
- **Cloud Run services in mockgcp.** Patch 0008 makes `services.patch`
  honour an update mask and give a container the API's default port and
  startup probe, as a live run showed. The fake still fills in fewer
  server defaults than the real API (no traffic), does not schedule
  anything, so a GPU `nodeSelector` is stored but never checked against
  the region's GPUs or quota, and `invokerIamDisabled` changes nothing it
  serves.
- **Found by live runs** (`docs/live-runs.md`). Where mockgcp and the
  real API were seen to differ, the fake was patched to match: a deleted
  Cloud SQL backup run is kept with status `DELETED` (patch 0015), and
  the Artifact Registry error above (patch 0016), a missing route
  policy's 400 (patch 0012), and a Cloud Run container's default port and
  startup probe (patch 0008). The same tests run
  against a real project with `task test-live`.
- **Not in the cluster.** The manager runs on the host, not as a pod; a
  Target's agent in the cluster cannot reach it. An in-cluster image is
  the way to make the fake cloud reachable from inside: `cmd/fakegcp` and
  `config/fake-cloud/`, used by `task operator-e2e`.
- **Not the operator.** Config Connector's operator (`ConfigConnector`,
  `ConfigConnectorContext` installation, per-namespace managers) is not
  run; the manager watches the whole cluster. `task operator-e2e` covers
  the operator (`docs/design.md` section 9).

## Found on the way

Google requires a service account ID of 6 to 30 characters, and mockgcp
refuses a shorter one as real Google Cloud would; the sample uses
`app-runtime`.
