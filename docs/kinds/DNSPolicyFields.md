---
kinds: [DNSPolicyFields]
apiVersion: dns.gcp.hfl.dev/v1alpha1
---
# DNSPolicyFields

`dns.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud DNS server policy's `dns64Config`, which Config Connector's `DNSPolicy` lacks (foundation item 15); a field kind (`docs/design.md` section 6.6, pattern 2, and decision 37) that sets only that field on a policy Config Connector's `DNSPolicy` (or anything else) made

**Source.** written here from the Cloud DNS API; the reference (`resourceRef`) and identity are Config Connector's `DNSPolicyRef` and `DNSPolicyIdentity` (`apis/dns/v1beta1`)

## Notes

2026-11-23: built (foundation item 15, decision 19). `spec.resourceRef`
names Config Connector's `DNSPolicy` by `name` (resolved from its spec and
project) or any policy by `external` (`projects/{project}/policies/{policy}`).
`spec.dns64Config.scope.allQueries` is required. The controller reads the
policy with `policies.get` and writes with `policies.patch`, a partial
update whose body has only `dns64Config`.

Why the field kind is safe beside Config Connector's object, and when it
is not: Config Connector's `DNSPolicy` runs Terraform by default, whose
patch sends `alternativeNameServerConfig`, `description`,
`enableInboundForwarding`, `enableLogging` and `networks`, never
`dns64Config`; the test sends such a patch and checks DNS64 stays on.
Config Connector's opt-in direct `DNSPolicy` controller
(`alpha.cnrm.cloud.google.com/reconciler: direct`) sends `policies.update`,
a whole replace, and would turn DNS64 off on each of its updates; do not
use this kind with it.

Delete turns DNS64 off (`allQueries: false`); with `deletion-policy:
abandon` it stays. A second object for the same policy in the namespace
is refused. Patch 0057 makes mockgcp's `policies.patch` a partial update,
as the API's is.

Not in this kind (foundation item 15): the forwarding targets'
`ipv6Address` and `domainName`, which sit in `alternativeNameServerConfig`,
which Terraform sends whole; a record set's routing policy fencing and
trickle traffic, which need a whole-object `DNSRecordSet`.

Live: pending (`task test-live KINDS=dns LIVE_RUN=TestDNSPolicyFields`
with `E2E_DNS_ADMIN=true`, as the other DNS kinds; the policy is the run's
own, bound to no network, and costs nothing). What to look for: whether
the API takes DNS64 on a policy bound to no network, and whether it
returns `dns64Config` with `allQueries: false` or drops it when off.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
