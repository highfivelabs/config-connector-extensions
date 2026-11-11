---
kinds: [NetworkSecuritySecurityProfileGroup]
apiVersion: networksecurity.gcp.hfl.dev/v1alpha1
---
# NetworkSecuritySecurityProfileGroup

`networksecurity.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a security profile group (the threat prevention, URL filtering, mirroring and intercept profiles a firewall policy rule applies) owned by an organization or a project (Terraform's `google_network_security_security_profile_group`, whose `parent` is either); Config Connector's `NetworkSecuritySecurityProfileGroup` (alpha) takes a project only

**Source.** Config Connector's direct `NetworkSecuritySecurityProfileGroup`, with an organization parent and its REST clients

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
