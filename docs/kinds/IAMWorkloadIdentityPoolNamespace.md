---
kinds: [IAMWorkloadIdentityPoolNamespace]
apiVersion: iam.gcp.hfl.dev/v1alpha1
---
# IAMWorkloadIdentityPoolNamespace

`iam.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a namespace in a workload identity pool (managed workload identities, Terraform's `google_iam_workload_identity_pool_namespace`); Config Connector has types for it only in a pull request, and no controller

**Source.** upstream PR #14209's types; controller and the pool reference written here

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | FAIL | the namespace was created twice: the read after the create operation answered 404, so the next reconcile sent the create again (the IAM API's read-after-create lag; the fake answers at once). Fixed: Create reads for up to 30 s (run `cce-11120814-5d2a`, the fallback project) | 1 pool with its namespaces (deleted) |
| 2026-11-12 | `78644ec` | PASS | PASS | none, with the fix (run `cce-11120932-1df3`, the fallback project) | 1 pool with its namespaces (deleted); `task live-list` empty |
