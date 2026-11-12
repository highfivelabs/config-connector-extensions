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
| 2026-11-12 | `4b47846` | PASS | PASS | none: the slot move (`TestSlotMoveSecretManagerSecret`, two managers as two slots: the new slot refused without the takeover annotation, adoption changed only the `hfl-dev-slot` label with the same name and creation time, the resumed old slot refused with no write, its delete refused, abandon kept the secret; run `cce-11120948-6337`, the fallback project) | 1 regional secret (deleted); `task live-list` found 0 |
