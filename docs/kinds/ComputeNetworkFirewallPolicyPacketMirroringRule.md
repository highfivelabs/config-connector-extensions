---
kinds: [ComputeNetworkFirewallPolicyPacketMirroringRule]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeNetworkFirewallPolicyPacketMirroringRule

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a packet mirroring rule of a global network firewall policy (which traffic is mirrored to a security profile group's mirroring endpoint group, not mirrored, or passed on), as Terraform's `google_compute_network_firewall_policy_packet_mirroring_rule` has it; in no Config Connector code or pull request

**Source.** written here after our `ComputeNetworkFirewallPolicyRule`; mockgcp patch 0032

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
