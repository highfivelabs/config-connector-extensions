---
kinds: [IAMWorkloadIdentityPoolProviderKey]
apiVersion: iam.gcp.hfl.dev/v1alpha1
---
# IAMWorkloadIdentityPoolProviderKey

`iam.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a key for a workload identity pool provider, whose public half (in status) encrypts the provider's SAML assertions; in no Config Connector code, pull request or Terraform resource

**Source.** written here from the IAM v1 API

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | FAIL | the test's read of a key that had just gone Ready answered 404 (the IAM API's read-after-create lag); the controller was right. Fixed in the test: it reads for up to 30 s (run `cce-11120814-5d2a`, the fallback project) | 1 pool, 1 SAML provider, 2 keys (deleted with the pool) |
| 2026-11-12 | `78644ec` | PASS | PASS | none, with the test fix (run `cce-11120932-1df3`, the fallback project) | 1 pool, 1 SAML provider, 2 keys (deleted with the pool); `task live-list` empty |
