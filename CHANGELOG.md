# Changelog

What changes for someone who installs config-connector-extensions. Nothing
has been released yet; each release moves its entries out of
"Unreleased".

## Unreleased

### Added

- **`networkservices.gcp.hfl.dev/v1beta1` `NetworkServicesGateway`**,
  generated from Discovery: a gateway of either type, `OPEN_MESH` or
  `SECURE_WEB_GATEWAY` (a Secure Web Proxy), with the API's whole field
  set (`google_network_services_gateway`). Config Connector's `v1beta1`
  kind cannot make a Secure Web Proxy (no `network`, `subnetwork`,
  `gatewaySecurityPolicy`, `certificateUrls`, `labels` or `routingMode`,
  and `scope` required), so ours is a stable equivalent with the same
  name (decision 26). The Cloud Router a Secure Web Proxy makes is left
  for whoever deletes the network, as Terraform leaves it by default.
- **`servicemanagement.gcp.hfl.dev/v1alpha1` `EndpointsService`**, a Cloud
  Endpoints service (`google_endpoints_service`): the managed service, its
  OpenAPI or gRPC configuration, and the rollout of that configuration to
  all traffic; a changed configuration is submitted and rolled out again.
  Config Connector has no kind for it, so it starts at `v1alpha1`.
  `IAMBinding` and `IAMMember` name it by `name`. The operator's role gains
  `servicemanagement.gcp.hfl.dev`.
- **`networksecurity.gcp.hfl.dev/v1beta1` `NetworkSecurityGatewaySecurityPolicy`,
  `NetworkSecurityGatewaySecurityPolicyRule`, `NetworkSecurityURLList` and
  `NetworkSecurityTLSInspectionPolicy`**, generated from Discovery: a Secure
  Web Proxy's policy, its rules, URL lists and TLS inspection policy
  (`google_network_security_gateway_security_policy`, `_rule`,
  `google_network_security_url_lists`,
  `google_network_security_tls_inspection_policy`). Config Connector's are
  alpha only, so ours are stable equivalents (decision 26). Other resources
  are named by their paths (a TLS inspection policy, a CA pool), not by
  references.
- **`apigee.gcp.hfl.dev/v1alpha1` `ApigeeDnsZone`**, generated from
  Discovery: a private DNS zone an Apigee organization peers with
  (`google_apigee_dns_zone`). Config Connector has no kind for it, so it
  starts at `v1alpha1`. The API cannot change a zone: every field is
  immutable. Live pending an Apigee organization.
- **`compute.gcp.hfl.dev/v1beta1` `ComputeRegionAutoscaler`**, a stable
  kind for Config Connector's alpha regional autoscaler, which has types
  but no controller upstream (decision 26), with upstream's fields.
- **`compute.gcp.hfl.dev/v1beta1` `ComputeRegionDiskResourcePolicyAttachment`**,
  a resource policy (a snapshot schedule) attached to a regional disk, with
  the fields of Config Connector's Terraform-based alpha kind (decision
  26); the controller is our zonal attachment's, for `regionDisks`.
- **`compute.gcp.hfl.dev/v1beta1` `ComputePerInstanceConfig`**, a stable
  per-instance config of a zonal managed instance group (decision 26), with
  the fields of Config Connector's Terraform-based alpha kind; adapted from
  our `ComputeRegionPerInstanceConfig`.
- **`apigee.gcp.hfl.dev/v1beta1` `ApigeeNATAddress`**, a stable copy of
  Config Connector's alpha Apigee NAT address (decision 26), with
  upstream's fields (`instanceId`, `resourceID`, both immutable) and
  `status.externalRef`. Live pending a provisioned Apigee instance
  (`E2E_APIGEE_INSTANCE`). The operator's role gains `apigee.gcp.hfl.dev`.
- **`compute.gcp.hfl.dev/v1beta1` `ComputeAutoscaler`**, a stable copy of
  Config Connector's alpha zonal autoscaler (decision 26), with upstream's
  fields.
- **`compute.gcp.hfl.dev/v1beta1` `ComputeDiskResourcePolicyAttachment`**, a
  stable copy of Config Connector's alpha attachment of a resource policy
  to a zonal disk (decision 26), with upstream's fields.
- **`compute.gcp.hfl.dev/v1beta1` `ComputeRegionPerInstanceConfig`**, a
  stable copy of Config Connector's alpha per-instance config of a
  regional managed instance group (decision 26), with upstream's fields.
- **`spanner.gcp.hfl.dev/v1beta1` `SpannerInstanceConfig`**, a stable copy
  of Config Connector's alpha user-managed Spanner instance configuration
  (decision 26), with upstream's fields; the fields the API cannot update
  are immutable. Live pending Spanner in the live-test project
  (`E2E_SPANNER=true`).
