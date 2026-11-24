---
kinds: [ComputeFirewallPolicyRule]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeFirewallPolicyRule

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a rule of a hierarchical (organization or folder) firewall policy with `ruleName`, `securityProfileGroup` and `tlsInspect` (Cloud NGFW Enterprise inspection), `targetSecureTags`, `match.srcSecureTags`, the network contexts and `match.srcNetworks`, which Config Connector's direct `ComputeFirewallPolicyRule` lacks; its update sends the whole rule, so ours owns the whole rule (foundation item 19; `docs/design.md` section 2.3)

**Source.** Config Connector's direct `ComputeFirewallPolicyRule` types, identity and controller; the spec mapping, comparison and patch body after our `ComputeNetworkFirewallPolicyRule`

## Which kind to use

Config Connector's `compute.cnrm.cloud.google.com/v1beta1`
`ComputeFirewallPolicyRule` for a rule that needs none of the fields
above; this one for a rule that does. The schema is Config Connector's
plus those fields, so a manifest moves by changing its `apiVersion` (and
`status.externalRef` is spelled the same). Never manage one rule with
both: each puts back the other's change on every reconcile.

To move a rule that Config Connector manages:

1. Create this kind's object with the same `firewallPolicyRef` and
   `priority` and the rule's fields (plus the new ones). It finds the
   rule, does not add one, and patches only what differs.
2. Wait for it to be Ready.
3. Annotate Config Connector's object with
   `cnrm.cloud.google.com/deletion-policy: abandon` and delete it.

Moving back, once Config Connector has the fields in a release, is the
same the other way round; then this kind is retired
(`docs/design.md` section 3.4).

## Notes

2026-11-23: built (foundation item 19). Types copied from Config
Connector's (`priority`, `action`, `direction`, `match`, `description`,
`disabled`, `enableLogging`, `firewallPolicyRef`, `targetResources`,
`targetServiceAccounts`) with the added fields named and shaped as our
`ComputeNetworkFirewallPolicyRule`'s, and its CEL rules: a security
profile group exactly when the action is `apply_security_profile_group`,
TLS inspection only with that action; the priority and the policy are
immutable. The controller makes upstream's calls (`firewallPolicies.getRule`,
`addRule`, `patchRule` with the priority as a query parameter only,
`removeRule`), compares the fields the spec manages (a network, a target
network and a group by path), and sends `patchRule` with every field of the
spec and `[]` for a list the spec emptied, `""` for a group and `false` for
TLS inspection taken off (as the global network rule does after a live
run showed `patchRule` keeps a list left out). A delete of a rule that was
made resolves nothing (`status.externalRef`). Patch 0061 makes mockgcp's
`addRule` add rather than replace.

The test (`TestComputeFirewallPolicyRule`) makes a policy under the test
folder (made up on the fake), a rule with a name and a source network
context, then sets and takes off a security profile group, TLS
inspection, source networks and a target network, and secure tags (fake
only), checks a resync writes nothing, takes over a rule already in the
policy at another priority with one patch and no `addRule`, and deletes
one rule and abandons the other. Live: pending the test folder (`task
test-live PROJECT=$GCP_PROJECT KINDS=compute
LIVE_RUN=TestComputeFirewallPolicyRule` with `E2E_TEST_FOLDER`; the group
case also needs `E2E_SECURITY_PROFILE_GROUP` and skips without it).

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
