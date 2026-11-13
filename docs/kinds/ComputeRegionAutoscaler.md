---
kinds: [ComputeRegionAutoscaler]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeRegionAutoscaler

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** an autoscaler for a regional managed instance group, as a stable kind; Config Connector has `ComputeRegionAutoscaler` only as alpha types and a mapper, with no controller (decision 26). Terraform's `google_compute_region_autoscaler` (rank 56 of the catalog's `blocked-kinds.md`; with `ComputeAutoscaler`, `ComputeRegionPerInstanceConfig` and `ComputePerInstanceConfig` it unblocks `compute-mig`)

**Source.** hand-written, not generated: types and mapper copied from Config Connector's alpha `ComputeRegionAutoscaler`; identity and controller adapted from its zonal `ComputeAutoscaler`, which upstream has a controller for (`UPSTREAM.md`). Compute Engine's insert takes the name in the body, not as an ID parameter, which `hack/discokind` does not express.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector builds and promotes it: `projectRef`,
`region`, `resourceID`, `description`, `target` (the regional group's URL,
`projects/{project}/regions/{region}/instanceGroupManagers/{name}`, or the
full `https://` form; upstream's regional type has no reference, unlike
the zonal one's `targetRef`) and `autoscalingPolicy`. `projectRef`,
`region`, `resourceID` and `target` are immutable (CEL).

Kept from upstream's mapper: `autoscalingPolicy.scaleDownControl` is in
the spec but not sent (Compute Engine v1 has no such field; use
`scaleInControl`).

As with `ComputeAutoscaler`, the comparison is of every spec field the API
returns, so a policy field the spec leaves out and the API defaults (mode
`ON`, a 60 s cool-down, CPU utilization 0.6, predictive method `NONE`)
shows as a difference and is patched on every reconcile; the test sets
them all.

mockgcp served no regional autoscalers; patch 0041 adds them, as 0039 adds
zonal ones (the target regional group must exist, target and region come
back as full URLs, the policy's defaults are filled in).

The test makes two regional groups of e2-micro VMs (size 0; each
autoscaler holds one at one VM) through the API: create, a resync that
writes nothing, a changed CPU target (one patch), the CRD refusing a
change of target, delete, abandon. The sweeper deletes regional
autoscalers with the zonal ones, by name through the API's aggregated list
(gcloud has no autoscaler commands).

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `6deeafd` | PASS | PASS | none: create on a regional group, a resync with no write (the policy's defaults all set in the spec), one patch for a changed CPU target, the CRD refusing a change of target, delete, abandon (run `cce-11130243-25ef`); the sweep and `task live-list` found 0 left | 2 regional groups with 1 e2-micro VM each (10 GB pd-standard), 1 template, 1 network and subnet; all deleted by the test |
