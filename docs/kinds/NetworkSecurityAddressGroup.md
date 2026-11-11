---
kinds: [NetworkSecurityAddressGroup]
apiVersion: networksecurity.gcp.hfl.dev/v1alpha1
---
# NetworkSecurityAddressGroup

`networksecurity.gcp.hfl.dev/v1alpha1`

**Gap it fills.** an address group (IP ranges that firewall policy rules name as `srcAddressGroups` or `destAddressGroups`) owned by an organization, for its hierarchical firewall policies, or by a project (Terraform's `google_network_security_address_group`, whose `parent` is either); Config Connector's `NetworkSecurityAddressGroup` (alpha) takes a project only

**Source.** Config Connector's direct `NetworkSecurityAddressGroup`, with an organization parent

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-11 | `e8a920c` | PASS | PASS | none for a project's address group (run `cce-11112128-d3db`, the fallback project; the organization subtest skips live) | 2 global address groups (deleted); `task live-list` empty |
