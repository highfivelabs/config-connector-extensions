---
kinds: [ComputeFirewallPolicyPacketMirroringRule]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeFirewallPolicyPacketMirroringRule

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a packet mirroring rule of a hierarchical (organization or folder) firewall policy (`firewallPolicies.addPacketMirroringRule`), which no Config Connector kind has (foundation item 2)

**Source.** written here after our `ComputeNetworkFirewallPolicyPacketMirroringRule` (the global network policy's), whose match, secure tag and layer 4 types, spec mapping, comparison and patch body it shares; the policy reference is Config Connector's `ComputeFirewallPolicyRef`

## Notes

2026-11-23: built (foundation item 2, its last buildable part). The spec
is the global kind's (`priority`, `action`, `direction`, `match`,
`ruleName`, `description`, `disabled`, `securityProfileGroup`,
`targetSecureTags`, a CEL rule that a `mirror` rule names a security
profile group) with `firewallPolicyRef` naming Config Connector's
`ComputeFirewallPolicy` by name (from its `status.selfLink`) or a policy
by `external` (`locations/global/firewallPolicies/{id}`). The identity is
ours, `locations/global/firewallPolicies/{id}/packetMirroringRules/{priority}`.

The four calls (`firewallPolicies.getPacketMirroringRule`,
`addPacketMirroringRule`, `patchPacketMirroringRule`,
`removePacketMirroringRule`) are in the compute beta (and alpha) API only,
and neither pinned Go client (cloud.google.com/go/compute v1,
google.golang.org/api compute/v0.beta at v0.287.1) has them, so the
controller sends them as REST to `compute/beta`, and waits for their global
organization operations with the v1 client. A patch sends every field of
the spec, an emptied list as `[]` (as the global kind). A missing rule is
a 404 or the 400 "does not contain a packet mirroring rule at priority"
(assumed after the global policy's; to be confirmed live). Patch 0058
serves the calls in mockgcp, whose protos are v1 only.

The test (`TestComputeFirewallPolicyPacketMirroringRule`) makes a policy
under the test folder (a made-up folder on the fake), rules at two
priorities, changes one, deletes one and abandons the other, and deletes
the policy. Live: pending the test folder (`task test-live KINDS=compute
LIVE_RUN=TestComputeFirewallPolicyPacketMirroringRule` with
`E2E_TEST_FOLDER`; it skips live without it). What to look for: the real
answer for a missing priority, whether a patch keeps fields the body
leaves out, and the permission a folder's policy needs
(`compute.firewallPolicies.update` on the folder). A live `mirror` rule
needs a security profile group with a custom mirroring profile, which no
test makes.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
