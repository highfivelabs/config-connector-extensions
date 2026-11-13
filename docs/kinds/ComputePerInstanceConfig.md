---
kinds: [ComputePerInstanceConfig]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputePerInstanceConfig

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** one named instance of a zonal managed instance group, with its preserved state (stateful disks, IPs, metadata), as a stable kind; Config Connector's `ComputePerInstanceConfig` is a Terraform-based alpha kind only (decision 26). Terraform's `google_compute_per_instance_config`, which the catalog's `compute-mig` module uses (rank 55 of its `blocked-kinds.md`)

**Source.** hand-written, not generated: adapted from our `ComputeRegionPerInstanceConfig` (itself copied from Config Connector's direct regional kind; `UPSTREAM.md`), with the zonal group's methods for the regional group's and the field names of Config Connector's Terraform-based `ComputePerInstanceConfig`. A per-instance config has no create, get or delete of its own: it is made with the group's `createInstances`, changed with `updatePerInstanceConfigs` and `applyUpdatesToInstances`, removed with `deletePerInstanceConfigs`, and found by listing the group's configs, so `hack/discokind` cannot express it.

## Notes

The spec has the fields of Config Connector's alpha kind, so an object
moves to upstream's kind unchanged if Config Connector promotes it:
`projectRef`, `zone`, `instanceGroupManagerRef` (Config Connector's
`ComputeInstanceGroupManager` by name, or `external`), `resourceID` (the
instance's name), `preservedState`, `minimalAction`,
`mostDisruptiveAllowedAction` and `removeInstanceStateOnDestroy`. The
first four are immutable (CEL). The preserved state's types are the
regional kind's (the same JSON). A referenced `ComputeInstanceGroupManager`
is read unstructured and converted, since our manager's scheme has only
our groups.

As the regional kind does, create calls `createInstances`, which adds the
named instance to the group (the group's target size grows by one);
Terraform calls `updatePerInstanceConfigs` instead and leaves the size
alone. An update writes the config and then applies it to the instance
with the spec's actions (defaults `NONE` and `REPLACE`). A zonal group has
no instance redistribution, so the regional kind's `NONE` requirement
does not apply.

mockgcp served per-instance configs for regional groups only; patch 0042
adds the zonal group's six methods, as the regional ones behave.

A deleted config stays listed, marked `DELETING` or `UNAPPLIED_DELETION`,
until its instance is next updated (the controller applies the update
only with `removeInstanceStateOnDestroy`); the object is gone at once,
and the fake removes the config at once.

The test makes its own group (a network, a subnet, an e2-micro template
and a zonal group of size 0 in `us-central1-a`) through the API, then:
create, a resync that writes nothing, a change of preserved metadata (one
update, one apply), the CRD refusing a change of group, delete, abandon.
Deleting the group at the end deletes the instances; the sweeper deletes
groups (zonal and regional), instances and templates by name.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
