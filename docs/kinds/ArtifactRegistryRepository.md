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

2026-11-23 (application queue item 4): `vulnerabilityScanningConfig`
(`enablementConfig`: `INHERITED` or `DISABLED`), `disallowUnspecifiedMode`
(immutable, by CEL) and a remote repository's `aptRepository` and
`yumRepository` (a public base and path, or a custom URI), uncommented
from upstream's generated types and mapped where upstream's generated
mapper says "MISSING". Upstream read first (read-only: master
`a5456d80`, releases to `v1.162.0`, the heads of the open pull requests
to #16337): master still generates them as unreachable, and no pull
request maps them. A spec without `vulnerabilityScanningConfig` or
`disallowUnspecifiedMode` leaves the repository's own (the API's default
scanning config is never a difference); a changed scanning config is
sent with the `vulnerabilityScanningConfig` mask alone. An Apt or Yum
upstream, like the other remote sources, cannot change in place.
Not built: `platformLogsConfig` and `remoteRepositoryConfig.noCache`
(newer than the pinned Go client and mockgcp's protos).
mockgcp patch 0068 takes the scanning config's update mask path and gives
a sent config its state. Tests: `TestArtifactRegistryRepositoryFields`
and `TestArtifactRegistryAptYumRemoteRepositories` (fake). Live pending:
`task test-live PROJECT=$GCP_PROJECT KINDS=artifactregistry
LIVE_RUN='TestArtifactRegistryRepositoryFields|TestArtifactRegistryAptYumRemoteRepositories'`
(empty repositories: free; the public Debian and Rocky upstreams are
validated live, the custom Yum one is not). To be seen live: whether the
API takes `vulnerabilityScanningConfig` in an update mask and what state
it reads for each enablement config.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-16 | `7732c02` | FAIL in the run (its mock log is empty: no test ran); PASS in `task ci` on the same commit (offload/ci) | PASS | run `cce-11160906-a434`: a changed custom upstream URI replaces the repository live as on mockgcp (decision 21, ARURI): one DELETE, no PATCH, a later `createTime`, the spec's fields on the new repository, Ready, and a resync after it sends nothing; credential and description changes delete nothing | 7 remote repositories (one replaced once), 1 Docker repository with 2 minimal images, 1 secret (deleted); sweep empty |
