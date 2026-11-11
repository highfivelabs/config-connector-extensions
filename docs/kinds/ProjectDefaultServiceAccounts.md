---
kinds: [ProjectDefaultServiceAccounts]
apiVersion: resourcemanager.gcp.hfl.dev/v1alpha1
---
# ProjectDefaultServiceAccounts

`resourcemanager.gcp.hfl.dev/v1alpha1`

**Gap it fills.** what a project does to its default Compute Engine and App Engine service accounts: `DELETE`, `DISABLE` or `DEPRIVILEGE` (out of every project role binding), kept on every reconcile, and undone on delete unless `restorePolicy: NONE`; in no Config Connector code or pull request

**Source.** written here after Terraform's `google_project_default_service_accounts`

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
