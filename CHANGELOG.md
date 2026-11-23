# Changelog

What changes for someone who installs config-connector-extensions. Nothing
has been released yet; each release moves its entries out of
"Unreleased".

## Unreleased

### Changed

- **`LoggingBillingAccountSink` is `v1beta1`**
  (`logging.gcp.hfl.dev/v1beta1`; `docs/design.md` decision 27: it passed
  live in run `cce-11181701-a6f8` as `live-tests@` and its CRD has not
  changed). The schema is unchanged; the CRD no longer serves `v1alpha1`,
  so change the `apiVersion`.

- **`ComputeNetworkFirewallPolicyPacketMirroringRule` is `v1beta1`**
  (`compute.gcp.hfl.dev/v1beta1`; `docs/design.md` decision 27: it passed
  live at `78644ec` in run `cce-11120932-1df3` and its CRD has not changed
  since). The schema is unchanged; the CRD no longer serves `v1alpha1`, so
  change the `apiVersion`.

- **`ContainerClusterFields` and `ContainerNodePoolFields` are
  `v1beta1`** (`container.gcp.hfl.dev/v1beta1`; `docs/design.md`
  decision 27: both passed live in run `cce-11160848-7b8d` and their CRDs
  have not changed since). Schemas are unchanged; the CRDs no longer
  serve `v1alpha1`, so change the `apiVersion`. `KMSCryptoKeyFields`
  stays `v1alpha1` (it cannot pass live in the live-test project).

- **A remote `ArtifactRegistryRepository` whose custom upstream URI
  changes is replaced** (`docs/design.md` decision 21): the controller
  deletes the repository and creates it again with the same name, as
  Terraform does, instead of going not Ready on the API's refusal
  ("Remote source configuartion cannot be updated"). The repository's
  cached packages are dropped and fetched again from the new upstream.
  Every other change is still made in place.

- **Six generated kinds are `v1beta1`** (`docs/design.md` decisions 27
  and 34: they passed live and their CRDs have not changed since):
  `ComputeRegionHealthAggregationPolicy` and `ComputeRegionBackendBucket`
  (`compute.gcp.hfl.dev`), `LoggingSavedQuery` and `LoggingLink`
  (`logging.gcp.hfl.dev`), `GKEHubRolloutSequence` (`gkehub.gcp.hfl.dev`)
  and `IAMOauthClient` (`iam.gcp.hfl.dev`). Schemas are unchanged; the
  CRDs no longer serve `v1alpha1`, so change the `apiVersion`. A cluster
  with `v1alpha1` objects of these kinds upgrades as for the sixteen
  below: abandon and delete them, replace the CRD, apply them again at
  `v1beta1`.

