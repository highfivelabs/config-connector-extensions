# Coverage

How much of Google Cloud's API surface can be managed as Kubernetes
resources: by Config Connector (**official**), and by the kinds in this
repository (**ours**) beyond that. The numbers behind the Coverage section of
the root `README.md` live here.

| File | One row per | What |
| --- | --- | --- |
| [`services.csv`](services.csv) | service | the rollup: resources and attributes, official, ours beyond official, uncovered, with percentages |
| [`resources.csv`](resources.csv) | resource | attributes of the resource, the kinds that cover them (official and ours), and notes on the mapping |
| [`attributes.csv`](attributes.csv) | attribute | the dotted path of a writable field, its type, whether the API requires it, who covers it (`official`, `ours`, `both`, `none`) and the CRD path that does |
| [`bars/`](bars) | service | `<service>.svg` and `_total.svg`: blue is official, green is ours beyond official, grey is uncovered |
| [`sources.yaml`](sources.yaml) | | the pinned inputs: commits, dates, digests |

The schema is the same in `config-connector-extensions`, `ack-extensions`
and `aso-extensions`; only the sources differ.

**These are estimates.** Coverage is decided by matching CRD field names to
API field names, not by running anything. "Known error" below says how far to
trust them: about 90% of the CRD fields we checked by hand were classified
correctly, and the misses are almost all fields the CRD renames or
restructures, which count as uncovered.

## Definitions

- **service**: a Google Cloud service API, named by its host
  (`run.googleapis.com`).
- **resource**: one resource type of that API that has a create operation,
  `<service>/<Name>` (`run.googleapis.com/Service`).
- **attribute**: a writable (input) field of the resource, as a dotted path,
  arrays as `[]`, maps as `{}`: `template.containers[].image`,
  `labels{}`. Only leaves count (a message with fields counts through its
  fields; a free-form object, or a message cut off at a recursive reference,
  counts as one). Output-only and read-only fields are not attributes (see
  "The denominator").
- **official**: covered by a Config Connector kind, from the CRDs at the pin
  in `UPSTREAM.md`.
- **ours**: covered by a kind in this repository's `config/crds/`. **Ours
  beyond official** is an attribute (or a whole resource) that ours covers
  and official does not.
- **uncovered**: in the API, in neither.
- **outside the denominator**: a kind of ours that fills a gap with no
  resource in the denominator to count it against (see below). Reported
  separately, never in the percentages.

Percentages are shares of `attributes_total`, to one decimal.
`attributes_official` counts every attribute official covers, including the
ones ours also covers; `attributes_ours_beyond` counts only the ones official
does not; `attributes_uncovered` is the rest, so the three add up to
`attributes_total`. A resource is counted official when an official kind maps
to it, and "ours beyond" in `services.csv` when only our kinds do.

## Pinned sources

Fetched 2026-11-09. `sources.yaml` holds the same, with a digest of the
files read from each; the generator refuses to run when an input differs.

