---
kinds: [ComputeRegionNetworkFirewallPolicyAssociation]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeRegionNetworkFirewallPolicyAssociation

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** attaches a regional network firewall policy to a VPC network; Config Connector has no association for it

**Source.** written here from the Compute Engine API

## Notes

2026-11-15: moved from `compute.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
