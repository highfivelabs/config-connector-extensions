---
kinds: [ComputeDiskResourcePolicyAttachment]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeDiskResourcePolicyAttachment

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** a resource policy (a snapshot schedule) attached to a zonal disk, as a stable kind; Config Connector's `ComputeDiskResourcePolicyAttachment` is alpha only (decision 26). Terraform's `google_compute_disk_resource_policy_attachment` (rank 60 of the catalog's `blocked-kinds.md`)

**Source.** hand-written, not generated: copied from Config Connector's direct `ComputeDiskResourcePolicyAttachment` (types, identity, controller; `UPSTREAM.md`). An attachment is made with the disk's `addResourcePolicies`, removed with `removeResourcePolicies`, and found in the disk's `resourcePolicies`; it has no create, get or delete of its own, so `hack/discokind` cannot express it.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `projectRef`, `zone`,
`diskRef` (Config Connector's `ComputeDisk` by name, or `external`) and
`resourceID`, the name of the resource policy in the disk's region. The
spec is immutable (CEL); upstream's controller ignores a change instead.

The test makes two 10 GB pd-standard disks and a daily snapshot schedule
through the API: create (one `addResourcePolicies`), a resync that writes
nothing, the CRD refusing a change, delete (the schedule detached),
abandon (kept). The disks are deleted before the schedule; the sweeper
deletes disks and then resource policies by name.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
