---
kinds: [RunService]
apiVersion: run.gcp.hfl.dev/v1alpha1
---
# RunService

`run.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud Run service with `invokerIamDisabled` (public without IAM, where an organization policy refuses `allUsers`) and GPUs (`template.nodeSelector`, `gpuZonalRedundancyDisabled`), which Config Connector's Terraform-based `RunService` cannot express; a narrow direct kind that owns the whole service

**Source.** Config Connector's direct `RunWorkerPool`, for a service; the service fields written here

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
