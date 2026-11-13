---
kinds: [ApigeeNATAddress]
apiVersion: apigee.gcp.hfl.dev/v1beta1
---
# ApigeeNATAddress

`apigee.gcp.hfl.dev/v1beta1`

**Gap it fills.** a NAT address reserved for an Apigee instance's egress, as a stable kind; Config Connector's `ApigeeNATAddress` is alpha only (decision 26). Terraform's `google_apigee_nat_address`, which the catalog's `apigee` module uses (rank 33 of its `blocked-kinds.md`)

**Source.** hand-written, not generated: copied from Config Connector's direct `ApigeeNATAddress` (types, identity, reference, mapper, controller; `UPSTREAM.md`). The address lives under an Apigee organization and instance, not a project or location, which `hack/discokind`'s parents do not express.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `instanceId`
(`organizations/O/instances/I`, or the name of a Config Connector
`ApigeeInstance` in the namespace) and `resourceID` (the address's name;
`metadata.name` when unset). Both are immutable (CEL): the API has no
update, so upstream's controller only refreshes status. Status adds
`externalRef` to upstream's `ipAddress` and `state`.

The address is created `RESERVED`; activating it (Terraform's `activate`)
is a separate call upstream's kind does not model, so neither does ours.

The test (`TestApigeeNATAddress`): create, a resync that writes nothing,
the CRD refusing a change of instance or name, delete, abandon, and a new
object acquiring the abandoned address by `resourceID` and deleting it.

Live is pending: it needs a provisioned Apigee organization and instance
(billed, and an hour or more to make), which the live-test project has
not, and `roles/apigee.admin` for the live-test service account, which it
lacks (read 2026-11-12). The test skips live unless `E2E_APIGEE_INSTANCE`
names an instance. What the fake cannot show: how long the API takes to
reserve an address, the address's real state transitions, and the API's
answer for an instance that does not exist (the fake does not check).

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
