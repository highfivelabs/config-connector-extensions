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

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
