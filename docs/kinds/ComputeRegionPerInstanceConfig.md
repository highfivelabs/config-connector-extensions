---
kinds: [ComputeRegionPerInstanceConfig]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeRegionPerInstanceConfig

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** one named instance of a regional managed instance group, with its preserved state (stateful disks, IPs, metadata), as a stable kind; Config Connector's `ComputeRegionPerInstanceConfig` is alpha only (decision 26). Terraform's `google_compute_region_per_instance_config`, which the catalog's `compute-mig` module uses (rank 69 of its `blocked-kinds.md`)

**Source.** hand-written, not generated: copied from Config Connector's direct `ComputeRegionPerInstanceConfig` (types, identity, mapper, controller; `UPSTREAM.md`). A per-instance config has no create, get or delete of its own: it is made with the group's `createInstances`, changed with `updatePerInstanceConfigs` and `applyUpdatesToInstances`, removed with `deletePerInstanceConfigs`, and found by listing the group's configs, so `hack/discokind` cannot express it.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `projectRef`, `region`,
`regionInstanceGroupManagerRef` (Config Connector's
`ComputeInstanceGroupManager` by name, or `external`), `resourceID` (the
instance's name), `preservedState`, `minimalAction`,
`mostDisruptiveAllowedAction` and `removeInstanceStateOnDestroy`. The
first four are immutable (CEL). A referenced `ComputeInstanceGroupManager`
is read unstructured and converted, since our manager's scheme has only
our groups.

As upstream does, create calls `createInstances`, which adds the named
instance to the group (the group's target size grows by one); Terraform
calls `updatePerInstanceConfigs` instead and leaves the size alone. An
update writes the config and then applies it to the instance with the
spec's actions (defaults `NONE` and `REPLACE`).

`createInstances` works only on a group whose
`updatePolicy.instanceRedistributionType` is `NONE` (the API answers 412
CONDITION_NOT_MET otherwise, run `cce-11122245-f5ea`), as a stateful
regional group has it; patch 0040 makes mockgcp refuse it too.

The test makes its own group (a network, a subnet, an e2-micro template
and a regional group of size 0, redistribution off) through the API, then: create, a resync
that writes nothing, a change of preserved metadata (one update, one
apply), the CRD refusing a change of group, delete, abandon. Deleting the
group at the end deletes the instances; the sweeper deletes groups,
instances and templates by name.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `b514499` | PASS | FAIL | create: 412 CONDITION_NOT_MET "CreateInstances can be used only when instance redistribution is disabled (set to NONE)": the test's group redistributed (fixed in the test; mockgcp now refuses it, patch 0040) (run `cce-11122245-f5ea`) | 1 network and subnet, 1 template, 1 regional group of size 0; nothing left after the sweep |
