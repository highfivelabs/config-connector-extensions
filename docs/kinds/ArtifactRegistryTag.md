---
kinds: [ArtifactRegistryTag]
apiVersion: artifactregistry.gcp.hfl.dev/v1beta1
---
# ArtifactRegistryTag

`artifactregistry.gcp.hfl.dev/v1beta1`

**Gap it fills.** a tag on a package's version (for example `prod` on an image digest), moved by changing `spec.version`; in no Config Connector code, pull request or Terraform resource

**Source.** written here from the Artifact Registry API

## Notes

2026-11-15: moved from `artifactregistry.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | PASS | none (run `cce-11120814-5d2a`, the fallback project) | 1 Docker repository with 2 minimal images (deleted); `task live-list` empty |
