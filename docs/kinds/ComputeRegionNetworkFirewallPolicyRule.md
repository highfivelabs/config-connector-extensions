---
kinds: [ComputeRegionNetworkFirewallPolicyRule]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeRegionNetworkFirewallPolicyRule

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** one rule of a regional network firewall policy, addressed by its priority (action, direction, match on ranges, protocols and ports, address groups, FQDNs, region codes, threat intelligence and secure tags, targets); Config Connector's `ComputeFirewallPolicyRule` is for organization and folder policies

**Source.** written here from the Compute Engine API, with the match fields of `ComputeFirewallPolicyRule`

## Notes

2026-11-15: moved from `compute.gcp.hfl.dev/v1alpha1` to `v1beta1` with its
schema unchanged (`docs/design.md` decision 27); `v1alpha1` is no longer
served, and `CHANGELOG.md` says how to upgrade.

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

2026-11-23 (foundation items 2 and 19): `spec.securityProfileGroup` and
`spec.tlsInspect` (with action `apply_security_profile_group`, CEL rules as
`ComputeNetworkFirewallPolicyRule`'s), and `spec.match.srcNetworkContext`,
`destNetworkContext` and `srcNetworks`. The controller now compares and
patches as the global network rule does (the same functions): taken off,
a group is sent as `""`, TLS inspection as `false`, a network context as
`UNSPECIFIED` and `srcNetworks` as `[]`; a group and a network compare by
path. Tested on the fake (`TestComputeRegionNetworkFirewallPolicyRule`:
the CEL refusals; set, a resync with no write, taken off). Live pending:
the group case needs `E2E_SECURITY_PROFILE_GROUP` and skips without it
(`task test-live PROJECT=$GCP_PROJECT KINDS=compute
LIVE_RUN=TestComputeRegionNetworkFirewallPolicyRule`); to be seen live:
whether a regional rule takes an organization's security profile group
and `srcNetworks`, and the form the API answers for them.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | PASS | none (run `cce-11120814-5d2a`, the fallback project) | 2 regional firewall policies and their rules, 1 network (deleted); `task live-list` empty |
