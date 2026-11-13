---
kinds: [ComputeAutoscaler]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeAutoscaler

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** an autoscaler for a zonal managed instance group, as a stable kind; Config Connector's `ComputeAutoscaler` is alpha only (decision 26). Terraform's `google_compute_autoscaler` (rank 59 of the catalog's `blocked-kinds.md`)

**Source.** hand-written, not generated: copied from Config Connector's direct `ComputeAutoscaler` (types, identity, mapper, controller; `UPSTREAM.md`). Compute Engine's insert takes the name in the body, not as an ID parameter, and its patch takes the name as a query parameter, which `hack/discokind` does not express.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `projectRef`, `zone`,
`resourceID`, `description`, `targetRef` (Config Connector's
`ComputeInstanceGroupManager` by name, or `external`) and
`autoscalingPolicy`. `projectRef`, `zone`, `resourceID` and `targetRef`
are immutable (CEL). Only the zonal autoscaler's types are copied; the
upstream file also holds the regional one's, which has no controller
upstream.

As upstream does, the comparison is of every spec field the API returns,
so a policy field the spec leaves out and the API defaults (mode `ON`, a
60 s cool-down, CPU utilization 0.6, predictive method `NONE`) shows as a
difference and is patched on every reconcile; the test sets them all.

mockgcp served no autoscalers; patch 0039 adds the zonal ones (the
target group must exist, target and zone come back as full URLs, the
policy's defaults are filled in as above, assumed from the API reference,
not yet seen live).

The test makes two zonal groups of e2-micro VMs (size 0; each autoscaler
holds one at one VM) through the API: create, a resync that writes
nothing, a changed CPU target (one patch), the CRD refusing a change of
target, delete, abandon. The sweeper deletes autoscalers by name through
the API (gcloud has no autoscaler commands).

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `6347445` | PASS | PASS | none: create on a zonal group, a resync with no write (the policy's defaults all set in the spec), one patch for a changed CPU target, the CRD refusing a change of target, delete, abandon (run `cce-11122300-27c2`) | 2 zonal groups with 1 e2-micro VM each (10 GB pd-standard), 1 template, 1 network and subnet; all deleted by the test |
