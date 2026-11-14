---
kinds: [ComputePerInstanceConfig]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputePerInstanceConfig

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** one named instance of a zonal managed instance group, with its preserved state (stateful disks, IPs, metadata), as a stable kind; Config Connector's `ComputePerInstanceConfig` is a Terraform-based alpha kind only (decision 26). Terraform's `google_compute_per_instance_config`, which the catalog's `compute-mig` module uses (rank 55 of its `blocked-kinds.md`)

**Source.** hand-written, not generated: adapted from our `ComputeRegionPerInstanceConfig` (itself copied from Config Connector's direct regional kind; `UPSTREAM.md`), with the zonal group's methods for the regional group's and the field names of Config Connector's Terraform-based `ComputePerInstanceConfig`. A per-instance config has no create, get or delete of its own: it is written with the group's `updatePerInstanceConfigs` (made and changed alike), applied to its instance with `applyUpdatesToInstances`, removed with `deletePerInstanceConfigs`, and found by listing the group's configs, so `hack/discokind` cannot express it.

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

Create and update both write the config with
`updatePerInstanceConfigs`, then apply it to the group's instance of that
name with the spec's actions (defaults `NONE` and `REPLACE`), when the
group has one. Create used `createInstances` until 2026-11-14, which makes
a new instance and raises the group's target size: the catalog's live run
in hfl-dev-514205 found the group past its `targetSize`, and it stayed
there. The pinned Terraform provider also calls `createInstances` on
create (and `updatePerInstanceConfigs` on update); using
`updatePerInstanceConfigs` for both is the builder's choice, so a config
never changes the group's size and names an instance the group already
has. Patch 0048 makes mockgcp's `createInstances` raise the target size,
as the API does, and the test checks it stays put.

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
| 2026-11-13 | `19f3801` | PASS | PASS | none: createInstances on a zonal group with preserved metadata, a resync with no write, one update and one apply for changed metadata, the CRD refusing a change of group, delete, abandon (run `cce-11130243-a782`) | 1 network and subnet, 1 template, 1 zonal group with 2 e2-micro VMs (10 GB pd-standard); all deleted by the test, the run's sweep matched nothing |
| 2026-11-14 | `64f375e` | PASS | PASS | none: a zonal group of targetSize 2, a config on one of its instances written with updatePerInstanceConfigs (no createInstances call), one apply, targetSize still 2; a resync with no write, one update and one apply for changed metadata, the CRD refusing a change of group, delete, abandon (run `cce-11140351-e031`) | 1 network and subnet, 1 template, 1 zonal group with 2 e2-micro VMs (10 GB pd-standard); all deleted by the test, the run's sweep matched nothing |
