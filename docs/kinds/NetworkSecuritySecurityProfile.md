---
kinds: [NetworkSecuritySecurityProfile]
apiVersion: networksecurity.gcp.hfl.dev/v1beta1
---
# NetworkSecuritySecurityProfile

`networksecurity.gcp.hfl.dev/v1beta1`

**Gap it fills.** a security profile (threat prevention overrides, URL filters, or a mirroring or intercept endpoint group) owned by an organization, for its hierarchical firewall policies, or by a project (Terraform's `google_network_security_security_profile`, whose `parent` is either); Config Connector's `NetworkSecuritySecurityProfile` (alpha) takes a project only

**Source.** Config Connector's direct `NetworkSecuritySecurityProfile`, with an organization parent and its REST clients

## Notes

2026-11-15: moved from `networksecurity.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | PASS | none (run `cce-11120814-5d2a`, the fallback project) | the test's security profiles (deleted); `task live-list` empty |
