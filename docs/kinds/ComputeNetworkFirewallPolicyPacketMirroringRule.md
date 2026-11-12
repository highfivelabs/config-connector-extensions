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
| 2026-11-12 | `e1e23f5` | PASS | FAIL | the patch subtest, now printing the Ready message, showed why the patch never landed (run `cce-11100936-3a4c`): the test emptied the INGRESS rule's `srcIpRanges`, and `patchPacketMirroringRule` answers 400 "Must specify src_address_groups, ... or src_threat_intelligence for ingress direction." The test now changes the source and empties the destination; patch 0032 refuses an INGRESS rule without a source, and the old test fails on the fake (run `cce-11120814-5d2a`, the fallback project) | 1 network firewall policy with 2 rules (deleted) |
| 2026-11-12 | `78644ec` | PASS | PASS | none: create, a resync with no write, one patch for a changed source, description and rule name and an emptied destination, a second rule, delete and abandon (run `cce-11120932-1df3`, the fallback project) | 1 network firewall policy with 2 rules (deleted); `task live-list` empty |