| Input | Pin | Used for |
| --- | --- | --- |
| [Google API Discovery documents](https://github.com/googleapis/discovery-artifact-manager) (`discoveries/*.json`) | `9f8a165ecc629ec648b3bf96353eceb9d7e1ed88`, committed 2026-11-09T11:43:52Z | the denominator for REST APIs |
| [googleapis protos](https://github.com/googleapis/googleapis) (`google/**/*.proto`) | `855217a6b3054992d6d0cb58370312bda024af04`, committed 2026-11-09T18:22:51Z | the denominator for gRPC-only APIs Discovery does not describe |
| [Config Connector](https://github.com/GoogleCloudPlatform/k8s-config-connector) `config/crds/resources/`, `config/servicemappings/`, `apis/*/*/*_types.go` | `672414b8191fc2d2c12fff53f6faccf5ae291eef` (a `master` commit, committed 2026-11-07T03:04:32Z; nearest release `v1.160.0`), the pin of `UPSTREAM.md` | official |
| this repository's `config/crds/` | the working tree | ours |

"Official at the pin" is `master` at that commit, not a release: it includes
CRDs of kinds not yet in a release. A CRD existing does not mean a controller
reconciles it well; the count is of what the schema exposes.

## The denominator

**Discovery services.** Each `discoveries/<name>.<version>.json` is an API.
Its service is the host of its `rootUrl` (`<name>.googleapis.com` when the
root is `www.googleapis.com`). A service with a stable version (`vN`) reads
all its stable versions, newest first; a service with none reads its newest
pre-release. A resource of an older version is dropped when a newer one has
the same path or the same short name (Cloud Run's v1 `namespaces.services` is
v2's `projects.locations.services`), and the legacy zonal surface of Kubernetes
Engine (`projects.zones.clusters`) is dropped for the same reason. Resources
that exist only in a beta or alpha version of a service that also has a stable
one are not in the denominator.

**Resources.** A REST resource with an `insert` or `create` method that takes a
request body. Its name is the last path segment, singular, with the parents
joined on when the name would repeat (`LakeTask`, `LocationSecret`); the
`api=` note in `resources.csv` gives the exact Discovery path. Resources
created by other verbs (`addVersion`, `import`, `upload`, a `set...`) are not
resources here. A resource whose body has no writable field (a key ring) is a
resource with zero attributes.

**Attributes.** The writable fields of the request body, walking `$ref`s.
A request wrapper (`CreateInstanceRequest{instanceId, instance}`) is looked
through: paths are relative to the one message-typed property that is the
resource. Excluded as not writable:

- a field Discovery marks `readOnly`, or whose description starts with
  "Output only" or "[Output Only]" (also after a short first sentence, as in
  "Immutable. Output only.");
- the fields every API assigns itself, which some documents do not mark
  (`kind`, `etag`, `selfLink`, `creationTimestamp`, `timeCreated`, `updated`,
  `generation`, `metageneration`, `fingerprint`, `labelFingerprint`,
  `projectNumber`) and `selfLinkWithId`.

A recursive message is cut where it recurs (counted once, `recursive-cut=N` in
`notes`). `required` is true when the description starts with "Required".
Query and path parameters (`parent`, `requestId`, `fooId`) are not
attributes.

**Services Discovery does not describe** (gRPC-only: Vision AI, Chronicle,
Live Stream, ...) are read from googleapis: every proto service with a
`google.api.default_host` not already in Discovery, at its newest stable
package. A resource is the message an AIP-133 `Create<Name>` RPC creates (the
request field whose message is `<Name>`, or its one message field); its
attributes are that message's fields without `OUTPUT_ONLY` in
`google.api.field_behavior`, `REQUIRED` marking `required`, names as JSON
(`lowerCamelCase`). Well-known types (`Timestamp`, `Duration`, `Struct`,
`FieldMask`, ...) are one leaf. Only the services Discovery lacks come from
protos; a REST API that Discovery has is never read from protos.

**Not Google Cloud.** Discovery also describes Workspace, advertising, Play,
Android, YouTube, Maps and other consumer APIs. They are not cloud
infrastructure and are left out by name (`exclude_services` in
`hack/coverage/overrides.yaml`, and the directories skipped in
`hack/coverage/protos.go`). This cut is made by hand; the list is under
"Statistics" below.

## Official and ours

Each CRD is read at its storage version (else the last served), `spec` only.
Its fields are flattened the same way as the API's:

- scalars, arrays (`[]`) and maps (`{}`) are leaves; messages are walked;
- a reference (`{name, external, namespace}`) is one leaf, and a `{value,
  valueFrom}` secret field is one leaf;
- an object with `x-kubernetes-preserve-unknown-fields` or no properties is
  free-form and covers everything under the matching API field.

**CRD to resource.** For a CRD of group `<g>.cnrm.cloud.google.com` (official)
or `<g>.gcp.hfl.dev` (ours):

1. the services of the group: Config Connector's own service mapping
   (`serviceHostName`) where it names one, a Discovery or proto service named
   `<g>`, or `cloud<g>` or `<g>admin` (`kms`, `cloudkms`; `sql`, `sqladmin`);
   `group_hosts` in `overrides.yaml` for the rest (`tags`, `filestore`,
   `dialogflowcx`, `visionai`, ...);
2. the kind without its group prefix (`RunService` is `service`,
   `PubSubTopic` is `topic`), singular, lower-case, against the resource's
   name by its last segment, then by last segments joined with their parents
   (`workloadidentitypoolnamespace`). A kind's proto names (from `+kcc:proto=`
   in its `_types.go`) are tried only when its own name finds nothing;
3. where several resources of one service have the same short name
   (`projects.secrets` and `projects.locations.secrets`), a kind with a
   `location` field covers both when each overlaps the CRD at least half as
   much as the other, otherwise the one it overlaps most (a kind without one
   covers the global one);
4. Compute's twins (`addresses` and `globalAddresses`): a kind with a
   `location`, `region` or `zone` field also covers `region<Name>`,
   `global<Name>` and `zone<Name>`, as Config Connector's do;
5. a last resort when nothing matched: a name that ends with the other (at
   least four characters: `logsink` and `sink`).

A kind may cover several resources and a resource may have several kinds. A
CRD that matches nothing is **unmapped** (listed under "Statistics").
`overrides.yaml` can name a resource for a kind (`kinds`) or say a kind is not
a resource (`no_resource`); it holds only what the rules cannot find, and each
entry is a decision to review.

**CRD field to API attribute.** For a CRD mapped to a resource, in order, each
step only for what the earlier ones left:

1. the same path after lower-casing, dropping `[]`/`{}` and singularising
   each segment (`lifecycleRule[]` and `lifecycle.rule[]` do not match;
   `networkInterface[]` and `networkInterfaces[]` do), with a reference's
   `Ref` dropped;
2. a reference to a thing the API names by a string (`kmsKeyRef` and
   `kmsKeyName`, `serviceAccountRef` and `serviceAccountEmail`; suffixes
   `name`, `id`, `email`, `uri`, `link`, `url`);
3. a flag the CRD flattens (`x` for `x.enabled`);
4. a free-form object covers every API field under it;
5. the CRD is more detailed than the API leaf (`password.value`);
6. the same tail under another head: one path a suffix of the other, at least
   two segments (`properties.machineType`, `machineType`);
7. the same leaf under a similar parent chain: leaf and each parent equal or
   one a prefix or suffix of the other, at most one level apart
   (`disks[].autoDelete` and `bootDisk.autoDelete`, `acceleratorCount` and
   `count`), list wrappers (`items`, `value`) ignored;
8. a leaf name of eight characters or more that only one unmatched API field
   and one unused CRD path have (`billing.requesterPays` and `requesterPays`).

Two more rules count as covered without a spec field: an API attribute `name`
by `metadata.name` (every kind is named by its object), and a top-level
`labels{}` by `metadata.labels` for a Terraform-based kind whose service
mapping has a `metadataMapping.labels` (the `official_path` says
`metadata.labels`). Direct kinds count labels only when their spec has
`labels`.

**`ours_from_upstream_pr`** is true when any file of one of the resource's
kinds is listed in `UPSTREAM.md` as copied from a pull request that is not in
the pin ("PR #..." in the From column), the kind's types or its controller or
mapper. It is per resource: for a kind that mixes PR code and our own (the
Artifact Registry repository), the whole row is flagged.

**Outside the denominator.** A kind of ours that no resource accounts for gets a
row with `attributes_total` 0 and `notes` starting `outside-denominator`, and
is not in `services.csv`. These are kinds for what the API does with other
verbs than create (a route policy is patched into a router; an item is merged
into a project's metadata; a secret version is added) or that are not an API
resource (`IAMRoleBinding`). They are listed under "Statistics", and their
coverage is real but not measured here.

## Known error and caveats

- **Matching is by name.** The hand check below puts the share of CRD fields
  classified correctly at 91.5%. The misses are not spread evenly: a CRD that
  keeps the API's shape (Cloud Scheduler, BigQuery, Cloud Run) is matched
  almost perfectly; a Terraform-derived CRD that renames or flattens (Compute
  Engine instances, Kubernetes Engine clusters) loses 14 to 15% of its fields to
  "uncovered" (`bootDisk`/`attachedDisk` for `disks[]`, `enableShieldedNodes` for
  `shieldedNodes.enabled`, `numNodes` for `nodeCount`). So official coverage is
  slightly **under**counted, most for the big older services (`compute`,
  `container`, `sqladmin`, `bigtableadmin`). A false match is rarer (about 2%
  of the fuzzy matches checked).
- **A field in the CRD is counted covered whether or not the controller
  honours it.** Nothing here runs a reconcile.
- **The denominator is a count of leaves.** A service with a few huge messages
  (`dlp`, `aiplatform`, `dialogflow`) is weighted by them, and the same message
  reached from two places counts twice. Percentages by resource or by kind
  would read differently.
- **Official kinds that map to no resource** are listed under "Statistics".
  About half are not creates (associations, rules, policies, IAM members, which
  the API changes with `add...`, `set...` or `patch`); the rest are gRPC-only
  services with a Discovery or proto mismatch, or kinds whose name the rules do
  not connect to a resource. Their fields are in the official column of no row,
  so official is undercounted by them.
- **Beta-only resources** of services that also have a stable version, and
  resources created by verbs other than insert or create, are not counted.
- **Services Discovery and the protos both lack** (some new or private APIs)
  are not counted, whatever Config Connector has for them.
- **The output-only rule depends on the document.** A field the API assigns but
  the document does not mark is counted as uncovered (the `serverNames` list in
  `discovery.go` catches the common ones); a field wrongly marked output-only is
  missed. Both move a service by a point or two.
- **The cut of non-cloud services is a judgement.** Moving a service between
  `exclude_services` and the denominator changes the totals; it does not change
  any other service.
- **`ours_from_upstream_pr` is per resource**, not per attribute.
- The percentages are as of the pins above; they move when any pin does.

## Spot checks

Fifteen resources checked by hand against the real CRDs and the API,
2026-11-09. For each, the CRD's spec fields except identity and placement
(`resourceID`, `projectRef`, `location`, `region`, `zone`) were counted, and
every field the generator left unmatched was judged against the API's field
list: does the API have a field for it (a miss) or not (right to leave it).
Every fuzzy match (steps 2 to 8) was judged too; the exact-name matches
(step 1) were not re-read.

| Official kind | Resource | CRD fields | Matched | Unmatched, no API field | Unmatched, API field exists (misses) |
| --- | --- | ---: | ---: | ---: | ---: |
| `PubSubTopic` | `pubsub/Topic` | 6 | 6 | 0 | 0 |
| `StorageBucket` | `storage/Bucket` | 41 | 39 | 0 | 2 (`encryption.kmsKeyRef`, `lifecycleRule[].condition.withState`) |
| `ComputeFirewall` | `compute/Firewall` | 17 | 16 | 0 | 1 (`enableLogging` for `logConfig.enable`) |
| `SQLUser` | `sqladmin/User` | 10 | 8 | 2 | 0 |
| `CloudSchedulerJob` | `cloudscheduler/Job` | 27 | 27 | 0 | 0 |
| `DNSManagedZone` | `dns/ManagedZone` | 18 | 15 | 2 | 1 (`reverseLookup` for `reverseLookupConfig`) |
| `BigQueryDataset` | `bigquery/Dataset` | 24 | 24 | 0 | 0 |
| `RedisInstance` | `redis/Instance` | 33 | 25 | 8 | 0 |
| `SecretManagerSecret` | `secretmanager/Secret` | 12 | 10 | 1 | 1 (`topics[].topicRef`) |
| `SpannerInstance` | `spanner/Instance` | 13 | 12 | 0 | 1 (`numNodes` for `nodeCount`) |
| `KMSCryptoKey` | `cloudkms/KeyRingCryptoKey` | 8 | 6 | 2 | 0 |
| `PubSubSubscription` | `pubsub/Subscription` | 28 | 27 | 1 | 0 |
| `RunJob` | `run/Job` | 67 | 67 | 0 | 0 |
| `ComputeInstance` | `compute/Instance` | 84 | 61 | 11 | about 12 |
| `ContainerCluster` | `container/Cluster` | 233 | 163 | about 35 | about 35 |
| **Total** | | 621 | 506 | about 62 | about 53 |

Agreement: (506 matched + 62 correctly unmatched) / 621 = 91.5%. Counting only
the thirteen resources whose CRDs keep the API's shape, the misses are 6 of 304
fields (2.0%). The two large Terraform-derived CRDs hold 47 of the 53 misses
(14 to 15% of their fields); their unmatched lists were judged by reading, and
the counts are approximate. Of about 55 fuzzy matches judged, one was wrong
(Kubernetes Engine's `nodePools[].autoscaling.enabled` matched
`clusterAutoscaling.enabled`).

Also checked: our own kinds. `RunService` covers 20 attributes beyond the
Terraform-based official kind (`invokerIamDisabled`, the GPU fields, the scaling
block); `SQLUser` adds `databaseRoles`; `SecretManagerSecret` covers the
regional `LocationSecret` resource as a whole (10 attributes) that official
does not, and no attribute of the global one beyond official (its `location` is
a parameter, not a body field); the
`ArtifactRegistryRepository` additions (`customRepository.uri`, upstream
credentials) show as 7 beyond; the IAM workload identity kinds and the Artifact
Registry tag, which have no official kind, show as whole resources.

## Regenerating

```sh
hack/coverage/run.sh                    # fetch the inputs if missing, regenerate everything
hack/coverage/run.sh --check            # fail if the committed outputs differ from a regeneration
hack/coverage/run.sh --update-sources   # after changing a pin in sources.yaml: refetch, record the digests
```

`task coverage` and `task coverage-check` run the first two. Inputs are cached
in `.cache/coverage/` (gitignored); Config Connector is read from
`third_party/k8s-config-connector` when that checkout is at the pin, else
fetched sparsely (about 50 MB). The run takes a few seconds. Output is
deterministic: no timestamps, sorted everywhere, and the fetch dates are the
ones in `sources.yaml`.

Regenerate and commit when `config/crds/` changes (a new or changed kind),
when `UPSTREAM.md` moves the Config Connector pin, or when a pin in
`sources.yaml` moves. `--check` is the test that you did.

To move a pin: change `commit`, `commit_date` and `fetched` in `sources.yaml`
(Config Connector's commit must equal `UPSTREAM.md`'s), run
`hack/coverage/run.sh --update-sources`, then `hack/coverage/run.sh`, and look
at the diff of `services.csv` before committing. A mapping the rules get wrong
is fixed in `hack/coverage/overrides.yaml` (a group's services, a kind's
resources, a kind that is not a resource); a rule that is wrong is fixed in
the Go.

## Statistics

<!-- stats:start -->
Generated by `hack/coverage`; do not edit between the markers.

| | |
| --- | --- |
| Services in the denominator (with at least one resource) | 207 |
| Resources (REST resources with an insert or create method) | 1244 |
| Attributes (writable fields) | 43589 |
| Resources with an official kind | 692 |
| Resources covered only by our kinds | 7 |
| Attributes: official / ours beyond official / uncovered | 14063 / 59 / 29467 |
| Official kinds (Config Connector CRDs in `*.cnrm.cloud.google.com`) | 801 |
| Official kinds mapped to at least one resource | 691 |
| Official kinds **unmapped** | 110 |
| Our kinds | 17 |
| Our kinds mapped to at least one resource | 11 |
| Our kinds outside the denominator | 6 |
| Discovery APIs excluded as not Google Cloud | 116 |
| Cloud services with no creatable resource (not rows) | 56 |

### Our kinds outside the denominator

- `ComputeProjectMetadataItem` (compute.gcp.hfl.dev): one key of a project's common instance metadata, written with projects.setCommonInstanceMetadata (no create method)
- `ComputeRegionNetworkEndpoint` (compute.gcp.hfl.dev): an endpoint of a regional network endpoint group, written with regionNetworkEndpointGroups.attachNetworkEndpoints
- `ComputeRouterRoutePolicy` (compute.gcp.hfl.dev): a route policy of a Cloud Router, written with routers.patchRoutePolicy
- `IAMRoleBinding` (iam.gcp.hfl.dev): one role binding in another resource's IAM policy, written with setIamPolicy
- `LoggingSettings` (logging.gcp.hfl.dev): the Log Router settings of an organization or folder, written with updateSettings (they always exist)
- `SecretManagerSecretVersion` (secretmanager.gcp.hfl.dev): a secret version, written with secrets.addVersion

### Official kinds that could not be mapped to a resource

These are in the official column of no row, so the official percentages undercount them. By reason:

- a secret version, written with secrets.addVersion (1): `SecretManagerSecretVersion`
- an IAM policy of another resource, written with setIamPolicy (1): `IAMPolicy`
- audit configs of another resource's IAM policy, written with setIamPolicy (1): `IAMAuditConfig`
- no Discovery service for the group (gRPC-only API, not in Discovery, or excluded) (15): `APIQuotaAdjusterSettings`, `APIQuotaPreference`, `AnalyticsAccount`, `ApigeeRegistryAPI`, `ApigeeRegistryArtifact`, `ApigeeRegistryInstance`, `ApigeeregistryAPIDeployment`, `ApigeeregistryAPISpec`, `ColabRuntime`, `ColabRuntimeTemplate`, `ConfigControllerInstance`, `MapManagementMapConfig`, `MapManagementStyleConfig`, `MapmanagementMapContextConfig`, `SourceRepoRepository`
- one member in another resource's IAM policy, written with setIamPolicy (1): `IAMPolicyMember`
- part of another resource's IAM policy, written with setIamPolicy (1): `IAMPartialPolicy`
- service found, no creatable resource with a matching name (90): `AIPlatformModel`, `AIPlatformModelMonitor`, `AIPlatformRAGCorpus`, `APIHubExternalAPI`, `AccessContextManagerAccessLevelCondition`, `AccessContextManagerServicePerimeterResource`, `ApigeeAddonsConfig`, `ApigeeSyncAuthorization`, `AppEngineFirewallRule`, `AppEngineServiceSplitTraffic`, `ArtifactRegistryVPCSCConfig`, `BatchTask`, `BeyondCorpAppConnection`, `BeyondCorpAppConnector`, `BeyondCorpAppGateway`, `BeyondCorpClientConnectorService`, `BigQueryDatasetAccess`, `BigtableGCPolicy`, `BigtableMemoryLayer`, `CloudBatchResourceAllowance`, `ComputeBackendBucketSignedURLKey`, `ComputeBackendServiceSignedURLKey`, `ComputeDiskResourcePolicyAttachment`, `ComputeFirewallPolicyAssociation`, `ComputeFirewallPolicyRule`, `ComputeGlobalNetworkEndpoint`, `ComputeInstanceGroupNamedPort`, `ComputeNetworkEndpoint`, `ComputeNetworkFirewallPolicyAssociation`, `ComputeNetworkFirewallPolicyRule`, `ComputeNetworkPeering`, `ComputeNetworkPeeringRoutesConfig`, `ComputeOrganizationSecurityPolicyAssociation`, `ComputeOrganizationSecurityPolicyRule`, `ComputePerInstanceConfig`, `ComputeProjectMetadata`, `ComputeRegionDiskResourcePolicyAttachment`, `ComputeRegionPerInstanceConfig`, `ComputeRouterInterface`, `ComputeRouterNAT`, `ComputeRouterPeer`, `ComputeSharedVPCServiceProject`, `DataCatalogEntry`, `DataCatalogEntryGroup`, `DataCatalogTag`, `DataCatalogTagTemplate`, `DataflowFlexTemplateJob`, `DataplexContent`, `DataplexDataAttribute`, `DataplexDataTaxonomy`, `DataplexEnvironment`, `DataprocJob`, `DialogflowFulfillment`, `DiscoveryEngineSampleQuerySet`, `DiscoveryEngineUserStore`, `DocumentAIProcessorDefaultVersion`, `DocumentAIProcessorVersion`, `EdgeContainerMachine`, `EventarcGoogleChannelConfig`, `FirebaseHostingChannel`, `FirebaseHostingSite`, `FirebaseProject`, `FirestoreDocument`, `FirestoreField`, `IAMAccessBoundaryPolicy`, `IAMAccessPolicy`, `IAMDenyPolicy`, `IAMPrincipalAccessBoundaryPolicy`, `IAPSettings`, `IdentityPlatformProjectDefaultConfig`, `KMSAutokeyConfig`, `KMSSecretCiphertext`, `ManagedKafkaConsumerGroup`, `ModelArmorFloorSetting`, `NetworkSecurityPartnerSSEGateway`, `NetworkSecurityPartnerSSERealm`, `NetworkServicesEdgeCacheKeyset`, `NetworkServicesEdgeCacheOrigin`, `NetworkServicesEdgeCacheService`, `OSConfigGuestPolicy`, `RedisClusterEndpoint`, `ResourceManagerPolicy`, `Service`, `ServiceIdentity`, `ServiceUsageConsumerQuotaOverride`, `SpeechRecognizer`, `StorageAnywhereCache`, `StorageHMACKey`, `StorageRapidCache`, `VertexAIExampleStore`

### Discovery APIs excluded as not Google Cloud

`abusiveexperiencereport`, `acceleratedmobilepageurl`, `addressvalidation`, `adexchangebuyer2`, `adexperiencereport`, `admin`, `admob`, `adsense`, `adsenseplatform`, `agenciesandbrands`, `airquality`, `analyticsadmin`, `analyticsdata`, `androiddeveloperidstatus`, `androiddeviceprovisioning`, `androidenterprise`, `androidmanagement`, `androidpublisher`, `appsmarket`, `areainsights`, `authorizedbuyersmarketplace`, `blogger`, `books`, `businessprofileperformance`, `calendar`, `chat`, `checks`, `chromemanagement`, `chromepolicy`, `chromeuxreport`, `chromewebstore`, `civicinfo`, `classroom`, `cloudsearch`, `css`, `curationpartners`, `customsearch`, `datamanager`, `dataportability`, `deepreview`, `devicerun`, `dfareporting`, `digitalassetlinks`, `discovery`, `displayvideo`, `docs`, `doubleclickbidmanager`, `doubleclicksearch`, `drive`, `driveactivity`, `drivelabels`, `factchecktools`, `fitness`, `forms`, `games`, `gamesConfiguration`, `gamesManagement`, `gmail`, `gmailpostmastertools`, `groupsmigration`, `groupssettings`, `homegraph`, `indexing`, `keep`, `kgsearch`, `libraryagent`, `licensing`, `localservices`, `manufacturers`, `marketingplatformadmin`, `meet`, `merchantapi`, `mybusinessaccountmanagement`, `mybusinessbusinessinformation`, `mybusinesslodging`, `mybusinessnotifications`, `mybusinessplaceactions`, `mybusinessverifications`, `oauth2`, `pagespeedonline`, `paymentsresellersubscription`, `people`, `places`, `playcustomapp`, `playdeveloperreporting`, `playgrouping`, `playintegrity`, `pollen`, `prod_tt_sasportal`, `readerrevenuesubscriptionlinking`, `realtimebidding`, `reseller`, `safebrowsing`, `sasportal`, `script`, `searchads360`, `searchconsole`, `sheets`, `siteVerification`, `slides`, `smartdevicemanagement`, `solar`, `streetviewpublish`, `tagmanager`, `tasks`, `travelimpactmodel`, `vault`, `verifiedaccess`, `versionhistory`, `walletobjects`, `webcontentpublisher`, `webfonts`, `workspaceevents`, `youtube`, `youtubeAnalytics`, `youtubereporting`

### Cloud services with no creatable resource

`accessapproval.googleapis.com`, `advisorynotifications.googleapis.com`, `agentidentitycredentials.googleapis.com`, `apigeeconnect.googleapis.com`, `apptopology.googleapis.com`, `bigtable.googleapis.com`, `buildeventservice.googleapis.com`, `capacityplanner.googleapis.com`, `cel.googleapis.com`, `cloudapiregistry.googleapis.com`, `cloudcommerceconsumerprocurement.googleapis.com`, `cloudcommerceprocurement.googleapis.com`, `clouderrorreporting.googleapis.com`, `cloudlocationfinder.googleapis.com`, `cloudoptimization.googleapis.com`, `cloudprivatecatalog.googleapis.com`, `cloudproductregistry.googleapis.com`, `cloudshell.googleapis.com`, `cloudtrace.googleapis.com`, `colaboratory.googleapis.com`, `connectgateway.googleapis.com`, `databasecenter.googleapis.com`, `developerknowledge.googleapis.com`, `domains.googleapis.com`, `eventarcpublishing.googleapis.com`, `fcmdata.googleapis.com`, `firebasehosting.googleapis.com`, `firebaseml.googleapis.com`, `generativelanguage.googleapis.com`, `gkerecommender.googleapis.com`, `iam-meta-api.googleapis.com`, `iamconnectorcredentials.googleapis.com`, `iamcredentials.googleapis.com`, `kmsinventory.googleapis.com`, `language.googleapis.com`, `lifesciences.googleapis.com`, `maintenance.googleapis.com`, `mediatranslation.googleapis.com`, `ondemandscanning.googleapis.com`, `phishingprotection.googleapis.com`, `policyanalyzer.googleapis.com`, `policytroubleshooter.googleapis.com`, `recommender.googleapis.com`, `runtimeconfig.googleapis.com`, `safebrowsingohttpgateway.googleapis.com`, `servicecontrol.googleapis.com`, `servicehealth.googleapis.com`, `serviceusage.googleapis.com`, `spanner-cloud-executor.googleapis.com`, `sts.googleapis.com`, `texttospeech.googleapis.com`, `threatintelligence.googleapis.com`, `trafficdirector.googleapis.com`, `universalledger.googleapis.com`, `videointelligence.googleapis.com`, `workloadidentity.googleapis.com`
<!-- stats:end -->
