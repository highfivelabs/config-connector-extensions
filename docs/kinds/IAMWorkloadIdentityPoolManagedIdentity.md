---
kinds: [IAMWorkloadIdentityPoolManagedIdentity]
apiVersion: iam.gcp.hfl.dev/v1alpha1
---
# IAMWorkloadIdentityPoolManagedIdentity

`iam.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a managed identity in a pool namespace, with the attestation rules that say which workloads may receive it (Terraform's `google_iam_workload_identity_pool_managed_identity`); types only in a Config Connector pull request, no controller

**Source.** upstream PR #14418's types; controller, the namespace reference and `attestationRules` written here

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
