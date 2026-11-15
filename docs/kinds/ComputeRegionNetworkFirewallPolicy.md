---
kinds: [ComputeRegionNetworkFirewallPolicy]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeRegionNetworkFirewallPolicy

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** a regional network firewall policy, for firewall rules that apply to a VPC in one region; Config Connector has only the global policy

**Source.** Config Connector's opt-in direct `ComputeNetworkFirewallPolicy`, for a region

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
