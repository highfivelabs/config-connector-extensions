---
kinds: [LoggingSettings]
apiVersion: logging.gcp.hfl.dev/v1beta1
---
# LoggingSettings

`logging.gcp.hfl.dev/v1beta1`

**Gap it fills.** an organization's or folder's Log Router settings: the CMEK key, the default log bucket location, the `_Default` sink off for new projects; in no Config Connector code (one closed scaffold PR)

**Source.** written here from the Cloud Logging API

## Notes

2026-11-15: moved from `logging.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

2026-11-16: a delete, and an abandon, resolve no reference and build no
client: the settings stay as they are and nothing is called, but the
delete used to resolve `folderRef` first, so once the Folder object was
gone (everything deleted at once: the catalog's folder chart on the fake
cloud, run `off-catalog-1116143047-24863`) the object never went. Three
mock cases: the Folder object deleted first, a Folder object that never
existed, and an abandoned object whose Folder object went first. The same
rule now holds for every kind (`pkg/deleting`, `docs/design.md` section 7
item 43).

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
