---
kinds: [FirestoreBackupSchedule]
apiVersion: firestore.gcp.hfl.dev/v1beta1
---
# FirestoreBackupSchedule

`firestore.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Firestore database's backup schedule (daily or weekly, with a retention), as a stable kind; Config Connector's `FirestoreBackupSchedule` is alpha only (decision 26). Terraform's `google_firestore_backup_schedule`, which the catalog's `firestore` module uses

**Source.** hand-written, not generated: copied from Config Connector's direct `FirestoreBackupSchedule` (types, identity, mapper, controller; `UPSTREAM.md`). The API assigns the schedule's ID at create (there is no ID parameter), so `hack/discokind` cannot express it.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `databaseRef` (Config
Connector's `FirestoreDatabase`, or `external: projects/P/databases/D`),
`retention`, and one of `dailyRecurrence` and `weeklyRecurrence`.
`databaseRef` is immutable and at most one recurrence is allowed (CEL).
The schedule's ID is the API's: the controller reads it from the create
answer and keeps it in `status.externalRef`, where the next reconcile
finds it. Upstream's kind has no `resourceID`, so an existing schedule
cannot be acquired by ID; neither can ours.

The test (`TestFirestoreBackupSchedule`): the CRD refusing two
recurrences, create, a resync that finds the schedule by its ID and writes
nothing, one `PATCH` with `updateMask=retention`, the CRD refusing a change
of database, delete, abandon.

Live is pending: it needs a Firestore database (a database's ID cannot be
reused for a while after deletion, so no test makes one) and
`roles/datastore.owner` for the live-test service account;
`firestore.googleapis.com` is off in the live-test project and the account
has no Firestore role (read 2026-11-12). The test skips live unless
`E2E_FIRESTORE_DATABASE` names a database. What the fake cannot show: the
API's limits (one daily and one weekly schedule per database, retention
up to 14 weeks), which it does not check.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
