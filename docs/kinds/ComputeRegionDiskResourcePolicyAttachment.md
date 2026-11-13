---
kinds: [ComputeRegionDiskResourcePolicyAttachment]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeRegionDiskResourcePolicyAttachment

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** a resource policy (a snapshot schedule) attached to a regional disk, as a stable kind; Config Connector's `ComputeRegionDiskResourcePolicyAttachment` is a Terraform-based alpha kind only (decision 26). Terraform's `google_compute_region_disk_resource_policy_attachment` (rank 58 of the catalog's `blocked-kinds.md`)

**Source.** hand-written, not generated: adapted from our `ComputeDiskResourcePolicyAttachment`, itself a copy of Config Connector's direct zonal attachment (types, identity, controller; `UPSTREAM.md`), for `regionDisks`. Upstream has no direct regional kind, only the Terraform-based one, whose field names the spec keeps. An attachment is made with the disk's `addResourcePolicies`, removed with `removeResourcePolicies`, and found in the disk's `resourcePolicies`; it has no create, get or delete of its own, so `hack/discokind` cannot express it.

## Notes

The spec has the fields of upstream's alpha kind, so an object moves to
upstream's kind unchanged if Config Connector promotes it: `projectRef`,
`region`, `diskRef` (Config Connector's `ComputeDisk` by name, a regional
one, or `external` as `projects/{project}/regions/{region}/disks/{disk}`)
and `resourceID`, the name of the resource policy in the disk's region.
The spec is immutable (CEL). A zonal disk, a disk in another region or
another project is refused before any call.

The test makes two 10 GB pd-balanced regional disks (replicas in
`us-central1-a` and `-b`; a regional pd-standard disk must be at least
200 GB) and a daily snapshot schedule through the API: create (one
`addResourcePolicies`), a resync that writes nothing, the CRD refusing a
change, delete (the schedule detached), abandon (kept). The disks are
deleted before the schedule; the sweeper deletes zonal and regional disks
and then resource policies by name.

mockgcp already served a regional disk's `addResourcePolicies` and
`removeResourcePolicies`, but answered `insert` and `delete` with a global
operation a waiting client cannot find; patch 0043 makes them regional.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
