---
kinds: [SpannerInstanceConfig]
apiVersion: spanner.gcp.hfl.dev/v1beta1
---
# SpannerInstanceConfig

`spanner.gcp.hfl.dev/v1beta1`

**Gap it fills.** a user-managed Spanner instance configuration (a Google-managed base configuration with optional read-only replicas added), as a stable kind; Config Connector's `SpannerInstanceConfig` is alpha only (decision 26). Terraform's `google_spanner_instance_config`, which the catalog's `spanner-instance` module uses (rank 16 of its `blocked-kinds.md`)

**Source.** hand-written, not generated: copied from Config Connector's direct `SpannerInstanceConfig` (types, identity, mapper, controller; `UPSTREAM.md`). The configuration lives under the project with no location (`projects/P/instanceConfigs/custom-...`) and its create body must carry the full name, which `hack/discokind`'s shape does not express.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `projectRef`, `resourceID`
(else `metadata.name`; a user-managed configuration's ID starts with
`custom-`), `displayName`, `replicas`, `baseConfigRef` (`external` only:
upstream resolves no name), `labels` and `leaderOptions`. The API updates
only the display name and labels, so the rest is immutable (CEL); upstream's
controller ignored a change of them.

Changes from upstream's controller: an update waits for its operation,
reads the configuration again and writes `status.externalRef` (upstream's
did neither); labels compare as maps; `CloudLabels` for the slot guard
(the configuration has labels).

mockgcp did not keep a create's labels and leader options; patch 0036
keeps them, as the API does (without it every first resync wrote the
labels).

Live is pending: `$GCP_PROJECT` has `spanner.googleapis.com` off and the
live-test service account lacks `roles/spanner.admin` (read 2026-11-12),
so the test skips live unless `E2E_SPANNER=true`. A configuration costs
nothing until an instance uses it. What the fake cannot show: the API's
check that `replicas` holds every replica of the base configuration plus
at least one of its optional replicas (mockgcp checks none, so the test's
replica list is a guess at `regional-us-central1`'s), what it fills in
(`leaderOptions`), and how long create takes.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