- **`datacatalog.gcp.hfl.dev/v1beta1` `DataCatalogTag`**, a stable copy of
  Config Connector's alpha Data Catalog tag (decision 26), with upstream's
  fields. The API assigns the tag's ID at create; `status.externalRef`
  keeps it, also across updates, and `resourceID` acquires an existing
  tag. Live pending Data Catalog in the live-test project
  (`E2E_DATA_CATALOG=true`).
- **`dns.gcp.hfl.dev/v1beta1` `DNSResponsePolicyRule`**, a stable copy of
  Config Connector's alpha, Terraform-based response policy rule (decision
  13), with upstream's fields and a controller written here. Live pending
  `roles/dns.admin` for the live-test account (`E2E_DNS_ADMIN=true`).
- **`aiplatform.gcp.hfl.dev/v1beta1` `AIPlatformReasoningEngine`**, a
  stable copy of Config Connector's alpha Vertex AI Agent Engine (decision
  26), with upstream's fields. The API assigns the engine's ID at create;
  `status.externalRef` keeps it, and `resourceID` acquires an existing
  engine. Live pending Vertex AI in the live-test project
  (`E2E_VERTEX_AI=true`).
- **`cloudbuild.gcp.hfl.dev/v1alpha1` `CloudBuildRepository`**, generated
  from Discovery: a repository of a Cloud Build connection
  (`google_cloudbuildv2_repository`), named as Config Connector's
  reference type names it. Config Connector has no kind for it, so it
  starts at `v1alpha1`.
- **`IAMBinding` and `IAMMember` on a Cloud Functions 2nd gen function**
  (`google_cloudfunctions2_function_iam_binding`, `_iam_member`): a
  resource row for `CloudFunctions2Function`, ours
  (`cloudfunctions2.gcp.hfl.dev/v1beta1`) found by name before Config
  Connector's alpha kind, through Cloud Functions' v2 IAM methods.
- **Six more stable kinds for Config Connector alpha-only kinds**
  (decision 26): `cloudfunctions2.gcp.hfl.dev/v1beta1`
  `CloudFunctions2Function`, and `dataplex.gcp.hfl.dev/v1beta1`
  `DataplexLake`, `DataplexZone`, `DataplexAsset`, `DataplexAspectType`
  and `DataplexDataScan`. `IAMMember` and `IAMBinding` find ours by name
  before Config Connector's alpha kinds for Dataplex, Cloud Deploy
  targets, API Gateway and Cloud Build connections.
- **Eight more stable kinds for Config Connector alpha-only kinds**
  (decision 26), generated from Discovery:
  `apigateway.gcp.hfl.dev/v1beta1` `APIGatewayAPIConfig` and
  `APIGatewayGateway`, `biglake.gcp.hfl.dev/v1beta1` `BigLakeCatalog` and
  `BigLakeDatabase`, `cloudbuild.gcp.hfl.dev/v1beta1`
  `CloudBuildConnection`, and `clouddeploy.gcp.hfl.dev/v1beta1`
  `CloudDeployTarget`, `CloudDeployDeployPolicy` and
  `CloudDeployAutomation`. The operator's role gains their four groups.
- **Eight stable kinds for Config Connector alpha-only kinds**
  (`docs/design.md` decision 26), generated from Discovery:
  `certificatemanager.gcp.hfl.dev/v1beta1`
  `CertificateManagerCertificateIssuanceConfig`,
  `backupdr.gcp.hfl.dev/v1beta1` `BackupDRManagementServer`,
  `networksecurity.gcp.hfl.dev/v1beta1` `NetworkSecurityDNSThreatDetector`,
  `gkebackup.gcp.hfl.dev/v1beta1` `GKEBackupBackupPlan`,
  `GKEBackupRestorePlan`, `GKEBackupBackupChannel` and
  `GKEBackupRestoreChannel`, and `compute.gcp.hfl.dev/v1beta1`
  `ComputeGlobalNetworkEndpointGroup`. The operator's role gains the
  `certificatemanager`, `backupdr` and `gkebackup` groups.
- **`compute.gcp.hfl.dev/v1beta1` `ComputeNetworkEndpoint`**, a stable
  copy of Config Connector's alpha zonal network endpoint (decision 26),
  with upstream's fields: `networkEndpointGroupRef`, `zone`,
  `ipAddress`, `instanceRef` and the port as `resourceID`.
