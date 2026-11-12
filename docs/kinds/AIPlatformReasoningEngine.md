---
kinds: [AIPlatformReasoningEngine]
apiVersion: aiplatform.gcp.hfl.dev/v1beta1
---
# AIPlatformReasoningEngine

`aiplatform.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Vertex AI Agent Engine (reasoning engine), as a stable kind; Config Connector's `AIPlatformReasoningEngine` is alpha only (decision 26). Terraform's `google_vertex_ai_reasoning_engine`, which the catalog's `geap-agent-runtime` module uses (rank 6 of its `blocked-kinds.md`)

**Source.** hand-written, not generated: copied from Config Connector's direct `AIPlatformReasoningEngine` (types, identity, mapper, controller; `UPSTREAM.md`). The API assigns the engine's ID at create (there is no ID parameter), so `hack/discokind` cannot express it.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `projectRef`, `location`,
`displayName`, `description`, `spec` (service account, package, deployment,
agent framework), `encryptionSpec`, `labels`, and `resourceID`, which is
only for acquiring an existing engine by its ID. A new engine's ID is the
API's: the controller reads it from the create operation's answer and
keeps it in `status.externalRef`, where the next reconcile finds it.
`projectRef`, `location` and `resourceID` are immutable (CEL). Upstream's
`classMethods` is commented out upstream, so it is not here either.

The controller talks to the regional REST endpoint
(`https://{location}-aiplatform.googleapis.com/v1/`) through
`pkg/discokind`'s `API`, with upstream's protos as JSON: the Vertex AI Go
client has no REST transport for reasoning engines, and the manager reaches
Google Cloud (and the fake) through its HTTP client. mockgcp serves
reasoning engines at the pin.

The test makes an engine with no code (sessions and memory only), as the
catalog's module can, so it needs no pickled agent or container image:
create, a resync that finds the engine by its ID and writes nothing, an
update of the description (one `PATCH` with `updateMask=description`), the
CRD refusing a change of location, abandon, and a second object acquiring
the abandoned engine by `resourceID` and deleting it.

Live is pending: `$GCP_PROJECT` has `aiplatform.googleapis.com` off and the
live-test service account lacks `roles/aiplatform.user` (read 2026-11-12),
so the test skips live unless `E2E_VERTEX_AI=true`. What the fake cannot
show: whether the real API accepts an engine with no `spec`, what it
defaults in `spec` (each default must come back as a no-op on resync), and
how long create and delete take.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
