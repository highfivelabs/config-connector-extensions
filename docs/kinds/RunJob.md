---
kinds: [RunJob]
apiVersion: run.gcp.hfl.dev/v1alpha1
---
# RunJob

`run.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud Run job with the fields Config Connector's Terraform-based `RunJob` cannot express: the job's labels and its executions' (`template.labels`), GPUs (`template.template.nodeSelector`, `gpuZonalRedundancyDisabled`), gRPC probes and `binaryAuthorization.policy` (application item 6); a direct kind that owns the whole job

**Source.** Config Connector's direct `RunJob` (opt-in on master), its types, identity, mappers, reference resolution and controller; the containers, volumes and VPC access are our `RunService`'s types

## Which kind to use

Config Connector's `run.cnrm.cloud.google.com/v1beta1` `RunJob` for a job
that needs none of the fields above; this one for a job that does. The
spec is Config Connector's direct `RunJob`'s plus those fields; the job's
labels are `metadata.labels`, as there. Terraform's update of a job sends
the whole job with no update mask (and the API's `UpdateJob` takes none),
so no field kind can add the fields beside Config Connector's object
(`docs/design.md` section 6.6, decision 11's case). To move a job: create
this kind's object with `spec.resourceID` naming it (it is acquired, then
updated to the spec), then delete Config Connector's object with
`cnrm.cloud.google.com/deletion-policy: abandon`. Never manage one job
with both kinds.

## Notes

2026-11-23: built (application item 6, `docs/design.md` section 6.7).
Compared like our `RunService`: the spec's fields against the job as read,
with the server's defaults (launch stage, task count, parallelism,
timeout, retries, service account, execution environment, a container's
resources, ports and probe timings) taken from what was read where the
spec leaves them unset, and no write when they agree. Upstream's
controller instead compares a cookie of the spec and the whole job as
read, so an execution or the job's reconciling turning false would cause
an update. An update sends the whole job (`UpdateJob` has no mask), so
whatever the spec leaves out of a job made elsewhere is cleared, as for
any whole-object kind. CEL: `projectRef`, `location` and `resourceID` are
immutable; `binaryAuthorization` takes one of `useDefault` and `policy`;
a container may not have `baseImageURI`, `resources.cpuIdle` or
`resources.startupCPUBoost` (the API takes them only for a service). A
`maxRetries` of 0 (never retry) is kept, which upstream's mapper drops.
Patch 0069 makes mockgcp's `UpdateJob` replace the job's other settable
fields, not only its template.

Not built: `volumeMounts[].subPath` (in the pinned Go client but not in
mockgcp's protos, so the fake drops it: it needs mockgcp's generated
protos regenerated, or a live-only test), and the preview fields
(`template.template.containers[].buildConfig`, `sourceCode`, Filestore
volumes). `startExecutionToken` and `runExecutionToken` start executions,
which this kind does not do.

The test (`TestRunJob`) refuses a service's container fields and a
default with a named policy, makes a job with labels, execution labels, a
GPU, `maxRetries` 0 and a named Binary Authorization policy (and checks
them in the cloud), a resync with no write, a change of the labels, image
and policy in one update that keeps the GPU and retries, a gRPC startup
probe (and a resync with its timings left to the server), the immutable
location, delete and abandon. Live: pending, `task test-live
PROJECT=$GCP_PROJECT KINDS=run LIVE_RUN=TestRunJob` (two jobs on Cloud
Run's public sample job image; a job costs nothing until it runs, and the
test runs none). The GPU only with `E2E_RUN_GPU=true` (as `RunService`'s),
the named policy only with `E2E_RUN_BINAUTHZ_POLICY` (a cloudRun platform
policy's name; without it the job uses the project's default policy).
The account's `roles/run.admin` and `roles/iam.serviceAccountUser` on the
compute default service account (both held for `RunService`) cover it.
What to look for: the server defaults the real API fills in beyond
mockgcp's (each must be taken from what was read, or the resync test
shows an update), whether the API takes probes on a job's container, and
whether it takes `maxRetries` 0.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
