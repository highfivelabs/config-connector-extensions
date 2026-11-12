---
kinds: [DNSResponsePolicyRule]
apiVersion: dns.gcp.hfl.dev/v1beta1
---
# DNSResponsePolicyRule

`dns.gcp.hfl.dev/v1beta1`

**Gap it fills.** a rule of a Cloud DNS response policy (answer a name with local data, or let it through with `bypassResponsePolicy`), as a stable kind; Config Connector's `DNSResponsePolicyRule` is alpha only and Terraform-based (decision 13). Terraform's `google_dns_response_policy_rule`, which the catalog's `dns-response-policy` module uses (rank 76 of its `blocked-kinds.md`)

**Source.** hand-written: upstream's types, identity and spec mappers copied (`UPSTREAM.md`), the controller written here after `DNSResponsePolicy`'s, since upstream's is Terraform's. Not generated: Cloud DNS v1 paths (`dns/v1/projects/{project}/responsePolicies/{policy}/rules`) take the rule's name in the body, not as an ID parameter, which `hack/discokind` cannot express.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `projectRef`,
`responsePolicy` (the policy's path, `projects/P/responsePolicies/X`
or with `locations/global`, as upstream requires; a bare name is refused),
`resourceID` (the rule's name), `dnsName`, `behavior` and `localData`.
`projectRef`, `responsePolicy` and `resourceID` are immutable (CEL).
`status.externalRef` is ours.

A changed rule is written whole with `update` (PUT), so a rule that moves
from local data to `bypassResponsePolicy`, or back, keeps nothing of the
old answer. A TTL the spec leaves out is taken as the API's default.

mockgcp served no rules at the pin; patch
0035 adds them (create, get, update, patch,
delete, list; a rule of a missing policy is 404, a rule without a DNS name
400). What the fake cannot show: whether the real API refuses a rule with
both or neither of `behavior` and `localData`, and the record types it
refuses (SOA, NS).

Live is pending, as `DNSResponsePolicy`'s: the live-test account has no
DNS role (`roles/dns.admin`), so the test skips live unless
`E2E_DNS_ADMIN=true`. The sweeper deletes a policy's rules before the
policy.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
