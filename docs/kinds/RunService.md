---
kinds: [RunService]
apiVersion: run.gcp.hfl.dev/v1beta1
---
# RunService

`run.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Cloud Run service with `invokerIamDisabled` (public without IAM, where an organization policy refuses `allUsers`) and GPUs (`template.nodeSelector`, `gpuZonalRedundancyDisabled`), which Config Connector's Terraform-based `RunService` cannot express; a narrow direct kind that owns the whole service

**Source.** Config Connector's direct `RunWorkerPool`, for a service; the service fields written here

## Notes

2026-11-15: moved from `run.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27, which covers field-gap
kinds since section 10 question 9 was answered); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

2026-11-23: `binaryAuthorization.policy` (a Binary Authorization
platform policy by name), with a CEL rule that it and `useDefault` are not
both set, in the Binary Authorization type `RunService` shares with the
new `RunWorkerPool` (whose test exercises it; `docs/kinds/RunWorkerPool.md`).
An optional field added: no other schema change. Live: not yet run with a
policy (it needs a cloudRun platform policy in the project).

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
