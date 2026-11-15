---
kinds: [StorageBucketObject]
apiVersion: storage.gcp.hfl.dev/v1beta1
---
# StorageBucketObject

`storage.gcp.hfl.dev/v1beta1`

**Gap it fills.** a small object in a bucket (a configuration file, a script), its content inline or from a Secret or ConfigMap; in no Config Connector code or pull request

**Source.** written here from the Cloud Storage JSON API

## Notes

2026-11-15: moved from `storage.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
