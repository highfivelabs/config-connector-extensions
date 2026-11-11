---
kinds: [IAMBinding]
apiVersion: iam.gcp.hfl.dev/v1beta1
---
# IAMBinding

`iam.gcp.hfl.dev/v1beta1`

**Gap it fills.** one role on one resource with its members, authoritative for that role (Terraform's `google_*_iam_binding`): members added out of band are removed; Config Connector's `IAMPolicyMember` is additive and `IAMPartialPolicy` keeps what it lists

**Source.** written here (`pkg/iampolicy`), reading and writing the policy over HTTP

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
