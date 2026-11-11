---
kinds: [SecretManagerSecret, SecretManagerSecretVersion]
apiVersion: secretmanager.gcp.hfl.dev/v1beta1
---
# SecretManagerSecret, SecretManagerSecretVersion

`secretmanager.gcp.hfl.dev/v1beta1`

**Gap it fills.** `spec.location`: a regional secret (Terraform's `google_secret_manager_regional_secret`), in no Config Connector release or on master

**Source.** master's direct controllers plus upstream PR #6651

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-11 | `e8a920c` | PASS | PASS | none (`TestRegionalSecret`, `TestGlobalSecretUnchanged`; run `cce-11112128-d3db`, the fallback project) | 1 regional and 1 global secret (deleted); `task live-list` empty |
