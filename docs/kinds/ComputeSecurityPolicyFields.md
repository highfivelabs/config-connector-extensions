---
kinds: [ComputeSecurityPolicyFields]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeSecurityPolicyFields

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud Armor security policy's Adaptive Protection threshold configs (`adaptiveProtectionConfig.layer7DdosDefenseConfig.thresholdConfigs`) and WAF request body inspection size (`advancedOptionsConfig.requestBodyInspectionSize`), which Config Connector's `ComputeSecurityPolicy` lacks (foundation item 27); a field kind (`docs/design.md` section 6.6, pattern 2, and decision 37) that sets only those fields on a global policy Config Connector's `ComputeSecurityPolicy` (or anything else) made

**Source.** written here from the Compute Engine API; the reference (`resourceRef`) and the policy's identity are Config Connector's `ComputeSecurityPolicyRef` and `ComputeSecurityPolicyIdentity` (`apis/compute/v1beta1`); the threshold config types are Config Connector's generated ones (`apis/compute/v1beta1/types.generated.go`, commented out there as unreachable; `UPSTREAM.md`)

## Notes

2026-11-23: built (foundation item 27, decision 19), the item's
highest-value fields first. `spec.resourceRef` names Config Connector's
`ComputeSecurityPolicy` by `name` (resolved from its spec's `resourceID`
and project) or any global policy by `external`
(`projects/{project}/global/securityPolicies/{name}`); a regional policy
is refused. `spec.adaptiveProtectionConfig.layer7DdosDefenseConfig.thresholdConfigs`
(each named; detection and auto-deploy thresholds and traffic
granularity, the API's names) is owned whole and compared by name, on
the fields the spec sets; `spec.advancedOptionsConfig.requestBodyInspectionSize`
(`8KB` to `64KB`) is compared ignoring case. At least one is set.

The controller reads the policy with `securityPolicies.get` and writes
with `securityPolicies.patch`, a JSON merge patch holding the fingerprint
and only the sub-objects with an owned field that differs, as read with
that field changed; to clear a field (delete, or an empty list) it adds
an update mask naming it, since a merge patch cannot remove one. Config
Connector's `ComputeSecurityPolicy` runs Terraform by default, whose
update sends only the sub-objects it changed (`adaptiveProtectionConfig`
with `enable`, `ruleVisibility` and `autoDeployConfig`;
`advancedOptionsConfig` with its own fields) and no mask: a merge keeps
ours; mockgcp replaces the sub-object, so the test checks that ours come
back at the next reconcile while Config Connector's change stays. Config
Connector's opt-in direct controller sends its whole object and reads a
threshold config with auto-deploy fields as its own `autoDeployConfig`:
this kind is not for a policy on the direct controller.

Delete clears both fields; with `deletion-policy: abandon` they stay. A
second object for the same policy in the namespace is refused.

Not built (item 27): `userDefinedFields` and `ddosProtectionConfig`
(regional network edge policies, which Config Connector's Terraform kind
does not make), and rule fields beyond Config Connector's.

The test (`TestComputeSecurityPolicyFields`) makes the run's own global
policy, attached to nothing, and deletes it. Live: pending, and skipped
unless `E2E_CLOUD_ARMOR=true` (a policy is billed by the month,
prorated; threshold configs may need Cloud Armor Enterprise):
`E2E_CLOUD_ARMOR=true task test-live PROJECT=$GCP_PROJECT KINDS=compute
LIVE_RUN=TestComputeSecurityPolicyFields`. What to look for: whether the
API merges a sub-object Terraform sends (ours kept) or replaces it, and
whether it takes nested update mask paths to clear.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
