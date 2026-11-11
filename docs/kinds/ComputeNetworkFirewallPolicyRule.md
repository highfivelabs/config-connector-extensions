---
kinds: [ComputeNetworkFirewallPolicyRule]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeNetworkFirewallPolicyRule

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** a rule of a global network firewall policy (direction, ranges, address groups, FQDNs, region codes, threat lists, network contexts, protocols and ports, service account and secure tag targets, logging), as a stable kind; the policy by Config Connector's `ComputeNetworkFirewallPolicy` or by path; Config Connector's is alpha and Terraform-based only (decision 13)

**Source.** written here after our regional rule kind; Config Connector's `ComputeNetworkFirewallPolicyRef`

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
