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

2026-11-23 (foundation item 26): `SecretManagerSecret` serves three more
fields. `versionDestroyTtl` (delayed destruction of versions, a duration
with an "s" suffix) and `customerManagedEncryption` (a regional secret's
own Cloud KMS key, by `kmsKeyRef`; a global secret's key stays in its
replication) are in Config Connector's types as `NOTYET`, still so on
master, with no pull request; ours uncomments them and maps them.
`tags` (resource manager tags bound at create, Terraform's `tags`) is
input only and immutable (CEL): sent with the create, never compared. A
change of the key is patched whole (`customerManagedEncryption`).
Tested on the fake (`TestSecretFields`: the create sends the three, a
resync patches nothing, a changed `versionDestroyTtl` is patched alone,
`tags` cannot change; mockgcp patch 0059 takes `versionDestroyTtl` in the
update mask). The fake keeps and returns `tags`, which the real API does
not. Live: `versionDestroyTtl` runs live in the default `secretmanager`
run; the key and the tags are fake only (the live-test project has no
key the Secret Manager service agent may use and no tag to bind). Not
built: `secretType` (immutable, a type for managed rotation features
nothing here uses).

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-11 | `e8a920c` | PASS | PASS | none (`TestRegionalSecret`, `TestGlobalSecretUnchanged`; run `cce-11112128-d3db`, the fallback project) | 1 regional and 1 global secret (deleted); `task live-list` empty |
| 2026-11-12 | `4b47846` | PASS | PASS | none: the slot move (`TestSlotMoveSecretManagerSecret`, two managers as two slots: the new slot refused without the takeover annotation, adoption changed only the `hfl-dev-slot` label with the same name and creation time, the resumed old slot refused with no write, its delete refused, abandon kept the secret; run `cce-11120948-6337`, the fallback project) | 1 regional secret (deleted); `task live-list` found 0 |
