---
kinds: [IAMMember]
apiVersion: iam.gcp.hfl.dev/v1beta1
---
# IAMMember

`iam.gcp.hfl.dev/v1beta1`

**Gap it fills.** one member of one role on one resource, additive (Terraform's `google_*_iam_member`), for the resource types Config Connector's `IAMPolicyMember` cannot reference: regional secrets, managed folders, log views, Cloud Run worker pools, backend services and about two dozen more; the name is a working one (`docs/design.md` section 10)

**Source.** written here, over the same `pkg/iampolicy` as `IAMBinding`

## Notes

2026-11-15: moved from `iam.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

BigQuery datasets (2026-11-12, for the catalog's
`google_bigquery_dataset_iam_member` and `_iam_binding`, which five of
its modules use; Config Connector has only the alpha
`BigQueryDatasetAccess`). A dataset has no `getIamPolicy` or
`setIamPolicy` (the pinned Discovery document has none), so
`pkg/iampolicy` reads and patches its access list as a policy
(`bigquery.go`, after Terraform's `google_bigquery_dataset_iam_*`, read
not copied): the legacy roles `READER`, `WRITER` and `OWNER` are
`roles/bigquery.dataViewer`, `dataEditor` and `dataOwner`; `userByEmail`
is `user:` or, for a service account's address, `serviceAccount:`;
authorized views, routines and datasets and the project special groups
are kept as they are. A write rereads the dataset, refuses on a changed
etag (the read-modify-write then repeats) and patches with `If-Match`.
`resourceRef.kind: BigQueryDataset`, by name (Config Connector's
`BigQueryDataset`) or `external: projects/P/datasets/D`. Proven on the
fake (`TestIAMMember`, "a BigQuery dataset"); live pending BigQuery for
the live-test account (`E2E_BIGQUERY_ADMIN=true`, `docs/live-runs.md`).

A log sink's writer identity (2026-11-15, for the catalog's folder and
organization charts, whose sinks are our `LoggingLogSink`):
`memberFrom.logSinkRef` (`name`, `namespace`, optional `apiVersion` to
pick ours or Config Connector's `LoggingLogSink`) grants the sink's
`status.writerIdentity`, waiting (`DependencyNotReady`) until the sink
has one. `IAMBinding.membersFrom[]` takes the same. A sink deleted
before the member does not hold the member's delete: the member is read
from `status.externalRef`; one that never resolved (no
`status.externalRef`) was never added and deletes without a call.
`docs/design.md` section 6.5 has the rest.

A parent being deleted (2026-11-13): as `IAMBinding`'s (see `IAMBinding.md`):
the delete succeeds without a call when the parent is gone or being
deleted with its resource.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | PASS | none; the backend service case took 24 minutes live, nearly all of it before its first subtest (making the health check and backend service), against 15 s for the others (run `cce-11120814-5d2a`, the fallback project) | 1 regional secret, 1 bucket with a managed folder, 1 health check and 1 backend service (deleted); `task live-list` empty |
| 2026-11-16 | `f4d648a` | PASS | PASS (regional secret only, `LIVE_RUN=TestIAMMember/a_regional_secret`) | `memberFrom.logSinkRef`: the member waits for its sink, then grants our `LoggingLogSink`'s writer identity (a project sink to a Pub/Sub topic path, matching no log); `IAMBinding.membersFrom` the same; the member removed after its sink was deleted; Config Connector's `LoggingLogSink` by its status. One difference: a project's sink to its own project's log bucket has no writer identity live (run `cce-11160035-9947` failed on it; the fake makes one up, `docs/fake-cloud.md`). Run `cce-11160100-52e3` | 1 regional secret, 1 project sink (deleted); sweeper found nothing left |
