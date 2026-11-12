---
kinds: [ComputeNetworkEndpoint]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeNetworkEndpoint

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** one endpoint (an address and port, optionally on a VM) in a zonal network endpoint group, as a stable kind; Config Connector's `ComputeNetworkEndpoint` is alpha only (decision 26). Terraform's `google_compute_network_endpoint`, which six catalog modules use (the internal and cross-region load balancers' hybrid and VM backends)

**Source.** hand-written, not generated: copied from Config Connector's direct `ComputeNetworkEndpoint` (types, identity, mapper, controller; `UPSTREAM.md`). An endpoint has no create, get or delete of its own (it is attached and detached through its group, and found by listing the group), so `hack/discokind` cannot express it.

## Notes

The spec keeps upstream's fields: `networkEndpointGroupRef` (Config
Connector's `ComputeNetworkEndpointGroup`, or `external:
projects/P/zones/Z/networkEndpointGroups/N`), `zone`, `ipAddress`,
`instanceRef`, and the port as `resourceID`, as upstream has it, so an
object moves to upstream's kind unchanged when Config Connector promotes
it. The CRD makes the spec immutable (CEL); upstream's controller refuses
a change instead. The test uses a `NON_GCP_PRIVATE_IP_PORT` group (hybrid
endpoints need no VM), so it runs live for the cost of a network and a
group.

One change from upstream's controller, found live: the API lists a
`NON_GCP_PRIVATE_IP_PORT` group's endpoints with `"instance": ""` and
refuses that same field on detach (400 "An instance name cannot be
associated with network endpoint of type NON_GCP_PRIVATE_IP_PORT"), so
the detach sends no empty instance. The fake accepts either.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `58c848a` | PASS | FAIL | the delete never finished; the test said nothing (it now prints the Ready condition) (run `cce-11121846-91d6`) | 1 network, 1 zonal NEG with 3 endpoints (deleted with the group) |
| 2026-11-12 | `9952ffc` | PASS | FAIL | detach: 400 "Invalid value for field 'resource.instance': ''. An instance name cannot be associated with network endpoint of type NON_GCP_PRIVATE_IP_PORT"; the endpoint was read back with an empty instance and sent so (fixed, with a unit test) (run `cce-11121922-5984`) | the same (deleted with the group) |
| 2026-11-12 | `912326c` | PASS | PASS | none, with the fix: two hybrid endpoints attached, a resync with no attach, the CRD refusing a change, delete detaching one, abandon keeping one (run `cce-11121948-f0b6`, the fallback project) | 1 network, 1 zonal NEG with 3 endpoints (deleted with the group); `task live-list` empty |
