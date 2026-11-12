---
kinds: [ComputeProjectMetadataItem]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeProjectMetadataItem

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** one key of a project's common instance metadata (for example `enable-oslogin`), leaving the other keys alone; Config Connector's `ComputeProjectMetadata` owns the whole metadata

**Source.** written here after Terraform's `google_compute_project_metadata_item`

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | PASS | none (run `cce-11120814-5d2a`, the fallback project) | the test's metadata keys (removed); `task live-list` empty |