- **Five kinds generated from Discovery** (`docs/design.md` section 2.4):
  `compute.gcp.hfl.dev/v1beta1` `ComputeNetworkAttachment`,
  `compute.gcp.hfl.dev/v1alpha1` `ComputeRegionInstanceTemplate`,
  `looker.gcp.hfl.dev/v1alpha1` `LookerInstance`, and
  `networkservices.gcp.hfl.dev/v1alpha1` `NetworkServicesAgentGateway` and
  `NetworkServicesAgentConnectivityTemplate`. The operator's role gains
  the two new groups. A generated Compute Engine kind now takes a
  resource's path and its full URL as the same value, so a spec that
  names a subnet by path no longer differs from what the API returns.
- **`IAMMember` and `IAMBinding` on a BigQuery dataset**
  (`resourceRef.kind: BigQueryDataset`). A dataset has no IAM policy
  calls; its access list is read and patched as one, as Terraform's
  `google_bigquery_dataset_iam_*` do. Authorized views, routines and
  datasets and the project special groups in the list are left as they
  are.
- **Five foundation kinds generated from Discovery** (`docs/design.md`
  section 2.4): `resourcemanager.gcp.hfl.dev/v1alpha1`
  `ResourceManagerCapabilityConfig` and `ResourceManagerBoundary`, and
  `networkconnectivity.gcp.hfl.dev/v1alpha1`
  `NetworkConnectivityPscAuthorizationPolicy`,
  `NetworkConnectivityAutomatedDnsRecord` and
  `NetworkConnectivitySpokeGatewayAdvertisedRoute`. The operator's role
  gains the `networkconnectivity.gcp.hfl.dev` group.
- **The slot guard, opt-in** (`docs/design.md` section 2.5). A manager
  given `--slot=<name>` (or `HFL_SLOT`; through the operator,
  `--manager-arg=--slot=<name>`) labels its objects `hfl.dev/slot`, labels
  the cloud resources that have labels `hfl-dev-slot`, and refuses to
  write or delete a resource another slot holds (`Ready` False, reason
  `SlotMismatch`, and a Warning event). An object annotated
  `hfl.dev/slot-takeover: <slot>` may claim that slot's resources, which
  is how a resource moves between two installs. Without a slot nothing
  changes. `AddToManager` takes a new `controller.Options` argument.

### Fixed

- **The fake cloud answered 404 to an IAM policy read or write on a
  generated kind** (Cloud Build v2 connections, API Gateway configs and
  gateways, Dataplex lakes, zones, aspect types and data scans): the
  generated kinds' fake (`pkg/discokind/fake`) took `name:getIamPolicy`
  for an object's name. Custom methods now reach mockgcp, which serves
  the policy verbs for every resource, so `IAMBinding` and `IAMMember`
  on these kinds become Ready in `bin/fakecloud` and `bin/fakegcp`.
- **Two `IAMMember` or `IAMBinding` objects on one BigQuery dataset could
  lose a member**: the access list is read, checked and patched in three
  calls, and two writers in one manager could both pass the check. Writes
  to one dataset now go one at a time within a manager; the API's
  `If-Match` still guards against other writers.
- **Generated kinds lacked every nested field called `name`**
  (`hack/discokind` dropped `name` at every level, not only the
  resource's own): `ComputeRegionInstanceTemplate`'s network interface
  and disk names, the BYOIP prefixes' and VM extension policies' named
  parts, and the Backup for GKE plans' selected applications gain them.
- **A generated kind whose create takes a `requestId` beside its ID**
  (`NetworkConnectivityPscAuthorizationPolicy`, `BackupDRManagementServer`)
  sent its ID as the `requestId`: `hack/discokind` picked any query
  parameter ending in `Id`, in map order. It now skips `requestId` and
  refuses two candidates.

### Changed

- **`IAMBinding` is `iam.gcp.hfl.dev/v1beta1`** (`docs/design.md`
  decision 24). Its schema is unchanged; only the version moved. The CRD
  no longer serves `v1alpha1`, so a manifest that says
  `apiVersion: iam.gcp.hfl.dev/v1alpha1` with `kind: IAMBinding` is
  refused: change the `apiVersion`. `IAMMember` stays `v1alpha1`.

  To upgrade a cluster that has `v1alpha1` `IAMBinding` objects (the API
  server will not apply a CRD that drops a version it has stored):

  1. Annotate each one `cnrm.cloud.google.com/deletion-policy: abandon`
     and delete it; the bindings stay in Google Cloud.
  2. Delete the `iambindings.iam.gcp.hfl.dev` CRD and install the new one.
  3. Apply the same objects at `v1beta1`. Each takes over the binding it
     names (the role, under its condition, on its resource) and sets its
     members, as the old one did.
