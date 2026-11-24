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

2026-11-23 (foundation item 19): `spec.securityProfileGroup` and
`spec.tlsInspect` (Cloud NGFW Enterprise's layer 7 inspection, with
action `apply_security_profile_group`; a CEL rule refuses a group without
that action, the action without a group, and `tlsInspect: true` with any
other action) and `spec.match.srcNetworks` (with `srcNetworkContext`
`VPC_NETWORKS`). They are sent only when the spec sets them; taken off,
the group is cleared with `""` and TLS inspection with `false`, and
`srcNetworks` with `[]`. A group and a network compare by their path, so a
URL the API answers for a path sent is the same. Live: pending; the
group case skips without `E2E_SECURITY_PROFILE_GROUP`, an existing
group the project may use (`docs/design.md` section 7, item 32).

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
