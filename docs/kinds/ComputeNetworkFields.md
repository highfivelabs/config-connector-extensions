---
kinds: [ComputeNetworkFields, ComputeSubnetworkFields]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeNetworkFields and ComputeSubnetworkFields

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a VPC network's BGP best path selection (`routingConfig.bgpBestPathSelectionMode`, `bgpAlwaysCompareMed`, `bgpInterRegionCost`) and a subnet's `allowSubnetCidrRoutesOverlap`, which Config Connector's `ComputeNetwork` and `ComputeSubnetwork` lack (foundation item 16); field kinds (`docs/design.md` section 6.6, pattern 2, and decision 37) that set only those fields on a network or subnet Config Connector (or anything else) made

**Source.** written here from the Compute Engine API; the references (`resourceRef`) and identities are Config Connector's `ComputeNetworkRef` (`apis/compute/refs`) and `ComputeSubnetworkRef` (`apis/compute/v1beta1`)

## Notes

2026-11-23: built (foundation item 16, decision 19). `spec.resourceRef`
names Config Connector's object by `name` (resolved from its
`status.externalRef`, else its `status.selfLink`) or any network or subnet
by `external`.

`ComputeNetworkFields.spec.routingConfig` owns the fields it sets of
`bgpBestPathSelectionMode` (`LEGACY` or `STANDARD`), `bgpAlwaysCompareMed`
and `bgpInterRegionCost` (`DEFAULT` or `ADD_COST_TO_MED`); a CEL rule
refuses the last two unless the spec sets the mode to `STANDARD`, as the
API does. `routingMode` stays Config Connector's. The controller reads the
network with `networks.get` and writes with `networks.patch`, a JSON merge
patch whose body is `routingConfig` with only the owned fields that differ
(and the mode whenever the spec sets it). Config Connector's
`ComputeNetwork` runs Terraform by default, which sends `routingConfig`
with `routingMode` alone when that changes; merged, it leaves the BGP
fields. The test sends that patch and checks. With the opt-in direct
controller, Config Connector sends `routingConfig` from its spec too, as
a merge patch without these fields, so they stay as well (not tested).

`ComputeSubnetworkFields.spec.allowSubnetCidrRoutesOverlap` is required.
The controller writes with `subnetworks.patch`, the field and the
fingerprint just read. Terraform's subnet updates each send one group of
fields with the fingerprint (the test sends its secondary range update),
never this one.

Neither object creates or deletes the resource: a missing one is waited
for (not Ready, "does not exist yet"). Delete and `deletion-policy:
abandon` leave the fields: going back to `LEGACY` would change how the
network's Cloud Routers choose routes, and turning the overlap off would
make the API refuse routes relying on it. A second object for the same
resource in the namespace is refused. Patch 0056 teaches mockgcp the
writes.

Live: pending (`task test-live KINDS=compute
LIVE_RUN='TestComputeNetworkFields|TestComputeSubnetworkFields'`; the
network and subnet are the run's own and cost nothing). What to look for:
whether the API resets `bgpAlwaysCompareMed` and `bgpInterRegionCost` when
the mode goes back to `LEGACY`, the defaults it reports with `STANDARD`,
and whether a subnet patch needs anything beside the fingerprint.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
