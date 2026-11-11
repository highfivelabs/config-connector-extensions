---
kinds: [ComputeRegionNetworkFirewallPolicyRule]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeRegionNetworkFirewallPolicyRule

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** one rule of a regional network firewall policy, addressed by its priority (action, direction, match on ranges, protocols and ports, address groups, FQDNs, region codes, threat intelligence and secure tags, targets); Config Connector's `ComputeFirewallPolicyRule` is for organization and folder policies

**Source.** written here from the Compute Engine API, with the match fields of `ComputeFirewallPolicyRule`

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
