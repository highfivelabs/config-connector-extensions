---
kinds: [ComputeRegionSecurityPolicyRule]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeRegionSecurityPolicyRule

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** Terraform's `google_compute_region_security_policy_rule`: a rule of a regional Cloud Armor security policy, a regional external load balancer's (`CLOUD_ARMOR`, by `match`) or a network edge policy's (`CLOUD_ARMOR_NETWORK`, by `networkMatch` and the policy's user-defined fields). Config Connector has no regional security policy or rule kind (foundation item 27)

**Source.** written here, after Terraform's resource (read, not copied); the rule's types (`match`, `rateLimitOptions`, `preconfiguredWafConfig`) copied from Config Connector's direct `ComputeSecurityPolicy` (`spec.rule[]`), `networkMatch` from its generated types

## Usage

The policy is a `ComputeRegionSecurityPolicy` object of ours, by
`securityPolicyRef.name`, or any regional policy by
`securityPolicyRef.external`
(`projects/{project}/regions/{region}/securityPolicies/{name}`). The
rule is named by `priority`; both are immutable.

```yaml
apiVersion: compute.gcp.hfl.dev/v1alpha1
kind: ComputeRegionSecurityPolicyRule
metadata:
  name: login-throttle
spec:
  securityPolicyRef:
    name: regional-armor
  priority: 1000
  action: throttle
  match:
    expr:
      expression: request.path.matches('/login')
  rateLimitOptions:
    conformAction: allow
    exceedAction: deny(429)
    rateLimitThreshold:
      count: 100
      intervalSec: 60
    enforceOnKey: IP
```

A rule at the default rule's priority (2147483647) takes over the rule the
API adds to a `CLOUD_ARMOR` policy (to change its action, say); deleting
that object leaves the rule as it is, since the API never removes it.

## Notes

2026-11-23: built (foundation item 27). Upstream read first (read-only:
master `bc464f55`, releases to `v1.162.0`, the heads of every open and
closed pull request from #13500 on): Config Connector has no regional
security policy rule, and its direct `ComputeSecurityPolicy` (opt-in)
keeps a policy's rules in its own body. The controller calls
`regionSecurityPolicies.getRule`, `addRule`, `patchRule` (with an update
mask naming the fields that differ, Terraform's paths: `network_match`'s
eight leaves for `networkMatch`) and `removeRule`, on
`google.golang.org/api`'s compute v1 client. It compares only the spec's
fields: unset, `""`, `false`, `0` and `[]` are the same, the action is
compared without case, and a rate limit keyed on nothing equals the API's
default key `ALL`. Not in the kind: `headerAction`, `redirectOptions` and
a rate limit's `exceedRedirectOptions` (the API takes them only on a
global `CLOUD_ARMOR` policy), and `match.exprOptions` (reCAPTCHA, global
only). The WAF exclusions keep Config Connector's names (`exclusion`,
`requestHeader`, `operator`, `value`); the controller sends the API's.

mockgcp patch 0063 serves the rule calls and adds a `CLOUD_ARMOR`
policy's default rule; `ComputeRegionSecurityPolicy`'s collection is now
served by mockgcp (`mockgcp: true`), so the policy and its rules share one
fake. The tests: `TestComputeRegionSecurityPolicyRule` (a `CLOUD_ARMOR`
policy object, a rule by source ranges, then an expression with
preview, a rate limit keyed on two keys and a WAF exclusion, set and taken
off with a resync writing nothing after each; the default rule taken over
without `addRule` and left by the delete; the CRD's refusals; abandon,
then an object by `external` taking the rule over; delete) and
`TestComputeRegionSecurityPolicyRuleNetwork` (fake only: a
`CLOUD_ARMOR_NETWORK` policy with a user-defined field, a rule on it, its
network match changed). The fake does not check which fields a policy's
type takes, the expression language, or a rule's limits.

Live: pending. `task test-live PROJECT=$GCP_PROJECT KINDS=compute
LIVE_RUN=TestComputeRegionSecurityPolicyRule` (a `CLOUD_ARMOR` policy in
us-central1 with a few rules for a few minutes: Cloud Armor Standard bills
a policy and its rules by the month while they exist); the network case
skips live (Cloud Armor Enterprise). To be seen live: the API's answer for
a missing rule (assumed 404, or 400 about the priority), whether
`patchRule` takes the update mask paths and clears a field the mask names
but the body leaves out, whether `evaluatePreconfiguredWaf` with a rate
limit is accepted on a regional policy, and the default rule's fields as
the API makes them.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
