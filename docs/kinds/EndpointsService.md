---
kinds: [EndpointsService]
apiVersion: servicemanagement.gcp.hfl.dev/v1alpha1
---
# EndpointsService

`servicemanagement.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud Endpoints service: a Service Management managed service, its configuration (an OpenAPI document, or a gRPC service configuration with its descriptor set) and the rollout that serves it. Config Connector has no kind for it. Terraform's `google_endpoints_service`, which the catalog's `endpoints` module uses (rank 16 of its `blocked-kinds.md`). It is named as Terraform names it (decision 18) and starts at `v1alpha1` (decision 26's last sentence). The group is the API's, `servicemanagement` (the service is `servicemanagement.googleapis.com`), as the other groups here are named, not the product's ("Endpoints").

**Source.** hand-written, not generated: Config Connector has no Service Management code at the pin, and one object is three API calls (create the service, submit a configuration, roll it out), which `hack/discokind` cannot express. Written on Config Connector's direct reconciler after serviceusage's `ServiceIdentity` controller, with the API's Go client (`google.golang.org/api/servicemanagement/v1`).

## Notes

The spec: `resourceID` (the service's name, a DNS name such as
`my-api.endpoints.my-project.cloud.goog`; else `metadata.name`),
`projectRef` (the producer project), both immutable (CEL), and exactly one
of `openapiConfig` or `grpcConfig` with `protocOutputBase64`, as
Terraform's arguments. The files are sent as Terraform sends them
(`OPEN_API_YAML`, or `OPEN_API_JSON` for a document that starts with `{`;
`SERVICE_CONFIG_YAML` and `FILE_DESCRIPTOR_SET_PROTO`).

Create makes the service, or undeletes it when the name is held by a
service deleted less than 30 days ago (the API answers 409; Terraform
fails there instead), then submits the files and rolls the new
configuration out to 100% of traffic. A later reconcile submits and rolls
out again only when the files' SHA-256 differs from `status.configDigest`,
or the configuration the service serves is not the one in
`status.observedState.configID` (one rolled out by hand is put back).
`status.observedState` is the active configuration as Terraform's computed
attributes read it: `configID`, `dnsAddress` (its name), `apis` and
`endpoints`. Delete deletes the service; the API keeps it, and its name,
for 30 days. A get of a service that does not exist, or that the caller
cannot see, answers 404 or 403: both count as "not there", and a real
permission gap then fails at the create.

`IAMBinding` and `IAMMember` name the service through this kind
(`pkg/iampolicy`, the `EndpointsService` row) by `name` or by `external`
(`services/<name>`).

mockgcp has no Service Management, so its fake is
`pkg/controller/direct/servicemanagement/fake`, in front of mockgcp
(`docs/fake-cloud.md`). What the fake cannot show: the API's reading of
the configuration (it does not parse OpenAPI or gRPC configurations or
check the document's `host`; `apis` stay empty), DNS, permissions, and
how long a rollout takes.

Live is pending: the live-test service account has no Service Management
role (run `cce-11130544-c338` was refused
`servicemanagement.services.list`, 403), so the test skips live unless
`E2E_SERVICE_MANAGEMENT=true`, which needs `roles/servicemanagement.admin`
for the account and `servicemanagement.googleapis.com` on
(`docs/live-runs.md`).

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-13 | `5179492`+ | PASS | FAIL, now SKIP | `live-skip`: 403, `servicemanagement.services.list` denied for `cce-live-tests` (run `cce-11130544-c338`); the test now skips live without `E2E_SERVICE_MANAGEMENT=true` | none (the first call was refused); `task live-list` empty |