- **`LookerInstance` takes its OAuth client secret from a Kubernetes
  Secret.** `spec.oauthConfig.clientSecret` is now Config Connector's
  secret shape (as `SQLUser`'s `password`): `value`, or
  `valueFrom.secretKeyRef` (`name`, `key`) of a Secret in the object's
  namespace, which the controller reads. A manifest that set
  `clientSecret` as a string must move it to `clientSecret.value` (the
  kind is `v1alpha1`). The API never returns the client ID or secret, so
  they are no longer a difference on every resync; they are sent again
  when `status.inputVersion`, a digest of the client ID and the Secret's
  name, key and `resourceVersion`, changes, as when the Secret is
  rotated (`docs/design.md`, decision 33). Generated kinds can mark such
  fields with `secrets` and `inputOnly` in `hack/discokind/kinds.yaml`.

### Fixed

- **Deleting or abandoning an object no longer waits forever when an
  object its spec names is already gone** (everything deleted at once: a
  namespace, a Helm release, a catalog chart). `LoggingSettings` stuck
  with "referenced Folder ... not found" though its delete makes no call.
  Now an abandon resolves nothing; a delete of an object that was made
  takes its name from `status.externalRef` (or `status.selfLink`) and
  resolves nothing; and an object never made, whose references no longer
  resolve, goes without a call. This covers every kind but the few whose
  status records no name (`docs/design.md` section 7, item 43).

- **`ContainerNodePoolFields` with `resourceRef.name` now becomes Ready
  when the referenced `ContainerNodePool`'s `clusterRef.external` is a
  bare cluster name** (as the catalog's `gke-nodepool` chart writes it).
  Config Connector's node pool reference read only a full cluster id, so
  the object stayed "reference not ready" while the node pool was Ready.
  The controller now resolves the node pool through its `clusterRef`
  itself: a bare name in the node pool's project (its
  `cnrm.cloud.google.com/project-id` annotation) and `spec.location`, a
  full id as it is, a name through the `ContainerCluster` object.

- **`IAMBinding` and `IAMMember` on a `CloudFunctions2Function` named by
  `name`** were refused on a real install: the manager's ClusterRole could
  not read `cloudfunctions2.cnrm.cloud.google.com` (nor, for the new row,
  `cloudfunctions.cnrm.cloud.google.com`). Both are in the role now.

- **A folder's or organization's `ServiceIdentity` never went Ready**
  against Google Cloud: the API answers `generateServiceIdentity` with an
  operation already done, named `operations/finished.DONE_OPERATION`, and
  refuses a read of it; the controller read it. A done operation is now
  taken as it is.

- **Deleting a BigLake catalog, database and table at once** no longer
  holds the database's and table's finalizers. BigLake's catalog delete
  takes the database and table along, and they then answer 403, as
  BigLake does for anything missing. A generated kind with
  `missingIs403` now takes a 403 on the read or the delete, while
  deleting, for gone when a listing proves it: the resource's
  collection, or, when that answers 403 too, a parent's, lists without
  it. A real denial still keeps the finalizer (`docs/design.md`,
  decision 32).

### Added

- **`ComputeNetworkFirewallPolicyRule`**: `spec.securityProfileGroup`
  and `spec.tlsInspect` (Cloud NGFW Enterprise inspection, with action
  `apply_security_profile_group`) and `spec.match.srcNetworks`.

- **`ComputeSecurityPolicyFields`** (`compute.gcp.hfl.dev/v1alpha1`, a
  field kind, `docs/design.md` decision 37): a global Cloud Armor
  policy's Adaptive Protection `thresholdConfigs` and WAF
  `requestBodyInspectionSize`, which Config Connector's
  `ComputeSecurityPolicy` lacks, set on a policy it (or anything else)
  made, by `spec.resourceRef`. It never creates or deletes the policy;
  deleting it clears the fields. Not for a policy on Config Connector's
  opt-in direct controller.

- **`AccessContextManagerAuthorizedOrgsDesc`**
  (`accesscontextmanager.gcp.hfl.dev/v1alpha1`, generated by
  `hack/discokind`): an access policy's authorized organizations
  description (`assetType`, `authorizationDirection`, `authorizationType`,
  `orgs`), which Config Connector has only in its open pull request
  #14202. `spec.accessPolicy` is the policy's number. When Config
  Connector releases its kind, objects move to it (`docs/design.md`
  section 2.3).

- **`PrivateCACAPoolFields`** and **`PrivateCACertificateTemplateFields`**
  (`privateca.gcp.hfl.dev/v1alpha1`, field kinds, `docs/design.md`
  decision 37): a CA pool's `issuancePolicy.backdateDuration` and
  `issuancePolicy.baselineValues.nameConstraints`, and a certificate
  template's `maximumLifetime` and `predefinedValues.nameConstraints`,
  which Config Connector's `PrivateCACAPool` and
  `PrivateCACertificateTemplate` lack, set on a pool or template it (or
  anything else) made, by `spec.resourceRef`. They never create or delete
  the resource; deleting one clears the fields it set. When Config
  Connector changes its own issuance policy or predefined values, the API
  clears ours until the next reconcile puts them back.

- **`SecretManagerSecret` `versionDestroyTtl`, `customerManagedEncryption`
  and `tags`**: delayed destruction of a secret's versions, a regional
  secret's own Cloud KMS key, and resource manager tags bound when the
  secret is made (immutable). Config Connector's types have the first two
  as not yet served.

- **`ContainerNodePoolFields`: soft eviction, memory and topology
  managers, single-process OOM kill** (`spec.config.kubeletConfig`'s
  `evictionSoft`, `evictionSoftGracePeriod`, `evictionMinimumReclaim`,
  `evictionMaxPodGracePeriodSeconds`, `memoryManager`, `topologyManager`,
  `singleProcessOomKill`): kubelet settings Config Connector's
  `ContainerNodePool` lacks. Optional; the CRD stays `v1beta1`.

- **`ComputeVPNTunnel` `spec.params.resourceManagerTags`**: resource
  manager tags bound to the tunnel when it is made (Terraform's `params`
  block). Immutable; the API takes them only at insert.

- **`IAMOauthClientCredential` and `IAMWorkforcePoolProviderScimToken`**
  (`iam.gcp.hfl.dev/v1alpha1`): an OAuth client's credential and a
  workforce pool provider SCIM tenant's token, which no Config Connector
  kind has. The secret the API returns (`clientSecret`, `securityToken`)
  is written to a Kubernetes Secret in the object's namespace, owned by the
  object and named by `spec.secretOutput.name` (default the object's name),
  never to status (`docs/design.md` decision 38). The API returns a SCIM
  token only when it makes it: if its Secret is lost, delete the object and
  make it again. The manager's ClusterRole now creates and updates
  Secrets (`config/operator/rbac.yaml`).

- **`ComputeFirewallPolicyPacketMirroringRule`**
  (`compute.gcp.hfl.dev/v1alpha1`): a packet mirroring rule of a
  hierarchical (organization or folder) firewall policy, which no Config
  Connector kind has; the same fields as
  `ComputeNetworkFirewallPolicyPacketMirroringRule`, on Config Connector's
  `ComputeFirewallPolicy` (`firewallPolicyRef`). The API has these calls
  in its beta version only.

- **`DNSPolicyFields`** (`dns.gcp.hfl.dev/v1alpha1`, a field kind,
  `docs/design.md` decision 37): a Cloud DNS server policy's
  `dns64Config`, which Config Connector's `DNSPolicy` lacks, set on a
  policy it (or anything else) made, by `spec.resourceRef`. It never
  creates or deletes the policy; deleting it turns DNS64 off. Not safe
  beside Config Connector's opt-in direct `DNSPolicy` controller, whose
  update replaces the whole policy.

- **`ComputeNetworkFields` and `ComputeSubnetworkFields`**
  (`compute.gcp.hfl.dev/v1alpha1`, field kinds, `docs/design.md` decision
  37): a VPC network's BGP best path selection
  (`routingConfig.bgpBestPathSelectionMode`, `bgpAlwaysCompareMed`,
  `bgpInterRegionCost`) and a subnet's `allowSubnetCidrRoutesOverlap`,
  which Config Connector's `ComputeNetwork` and `ComputeSubnetwork` lack,
  set on a network or subnet it (or anything else) made, by
  `spec.resourceRef`. They never create or delete the resource; deleting
  them leaves the fields.

- **`LoggingLogBucketFields`** (`logging.gcp.hfl.dev/v1alpha1`, a field
  kind, `docs/design.md` decision 37): a log bucket's `indexConfigs`,
  `restrictedFields` and `cmekSettings`, which Config Connector's
  `LoggingLogBucket` lacks, set on a bucket it (or anything else) made, by
  `spec.resourceRef`, with an update mask naming only them. It never
  creates or deletes the bucket; deleting it clears the index configs and
  restricted fields, and leaves the CMEK key (the API cannot remove it).

- **`ComputeServiceAttachmentFields`** (`compute.gcp.hfl.dev/v1alpha1`,
  a field kind, `docs/design.md` decision 37): a Private Service Connect
  service attachment's `reconcileConnections` and
  `propagatedConnectionLimit`, which Config Connector's
  `ComputeServiceAttachment` lacks, set on a service attachment it (or
  anything else) made, by `spec.resourceRef`. It never creates or deletes
  the service attachment; deleting it leaves the fields.

- **`SecurityCenterBigQueryExport` and `SecurityCenterNotificationConfig`**
  (`securitycenter.gcp.hfl.dev/v1beta1`, generated by `hack/discokind`):
  a Security Command Center findings export to BigQuery and a findings
  notification to Pub/Sub, of an organization, folder or project
  (`organizationRef`, `folderRef` or `projectRef`). Config Connector's
  kinds of these names are alpha and take an organization only
  (`docs/design.md` decisions 16 and 26). Not yet run live.

- **`ComputeInterconnectAttachmentGroup` and `DNSOutboundEndpoint`**
  (`compute.gcp.hfl.dev/v1alpha1`, `dns.gcp.hfl.dev/v1alpha1`, generated by
  `hack/discokind`): a group of a Cross-Site or Dedicated Interconnect's
  VLAN attachments, and a Cloud DNS outbound endpoint (the subnetwork
  address a VPC network's forwarded DNS queries leave from). Config
  Connector has neither. Not yet run live.

- **Field kinds: `KMSCryptoKeyFields`, `ContainerClusterFields`,
  `ContainerNodePoolFields`** (`kms.gcp.hfl.dev/v1alpha1`,
  `container.gcp.hfl.dev/v1alpha1`; `docs/design.md` decision 37). Each
  names Config Connector's object (`spec.resourceRef`) and sets only the
  fields it lists, which Config Connector's kind lacks: a key's
  `keyAccessJustificationsPolicy`; a cluster's `secretManagerConfig`,
  `rbacBindingConfig`, `anonymousAuthenticationConfig`,
  `addonsConfig.statefulHaConfig` and auto-provisioned node pools'
  kubelet read-only port; a node pool's kubelet read-only port, container
  log size and files, unsafe sysctls and parallel image pulls. They never
  create or delete the resource. The manager's ClusterRole now reads
  `container.cnrm.cloud.google.com`.

- **`IAMMember.memberFrom.logSinkRef` and `IAMBinding.membersFrom[].logSinkRef`**:
  a `LoggingLogSink`'s writer identity (`status.writerIdentity`) as the
  member, as Config Connector's `IAMPolicyMember` takes it, for this
  project's `LoggingLogSink` (`logging.gcp.hfl.dev`) and Config
  Connector's; `apiVersion` picks one when both have a sink of that name.
  The object waits (`DependencyNotReady`) until the sink has a writer
  identity. `memberFrom.serviceAccountRef` is now optional: exactly one
  of `serviceAccountRef` and `logSinkRef` is required (Go clients: the
  field is a pointer).

- **`IAMBinding` and `IAMMember` take a `CloudFunctionsFunction`** (Config
  Connector's 1st gen function, `cloudfunctions.cnrm.cloud.google.com`) as
  `resourceRef`, by `name` (its project, `spec.region` and `resourceID`) or
  by `external` (`projects/{project}/locations/{region}/functions/{id}`),
  through the Cloud Functions v1 API's policy verbs. Asked for by the
  catalog's `cloud-function-v1` chart.
- **`logging.gcp.hfl.dev/v1beta1` `LoggingLogSink`**, our own log sink of
  a project, folder or organization, owning the whole sink, with what
  Config Connector's `LoggingLogSink` lacks: `interceptChildren` (a
  folder's or organization's sink that keeps the logs it routes from its
  children's sinks) and a Cloud project destination
  (`destination.projectRef`), which an intercepting sink needs. The spec
  is otherwise Config Connector's, so a manifest moves by changing its
  `apiVersion`; exactly one parent is required (Config Connector's falls
  back to the namespace's project). A workload moves a sink to ours by
  acquiring it (`resourceID`) and abandoning Config Connector's object
  (`docs/design.md` section 6.4).
- **`biglake.gcp.hfl.dev/v1beta1` `BigLakeTable`**, generated from
  Discovery: a table in a BigLake Metastore database
  (`google_biglake_table`). Config Connector's own `BigLakeTable`
  (`bigquerybiglake.cnrm.cloud.google.com`) takes BigLake's 403 for a
  table that does not exist yet for a denial and never makes it; ours
  takes that 403 before its create as not found (`missingIs403`). A
  workload moves a table to ours by acquiring it (`resourceID`) and
  abandoning Config Connector's object.
- **`discoveryengine.gcp.hfl.dev/v1alpha1` `DiscoveryEngineCmekConfig`**,
  generated from Discovery: the Cloud KMS key Vertex AI Search encrypts a
  location's data with (`google_discovery_engine_cmek_config`). Config
  Connector has no kind for it, so it starts at `v1alpha1`. The API has no
  create; `hack/discokind` now generates such a collection, patching to
  make it (`PatchCreates`), and learned `locationHost` (a location served
  at its own host) and `query` (`setDefault`, a query parameter). Unlike
  Terraform's `set_default`, `setDefault` has no default. Live pending a KMS
  key the Discovery Engine service agent may use.
- **`firestore.gcp.hfl.dev/v1beta1` `FirestoreBackupSchedule`**, a stable
  copy of Config Connector's alpha Firestore backup schedule (decision 26),
  with upstream's fields. The API assigns the schedule's ID at create;
  `status.externalRef` keeps it. Live pending a Firestore database in the
  live-test project (`E2E_FIRESTORE_DATABASE`). The operator's role gains
  `firestore.gcp.hfl.dev`.
- **`networksecurity.gcp.hfl.dev/v1beta1` `NetworkSecurityAuthzPolicy`**,
  generated from Discovery: an authorization policy
  (`google_network_security_authz_policy`) whose `target.resources` may
  name an agent gateway (`projects/P/locations/L/agentGateways/G`), a
  Secure Web Proxy gateway or forwarding rules, so an agent gateway can
  delegate to a `NetworkServicesAuthzExtension` (IAP, Model Armor) with
  `action: CUSTOM`. Config Connector's alpha kind takes only forwarding
  rules as targets; ours keeps the name at `v1beta1` (decision 26).
- **`networkservices.gcp.hfl.dev/v1beta1` `NetworkServicesAuthzExtension`**,
  generated from Discovery: an authorization extension that a load
  balancer or an agent gateway calls before it allows a request
  (`google_network_services_authz_extension`), including `service`, so it
  can delegate to IAP (`iap.googleapis.com`) or Model Armor. Config
  Connector's alpha kind requires a backend service and has no `service`
  field; ours keeps the name at `v1beta1` (decision 26).
- **`compute.gcp.hfl.dev/v1beta1` `ComputeInterconnectAttachment`**,
  generated from Discovery: a VLAN attachment
  (`google_compute_interconnect_attachment`) with the API's whole field
  set, including `candidateCloudRouterIpAddress` and
  `candidateCustomerRouterIpAddress` (and their IPv6 forms), which Config
  Connector's `v1beta1` kind lacks. Same name (decisions 19 and 26). Live
  pending a Dedicated Interconnect.
- **`IAMPolicyBinding` binds an access policy to a resource**:
  `spec.target.resource` (the organization, folder, project or a resource
  in it, such as an agent gateway), beside `principalSet` (exactly one);
  with `policyKind: ACCESS`, `policyRef.name` names an `IAMAccessPolicy`.
  A binding's delete no longer waits on a policy or parent object deleted
  with it.
- **`iam.gcp.hfl.dev/v1beta1` `IAMAccessPolicy`**, an IAM v3 access policy
  of an organization, folder or project
  (`google_iam_projects_access_policy` and its organization and folder
  forms): its display name, annotations and rules. Config Connector's
  alpha kind takes an organization only and has no controller; ours keeps
  the name at `v1beta1`. Live pending an access policy role for the
  live-test account.
- **`networkservices.gcp.hfl.dev/v1beta1` `NetworkServicesGateway`**,
  generated from Discovery: a gateway of either type, `OPEN_MESH` or
  `SECURE_WEB_GATEWAY` (a Secure Web Proxy), with the API's whole field
  set (`google_network_services_gateway`). Config Connector's `v1beta1`
  kind cannot make a Secure Web Proxy (no `network`, `subnetwork`,
  `gatewaySecurityPolicy`, `certificateUrls`, `labels` or `routingMode`,
  and `scope` required), so ours is a stable equivalent with the same
  name (decision 26). The Cloud Router a Secure Web Proxy makes is left
  for whoever deletes the network, as Terraform leaves it by default.
- **`iam.gcp.hfl.dev/v1alpha1` `IAMPolicyBinding`**: an IAM v3 policy
  binding, a principal access boundary policy bound to the principals of
  an organization, folder or project (`google_iam_projects_policy_binding`,
  `google_iam_folders_policy_binding`, `google_iam_organizations_policy_binding`).
  The policy is an `IAMPrincipalAccessBoundaryPolicy` by name or a
  policy's resource name. Config Connector has no released kind, so it
  starts at `v1alpha1`. Live pending an organization's principal access
  boundary policy the live-test account may bind.
- **`servicemanagement.gcp.hfl.dev/v1alpha1` `EndpointsService`**, a Cloud
  Endpoints service (`google_endpoints_service`): the managed service, its
  OpenAPI or gRPC configuration, and the rollout of that configuration to
  all traffic; a changed configuration is submitted and rolled out again.
  Config Connector has no kind for it, so it starts at `v1alpha1`.
  `IAMBinding` and `IAMMember` name it by `name`. The operator's role gains
  `servicemanagement.gcp.hfl.dev`.
- **`chronicle.gcp.hfl.dev/v1beta1` `ChronicleReferenceList`**, a Google
  SecOps (Chronicle) reference list in an instance
  (`google_chronicle_reference_list`): its description, entries, syntax
  type and scope. Config Connector 1.160.0's kind is alpha with no
  instance field, so it always fails with "cannot resolve instance"; ours
  keeps the name and carries `instance` (decision 26). The list's ID
  (`resourceID`, or `metadata.name`) takes letters, digits and
  underscores only. The API cannot delete a reference list, so deleting
  the object leaves it, as Terraform does; a new object with the same ID
  takes it back. Live pending a SecOps instance.
- **`chronicle.gcp.hfl.dev/v1beta1` `ChronicleRule`**, a Google SecOps
  (Chronicle) detection rule in an instance (`google_chronicle_rule`).
  Config Connector 1.160.0's `ChronicleRule` is alpha and has no instance
  field, so it always fails with "cannot resolve instance"; ours keeps the
  name and carries `instance` (decision 26). The API assigns the rule's ID:
  `status.externalRef` records it, and `spec.resourceID` acquires a rule
  that exists. `deletionPolicy: FORCE` deletes a rule's retrohunts and
  detections with it. `ChronicleRuleDeployment`'s `ruleRef` by `name` now
  finds ours first, and its delete no longer waits on a rule deleted with
  it. Live pending a SecOps instance.
- **`chronicle.gcp.hfl.dev/v1alpha1` `ChronicleRuleDeployment`**, the
  deployment of a Google SecOps (Chronicle) rule
  (`google_chronicle_rule_deployment`): whether the rule runs, alerts or is
  archived, its run frequency and schedule. The deployment comes with the
  rule, so the object sets it and deleting the object leaves it, as
  Terraform does. `ruleRef` takes Config Connector's `ChronicleRule` or an
  external rule name. Config Connector has no kind for it, so it starts at
  `v1alpha1`. Live pending a SecOps instance. The operator's role gains
  `chronicle.gcp.hfl.dev`.
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

- **`BigLakeDatabase` was never made**: BigLake answers 403, not 404, for
  a database that does not exist, and the read before the create took
  it for a denial ("Permission 'biglake.databases.get' denied"). It now
  reads that 403 as not found until the database is made, as
  `BigLakeCatalog` does.
- **`IAMDenyPolicy` waited forever to be deleted when the Project, Folder
  or Organization object its `projectRef`, `folderRef` or
  `organizationRef` names by `name` was already gone**: its delete
  resolved the parent again ("referenced Folder ... not found"). The
  delete now takes the policy's name from `status.externalRef`, as the
  generated kinds do; `deletion-policy: abandon` still keeps the policy.
- **`LoggingBillingAccountSink` took a Cloud Storage bucket only as
  `projects/<p>/buckets/<name>` or `projects/_/buckets/<name>`**: a bucket's
  bare name, `gs://<name>` and `storage.googleapis.com/<name>` (Terraform's
  form) were refused by the reference's validation, as were the
  `logging.googleapis.com/`, `bigquery.googleapis.com/` and
  `pubsub.googleapis.com/` prefixes. Each is now taken; a `StorageBucket`
  object by `name` works as before.
- **A generated kind (`hack/discokind`) waited forever to be deleted when
  the parent object its `projectRef`, `folderRef` or `organizationRef`
  names by `name` was already gone** (everything deleted at once): its
  delete resolved the parent again and failed with "referenced Project ...
  not found". The delete now takes the resource's name from
  `status.externalRef` (the parts of a name cannot change) and does not
  resolve the parent; `deletion-policy: abandon` still keeps the resource.
  An object being deleted that never had a resource (no
  `status.externalRef`) and whose parent cannot be resolved goes without
  a call. Every generated kind, `ComputeNetworkAttachment` included.
- **`IAMBinding` and `IAMMember` waited forever to be deleted when the
  object they name was being deleted too** (a namespace or a release
  deleted at once): the binding's delete stuck on "reference ... is not
  ready" while the parent, an `APIGatewayAPI` for example, was going. A
  parent that is gone, or being deleted without `deletion-policy: abandon`,
  takes the policy with it, so the binding's delete now succeeds without a
  call; one kept with abandon still has the binding removed.

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

- **`ServiceIdentity` is `v1beta1`** (`serviceusage.gcp.hfl.dev/v1beta1`,
  `docs/design.md` decision 27: it passed live on 2026-11-15 on a test
  folder). The schema is unchanged; the CRD no longer serves `v1alpha1`,
  so change the `apiVersion`; upgrade as below. A service identity is
  never deleted, so abandoning the old object loses nothing.

- **`RunService` is `v1beta1`** (`run.gcp.hfl.dev/v1beta1`, `docs/design.md`
  decision 27, which since 2026-11-15 covers field-gap kinds too: it passed
  live and its schema has not changed since). The schema is unchanged; the
  CRD no longer serves `v1alpha1`, so change the `apiVersion`. A cluster
  with `v1alpha1` `RunService` objects upgrades as below: abandon and
  delete them, replace the CRD, apply them again at `v1beta1`, which takes
  over the same Cloud Run service.

- **Sixteen more of our kinds are `v1beta1`** (`docs/design.md`
  decision 27: they passed live and their schemas have not changed since):
  `ArtifactRegistryTag`, `ComputeProjectMetadataItem`,
  `ComputeRegionNetworkEndpoint`, `ComputeRegionNetworkFirewallPolicy`,
  `ComputeRegionNetworkFirewallPolicyAssociation`,
  `ComputeRegionNetworkFirewallPolicyRule`, `ComputeRouterRoutePolicy`,
  `IAMMember`, `IAMWorkloadIdentityPoolNamespace`,
  `IAMWorkloadIdentityPoolManagedIdentity`,
  `IAMWorkloadIdentityPoolProviderKey`, `NetworkSecurityAddressGroup`,
  `NetworkSecuritySecurityProfile`, `NetworkSecuritySecurityProfileGroup`,
  `SQLAdminBackup` and `StorageBucketObject`, each in its own group.
  Schemas are unchanged; the CRDs no longer serve `v1alpha1`, so change
  the `apiVersion`. A cluster with `v1alpha1` objects of these kinds
  upgrades as below: abandon and delete them, replace the CRD, apply them
  again at `v1beta1`.

- **`ComputeRouter`, `LoggingSettings` and `SecurityCenterMuteConfig` are
  `v1beta1`** (`docs/design.md` decision 27): `compute.gcp.hfl.dev/v1beta1`,
  `logging.gcp.hfl.dev/v1beta1` and `securitycenter.gcp.hfl.dev/v1beta1`.
  Their schemas are unchanged; only the version moved. The CRDs no longer
  serve `v1alpha1`, so a manifest that names `v1alpha1` with one of these
  kinds is refused: change the `apiVersion`. `ComputeRouterRoutePolicy`
  stays `v1alpha1` and its `routerRef` finds our router at `v1beta1`.

  To upgrade a cluster that has `v1alpha1` objects of these kinds (the API
  server will not apply a CRD that drops a version it has stored), for
  each kind:

  1. Annotate each object `cnrm.cloud.google.com/deletion-policy: abandon`
     and delete it; the router, settings or mute config stays in Google
     Cloud.
  2. Delete the kind's CRD (`computerouters.compute.gcp.hfl.dev`,
     `loggingsettings.logging.gcp.hfl.dev` or
     `securitycentermuteconfigs.securitycenter.gcp.hfl.dev`) and install
     the new one.
  3. Apply the same objects at `v1beta1`. Each acquires the resource it
     names, as the old one did.

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
