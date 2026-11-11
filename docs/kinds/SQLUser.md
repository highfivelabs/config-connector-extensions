---
kinds: [SQLUser]
apiVersion: sql.gcp.hfl.dev/v1beta1
---
# SQLUser

`sql.gcp.hfl.dev/v1beta1`

**Gap it fills.** `spec.databaseRoles`: Config Connector's `SQLUser` cannot grant a Cloud SQL user its database roles (`User.databaseRoles`, Terraform's `google_sql_user.database_roles`)

**Source.** ours, on upstream's `SQLSSLCert` controller; upstream PR #7148 is watched

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
