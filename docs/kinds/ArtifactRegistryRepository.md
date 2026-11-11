---
kinds: [ArtifactRegistryRepository]
apiVersion: artifactregistry.gcp.hfl.dev/v1beta1
---
# ArtifactRegistryRepository

`artifactregistry.gcp.hfl.dev/v1beta1`

**Gap it fills.** `remoteRepositoryConfig.<format>Repository.customRepository.uri`: a remote repository that pulls through an upstream of your choosing (Docker, Maven, npm, Python), not only the public ones Artifact Registry names, with `upstreamCredentials` (username and Secret Manager password version) for one that needs a login; in no Config Connector release or on master

**Source.** master's direct controller plus upstream PR #13517; `upstreamCredentials` written here

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
