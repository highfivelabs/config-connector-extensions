# config-connector-extensions design

config-connector-extensions has the kinds in `docs/kinds/`, one file
each (`task kinds` lists them): gap kinds copied from Config Connector's
nearest code (section 2.3) and kinds generated from the APIs' Discovery
documents (section 2.4). Each
passes its end-to-end test against mockgcp; those in `docs/live-runs.md`
also pass it against a real project. An operator runs the managers beside
Config Connector's, each as its own Kubernetes service account carrying
Config Connector's Google identity, and a kind test proves it in cluster and namespaced mode
against an in-cluster fake cloud (section 9). Images are built locally
with ko; there is no registry or release yet. This document says how it is
built, why, how it stays in step with Config Connector, how it is tested,
how gaps are chosen, and what is left.

## 1. Background

- **One repository per provider.** Each provider's gap kinds are built with
  that provider's generator against that provider's model input, on
  unrelated cadences. So `config-connector-extensions` beside Config
  Connector, and `ack-extensions` beside ACK and `aso-extensions` beside
  Azure Service Operator,
  each versioned on its own and installed beside the upstream controller
  it extends.
- **Our kinds live in our groups.** Config Connector's registration
  controller skips any `managed-by-kcc` CRD missing from its static table,
  and its webhooks and ClusterRoles are generated from the in-tree set. A
  gap kind is therefore `<kind>.<service>.gcp.hfl.dev`, reconciled by our
  process, never by theirs.
- **For GCP, build on the direct family.** Config Connector is three
  reconciler families in one binary: Terraform (vendored fork of
  `terraform-provider-google-beta`, MPL-2.0), DCL, and "direct"
  (`directbase`: a `Model → Adapter{Find, Create, Update, Delete, Export}`
  interface over the Google Cloud Go clients). Upstream says everything is
  migrating to direct; at our pin the static table has 449 direct, 225
  Terraform, 65 DCL and 4 IAM kinds of 743. A direct kind is types, mapper,
  identity and reference (from `dev/tools/controllerbuilder` where the
  upstream siblings are generated), a hand-written controller (a few
  hundred lines) on `directbase`, and a mockgcp fixture.
- **GCP is where gaps close fastest upstream**, so
  `config-connector-extensions` expects to retire kinds as often as it
  adds them; a kind moves from our group to
  theirs when upstream ships it (section 3.4).
- **Hermetic tests exist upstream**: mockgcp (a fake GCP, separate Go
  module, in the Config Connector repository) plus golden `_http.log`
  files, maintained by Google.
- **The knobs** (deletion policy, adoption, identity per namespace,
  reconcile cadence) are Config Connector's annotations; because we run
  Config Connector's reconciler, our kinds honour the same ones
  (`cnrm.cloud.google.com/deletion-policy: abandon`, `spec.resourceID`
  acquisition, `.../project-id`, `.../reconcile-interval-in-seconds`), so
  a tool that writes them for Config Connector writes them for us too.

## 2. Architecture chosen

### 2.1 Config Connector as a pinned module, not copied

Options considered for the shared machinery (reconciler, references,
identity, config, mockgcp):

| Option | Verdict |
| --- | --- |
| A. Use Config Connector as a Go module from a checkout at a pinned `master` commit, through `replace` | **Chosen.** The framework stays byte-identical to upstream; syncing is moving the pin. |
| B. Copy `directbase` and what it needs into this repository | Rejected: `go list -deps ./pkg/controller/direct/directbase` pulls 91 Config Connector packages, and through `krmtotf`/`servicemappings` the Terraform fork and DCL. Copying that is a fork, not a copy. |
| C. Copy the Terraform-family `SQLUser` (CRD from the TF schema, `krmtotf`, the provider fork) | Rejected: it needs the whole Terraform family, and the code that would change (`resource_sql_user.go`) is MPL-2.0 in a fork with no patch series. Upstream is moving off it anyway. |

Option A has a cost: Config Connector's `go.mod` requires the Terraform
fork at `v0.0.0-00010101000000-000000000000` and resolves it, DCL and
mockgcp from its own tree by `replace`. Replace directives do not apply to
dependents, so ours repeats them pointing into the checkout, plus its
`k8s.io/*` version pins (`hack/check-go-mod.sh` fails if any of its pins
is missing from ours). The checkout is about 650 MB and is fetched, not
committed (`hack/upstream.sh`). The binary links what `directbase` links:
`bin/manager` is 119 MB.

### 2.2 Our own manager beside Config Connector

`cmd/manager` is a small controller-runtime manager. For each gap kind,
`pkg/controller` adds Config Connector's direct reconciler
(`directbase.AddController`) and its deletion defender
(`deletiondefender.Add`), both unchanged. The deletion defender is not
optional: the direct reconciler puts the defender's finalizer on every
object and will not delete until the defender removes it; the defender
flips objects to abandon when their CRD is being deleted, so uninstalling
config-connector-extensions never deletes cloud resources.

The reconciler reads `ConfigConnector` / `ConfigConnectorContext` (actuation
mode, namespaced vs cluster mode), so those CRDs must exist on the cluster,
which they do wherever Config Connector is installed. In namespaced mode it
requires a `ConfigConnectorContext` in the object's namespace, as Config
Connector does.

Identity: the manager authenticates with Application Default Credentials.
In a cluster it runs as its own Kubernetes service account beside Config
Connector's for the scope, carrying Config Connector's Workload Identity
annotation, so it acts as the Google service account Config Connector has
there, per namespace in namespaced mode (section 9). Sharing Config
Connector's own service account is an opt-in.

### 2.3 Per-kind code: copy the nearest upstream code

The rule: where Config Connector has the resource on `master`, copy its
code and change as little as possible. For `SQLUser`:

- On `master` `SQLUser` is a **Terraform-family** kind
  (`static_config.go`: `DefaultController: Terraform`, only Terraform
  supported). There is no direct `SQLUser` controller to copy.
- **Types**: copied from Config Connector's generated Go types for the
  Terraform-based CRD (`pkg/clients/generated/apis/sql/v1beta1/sqluser_types.go`),
  so the schema is the one users of `sql.cnrm.cloud.google.com/SQLUser`
  already write. `instanceRef` and `password` use the shared types Config
  Connector's direct controllers use for the same shapes
  (`refs/v1beta1.SQLInstanceRef`, as `SQLSSLCert` did when it went direct;
  `refs/v1beta1/secret.Legacy`, as `SQLInstance` does). Flattened, the CRD
  has upstream's property paths plus `spec.databaseRoles` and
  `status.appliedDatabaseRoles`, and nothing else differs.
- **Controller and identity**: copied from the direct `SQLSSLCert`
  controller, the sibling Cloud SQL sub-resource that already moved from
  Terraform to direct on the same `sqladmin/v1beta4` client, changed for
  users.
- **Behaviour** of `databaseRoles` follows Terraform's `google_sql_user`
  (read, not copied): roles go in the insert body; when they change, they
  go to `users.update` as `databaseRoles` with `revokeExistingRoles=true`.
- **controllerbuilder is not used for this kind.** Config Connector's
  Cloud SQL direct controllers talk to the REST client
  (`google.golang.org/api/sqladmin/v1beta4`) with hand-written conversion,
  so copying them means not generating. For kinds whose upstream siblings
  are generated from protos, use `controllerbuilder` as the analysis
  recommends (section 3.3).

Upstream PR #7148 (open since 2026-03-24, not approved, last updated
2026-06-04, head `eb5d176a`) migrates `SQLUser` to a direct controller and
writes `databaseRoles` but leaves it commented out (`/*NOTYET*/`) in the
types, the diff and the mapping. The intent is to use it once it does
what this kind needs; it does not yet, so ours stays, and
`hack/upstream-pr.sh` watches it (UPSTREAM.md, "Watched upstream change").
It is the shape upstream is likely to take, and the kind we would migrate
to. Two
differences to settle if it merges: it compares `spec.databaseRoles` with
the roles the API returns on `users.get`, where we compare with what we
last applied (Terraform does not read roles back either, and we have not
verified that the API returns them); and its CRD will be in
`sql.cnrm.cloud.google.com`, so moving means re-creating objects in the
other group with `cnrm.cloud.google.com/deletion-policy: abandon` and
acquiring by `resourceID`.

**`AccessContextManagerAuthorizedOrgsDesc` and open PR #14202**
(foundation item 21, 2026-11-23). Upstream's open pull request #14202
(fetched read-only, head `2fe675c2`) adds
`accesscontextmanager.cnrm.cloud.google.com/v1alpha1`
`AccessContextManagerAuthorizedOrgsDesc` as a **Terraform-based** kind: a
service mapping for `google_access_context_manager_authorized_orgs_desc`
(in Config Connector's Terraform fork), a generated CRD and fixtures, and
no mockgcp. There is no Go code to copy, and this manager runs no
Terraform-based kind (section 2.1, option B), so ours is generated by
`hack/discokind` (section 2.4) in `accesscontextmanager.gcp.hfl.dev/v1alpha1`,
with what the pull request has copied: its fixtures, its field names
(`assetType`, `authorizationDirection`, `authorizationType`, `orgs`,
`resourceID`, the API's, as Discovery gives them) and its immutable
fields (the three types; only `orgs` changes). One difference: the access
policy is `spec.accessPolicy`, the policy's number, where the pull
request has `spec.accessPolicyRef` to an `AccessContextManagerAccessPolicy`
(`external: accessPolicies/{number}`); `hack/discokind` names a name's
other parts by ID. The API's create takes no ID parameter, only the full
name in the body; `nameInBody` now covers that (`hack/discokind`,
`pkg/discokind`, its fake).
**When #14202 merges and a release has it**: move the pin to that
release (section 3.2); for each object, create Config Connector's
`AccessContextManagerAuthorizedOrgsDesc` with the same `resourceID` and
`accessPolicyRef.external: accessPolicies/{accessPolicy}` (it acquires
the existing resource), wait for it to be Ready, then delete ours with
`cnrm.cloud.google.com/deletion-policy: abandon`; then retire the kind
(section 3.4: its `kinds.yaml` entry, fixtures, CRD and doc) and drop the
row in `UPSTREAM.md`. Until then `hack/upstream-pr.sh`'s table in
`UPSTREAM.md` ("Copied from open pull requests") records the head read.

**`ComputeFirewallPolicyRule`, ours beside Config Connector's**
(foundation item 19, 2026-11-23). Config Connector's hierarchical
`ComputeFirewallPolicyRule` is direct, without `ruleName`,
`securityProfileGroup`, `tlsInspect`, `targetSecureTags` (its mapper
marks the four `MISSING`), `match.srcSecureTags`, the network contexts
and `match.srcNetworks`, and its update sends the whole rule to
`patchRule`, so a field kind beside it (decision 37) would be reset by
every change it makes: section 6.6's pattern 3, a kind that owns the
whole rule, as `RunService` (decision 11) and `ComputeRouter` (decision
14) do. Upstream was read again (master `bc464f5`; open pull requests
searched for the kind and the fields): only #5440 (2025-10-16) touches
the kind, changing its Go types' optionality, with no new field. Ours is
`compute.gcp.hfl.dev/v1alpha1` `ComputeFirewallPolicyRule` (decision 18:
Config Connector's name), with Config Connector's types and controller
copied (`UPSTREAM.md`), so a spec written for Config Connector's kind
works with ours by changing the `apiVersion`, and `status.externalRef`
is spelled the same (`locations/global/firewallPolicies/{policy}/rules/{priority}`).
The spec mapping, comparison and patch body are our
`ComputeNetworkFirewallPolicyRule`'s (a list the spec empties is sent as
`[]`, which `patchRule` needs; upstream's leaves it out).
**Which to use:** Config Connector's kind for a rule that needs none of
the added fields; ours for one that does. Never both for one rule: each
would put back the other's change on every reconcile. To move a rule,
create ours with the same `firewallPolicyRef` and `priority` (it finds
the rule and patches only what differs, without `addRule`), wait for it
to be Ready, then delete Config Connector's object with
`cnrm.cloud.google.com/deletion-policy: abandon`. When Config Connector's
kind gains the fields and a release has them, move back the same way and
retire ours (section 3.4).

**`PrivateCACertificateAuthority`, ours beside Config Connector's**
(foundation item 25, 2026-11-23). The API takes an authority's
`config` (its subject, X.509 extensions, `subjectKeyId`), key, lifetime,
type and bucket only at create, so the fields Config Connector's kind
lacks there (`config.x509Config.nameConstraints` and `aiaOcspServers`,
`config.subjectKeyId`) can only come from a kind that makes the
authority: section 6.6's pattern 1, a create-only whole-object kind, as
`ComputeVPNTunnel` is. Ours, `privateca.gcp.hfl.dev/v1alpha1`
`PrivateCACertificateAuthority`, copies Config Connector's direct types,
mapper and controller (`UPSTREAM.md`); every field but the labels and
`userDefinedAccessUrls` (which the API changes) is immutable by CEL, and
nothing else is compared after create. As Terraform does (read, not
copied), a self-signed authority is enabled after create and an enabled
one disabled before the delete; `spec.skipGracePeriod` (Terraform's
`skip_grace_period`) deletes it without the 30-day grace period. Upstream
(the pin and master): Config Connector's direct controller has none of
these fields and enables nothing; no pull request adds them.
**Which to use:** Config Connector's kind for an authority that needs
none of the added fields; ours for a new authority that does. An existing
authority cannot gain them (the API takes them only at create), so it
stays on Config Connector's kind; an object of ours for an existing
authority acquires it (its config is not compared) and from then owns
its labels and URLs. Never both for one authority.

### 2.4 Generated kinds: standard collections from Discovery (`discokind`)

Where the API resource is a standard collection (create with an ID query
parameter, get and delete by name, optionally patch with an update mask)
and Config Connector has no code for it, the kind is generated, not
copied: `hack/discokind` reads `hack/discokind/kinds.yaml` and the
Discovery document pinned in `docs/coverage/sources.yaml`, and writes the
kind's types and descriptor; `pkg/discokind` reconciles every such kind
with one controller (the spec is the request body; read-only fields go to
`status.observedState`), `pkg/discokind/fake` serves every such collection
in front of mockgcp, and `pkg/discokind/kindtest` is every such kind's
end-to-end test, on the fake or live. Nothing is copied from upstream,
since upstream has nothing for these kinds; when upstream adds a kind for
one, it retires as any other (section 3.4). Why, and what it reaches:
`docs/throughput.md`. How it is tested: `docs/testing.md`. Limits: no
references beyond the parent (the other parts of a name are IDs), no
secrets in status: a secret the API returns (`secretOutputs` in
`kinds.yaml`) goes to a Kubernetes Secret the object owns, named by
`spec.secretOutput.name`, never to status (decision 38, 2026-11-23). A string field the API takes as a secret can take
Config Connector's `secret.Legacy` instead (`secrets` in `kinds.yaml`:
`value`, or `valueFrom.secretKeyRef` to a Secret in the object's
namespace, read by the controller and sent as the string), and a field
the API never returns is marked `inputOnly`; both are left out of the
comparison and sent again when `status.inputVersion` changes (decision
33, `LookerInstance`'s OAuth client, 2026-11-15). Compute Engine's shape is covered too
(`InsertStyle`, 2026-11-11): insert with the ID as the body's name, a
patch that merges the fields sent with the read fingerprint (or an update
that replaces the resource), and compute operations polled at their
`selfLink`. A kind marked `nameInBody` in `kinds.yaml` also sends its
full name as the create's body `name` (Cloud Functions v2 refuses a
create without it; 2026-11-14). Two more found by the catalog's live
runs (2026-11-14): a method answers a long-running operation when its
response is `Operation`, `GoogleLongrunningOperation` or an API's own
copy of it under another name (API Gateway's `ApigatewayOperation`, by
its fields); the generator had missed that one, so API Gateway's deletes
dropped the finalizer before their operations ended, a config deleted
beside its gateway failed "in use" in the cloud and stayed there, FAILED,
and its API could not go. A delete now waits for its operation, and one
refused because the resource is still in use (FAILED_PRECONDITION, "in
use", "nested resources") is tried again for up to ten minutes before
the reconciler's own retries take over. `missingIs403` marks a kind whose
API answers 403, not 404, for a resource that does not exist
(`BigLakeCatalog`; `BigLakeDatabase` and `BigLakeTable` too, from the
catalog repository's live run `cat-11141340-9f1f`, 2026-11-14: BigLake
answers a missing database or table 403 as it does a missing catalog): the read before a create takes that 403 as not found
only while the object has never made or acquired its resource (no
`status.externalRef`), and a real denial then fails the create; once
made, a 403 is a denial on update as for any kind. While deleting, a 403
on the read or on the delete counts as gone only when a listing proves
it (decision 32, 2026-11-14: a catalog's delete takes its databases and
tables along, and in the catalog repository's run `cat-11141549-4cb0`
their finalizers then held on the 403 for 30 minutes). Terraform
has no special handling here because it never reads before a create.
`getQuery` sends query parameters with every get (`view: FULL` for API
Gateway configs, whose source files a get leaves out otherwise).
`BigLakeTable` (2026-11-14) is a parallel kind for one Config Connector
already serves (decisions 19 and 26): Config Connector's own
`bigquerybiglake.cnrm.cloud.google.com/v1beta1` `BigLakeTable` (released
in 1.137) reads a table with Find before its create and takes only a 404
as not found (`biglaketable_controller.go` at the pin), so against the
real API it fails "biglake.tables.get denied" and never makes a table;
its golden logs show a 404 because mockgcp answers one. Upstream has a
precedent for the fix (its `CloudIdentityGroup` and Data Catalog kinds
take a 403 in Find as not found), but no issue or pull request for
BigLake. Ours is generated by `hack/discokind` with `missingIs403`, owns
the whole table, and a workload moves to it by acquiring the same table
(`resourceID`) and abandoning Config Connector's object; it retires when
upstream's Find takes the 403 (section 3.4). Upstream's direct
`BigLakeCatalog` and `BigLakeDatabase` (alpha, at the pin) have the same
Find. The
first kinds: `LoggingSavedQuery`, `IAMOauthClient`,
`IAMWorkforcePoolProviderKey` (2026-11-11); then
`IAMWorkforcePoolProviderScimTenant`, `GKEHubMembershipRBACRoleBinding`,
`GKEHubRolloutSequence` (foundation item 5) and `LoggingLink` with
folder, organization and billing account parents. Then fifteen Compute
Engine kinds: regional notification endpoints, health check services,
health aggregation policies, health sources, composite health checks and
backend buckets; global and zonal VM extension policies; rollout plans;
BYOIP public advertised and delegated prefixes (foundation item 7);
Cross-Site networks and wire groups and Interconnect groups. Then
(2026-11-12) five that the catalog's modules create and Config Connector
lacks: `ComputeNetworkAttachment` (Config Connector's is alpha; VPC
networking is a foundation area, so ours is `v1beta1` under decision 13),
`ComputeRegionInstanceTemplate` (Config Connector's template is global
only; the API cannot change a template, so every field is immutable),
`LookerInstance`, and `NetworkServicesAgentGateway` and
`NetworkServicesAgentConnectivityTemplate` (preview). Compute Engine
answers a resource path a request sent (`projects/P/regions/R/subnetworks/S`)
with its full URL, so the controller's comparison takes the two as one
(`sameComputeURL`); without it a network attachment's `subnetworks` would
differ on every resync.
A generated kind's delete takes the resource's name from
`status.externalRef` and does not resolve its parent again (2026-11-13,
`IAMBinding`'s rule, `pkg/iampolicy.Resolve`): a parent object deleted
first, when everything is deleted at once, held the delete on "referenced
Project ... not found". An object that never had a resource goes without
a call (`TestComputeNetworkAttachmentWithItsParent`). The catalog's stuck
`net-vpc` attachment ("reference ComputeSubnetwork ... is not found") was
Config Connector's alpha `ComputeNetworkAttachment`, whose
`subnetworkRefs` its directbase resolves before every delete; ours takes
the subnet's URL in `subnetworks` and never reads a subnet object, so a
chart that uses ours (as `net-address` does) does not stick.
The hand-written `IAMDenyPolicy` takes the same rule (2026-11-13,
`docs/kinds/IAMDenyPolicy.md`): its Folder object deleted first held its
delete on "referenced Folder ... not found".
Then five foundation (tier 1)
gaps of the same shape (2026-11-12): Resource
Manager's `ResourceManagerCapabilityConfig` (an organization, folder or
project) and `ResourceManagerBoundary` (an organization or folder), and
Network Connectivity's `NetworkConnectivityPscAuthorizationPolicy`,
`NetworkConnectivityAutomatedDnsRecord` and
`NetworkConnectivitySpokeGatewayAdvertisedRoute`; all five are proven on
the fake and skip live (organization access, or a role and paid
prerequisites the live-test project lacks; each fixture's `live-skip`
says which). Then eight copies of Config Connector alpha-only kinds as
`v1beta1` (decision 26, 2026-11-12), in the catalog's
`blocked-kinds.md` order: `CertificateManagerCertificateIssuanceConfig`,
`BackupDRManagementServer`, `NetworkSecurityDNSThreatDetector`, the four
Backup for GKE kinds (`GKEBackupBackupPlan`, `GKEBackupRestorePlan`,
`GKEBackupBackupChannel`, `GKEBackupRestoreChannel`) and
`ComputeGlobalNetworkEndpointGroup`. Then eight more (2026-11-12):
`APIGatewayAPIConfig`, `APIGatewayGateway`, `BigLakeCatalog`,
`BigLakeDatabase`, `CloudBuildConnection`, `CloudDeployTarget`,
`CloudDeployDeployPolicy` and `CloudDeployAutomation`. Then six more:
`CloudFunctions2Function` and Dataplex's `DataplexLake`, `DataplexZone`,
`DataplexAsset`, `DataplexAspectType` and `DataplexDataScan`; the IAM
rows for these resources list our kind before Config Connector's alpha
one, so a `resourceRef` by name finds ours. Generating them found that
`hack/discokind` dropped every field called `name`, nested ones too (an
asset's bucket, an aspect type's template, a template's network
interface); only the resource's own top-level name is now left out.
Then `CloudBuildRepository` (`v1alpha1`, 2026-11-12), a Cloud Build
connection's repository, for the catalog's `cloud-build-v2-connection`
module (`google_cloudbuildv2_repository`): Config Connector names the kind
in its reference type (`apis/cloudbuild/v1beta1/repository_reference.go`)
but has no kind, so it takes that name (decision 18) and starts at
`v1alpha1` (decision 26's last sentence; question 6's first half). The
API cannot patch a repository, so every field is immutable. Live waits,
as the connection's does, on a test GitHub app.
Then (2026-11-12) the catalog's `net-swp` module's four Secure Web Proxy
kinds, Config Connector alpha-only and so `v1beta1` (decision 26):
`NetworkSecurityGatewaySecurityPolicy`,
`NetworkSecurityGatewaySecurityPolicyRule` (the catalog listed it with no
kind; upstream added its alpha kind before our pin),
`NetworkSecurityURLList` and `NetworkSecurityTLSInspectionPolicy`; a rule's
live test makes its policy first (`gatewaysecuritypolicy_test.go`), and the
sweeper deletes a run's rules before the discokind loop deletes its
policies. These collections have no `global` location (listing it is a
400, run `cce-11130256-843f`), so `kinds.yaml` gained `sweepLocation`:
the sweeper lists them in `us-central1`, where their live fixtures make
them. The policy, its rule and the URL list pass live (runs
`cce-11130311-56bc`, `cce-11130336-5f6f`); the rule's fixtures name the
policy by `${KIND_NAME}`, a placeholder that stays the first object's
`${NAME}` in the abandoned object's fixture too. Config Connector's alpha types reference the TLS inspection
policy and CA pool; ours take their paths, as discokind does. A TLS
inspection policy skips live (it needs a CA pool). And `ApigeeDnsZone`
(`v1alpha1`, no Config Connector kind; Terraform's name, decision 18) for
the catalog's `apigee` module: the API cannot patch a zone, so every field
is immutable; its `organization` is the Apigee organization (a project's
ID), and it skips live until the live-test project has an Apigee
organization.
Then (2026-11-13) a collection with no create: `DiscoveryEngineCmekConfig`
(`v1alpha1`, no Config Connector kind; Terraform's name, decision 18;
the catalog's `ai-applications` module). Discovery Engine makes a CMEK
config with `cmekConfigs.patch`, which takes no update mask, so
`hack/discokind` reads a collection with patch and no create as
`PatchCreates` (create and update both patch the spec's body, with no
mask when the method takes none). Three `kinds.yaml` fields came with it:
`locationHost` (Discovery Engine serves `us` and `eu` only at
`https://<location>-discoveryengine.googleapis.com/`, which the Discovery
document does not say; the fake refuses a location at another host, as the
API does), `query` (patch's `setDefault` is a spec field sent as a query
parameter with every write; the API never returns it, so it is never a
difference) and, in the sweeper's list, the location's host. It skips
live: it needs the API on and a KMS key the project's Discovery Engine
service agent may use, an IAM grant the live tests may not make.

Then `NetworkServicesGateway` (`networkservices.gcp.hfl.dev/v1beta1`,
2026-11-13), the gateway a Secure Web Proxy is, for the catalog's
`net-swp` module. Config Connector's kind is `v1beta1`, but at 1.160.0 it
cannot make a `SECURE_WEB_GATEWAY`: it lacks `network`, `subnetwork`,
`gatewaySecurityPolicy`, `certificateUrls`, `labels` and `routingMode`,
and requires `scope`. Ours keeps the name (decision 18) at `v1beta1`, as
a stable equivalent under decision 26, generated with the API's whole
field set for both types (`OPEN_MESH` and `SECURE_WEB_GATEWAY`). The
fields Terraform's `google_network_services_gateway` forces a new gateway
for are immutable here. `kindtest.RunCase` runs a kind's test with a
second set of fixtures, so both types are tested. A Secure Web Proxy
makes a Cloud Router with a Cloud NAT (`swg-autogen-router-...`) in its
network, which the API keeps after the last gateway goes; ours keeps it
too, as Terraform does by default, and the live test and the sweeper
delete it. It skips live until `networkservices.googleapis.com` is on in
the live-test project (it is off; the live-test account already has the
permissions through `roles/compute.networkAdmin`).
`docs/kinds/NetworkServicesGateway.md` has the rest.

Then `NetworkServicesAuthzExtension` (`networkservices.gcp.hfl.dev/v1beta1`,
2026-11-13), generated, for the catalog's `geap-agent-gateway` module
(`google_network_services_authz_extension`): an agent gateway's
authorization goes to IAP or Model Armor through an extension's `service`
(`iap.googleapis.com`, a Model Armor endpoint). Config Connector's kind is
alpha, requires a backend service and has no `service` field, so it cannot
delegate to either; ours keeps the name at `v1beta1` (decision 26) with the
API's whole field set (`service`, `timeout`, `failOpen`, `metadata`,
`forwardHeaders`, `forwardAttributes`, `wireFormat`, `authority`,
`loadBalancingScheme`, `labels`). Every field patches in place. It skips
live as `NetworkServicesGateway` does (`networkservices.googleapis.com` is
off in the live-test project).

Then `NetworkSecurityAuthzPolicy` (`networksecurity.gcp.hfl.dev/v1beta1`,
2026-11-13), generated, for the same module: fabric's `iap_config` and
`model_armor_config` bind a `NetworkServicesAuthzExtension` to the agent
gateway through an authorization policy whose target is the gateway
(`google_network_security_authz_policy`, `action: CUSTOM`,
`customProvider.authzExtension`). The API takes it: `target.resources` is
"forwarding rules, Secure Web Proxy gateways or agent gateways", and
`target.loadBalancingScheme` must be left out for an agent gateway.
Config Connector's kind is alpha and its `target.resources` are
`ForwardingRuleRef`s, which it validates as forwarding rules
(`ForwardingRuleRef.ValidateExternal`), so it cannot name an agent
gateway; no open upstream pull request changes that (checked 2026-11-13).
Ours keeps the name at `v1beta1` (decision 26) with the API's whole field
set; `target.resources` and `customProvider.authzExtension.resources` are
the API's resource names (`projects/P/locations/L/agentGateways/G`).
`policyProfile` is immutable. It skips live as
`NetworkServicesAuthzExtension` does: an agent gateway needs
`networkservices.googleapis.com`, off in the live-test project.

Then `ComputeInterconnectAttachment` (`compute.gcp.hfl.dev/v1beta1`,
2026-11-13), generated, for the catalog's VPN and interconnect ports:
Config Connector's `v1beta1` kind lacks the candidate Cloud Router and
customer router addresses (`candidateCloudRouterIpAddress`,
`candidateCustomerRouterIpAddress`, and their IPv6 forms), so ours keeps
the name at `v1beta1` with the API's whole field set (decisions 19 and 26,
as `NetworkServicesGateway`). The fields Terraform's
`google_compute_interconnect_attachment` forces a new attachment for are
immutable. It skips live: the live-test project has no Dedicated
Interconnect. Its test adds to the `compute` package's time (section 10,
question 8).

Then `EndpointsService` (`servicemanagement.gcp.hfl.dev/v1alpha1`,
2026-11-13), hand-written rather than generated: a Cloud Endpoints service
is three Service Management calls (create the service, submit a
configuration, roll it out), which is not a discokind shape, and Config
Connector has no Service Management code to copy. It is Terraform's
`google_endpoints_service` (decision 18; Config Connector has no kind) for
the catalog's `endpoints` module, at `v1alpha1` (decision 26's last
sentence). Its group is the API's (`servicemanagement`), as every group here
is named after its API, not the product. A configuration is submitted and
rolled out again only when the files change (`status.configDigest`) or
another configuration was rolled out by hand. mockgcp has no Service
Management, so a fake in front of it serves the calls
(`docs/fake-cloud.md`). `docs/kinds/EndpointsService.md` has the rest.

Then `IAMPolicyBinding` (`iam.gcp.hfl.dev/v1alpha1`, 2026-11-13),
hand-written: an IAM v3 policy binding, which binds a principal access
boundary policy to the principals of an organization, folder or project
(Terraform's `google_iam_{organizations,folders,projects}_policy_binding`;
the catalog's `geap-agent-gateway` module needs the project's, proposed
there as `IamProjectsPolicyBinding`). IAM v3 has no Discovery document at
the pinned discovery-artifact-manager commit nor at its master
(2026-11-13), so `hack/discokind` cannot generate it. Upstream, read
2026-11-13: no release or `master` has it; open PR #14133 (last updated
2026-10-29) generates `IAMPolicyBinding` types, identity, reference and
mapper for an organization's binding, with no controller. The kind takes
that name (decision 18; decision 12 already kept `IAMPolicyBinding` free
for it) and, as Config Connector has released no kind, starts at
`v1alpha1` (decision 26's last sentence). The types are copied from the
PR; the parent is one of `organizationRef`, `folderRef` and `projectRef`
(decision 18), the policy a `policyRef` to our
`IAMPrincipalAccessBoundaryPolicy` or a policy's name, and the controller
is written here. mockgcp patch 0044 serves the bindings.
`docs/kinds/IAMPolicyBinding.md` has the rest.
An `IAMPolicyBinding` also binds an access policy to a resource
(2026-11-13, for `geap-agent-gateway`): `target.resource` (on
`cloud.google.com/go/iam` v1.14.0) beside `principalSet`, and, with
`policyKind: ACCESS`, `policyRef.name` names an `IAMAccessPolicy`;
mockgcp patch 0046. The kind stays `v1alpha1` (Config Connector has no
released kind; decision 26's last sentence).

Then `IAMAccessPolicy` (`iam.gcp.hfl.dev/v1beta1`, 2026-11-13),
hand-written, for the catalog's `geap-agent-gateway` module
(`google_iam_projects_access_policy`, the policy an agent gateway's access
rules are). Config Connector has the kind at `v1alpha1` with an
organization parent only and no controller; ours keeps the name at
`v1beta1` (decision 26) with an organization, folder or project parent
(decision 18), and imports upstream's `AccessPolicyDetails` and observed
state. IAM v3 has no Discovery document at the pin, so it is not
generated. `cloud.google.com/go/iam` moved from v1.11.0 to v1.14.0, the
first with access policies in `apiv3` (no other module moved). The API's
update replaces the policy (no update mask), so any difference sends the
whole spec. mockgcp patch 0045 serves the policies; a policy a binding
names is refused deletion without force, as the API does. Live skips: the
live-test account has no access policy role. `docs/kinds/IAMAccessPolicy.md`
has the rest.

Then `ChronicleRuleDeployment` (`chronicle.gcp.hfl.dev/v1alpha1`,
2026-11-13), hand-written: the API has no Discovery document at the
pinned discovery-artifact-manager commit (nor at its head), and a rule's
deployment is a singleton with get and patch only, which is not a
discokind shape. Config Connector has `ChronicleRule` (alpha, at the pin)
but no deployment kind, so ours takes Terraform's name
(`google_chronicle_rule_deployment`, decision 18) for the catalog's
`secops-rules` module, at `v1alpha1` (decision 26's last sentence). Its
`ruleRef` is Config Connector's own `ChronicleRuleRef`, by import. As
Terraform, there is no create or delete: the object patches the fields
it sets, and deleting it leaves the deployment. It speaks REST JSON to
`<location>-chronicle.googleapis.com` (Terraform's host) rather than
`cloud.google.com/go/chronicle` v1.5.0, which lacks the customizable run
frequencies and `scheduleCustomizations`. mockgcp has no Chronicle, so a
fake serves it (`docs/fake-cloud.md`); live skips, needing a SecOps
instance. `docs/kinds/ChronicleRuleDeployment.md` has the rest.

Then `ChronicleRule` (`chronicle.gcp.hfl.dev/v1beta1`, 2026-11-13),
hand-written for the catalog's `secops-rules` module
(`google_chronicle_rule`). Config Connector 1.160.0 has the kind at
`v1alpha1` with no instance field: its identity is
`projects/{project}/locations/{location}/instances/{instance}/rules/{rule}`,
but `getIdentityFromChronicleRuleSpec` leaves the instance empty and fails
with "cannot resolve instance for ChronicleRule", so no object of it can
become Ready. Ours keeps the name (decision 18) at `v1beta1` (decision 26),
copies upstream's types with `instance` added, and imports upstream's
identity and observed state. The API assigns a rule's ID (`ru_...`):
create records it in `status.externalRef`, and `spec.resourceID` acquires a
rule that exists (a missing one is not Ready; the ID cannot be chosen).
`deletionPolicy` (`DEFAULT`, `FORCE`) is Terraform's. The same REST client
and host as the deployment; the Chronicle fake gained rule patch.
`ChronicleRuleDeployment`'s `ruleRef` by `name` finds ours first, then
Config Connector's. `docs/kinds/ChronicleRule.md` has the rest.

Then `ChronicleReferenceList` (`chronicle.gcp.hfl.dev/v1beta1`,
2026-11-13), the third kind `secops-rules` needs
(`google_chronicle_reference_list`), for the same reason: Config
Connector's is alpha with no instance field. Types copied with `instance`
added; upstream's identity, leaf types and observed state imported. The
ID is the user's (`referenceListId`; `resourceID` or `metadata.name`),
letters, digits and underscores only (a CEL pattern; a `metadata.name`
with a hyphen needs `resourceID`). The API has no delete for a reference
list, so deleting the object leaves it, as Terraform does, and a new
object with the same ID takes it back (find, then patch). Reads use the
full view, the only one with the entries. `docs/kinds/ChronicleReferenceList.md`
has the rest.

Then `SecurityCenterBigQueryExport` and `SecurityCenterNotificationConfig`
(`securitycenter.gcp.hfl.dev/v1beta1`, 2026-11-19), foundation item 20
under decision 16: Config Connector's kinds are `v1alpha1` and take an
organization only, so ours are stable equivalents (decision 26) taking an
organization, folder or project, generated from the `securitycenter` v1
Discovery document (the API `SecurityCenterMuteConfig` uses; the
collections are standard under all three parents). Upstream rechecked
2026-11-19 (read-only): #7673 has not moved, #13932 only aligns the
export's mock logs. The project's fixtures are the generated tests;
`kindtest.RunCase` runs a folder's and an organization's on the fake
(`parents_test.go`). All skip live: Security Command Center is not active
in the live project, as for the mute config. The API's descriptions are
empty here, so nothing is marked required: a missing `dataset` or
`pubsubTopic` is the API's 400, shown in the object's Ready message. The
two custom module kinds are not this shape (create takes no ID; the server
names the module).

Then (2026-11-19) two foundation whole-resource gaps with no Config
Connector kind, from `docs/coverage/foundation.md`'s "Every resource":
`ComputeInterconnectAttachmentGroup` (`compute.gcp.hfl.dev/v1alpha1`,
item 7's last, beside `ComputeInterconnectGroup`; its `etag` is left out
of the spec with `skip`, since the API generates it) and
`DNSOutboundEndpoint` (`dns.gcp.hfl.dev/v1alpha1`, Cloud DNS's
`outboundEndpoints`, a long-running create; Terraform's
`google_dns_outbound_endpoint`, named under decision 18). Cloud DNS's create takes
`clientOperationId` beside `outboundEndpointId`, so the generator's ID
parameter leaves it out as it does `requestId` (`isIDParam`). Both skip
live: the attachment group needs Dedicated Interconnects, the endpoint
Cloud DNS in the live project (as `DNSResponsePolicy`) and a network and
subnetwork Setup.

Then (2026-11-23) `ComputeServiceAttachment`
(`compute.gcp.hfl.dev/v1alpha1`), generated, for foundation item 11's
fields that no field kind can own: Config Connector's DCL-based kind sends
`consumerAcceptLists` whole and has no `networkUrl` in it, and lacks
`domainNames` (create only), `metadata` and `natIpsPerEndpoint`. Ours
owns the whole attachment with the API's field set (decision 19, as
`ComputeInterconnectAttachment`), at `v1alpha1` until it passes live
(decisions 27 and 34). It is the first generated kind for a collection
mockgcp already serves and other tests make resources in
(`ComputeServiceAttachmentFields`'), so `kinds.yaml` gained `mockgcp`:
`pkg/discokind/fake` passes the collection's requests to mockgcp instead
of serving them (`Kind.Mockgcp`), and the kind is left out of
`hack/live/discokinds.tsv`, since `sweep.sh` sweeps service attachments
in its own section, before the forwarding rules and subnets they use. Its
live Setup makes a producer (an internal passthrough load balancer and a
PSC NAT subnet) for the object and its abandoned one. Then
`ComputeSubnetwork` (`compute.gcp.hfl.dev/v1alpha1`, the same day, on
mockgcp's subnetworks the same way), for foundation item 16's fields the
API takes only at insert (`ipCollection`, `externalIpv6Prefix`,
`internalIpv6Prefix`, `resolveSubnetMask`, `params.resourceManagerTags`,
the last input-only) and the secondary ranges' `ipCollection` and
`ipVersion`, which Terraform sends in a whole list. `ipCidrRange` and
`privateIpGoogleAccess` are immutable in ours: the API changes them only
by `expandIpCidrRange` and `setPrivateIpGoogleAccess`, which the generated
controller does not call, so a subnet that needs those changes stays on
Config Connector's kind. A second fixture set (`insertonly`) makes a
BYOIP, tagged subnet on the fake; it skips live (no sub-prefix or tag in
the live-test project).

What the first live runs added (2026-11-12, run `cce-11120814-5d2a`):
a string field whose Discovery description says "Cannot exceed N
characters" gets that `maxLength` in the CRD, and the fixture check
holds fixtures to it (an OAuth client's 41-character display name was
refused by every patch, and the reconciler backed off without a log
line); `kindtest` prints the object's Ready message when the cloud does
not reach the spec; and a kind can have a live `Setup` for what its
fixtures cannot make (`GKEHubRolloutSequence` makes the project's fleet,
free, and deletes it). `ComputeRegionNotificationEndpoint` skips live:
Health Check as a Service is gated per project. `ComputeRegionBackendBucket` and
`LoggingLink` got Setups of their own (2026-11-12, run
`cce-11121541-aa4d`): a Cloud Storage bucket, and a log bucket with Log
Analytics on, for the test's object and its abandoned one; both pass
live.

### Switching to PR #7148

When `hack/upstream-pr.sh` reports `databaseRoles` live:

1. Record the new head in UPSTREAM.md ("PR head", "Read on").
2. Fetch it into the checkout (the script does) and copy the PR's
   `sqluser_types.go`, `sqluser_controller.go`, `sqluser_mappings.go` and
   `sqluser_resolverefs.go` over ours with the group change only, keeping
   Google's headers and a "Modified by" note; record each in "What is
   copied" with the PR head as the source.
3. Re-apply our differences as small edits: revoke on removal
   (`revokeExistingRoles`) if the PR adds without revoking, and
   `status.appliedDatabaseRoles` if the API is shown not to return roles.
4. `task check`; the mockgcp test must still pass unchanged.

Refreshing later is the same steps against the newer head; once the PR
merges, retire the kind instead ("Retiring a gap kind").

### 2.5 Version slots: release, adopt, pause and the slot guard

A hosted Bundt runs its infrastructure clusters as version slots (for
example `current` and `previous`), each running one version of every
provider, and moves an organization between slots with one sequence:
pause or abandon on the old slot, adopt on the new, check there is no
diff, delete the old object. These kinds take part on the same terms as
Config Connector's (checked 2026-11-12 at the pin), because every kind,
the `discokind` ones included, runs on Config Connector's unchanged direct
reconciler (`third_party/.../pkg/controller/direct/directbase/directbase_controller.go`):

| Kind family | Release without deleting | Adopt an existing resource | Pause |
| --- | --- | --- | --- |
| Every kind (typed kinds under `pkg/controller/direct/*`, and the `discokind` kinds) | had: `cnrm.cloud.google.com/deletion-policy: abandon` (`directbase` `doReconcile`, `k8s.HasAbandonAnnotation`); the deletion defender abandons when the CRD is deleted | had: `Find` by the object's identity, then `Update`, never `Create`, when found (Config Connector's "acquire") | had: `cnrm.cloud.google.com/actuation-mode: Paused` per object, or `spec.actuation: Paused` on `ConfigConnector` / `ConfigConnectorContext` (`resourceactuation.ShouldSkipActuation`); `cnrm.cloud.google.com/reconcile-interval-in-seconds: "0"` stops drift correction only |
| Kinds whose ID the server assigns: `ComputeOrganizationSecurityPolicy`, `SQLAdminBackup`, `SecretManagerSecretVersion` | had | had, only with `spec.resourceID` set to the existing ID (without it the first reconcile creates a new one) | had |
| Singletons and parts of a shared resource: `LoggingSettings`, `ComputeProjectDefaultNetworkTier`, `ProjectDefaultServiceAccounts`, `ServiceIdentity`, `ComputeProjectMetadataItem`, `IAMBinding`, `IAMMember`, the firewall policy and security policy rules and associations | had (abandon leaves the setting, key, binding or rule) | had: they always exist, or are named by their parent and key | had |
| Every kind | **added:** the slot guard (below) | **added:** `hfl.dev/slot-takeover` lets the new slot claim a resource the old one marked | (the guard makes a resumed old slot harmless where it can mark) |

Two upstream behaviours a move must allow for: a `Paused` object is never
finalized, so the old object is resumed with `deletion-policy: abandon`
in the same update and then deleted (abandoning makes no cloud call); and
the reconciler's event filter (`kccpredicate.UnderlyingResourceOutOfSyncPredicate`)
ignores most annotation changes, so annotations that should act at once
are set when the object is created.

**The slot guard** (`pkg/slotguard`, added 2026-11-12) is opt-in: off
unless the manager has `--slot=<name>` or `HFL_SLOT` (the flag wins; the
operator passes it with `--manager-arg=--slot=<name>`). A slot is 1-63
lowercase letters, digits, `-` or `_`, so it is both a Google and a
Kubernetes label value. With a slot, every kind's model is wrapped
(`pkg/controller/controllers.go`, `AddToManager`); the reconciler is
unchanged:

- The object gets the label `hfl.dev/slot=<slot>`.
- A kind whose resource has labels gets the label `hfl-dev-slot=<slot>`
  in the labels it asks for, so create writes it and an adoption adds it
  (Google label keys cannot hold `.` or `/`). Where the labels come from
  is the kind's (`kind.labels`): `metadata.labels`, or `spec.labels` when
  the CRD has it; `TestSlotLabelSources` pins the list.
- `Find` (a read) reads the resource's `hfl-dev-slot`. If it names another
  slot, nothing is sent: an update sets `Ready` False with reason
  `SlotMismatch` and a Warning event `SlotMismatch` ("cloud resource is
  held by slot <x>; this controller is slot <y>") and is retried with the
  workqueue's backoff; a delete is refused, which the reconciler reports
  as `Ready` False reason `DeleteFailed` with message `SlotMismatch: ...`
  (upstream's handler names the reason), with the same event. A resource
  with no slot label is claimed, and so is one held by the slot the
  object's `hfl.dev/slot-takeover` annotation names.

The move, as `task test:slot-move` runs it (`pkg/test/slotmove`, two
managers on one cluster and one fake cloud, each watching its own
namespace):

1. Old slot: `cnrm.cloud.google.com/actuation-mode: Paused`.
2. New slot: apply the object with `hfl.dev/slot-takeover: <old slot>`.
   It finds the resource by name and updates only the slot label: same
   name, same creation time, nothing else changed. (Without the
   annotation it is refused with `SlotMismatch`.)
3. Old slot: resumed, it is refused (`SlotMismatch`, no write), and a
   delete without abandon is refused too.
4. Old slot: `deletion-policy: abandon` (and resume) and delete: the
   resource stays, held by the new slot.

**Kinds the guard cannot mark** (their resources have no labels): every
kind not in `slotLabelled` (`pkg/controller/slotlabels_test.go`), that is
all but `SecretManagerSecret`, `ArtifactRegistryRepository`,
`DNSResponsePolicy`, `RunService`, `ComputeVPNTunnel`, the three
`NetworkSecurity*` kinds, the two `GKEHub*` kinds, the two `NetworkServicesAgent*` kinds and three `NetworkConnectivity*` kinds (the PSC authorization policy, the automated DNS record and the spoke gateway route), the four `GKEBackup*` kinds, `CertificateManagerCertificateIssuanceConfig`, `BackupDRManagementServer`, `NetworkSecurityDNSThreatDetector`, `NetworkSecurityAuthzPolicy`, the two `APIGateway*` kinds, the three `CloudDeploy*` kinds, `CloudFunctions2Function`, the five `Dataplex*` kinds, `AIPlatformReasoningEngine`, `SpannerInstanceConfig` and `DNSOutboundEndpoint`. These move the same
way, but nothing in the cloud stops a resumed old slot, so Bundt must
keep the old slot's object paused (or abandoned) for the whole move, as
the `LoggingSavedQuery` move does. `StorageBucketObject` could carry the
mark in its custom metadata and the compute kinds that have
`labelFingerprint` in theirs; neither is built.

## 3. Keeping in sync with upstream

### 3.1 Pin, patches, record

- `UPSTREAM.md` holds the pinned commit (one row the scripts read), the
  nearest release, and a table of every copied file with its source path
  and what changed. Each copied file also says so in its header.
- `hack/upstream.sh` fetches exactly that commit (shallow) into
  `third_party/k8s-config-connector` and applies
  `patches/k8s-config-connector/*.patch` in order;
  `hack/upstream.sh --check` fails unless the checkout is the pin plus
  exactly those patches.
- Every patch says whether it should go upstream.

### 3.2 Moving the pin

The procedure is in `UPSTREAM.md`. Cadence: when a gap closes upstream,
when a copied file changes upstream, or before a release, at least
monthly while upstream ships every two to three weeks. A release names
the Config Connector release it is installed beside, and pins a `master`
commit at or after that release.

### 3.3 Generating CRDs and code

- `hack/generate.sh` is Config Connector's `dev/tasks/generate-crds` for
  our tree: controller-gen v0.16.5 (`object` and `crd`), then Config
  Connector's own `scripts/add-validation-to-crds` and `scripts/crd-tools`
  run from the pinned checkout. `task check` fails if its output differs
  from what is committed.
- For proto-backed kinds, run Config Connector's `controllerbuilder`
  (`generate-types`, `generate-mapper`) from the checkout against the
  googleapis commit in its `apis/git.versions`, with `--api-version
  <service>.gcp.hfl.dev/v1beta1`, then hand-edit types and controller as
  upstream does. Not exercised yet.

### 3.4 Retiring a gap kind

When a Config Connector release gains the kind or field: copy nothing
new; mark our kind deprecated; document the migration (re-create in their
group with acquisition by `resourceID`, then delete ours with
`deletion-policy: abandon`); after a support window, remove the kind
here.

## 4. Testing with a fake cloud

- **Fake cloud: mockgcp, from the pinned checkout.** mockgcp is Config
  Connector's in-process fake of GCP's APIs (gRPC services behind
  grpc-gateway, in-memory storage, HTTP served through an
  `http.RoundTripper`). We import it as a module from the checkout, so it
  moves with the pin.
- **It lacked database roles.** Its Cloud SQL `User` proto comes from
  googleapis `731d7f2a` (mockgcp's pin), which predates `database_roles`.
  Patch 0001 adds `User.database_roles` and the update request's
  `database_roles` / `revoke_existing_roles` (googleapis `master` field
  numbers), regenerated with mockgcp's own toolchain (protoc 3.12.4,
  protoc-gen-go v1.28.1, protoc-gen-go-grpc v1.2.0, grpc-gateway v2.11.3)
  by `hack/regen-mocksql-users.sh`; and makes `users.update` apply roles
  as the API documents (body roles ignored; request roles added, or
  replacing the existing ones with `revoke_existing_roles`). With
  `ONLY_UPSTREAM=1` the script reproduces upstream's committed generated
  files byte for byte, which is how we know the patch is only our fields.
- **Patch 0002**: mockgcp listens on `127.0.0.2`, which macOS does not
  have; `MOCKGCP_LISTEN_ADDRESS` overrides it, and the Taskfile sets it.
- **The test** (`pkg/controller/direct/sql/sqluser_mockgcp_test.go`) runs
  a real kube-apiserver and etcd (envtest, Kubernetes 1.33), our manager
  code (Config Connector's direct reconciler and deletion defender with
  our adapter), and mockgcp as Cloud SQL. It plays a common case: an
  application's service account as a `CLOUD_IAM_SERVICE_ACCOUNT` user with
  `cloudsqlsuperuser`, then adds a role, removes one, and deletes, reading
  the fake cloud after each step through the real `sqladmin` client.
- **Results** (2026-11-07, `task check`, macOS arm64, Go 1.27.1):

  ```text
  ok: third_party/k8s-config-connector is 672414b8191fc2d2c12fff53f6faccf5ae291eef plus 2 patch(es)
  ok: go.mod carries every replace pin of the pinned Config Connector
  --- PASS: TestSQLUserDatabaseRoles (6.01s)
      --- PASS: TestSQLUserDatabaseRoles/create_grants_the_roles (0.21s)
      --- PASS: TestSQLUserDatabaseRoles/adding_a_role_grants_it (0.22s)
      --- PASS: TestSQLUserDatabaseRoles/removing_a_role_revokes_it (0.22s)
      --- PASS: TestSQLUserDatabaseRoles/delete_removes_the_user (0.22s)
  ok  github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/sql
  ```

  The test was also run with `RevokeExistingRoles(false)` in the
  controller and failed as it should ("cloud roles = [cloudsqlsuperuser
  pg_read_all_data], want [pg_read_all_data]").
- **Live runs** (`docs/live-runs.md`). The same tests run against a real
  project with `E2E_GCP_TARGET=real` (`task test-live`, on an offload VM as
  a least-privilege service account, never from `task check`). The ledger
  there records each run, which kinds have passed live, and what differed
  from mockgcp. Found live on 2026-11-09:
  - `users.get` does return `databaseRoles`, and inserts, adds and
    revokes (`revokeExistingRoles`) behave as patch 0001 fakes them;
  - a deleted Cloud SQL backup run is still returned, with status
    `DELETED`: `SQLAdminBackup` now takes that as gone, and patch 0015
    fakes it;
  - Artifact Registry refuses a remote upstream it cannot reach unless
    `disableUpstreamValidation` is set, and refuses a change of a remote
    repository's source ("Remote source configuartion cannot be
    updated"), which patch 0016 fakes.
- **Next** (section 7): port Config Connector's own `SQLUser` fixtures
  (`pkg/test/resourcefixture/testdata/basic/sql/v1beta1/sqluser/`) to our
  group and run them through its golden harness, which records
  `_http.log`.

## 5. The first resource: SQLUser

| | |
| --- | --- |
| Group, version, kind | `sql.gcp.hfl.dev/v1beta1` `SQLUser`, short names `hflsqluser`, `hflsqlusers` (not Config Connector's `gcpsqluser`, so `kubectl` is never ambiguous), categories `gcp`, `hfl` |
| CRD | `config/crds/sql.gcp.hfl.dev_sqlusers.yaml`, labelled `gcp.hfl.dev/gap: "true"`, annotated `gcp.hfl.dev/upstream-kind: sql.cnrm.cloud.google.com/SQLUser`; no `cnrm.cloud.google.com/*` labels, so Config Connector never claims it |
| Identity | `projects/{project}/instances/{instance}/users/{name}`; name from `spec.resourceID`, else `metadata.name`; project from `instanceRef` (external, or the referenced `SQLInstance` and its project) |
| Create | `users.insert` with name, host, type, password (from `spec.password` value or Secret), password policy, `databaseRoles` |
| Update | type immutable (error); password sent when `metadata.generation` is not yet observed; password policy when it differs; roles, with `revokeExistingRoles`, when `spec.databaseRoles` differs from `status.appliedDatabaseRoles` |
| Delete | `users.delete`, unless `deletion-policy: abandon` |
| Status | `conditions`, `observedGeneration`, `sqlServerUserDetails` (as upstream), `appliedDatabaseRoles` (new) |

Decisions (section 8):

- `databaseRoles` is the whole set of the user's roles, declaratively: a
  role removed from the list is revoked (`revokeExistingRoles=true`, as
  Terraform does). Out-of-band grants are
  not detected while the list is unchanged.
- What was applied is kept in `status.appliedDatabaseRoles`, because the
  API is not relied on to return a user's roles.
- A password from a Secret is re-sent when the spec generation changes,
  not when only the Secret changes (Config Connector tracks Secret
  versions with an annotation; not done here).

## 6. Choosing gaps

A gap becomes a kind here only when a real workload needs it and no
existing Config Connector kind covers it. Counting missing fields does not
rank them: Config Connector's existing kinds lack thousands of fields that
Terraform or the API have, and most of them nobody is blocked on.

### 6.1 Upstream first

- **Code on Config Connector `master`, not released.** If the next release
  is near, no kind of ours is needed: install beside that release.
- **An open upstream pull request.** Copy its code under the copy-upstream
  rule, and watch the pull request (`hack/upstream-pr.sh`); when it merges
  and a release ships it, retire ours (section 3.4).
- **A field missing on a large existing kind** (such as
  `SQLInstance.retainBackupsOnDelete`): wait for the Config Connector
  release that carries it. A gap kind for an existing kind, carrying the
  field, is built only when a workload cannot work without the field and
  the release is not expected soon enough. It lives in our group, as
  `SQLUser` does, and retires when the release lands. Config Connector's
  own manager is never rebuilt with a patch.
- **In the Google API, in no Config Connector code or pull request.**
  Written here from the API and Terraform's behaviour, read not copied.

### 6.2 What "missing" means

Comparing Config Connector's CRDs with Terraform's resources and Google's
Discovery documents overcounts gaps:

- Terraform attributes are best read from the provider's schema, not its
  docs; hand-written docs parse unreliably.
- Discovery documents lack field behaviours, so output-only is a guess and
  required or immutable are unknown.
- Only APIs vendored in google-api-go-client are visible.
- Terraform resources and API types are not one-to-one (IAM add-ons,
  singletons, virtual resources); many IAM add-ons are covered by Config
  Connector's generic `IAMPolicyMember`.
- Config Connector's Terraform kinds run a pinned fork that lags upstream
  Terraform, so "missing" mixes "never modelled" with "not yet
  re-vendored".
- Config Connector sometimes names a field differently from Terraform, so
  a name match is only a heuristic.

### 6.3 Candidates (read 2026-11-07)

From the open pull requests on Config Connector (read-only) and the pinned
checkout (`672414b8`). `StorageManagedFolder` and `ArtifactRegistryRule`
already have direct controllers on master: no gap kind, install beside
the release that ships them.

Upstream code exists, not released:

| Rank | Candidate | Upstream | Why | Done |
| --- | --- | --- | --- | --- |
| 1 | `SQLAdminBackup`, an on-demand Cloud SQL backup | types on master; controller and mockgcp in PR #12553 | a backup before a change that can lose data (a major upgrade, a destructive migration) | built |
| 2 | `SecretManagerSecret.spec.location`, regional secrets and their versions | PR #6651 (and its duplicate #5730), on the direct controller | secrets kept in one region (data residency) | built |
| 3 | `SQLUser` on the direct reconciler | PR #7148 | ours exists; switch when `databaseRoles` is live (section 2) | watched |
| 4 | `SQLInstance`: `retainBackupsOnDelete` (#14854), `diskEncryptionConfiguration` (#7227), PSC auto-connections (#14419), performance capture (#14431) | PRs on a large direct kind | deleting an instance must not take its backups | wait for the release (6.1) |
| 5 | `ContainerNodePool` `autoscaling.enabled` (#14182), kubelet sysctls (#14187) | PRs | clusters made from manifests | wait for the release |
| 6 | `StorageBucket` `rpo` (#13713), encryption enforcement (#15145), direct controller (#11533) | PRs; the kind is Terraform-based on master | buckets | wait for the release |
| 7 | `ArtifactRegistryRepository` custom remote repositories (#13517) | PR | pulling through a private upstream | built |
| 8 | `RunService` `invokerIamDisabled` (#14417), GPUs (#7232) | PRs, Terraform only; no direct `RunService` upstream | Cloud Run services | built, as a narrow direct `RunService` (decision 11) |
| 9 | `IAMWorkloadIdentityPoolNamespace`, `...ManagedIdentity` (#14209, #14418), pool `mode` (#14227) | types only, no controller, no mockgcp | workload identity federation | namespace and managed identity built; `mode` needs the DCL fork |

In the Google API, in no Config Connector code or pull request
(unverified against the live API), in the order decision 10 takes them:

| Candidate | Done |
| --- | --- |
| Artifact Registry package tags | built, `ArtifactRegistryTag` (2026-11-09) |
| a remote repository's `upstreamCredentials` | built (decision 8) |
| Workload Identity pool provider keys | built, `IAMWorkloadIdentityPoolProviderKey` (2026-11-09) |
| Cloud SQL `blueGreenDeployments` | skipped: not in the API. Neither `sqladmin/v1` nor `sqladmin/v1beta4` in google-api-go-client v0.287.1 (the pin's) has it; the entry was wrong |

Two entries were dropped from this list on 2026-11-09: Artifact Registry
repository attachments have a direct controller on `master`
(`ArtifactRegistryAttachment`, not in `v1.160.0`; install beside the
release that ships it), and Storage HMAC keys are Config Connector's
`StorageHMACKey`, already in `v1.160.0`.

Rechecked 2026-11-08 (read-only, `gh` and `git fetch` of `master` at
`c8208a77`), before building rank 7. No candidate above merged or was
released: the latest release is still `v1.160.0` (2026-10-29), and every
pull request in the table is open with review required or changes
requested, none approved. Ranks 4 to 6 stay "wait for the release" (6.1).
Rank 7, PR #13517 (head `eb2774ce`, opened 2026-09-29 by upstream's
coding bot), has no human review (only bots fixing its CI), and was
paused by upstream's bot for inactivity on 2026-10-13: not close to
merging. `master`'s Artifact Registry code is unchanged since our pin.

Rechecked 2026-11-09 (read-only, `master` at `7822e4bd`), for ranks 8
and 9 after rank 7 was built. Still no release after `v1.160.0`; none of
their pull requests merged or was approved. Neither can be built by
copying upstream code, so both waited on a question, since answered
(decisions 9 and 10):

- Rank 8: `RunService` is Terraform-only on `master` (`static_config.go`
  lists Terraform as its only controller), and #14417 (opened 2026-10-25,
  review required, no reviews) and #7232 (opened 2026-03-27, review
  required) change only the Terraform fork and its generated types.
  Upstream has no direct `RunService` types or controller, and no pull
  request for one; `apis/run/v1beta1` has only its identity and reference.
- Rank 9: #14209 (changes requested) and #14418 (review required) add
  v1alpha1 types and identities only, no controller. mockgcp serves no
  workload identity pools at all (the pool is DCL-based), and #14227
  (`mode`, changes requested) changes the DCL fork, which we do not run.

### 6.4 Notes on what was built

What was built is described in "What is copied" in `UPSTREAM.md`, and:

- `SQLAdminBackup` uses the sqladmin v1beta4 REST client instead of the
  PR's `cloud.google.com/go/sql`, which is not in the pin's `go.mod`:
  adding it moved about 65 shared modules (gRPC, genproto, otel and the
  `cloud.google.com/go/*` clients) off the versions Config Connector is
  built and tested with. Patch 0003 is the PR's mockgcp.
- The Secret Manager kinds are master's direct controllers with the PR's
  change. Two fixes of ours: an unset `spec.enabled` on a version means
  `true` (upstream dereferences it unchecked; there the direct controller
  is opt-in behind Terraform's, which defaults it), and the version's
  `secretRef` is typed without its alias (controller-gen v0.16.5 panics on
  aliases under Go 1.27). Patch 0004 is the PR's mockgcp plus versions
  under a regional secret, which the PR's mock never names. The PR's mock
  serves only `us-central1`'s regional endpoint; other regions need their
  hosts added.
- Tests (`task test`, macOS arm64, Go 1.27.1, 2026-11-07):

  ```text
  --- PASS: TestSQLAdminBackup (10.10s)
      create takes an on-demand backup; a resync takes no second backup;
      a backup cannot be changed; delete removes the backup; abandon keeps it
  --- PASS: TestRegionalSecret (7.20s)
      created in its region (every request at the regional endpoint);
      a version holds the Kubernetes Secret's value; cannot move region;
      delete removes the version and the secret
  --- PASS: TestGlobalSecretUnchanged (5.86s)
  ```

  The regional-endpoint check was confirmed to fail with the endpoint
  removed from the controller.
- `ArtifactRegistryRepository` is master's direct controller (on `master`
  it is opt-in; Terraform is the default there) with PR #13517's change:
  `remoteRepositoryConfig.{docker,maven,npm,python}Repository.customRepository.uri`,
  a remote repository's upstream at a URI of the user's choosing instead
  of one of the public upstreams Artifact Registry names. The PR changes
  only the types, the mapper and the fuzzer for the direct controller (and
  Terraform's fork, which we do not use); the controller itself is
  unchanged. mockgcp already stores the field (it keeps the client
  library's protos as sent), so no mockgcp patch is needed. The CRD has
  exactly the PR's property paths.
  - A virtual repository's `upstreamPolicies[].repositoryRef.name` names a
    repository of ours, as the Secret Manager version's `secretRef` does;
    a Config Connector repository is named by `repositoryRef.external`.
  - What the PR does not map, ours does not either: Apt and Yum remote
    repositories. `upstreamCredentials` and `disableUpstreamValidation`
    were added later, written here (below).
  - Changing `customRepository.uri` on an existing repository: Terraform
    marks it ForceNew; Config Connector's direct controller sends
    `remoteRepositoryConfig` in an update mask, which mockgcp refuses
    ("not supported by mockgcp"), so the object goes not Ready and the
    fake cloud keeps the old URI. What the real API does is not known.
    Since 2026-11-16 (decision 21, ARURI) ours deletes the repository
    and makes it again with the same name, as Terraform does; the API
    refuses the change in place (section 7 item 14).
  - A resync (Config Connector's force-reconcile annotation) reads the
    repository back and sends no update, which shows the mapper
    round-trips `customRepository`: a dropped field would be a diff.
- Tests (`go test ./pkg/controller/direct/artifactregistry/`, envtest
  Kubernetes 1.33, macOS arm64, Go 1.27.1, 2026-11-08):

  ```text
  --- PASS: TestArtifactRegistryRepositoryFuzz (1.45s)
      1000 seeds through the spec and status mappers, customRepository included
  --- PASS: TestArtifactRegistryRemoteRepository (39.69s)
      create sets the custom upstream; a resync sends no update; a
      description change keeps the custom upstream; a changed upstream URI
      is not applied in the fake cloud; delete removes the repository;
      abandon keeps the repository
  --- PASS: TestArtifactRegistryCustomUpstreamFormats (18.49s)
      MAVEN, NPM, PYTHON
  ```

  With the mapper's read-back of the Docker `customRepository` removed,
  the fuzz test and "a resync sends no update" fail (the controller sends
  an update that mockgcp refuses), so the round trip is what they check.
- `task check` and `task operator-e2e` on a Linux amd64 VM (16 CPUs, Go
  1.27.3, kind 0.27.0, ko 0.19.1, kustomize 5.7.1), 2026-11-09: both pass.
  Every package's tests pass, the artifactregistry package's included; the
  operator installs `artifactregistryrepositories.artifactregistry.gcp.hfl.dev`
  with the other gap CRDs and its managers' roles cover the new group. The
  operator test applies no Artifact Registry object; the kind's behaviour
  is proven by the envtest test above. `tests/operator/run.sh` needs
  `kustomize` on the path.
- `upstreamCredentials` and `disableUpstreamValidation` on
  `ArtifactRegistryRepository` (decision 8) are **written here**, from the
  Artifact Registry API and Terraform's `google_artifact_registry_repository`
  (provider `main` at `42eb358f`, read, not copied). No Config Connector
  code or pull request has them.
  - `remoteRepositoryConfig.upstreamCredentials.usernamePasswordCredentials`
    takes `username` and `passwordSecretVersionRef`, Config Connector's
    `refs/v1beta1.SecretManagerSecretVersionRef` (as `CloudBuildConnection`
    and `DataformRepository` use it): a Config Connector
    `SecretManagerSecretVersion` by name, resolved to
    `projects/{p}/secrets/{s}/versions/{v}`, or that string as `external`.
  - Credentials change in place, as Terraform does: when only they differ,
    the update mask names
    `remoteRepositoryConfig.upstreamCredentials.usernamePasswordCredentials.{username,passwordSecretVersion}`
    instead of the whole `remoteRepositoryConfig`.
  - `disableUpstreamValidation` is input only (the API does not return
    it), so it is copied from the spec onto what was read before
    comparing, and never causes an update by itself. It is sent with
    every create and with any update of the remote config.
  - Patch 0005: mockgcp accepts those two mask paths and does not store
    `disableUpstreamValidation`, as the API documents.
  - Written-here code is marked "Written here" in the copied files and
    lives in `upstreamcredentials_types.go` and
    `upstreamcredentials_mapping.go`. `pkg/test/mockenv` now installs
    Config Connector's two Secret Manager CRDs, so a reference by name
    resolves in tests.
- `IAMWorkloadIdentityPoolNamespace` (`iam.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; decision
  9). Rechecked 2026-11-09: PR #14209 (head `a5e8f5a9`) is open; changes
  were requested on its CI files, not on the types. It is the first kind
  here whose controller is **written here** rather than copied: upstream
  has only the types.
  - Copied from the PR: the types, identity and reference, and the
    mockgcp proto. Written here: `spec.workloadIdentityPoolRef` (the PR's
    identity leaves the pool empty and always fails), the direct
    controller on `google.golang.org/api/iam/v1`, and the mockgcp service.
  - `workloadIdentityPoolRef` names Config Connector's `IAMWorkloadIdentityPool`
    (DCL-based, no reference type upstream) by name, or a pool as
    `external`. A pool in another project or location than the namespace's
    `projectRef` and `location` is refused.
  - Behaviour follows Terraform's
    `google_iam_workload_identity_pool_namespace` (read, not copied):
    `description` and `disabled` change in place (`updateMask`); a namespace
    in state `DELETED` (soft-deleted, purged after about 30 days) is absent;
    its ID cannot be reused until purged, so re-creating it fails with the
    API's error, as in Terraform. Changes are long-running operations, polled
    until done.
  - Patch 0006: mockgcp gains the namespace service, with soft delete. Its
    Go is generated by `hack/regen-mockiam-workload-identity.sh` with mockgcp's
    toolchain and reproduces byte for byte.
  - `pkg/test/mockenv` also installs Config Connector's `IAMWorkloadIdentityPool`
    CRD, so the pool resolves by name in tests.
- `IAMWorkloadIdentityPoolManagedIdentity` (`iam.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`;
  decision 9). Rechecked 2026-11-09: PR #14418 (head `b080f5dc`) is open,
  review required, no reviews; types only, like #14209.
  - Copied from the PR: the types, identity and reference, and the
    mockgcp proto. Written here: `spec.workloadIdentityPoolNamespaceRef`
    (our `IAMWorkloadIdentityPoolNamespace` by name, once it has a
    `status.externalRef`, or a namespace as `external`; one in another
    project or location is refused), `spec.attestationRules`, the
    controller, and the mockgcp service.
  - `attestationRules` follow Terraform's `attestation_rules` (read, not
    copied): set with `setAttestationRules` after the identity is made and
    whenever they differ, read with `listAttestationRules`, the list being
    the whole set. Unset leaves the rules alone; an empty list removes them
    all. The PR leaves them out, but a managed identity without them can be
    given to no workload.
  - Patch 0007, on top of 0006: the managed identity service and its
    attestation rules. The PR's proto has no attestation-rule methods; they
    are added from the IAM v1 API, and the proto's duplicate operation
    metadata is dropped.
- `RunService` (`run.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; decisions 10 and 11). Rechecked
  2026-11-09: PR #14417 (`invokerIamDisabled`, head `9297537a`) and PR
  #7232 (GPUs) are open, review required, and change only Config
  Connector's Terraform fork; no direct `RunService` exists upstream.
  - Copied from Config Connector's direct `RunWorkerPool` (master): the
    types (template, containers, env, volumes, VPC access, node selector),
    identity, reference, mappers, reference resolution and controller,
    renamed for a service, without instance splits.
  - Written here, from the Cloud Run v2 API: the service's `ingress`,
    `invokerIamDisabled`, `defaultUriDisabled` and `scaling`; the
    template's `scaling`, `timeout`, `maxInstanceRequestConcurrency`,
    `executionEnvironment`, `sessionAffinity`, `healthCheckDisabled` and
    `gpuZonalRedundancyDisabled`; the URIs in status; and the server
    defaults a service has that a worker pool does not (ingress, timeout,
    concurrency, execution environment, template scaling), taken from what
    was read when the spec leaves them unset.
  - Not modelled: `traffic` (a service sends all traffic to its latest
    revision), IAP, multi-region settings, the build config.
  - Secret references are typed `SecretManagerSecretRef` and
    `SecretManagerSecretVersionRef`, not their aliases (controller-gen
    v0.16.5 panics on aliases, as for the Secret Manager kinds).
  - Patch 0008: mockgcp's `services.patch` honours an update mask (exact
    top-level fields), and keeps `invokerIamDisabled` without one (PR
    #14417's line).
  - The managers' role also reads the `compute`, `vpcaccess`, `storage`
    and `networkservices` Config Connector groups the template references.
- `ArtifactRegistryTag` (`artifactregistry.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; decision
  10, the first unranked candidate). **Written here** in full: no Config
  Connector code or pull request, and no Terraform resource, has package
  tags; the kind follows the Artifact Registry API (`tags.create`, `get`,
  `patch` with update mask `version`, `delete`) on the client the
  repository controller already uses.
  - `spec.repositoryRef` is Config Connector's `ArtifactRegistryRepositoryRef`
    (a Config Connector repository by name, or `external`), `spec.package`
    the package ID (a "/" in it is sent as `%2F`, as resource names encode
    it), `spec.resourceID` the tag, `spec.version` the version's ID (for a
    Docker image its digest). Moving the tag is changing `version`.
  - Patch 0009: mockgcp serves tags in an existing repository; it holds no
    packages, so neither the package nor the version is checked.
  - The managers' role also reads `artifactregistry.cnrm.cloud.google.com`;
    `pkg/test/mockenv` installs Config Connector's repository CRD.
- `IAMWorkloadIdentityPoolProviderKey` (`iam.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; decision
  10, the last unranked candidate). **Written here** in full: no Config
  Connector code or pull request, and no Terraform resource (Terraform has
  only `google_iam_workforce_pool_provider_key`, the workforce analogue),
  has workload identity pool provider keys.
  - `spec.workloadIdentityPoolProviderRef` names Config Connector's
    (DCL-based) `IAMWorkloadIdentityPoolProvider` by name, reading its pool
    from its own `workloadIdentityPoolRef`, or a provider as `external`;
    `spec.use` (`ENCRYPTION`) and `spec.keyData.keySpec` (`RSA_2048`,
    `RSA_3072`, `RSA_4096`). Every field is immutable, as in Terraform's
    workforce analogue: a change leaves the object not Ready with the
    field named, and is not applied.
  - `status.observedState.keyData` carries the public key (an X.509 PEM)
    and its validity window, to configure in the identity provider.
  - A deleted key is soft-deleted (state `DELETED`) and counts as absent,
    as for namespaces.
  - Patch 0010: mockgcp serves provider keys. The proto is written here
    from the API's discovery document (googleapis has none), generated by
    the same script, and the mock returns a placeholder PEM, not a
    certificate.
  - With this, every ranked and unranked candidate in section 6.3 is built,
    watched, waiting for a release, or skipped with its reason.
- `AssetFeed` (`asset.gcp.hfl.dev/v1beta1`, 2026-11-15, the builder's
  RECPF answer, under decision 19): a Cloud Asset feed of an organization,
  folder or project whose parent comparison treats a project's number and
  its ID as one project. Config Connector's direct `AssetFeed` (`v1beta1`)
  compares the name Google Cloud gives a project's feed,
  `projects/<number>/feeds/<id>`, with the project ID its `projectRef`
  resolves to on every reconcile after the create, so the feed is never
  updated and cannot be deleted (catalog PR #215, fake-cloud runs
  `off-catalog-1115103624-5909` and `off-catalog-1115110334-29801`); this
  repository cannot patch the controller Config Connector runs (section 10,
  question 7). Upstream, checked 2026-11-15 (read-only): the same
  comparison at the pin and on `master`, no open pull request. **Copied**:
  the types (with `FeedOutputConfig` and `Expr`), identity, controller and
  mapper. **Changed**: when the project segments differ, the identity reads
  the ID's number with Resource Manager's `projects.get` and compares, and
  is then the recorded name; a delete names the feed from
  `status.externalRef` without resolving the parent (gone first when
  everything is deleted at once, the catalog's second failure); the update
  compares with the name the API returned; exactly one parent, the parent
  and `resourceID` immutable, by CEL. `v1beta1` as `NetworkServicesGateway`
  (decisions 19 and 26). A unit test runs upstream's own identity and fails
  when upstream fixes it (then this kind retires, section 3.4). mockgcp
  serves feeds unpatched. Live: pending, needs `roles/cloudasset.owner` for
  `cce-live-tests` (`E2E_CLOUD_ASSET=true`). `docs/kinds/AssetFeed.md` has
  the rest.
- `LoggingLogSink` (`logging.gcp.hfl.dev/v1beta1`; built 2026-11-15 on the
  builder's answer to section 10 question 7, CCEQ: "our own
  LoggingLogSink (v1beta1)"; decisions 19 and 26). Our own kind for the
  whole sink of a project, folder or organization, so that the catalog's
  folder and organization charts can render an intercepting sink
  (Terraform's `intercept_children`, which fabric allows on a
  `project`-type sink only). Upstream, rechecked 2026-11-15 (read-only):
  Config Connector's `LoggingLogSink` is `v1beta1` (Terraform-based by
  default, a direct controller opt-in) with neither `interceptChildren` nor
  a Cloud project destination, at the pin, in v1.160.0 and on `master`
  (`2374254`, 2026-11-15); no pull request adds them (searched
  "interceptChildren", "LoggingLogSink": #14429 resolves the project
  annotation, #11575 adds export). **Copied** from upstream's direct
  controller with as little change as possible, after
  `LoggingBillingAccountSink`: the types, identity, controller and the
  sink's mapper functions. Changed:
  - `spec.interceptChildren`, and a Cloud project as a destination
    (`destination.projectRef`, sent as
    `logging.googleapis.com/projects/{id}`), which an intercepting sink
    needs; CEL rules carry the API's rules for it (a folder or organization
    parent, `includeChildren` true, a project destination), refuse more
    or fewer than one parent and one destination (upstream falls back to
    the namespace's project and takes the first destination set), and keep
    the parents, `resourceID`, `includeChildren` and `interceptChildren`
    immutable (Terraform's `include_children` is `ForceNew`).
  - The Logging REST client (`google.golang.org/api/logging/v2`) in place
    of upstream's protobuf client: the `LogSink` protobuf at the pinned
    `cloud.google.com/go/logging` v1.18.0 has no `interceptChildren`, so
    the field cannot be sent through it; the update mask is built by
    comparing the same top-level fields upstream's
    `DiffForTopLevelFields` does, BigQuery options only when the spec sets
    them (the API fills them in for a BigQuery destination).
  - `uniqueWriterIdentity` is sent true for a folder's or organization's
    sink (the API refuses false there; Terraform's folder and organization
    sinks always send true) and for a sink whose writer identity is
    already unique (the API refuses going back to the shared one); a
    project's sink otherwise sends the spec's, as upstream does.
  - `status.externalRef`, checked by the identity; a delete uses it and
    resolves neither the parent nor the destination again, so a folder
    deleted together with its sinks does not hold them (the rule our
    generated kinds and `IAMDenyPolicy` follow).
  Generated: the CRD and deepcopy (controller-gen); `hack/scaffold-kind.sh`
  was not run (upstream's controller is the nearer code), nor
  `hack/discokind` (a sink's create takes the name in the body and
  `uniqueWriterIdentity` as a parameter). Mockgcp patch 0049 keeps
  `interceptChildren` beside each sink on the HTTP layer
  (`docs/fake-cloud.md`). Proven on the fake: a project's sink (create, a
  resync with no write, one update of `disabled` and `exclusions`, the CRD
  refusing a parent change, no or two parents, two destinations and a
  project's intercepting sink, delete, abandon), a folder's intercepting
  sink to a project (and the CRD refusing to change `interceptChildren`,
  or one without `includeChildren` or a project destination), and an
  organization's sink to a bucket. Live: `docs/kinds/LoggingLogSink.md`.
  The writer identity is granted with our `IAMMember`'s
  `memberFrom.logSinkRef` (2026-11-15, section 6.5 at `IAMMember`),
  since Config Connector's `IAMPolicyMember.memberFrom.logSinkRef`
  resolves Config Connector's `LoggingLogSink` only.
  Left: the catalog's folder and organization charts move their sinks to
  this kind (and drop their refusals of `intercept_children` and type
  `project`), granting with `IAMMember`; the kind retires when
  upstream ships `interceptChildren` and a project destination (section
  3.4).

### 6.5 From the seed catalog (read 2026-11-09)

The catalog agent's inventory of Cloud Foundation Fabric v60.0.0 against
Config Connector v1.160.0 (`highfivelabs/catalog`, `docs/gcp-inventory.md`
at `a5a451a`, "Gaps for config-connector-extensions") ranks what fabric
modules need and no Config Connector kind covers, by how many modules need
it. Rechecked here read-only on 2026-11-09 against Config Connector
`v1.160.0`, `master` at `b795cda8` and its open pull requests; rows follow
this repository's rules (6.1, decision 5, section 10).

| Rank | Gap | Modules | Upstream (2026-11-09) | Here |
| ---: | --- | ---: | --- | --- |
| 1 | Role-authoritative IAM (`google_*_iam_binding`): the members of one role on one resource, removing any others | 43 | none. PR #14133 (open, review required) adds an `IAMPolicyBinding` that is IAM v3's `PolicyBinding` (principal access boundary), a different resource with the name the catalog suggests | built (2026-11-09), as `IAMRoleBinding`, now `IAMBinding` (decision 12) |
| 2 | IAM on the 32 resource types `IAMPolicyMember` cannot reference | 24 | none | built (2026-11-09), as `IAMRoleMember`, now `IAMMember` (decision 12), over the same resource table as `IAMBinding`, which accepts the same 32 resource types |
| 3 | `ComputeRegionNetworkEndpoint` | 9 | no kind or PR (the global and zonal endpoint kinds are Terraform-based alphas) | built (2026-11-09) |
| 4 | `ComputeRouterRoutePolicy` | 5 | no kind or PR | built (2026-11-09) |
| 5 | `StorageBucketObject` | 4 | no kind or PR | built (2026-11-09) |
| 6 | Regional network firewall policies: the policy, its rules, its association | 2 | no kind or PR (the global `ComputeNetworkFirewallPolicy` is Terraform-based by default, with an opt-in direct controller; its rule and association kinds are Terraform-only) | the policy, its rules and its association built (2026-11-09) |
| 7 | Hierarchy settings: folder and organization service identities, logging settings, a billing account's log sink | 3 | `ServiceIdentity` exists (Terraform-based) for projects only; `LoggingLogSink` takes project, folder and organization parents, not a billing account; no `LoggingSettings` | `LoggingSettings` built (2026-11-09; Config Connector's PR #3730, closed, was a scaffold with no fields); folder and organization `ServiceIdentity` built (2026-11-09, `docs/coverage/foundation.md` item 1); the rest are field gaps on existing kinds: wait for the release (decision 5) |
| 8 | Values known only after apply, as references (`TagsTagBinding.parentRef` to an `IAMServiceAccount`, a project's service agents in `memberFrom`, `StorageBucket.encryption.kmsKeyHandleRef`) | 3+ | none found | field gaps on existing kinds: wait for the release (decision 5) |
| 9 | Field gaps on foundational kinds: `StorageBucket` `rpo`, `hierarchicalNamespace`, `enableObjectRetention`; `ComputeNetwork` BGP settings and `status.networkId`; `ComputeSubnetwork` `ipCollection`, `allowSubnetCidrRoutesOverlap` | 4 | `rpo` is PR #13713 (open, review required); none found for the rest | wait for the release (decision 5) |
| 10 | Project settings: default service accounts, default network tier, per-key compute metadata | 1 | no kinds or PRs (`ComputeProjectMetadata`, whole-project, is Terraform-based) | per-key metadata built (2026-11-09, `ComputeProjectMetadataItem`); the default network tier built (2026-11-09, `ComputeProjectDefaultNetworkTier`); default service accounts built (2026-11-09, `ProjectDefaultServiceAccounts`) |

Notes on what was built from this table:

- `ComputeRegionNetworkEndpoint` (`compute.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; rank 3).
  **Written here**, after Config Connector's zonal `ComputeNetworkEndpoint`
  (whose types have only the group, `ipAddress` and `instanceRef`), with
  the fields Terraform's `google_compute_region_network_endpoint` has:
  `regionNetworkEndpointGroupRef` (Config Connector's
  `ComputeRegionNetworkEndpointGroup` by name, or `external`), `port`,
  `ipAddress`, `fqdn`, `clientDestinationPort`, `instance`. The controller
  follows Terraform (read, not copied): `attachNetworkEndpoints`, found by
  listing the group's endpoints and matching address and port,
  `detachNetworkEndpoints`; every field is immutable (a change leaves the
  object not Ready and attaches nothing). An endpoint has no resource name
  in the API, so `status.externalRef` is the group's name plus
  `/networkEndpoints/{address}:{port}`, ours. Patch 0011: mockgcp attaches,
  detaches and lists a regional group's endpoints, adapted from its zonal
  ones.
- `ComputeRouterRoutePolicy` (`compute.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; rank 4).
  **Written here** from the Compute Engine API's `RoutePolicy`:
  `routerRef` (a `ComputeRouter` by name, ours or Config Connector's, or
  `external`),
  `resourceID`, `type`, `description`, and `terms` (priority, a CEL
  `match`, CEL `actions`). The controller follows Terraform's
  `google_compute_router_route_policy` (read, not copied): `getRoutePolicy`
  reads it, `updateRoutePolicy` creates it and replaces it whole (with the
  fingerprint it read), `deleteRoutePolicy` deletes it; type, description
  and terms change in place. A route policy has no API resource name, so
  `status.externalRef` is the router's name plus `/routePolicies/{name}`,
  ours. Patch 0012: mockgcp serves a router's route policies with a new
  fingerprint on each write.
  `routerRef.name` finds this project's `ComputeRouter`
  (`compute.gcp.hfl.dev`, decision 14) as well as Config Connector's
  (section 7 item 28, 2026-11-10): the controller reads both kinds in the
  namespace and takes the one that exists. A name both kinds have, as
  while a router moves to ours (ours acquired, the official object not
  yet abandoned), is refused ("is both a compute.gcp.hfl.dev and a
  compute.cnrm.cloud.google.com object; name the router by
  routerRef.external") rather than picked silently. The field's shape is
  unchanged, so existing objects keep working.
- `StorageBucketObject` (`storage.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; rank 5). **Written
  here**: `bucketRef` (Config Connector's `StorageBucketRef`), `resourceID`
  (the object name; "/" allowed), the content inline or `contentFrom` a
  Secret's or ConfigMap's key, `contentType`, `cacheControl`,
  `contentDisposition`, `contentEncoding`, `contentLanguage`, `metadata`;
  status carries the generation, MD5, CRC32C, size and media link. The
  controller follows Terraform's `google_storage_bucket_object` (read, not
  copied): `objects.insert` uploads it; a changed content (by MD5) or any
  other changed field uploads it again, a new generation, since Terraform
  replaces the object for each; `objects.delete`. It is for small objects
  (a configuration file, a script): the content lives in a Kubernetes
  object, so about 1 MiB at most. A changed Secret or ConfigMap is uploaded
  at the next reconcile (`reconcile-interval-in-seconds`), not when it
  changes. Patch 0013: mockgcp's objects were stubs; it now serves media
  and multipart uploads, content reads, get and delete. The managers' role
  now reads ConfigMaps.
- `ComputeRegionNetworkFirewallPolicy` (`compute.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`;
  rank 6, the policy). **Copied** from Config Connector's opt-in direct
  `ComputeNetworkFirewallPolicy` (types, identity, controller, the status
  mapping) for a region: `spec.region`, the regional client,
  `projects/{project}/regions/{region}/firewallPolicies/{name}` as the
  identity, and Terraform's `regionNetworkFirewallPolicyId` in status.
  Written here: the spec mapping (`description`, the one field that
  changes in place), and an unset description is sent as "" so that
  removing it clears it, as Terraform's
  `google_compute_region_network_firewall_policy` does. A changed region,
  project or name leaves the object not Ready (the identity is checked
  against `status.selfLink`, as upstream does). `policyType` (VPC or RDMA)
  is not a field yet: the API has it, Terraform's vendored resource does
  not. Patch 0014: mockgcp serves regional policies, adapted from its
  global ones. The rules and the association are separate kinds, next.
  Live: passes (`docs/live-runs.md`).
- `ComputeRegionNetworkFirewallPolicyRule` (`compute.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`;
  rank 6, the rules). **Written here**: no Config Connector code has a
  regional policy's rule (`ComputeFirewallPolicyRule` is the rule of an
  organization or folder policy; the global network policy's rule is
  Terraform-only). Fields: `firewallPolicyRef`
  (`ComputeRegionNetworkFirewallPolicyRef`: a `ComputeRegionNetworkFirewallPolicy`
  by `name`, or `external`; the association kind reuses it), `priority`,
  `action`, `direction`, `match` (`layer4Configs`, source and destination
  ranges, address groups, FQDNs, region codes and threat intelligence
  lists, source secure tags), `ruleName`, `description`, `disabled`,
  `enableLogging`, `targetServiceAccounts`, `targetSecureTags`. Not here
  yet: `securityProfileGroup`, `tlsInspect` and `targetResources`
  (the API has them; Terraform's rule has them; add on demand). The
  controller follows Terraform's
  `google_compute_region_network_firewall_policy_rule` (read, not copied):
  a rule has no resource name, only its priority in the policy, so
  `getRule`, `addRule`, `patchRule` and `removeRule` all take the priority,
  and `status.externalRef` is ours,
  `projects/{p}/regions/{r}/firewallPolicies/{policy}/rules/{priority}`.
  The priority and the policy are immutable (CEL, and the identity is
  checked against `status.externalRef`). A rule already at the priority
  is taken over (Find finds it, Update brings it to the spec). `patchRule`
  is sent only when the fields the spec manages differ from what was read,
  so a resync writes nothing. What the real API does (live run, 2026-11-09)
  and the fake cannot show: a priority with no rule is answered with 400,
  "The firewall policy does not contain a rule at priority N", not 404
  (Find and Delete take that as "gone"); `patchRule` **keeps a list the
  body leaves out** (a rule patched without its `srcIpRanges` kept them),
  so `patchRule` is sent through the Compute Engine REST client (the
  generated gRPC-style client leaves an empty value out of the body) with
  a list the cloud's rule has and the spec has not in `ForceSendFields`
  (sent as `[]`), and the optional strings and booleans always sent, `""`
  and false when unset; and a rule just changed is "not ready" to the API
  for a while (a patch then answers 400 "The rule at priority N is not
  ready"), which the reconciler's retry absorbs.
  Patch 0020: mockgcp serves a regional policy's rules, adapted from the
  global ones (its `patchRule` replaces the whole rule).
  Live: passes (`docs/live-runs.md`).
- `ComputeRegionNetworkFirewallPolicyAssociation` (`compute.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`;
  rank 6, the association). **Written here**: attaches a regional policy
  to a VPC network. `firewallPolicyRef` (the
  `ComputeRegionNetworkFirewallPolicyRef` the rule kind added),
  `networkRef` (Config Connector's `ComputeNetworkRef`: a `ComputeNetwork`
  by `name` or `external`) and `resourceID` (the association's name in
  the policy; `metadata.name` by default), all immutable (CEL).
  `getAssociation`, `addAssociation` and `removeAssociation` address it by
  name; `status.externalRef` is ours,
  `projects/{p}/regions/{r}/firewallPolicies/{policy}/associations/{name}`.
  Nothing about an association changes in place, so Update only checks
  that the cloud's association is attached to the network the spec
  names: one that exists with the name but another network is an error,
  never a silent move. `replaceExistingAssociation` is not sent: a
  network that already has a policy attached is the API's error on the
  object. The real API does not return the policy's ID in an association
  (an earlier draft had a status field for it). A missing association is
  a 400 ("Invalid value for field 'name': ... An association with that
  name does not exist."), which Find and Delete take as "gone". The
  sweeper removes a policy's associations before the policy
  (`hack/live/sweep.sh`).
  Patch 0021: mockgcp serves a regional policy's associations (the global
  mock has none).
  Generated versus written: all written by hand. Config Connector's
  controllerbuilder (`hack/scaffold-kind.sh`) generates from a proto
  resource; an association is a list inside the policy's proto with no
  resource of its own.
  Live: passes (`docs/live-runs.md`).
- `LoggingSettings` (`logging.gcp.hfl.dev/v1beta1` since decision 27,
  first `v1alpha1`; rank 7). **Written
  here** from the Cloud Logging API's `Settings`: `organizationRef` or
  `folderRef` (exactly one, checked by the CRD), `kmsKeyRef` (Config
  Connector's `KMSCryptoKeyRef`), `storageLocation`, `disableDefaultSink`;
  status carries the Log Router's service accounts. The controller
  follows Terraform's `google_logging_organization_settings` and
  `google_logging_folder_settings` (read, not copied): the settings
  always exist, so create and update are both `settings.patch` with a
  mask of the fields the spec sets and the cloud does not match; a field
  left unset (or taken off the spec) is not managed; delete leaves the
  settings as they are. Project settings are read-only in the API and
  not a field here; `defaultSinkConfig` waits for mockgcp's logging
  protos to have it. Patch 0017: mockgcp serves `getSettings` and
  `updateSettings`.
  Live: pending, needs a test folder with `roles/logging.admin` and a
  KMS key (`docs/live-runs.md`).

- `ServiceIdentity` (`serviceusage.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; rank 7, and
  `docs/coverage/foundation.md` item 1). A folder's or an organization's
  service agent: `folderRef` or `organizationRef` (exactly one, checked
  by the CRD), `resourceID` (the service; `metadata.name` by default);
  status `email` (as upstream's), `externalRef` and `observedState`
  (`email`, `uniqueID`). Upstream (read 2026-11-09): Config Connector's
  direct `ServiceIdentity` takes a project only, and no pull request adds
  a folder or an organization. **The first kind built from
  `hack/scaffold-kind.sh`**: Config Connector's `controllerbuilder`
  generated the types scaffold, `types.generated.go` and
  `mapper.generated.go` from `google.api.serviceusage.v1beta1`, with
  `hack/scaffold/serviceusage/overlay.proto` marking `email` and
  `unique_id` output-only (the proto marks nothing, so without it they
  landed in the spec). The scaffold's `projectRef` and `location` were
  replaced by the two parents; the controller is upstream's, copied
  (`generate-controller` writes nothing for a service upstream already
  has a controller for), with the parent and status changed. The API has
  only `services.generateServiceIdentity`, so, as upstream's, Find trusts
  `status.email`, create generates, update rewrites status, and delete
  leaves the identity; a changed parent or service is refused (the
  identity against `status.externalRef`). Terraform's
  `google_folder_service_identity` and
  `google_organization_service_identity` were read for the request's
  path, not copied (Magic Modules hand-writes them; there is no product
  YAML to take behaviour from). Patch 0022: mockgcp serves the folder and
  organization parents. Live: passes on the test folder (2026-11-15, run
  `cce-11152310-b63f`, after a fix: the API answers an operation already
  done, named `operations/finished.DONE_OPERATION`, that a get refuses;
  patch 0051 fakes it); a live run never uses the organization.

- `ComputeProjectDefaultNetworkTier` (`compute.gcp.hfl.dev/v1alpha1`,
  rank 10, and `docs/coverage/foundation.md` item 1). **Written here**
  after Terraform's `google_compute_project_default_network_tier` (read,
  not copied), in the shape of `ComputeProjectMetadataItem`: `projectRef`
  and `networkTier` (`PREMIUM` or `STANDARD`, checked by the CRD). The
  setting always exists, so Find reads the project, create and update
  call `projects.setDefaultNetworkTier` only when the tier differs (a
  tier changed by hand is put back at the next reconcile), and delete
  leaves the tier as it is, as Terraform's does. Upstream (read
  2026-11-09): no kind or pull request. Not generated: Compute Engine has
  no googleapis proto resource for it, so `hack/scaffold-kind.sh` does
  not apply; the Terraform route (catalog `docs/terraform-leverage.md`,
  spike 2) was not taken, since it would load Config Connector's
  Terraform family into our manager (decision 2.1). Patch 0023: mockgcp
  serves `setDefaultNetworkTier`. Live: pending, needs a test project
  under the test folder (`E2E_TEST_FOLDER_PROJECT`,
  `mockenv.FolderProject`; never `$GCP_PROJECT`, whose tier the offload
  VMs rely on) and `compute.projects.setDefaultNetworkTier` there, which
  `cce-live-tests` lacks (the spike's 403).

- `ProjectDefaultServiceAccounts` (`resourcemanager.gcp.hfl.dev/v1alpha1`,
  rank 10, and `docs/coverage/foundation.md` item 1). **Written here**
  after Terraform's `google_project_default_service_accounts` (google-beta
  `24c6f5fd`, read, not copied; Magic Modules hand-writes it, so there is
  no product YAML): `projectRef`, `action` (`DELETE`, `DISABLE` or
  `DEPRIVILEGE`, immutable by CEL, as Terraform's is ForceNew) and
  `restorePolicy` (`REVERT` by default, `REVERT_AND_IGNORE_FAILURE`,
  `NONE`); status `serviceAccounts`, unique ID to email, as Terraform's
  state. The defaults are the accounts whose display name is "Compute
  Engine default service account" or "App Engine default service
  account" (Terraform's test; their emails may change). `DELETE` and
  `DISABLE` go through IAM v1 by unique ID; `DEPRIVILEGE` removes
  `serviceAccount:{email}` from every binding of the project's policy
  through `pkg/iampolicy` (version 3, etag retry), dropping a binding
  left empty and keeping conditions. On delete, unless `NONE`, `DELETE`
  is undone with `undelete` (by unique ID; an account that exists is
  left alone) and `DISABLE` with `enable`; `REVERT_AND_IGNORE_FAILURE`
  logs a failure and lets the object go; `DEPRIVILEGE` is never undone;
  abandon leaves everything. Two choices of ours, not Terraform's:
  Terraform acts once, at create (its read is a no-op), where this acts
  on every reconcile, so an account re-enabled by hand, or one that
  appears when App Engine is turned on later, is acted on, and status
  only grows; and Terraform reads the policy as version 1, which loses
  conditional bindings. Upstream (read 2026-11-09): no kind or pull
  request. Not generated: there is no proto message for it, so
  `hack/scaffold-kind.sh` does not apply (0% generated beyond the CRD and
  deepcopy; the group file is copied). Patch 0024: mockgcp lists and
  undeletes service accounts and takes unique IDs. Live: pending, needs a
  test project under the test folder (`E2E_TEST_FOLDER_PROJECT`,
  `mockenv.FolderProject`), never `$GCP_PROJECT`, whose compute default
  account the offload VMs and `RunService` use.

- `NetworkSecurityAddressGroup` (`networksecurity.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`;
  `docs/coverage/foundation.md` item 6). An address group, the ranges
  that firewall policy rules name as `srcAddressGroups` or
  `destAddressGroups`, owned by an organization (for its hierarchical
  firewall policies) or by a project. Named by decision 18: Config
  Connector's name in our group, with the parent a field, as Terraform's
  `google_network_security_address_group` takes `parent`
  (`organizations/{id}` or `projects/{id}`). Upstream (read 2026-11-09,
  pin `672414b8`): Config Connector's direct `NetworkSecurityAddressGroup`
  is `v1alpha1` and project-only, and no pull request adds the
  organization; the API's `OrganizationAddressGroupService` has the same
  methods and messages as the project's `AddressGroupService`.
  **Copied**: the types, identity (and its test), controller, fuzzer and
  the address group's functions of `mapper.generated.go` (which upstream's
  `controllerbuilder` generated); `hack/scaffold-kind.sh` was not used,
  since upstream already has the kind and the copy is nearer than a fresh
  scaffold. Changed: `organizationRef` beside `projectRef`, exactly one
  (CEL); the identity takes either parent; an organization's group goes
  through the `OrganizationAddressGroupService` client (its operations
  are the same Go types); `capacity` is immutable (CEL), as Magic
  Modules' `AddressGroup.yaml` (`aad42de6`) marks it, where upstream sends
  the change and lets the API answer. A changed parent is refused (the
  identity against `status.externalRef`). Share of the 1,238 non-test
  lines: 42% generated (controller-gen's CRD and deepcopy here, 36%;
  upstream's `controllerbuilder` mapper, 6%), 49% copied unchanged, 9%
  written or changed here. Patch 0025: mockgcp serves the organization
  service. Live: a project's address group passes (run
`cce-11092303-4b16`, no difference from the fake); an
  organization's is "live: pending, needs test folder" (it needs
  organization access, and a live run never touches the organization).

- `NetworkSecuritySecurityProfile` (`networksecurity.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`;
  `docs/coverage/foundation.md` item 6). A security profile: what a
  firewall policy rule with `apply_security_profile_group` does with
  threats (severity, threat and antivirus overrides), URLs (filters), or
  where it mirrors or intercepts packets, owned by an organization or a
  project. Named by decision 18, as the address group is; Terraform's
  `google_network_security_security_profile` takes `parent`
  (`organizations/{id}` or `projects/{id}`). Upstream (read 2026-11-09,
  pin `672414b8`): Config Connector's direct kind is `v1alpha1` and
  project-only, no pull request adds the organization, and its
  controller dials gRPC (`GRPCClientOptions`), which our manager does
  not configure. **Copied**: the types, identity (and its test),
  reference, controller, fuzzer and the profile's functions of
  `mapper.generated.go` (upstream's `controllerbuilder` output); the
  intercept and mirroring endpoint group references stay Config
  Connector's kinds. Changed: `organizationRef` beside `projectRef`,
  exactly one (CEL); the REST clients, the organization service's for an
  organization parent; an unchanged object still gets its status. From
  Magic Modules' `SecurityProfile.yaml` (`aad42de6`): `type` is
  required, an enum and immutable (CEL), at most one profile is set
  (CEL; the proto's `oneof`), the override and filter lists compare as
  sets, and the URL filter of priority 2147483647 that the API adds is
  not a difference unless the spec names it (its `url_filters`
  flattener). Not carried: `wildfireAnalysisProfile` and
  `customMirroringProfile.mirroringDeploymentGroups`, which the pinned
  `networksecurity` v1 client (v0.17.0) does not have (decision 5). Share
  of the 2,009 non-test lines: 55% generated (controller-gen's CRD and
  deepcopy here, 42%; upstream's `controllerbuilder` mapper, 13%), 33%
  copied unchanged, 12% written or changed here. Patch 0026: mockgcp
  serves the organization service for profiles and profile groups with
  its own server (the project service's parses only project names).
  Live: a project's profile passes (run `cce-11092339-c43e`, no
  difference from the fake: create, a resync with no patch for a threat
  prevention and a URL filtering profile, one patch for an override and
  description change, delete and abandon; the account's
  `roles/compute.networkAdmin` was enough); an organization's is "live:
  pending, needs test folder" (it needs organization access, and a live
  run never touches the organization).

- `NetworkSecuritySecurityProfileGroup` (`networksecurity.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`;
  `docs/coverage/foundation.md` item 6, its last kind). The group a
  firewall policy rule applies (`securityProfileGroup` with action
  `apply_security_profile_group`): one profile of each type, by
  reference to our `NetworkSecuritySecurityProfile`. Owned by an
  organization or a project, as Terraform's
  `google_network_security_security_profile_group` takes `parent`
  (decision 18). Upstream (read 2026-11-09, pin `672414b8`): `v1alpha1`,
  project-only, gRPC, as the profile. **Copied**: the types, identity
  (and its test), controller, fuzzer and the group's functions of
  `mapper.generated.go`; the controller in the shape of our profile's
  (the two upstream controllers differ only in names). Changed:
  `organizationRef` beside `projectRef`, exactly one (CEL); the REST
  clients, the organization service's for an organization parent; an
  unchanged object still gets its status. Magic Modules'
  `SecurityProfileGroup.yaml` (`aad42de6`) marks only the parent,
  location and name immutable, as the identity check already does. Not
  carried: `wildfireAnalysisProfile` (decision 5, as for the profile).
  Share of the 1,232 non-test lines: 48% generated (controller-gen's CRD
  and deepcopy here, 42%; upstream's `controllerbuilder` mapper, 6%),
  38% copied unchanged, 14% written or changed here (the sweeper's
  groups-then-profiles pass among them). Patch 0026, from the profile's
  PR, serves an organization's groups. Live: a project's group passes
  (run `cce-11092345-9b77`, no difference from the fake: the API
  returns a profile reference as it was sent, so a resync writes
  nothing); an organization's is "live: pending, needs test folder".

- `ComputeRouter` (`compute.gcp.hfl.dev/v1beta1` since decision 27, first
  `v1alpha1`; decision 14,
  `docs/coverage/foundation.md` item 9). A whole Cloud Router in one
  object: BGP (with `identifierRange`), interfaces, BGP peers (BFD,
  custom learned ranges, IPv4 and IPv6 next hops, import and export
  route policies, MD5 key names), Cloud NATs (with `autoNetworkTier` and
  NAT64) and the MD5 authentication keys. Named by decision 18:
  Config Connector's name in our group. Upstream (read 2026-11-09, pin
  `672414b8`): the direct `ComputeRouter` has only the description, the
  BGP basics, `encryptedInterconnectRouter`, the network and region; PR
  #13960 (head `9b916158`, open, review required) changes only its
  migration diff. The interfaces, peers and NATs are upstream's
  Terraform-based (and, for NAT and interface, direct) sibling kinds,
  without the fields listed in item 9. **Generated**:
  `hack/scaffold-kind.sh google.cloud.compute.v1
  compute.gcp.hfl.dev/v1alpha2 ComputeRouter:Router` (v1alpha2 because
  upstream has compute v1alpha1; the compute proto has no field
  behaviour or resource annotations, so no overlay). Kept unedited: the
  BFD, custom learned range and NAT64 subnetwork types and their mapper
  functions; the generated `RouterBgpPeer` field set and mapper are the
  base of ours, renamed to Config Connector's `ComputeRouterPeer` field
  names. **Copied**: the direct `ComputeRouter` types, identity and
  controller, and upstream's router, NAT and interface mappers (for our
  inline NAT and interface types, which reuse upstream's NAT rule, log
  config, subnetwork and reference types). **Changed or written**:
  - the router is written with `routers.update` (PUT): the body is the
    router as read with the owned fields replaced, because
    `routers.patch` keeps a list the body leaves out and the client
    leaves an empty list out (the firewall rule's lesson), so a removed
    peer, NAT, interface or key would stay. Peers and interfaces the API
    made for an interconnect attachment (`MANAGED_BY_ATTACHMENT`) are
    kept unless the spec names them. Live runs showed the API takes one
    feature per write ("Creating a router with multiple features is
    currently unsupported", then "Updating multiple Router features is
    currently unsupported"), so the router is inserted bare and then
    written one feature at a time, as Terraform's router, interface,
    peer and NAT resources each patch only their own: changed key
    values, the base fields, the interfaces (keeping those a remaining
    peer is on), the peers with their keys, the interfaces as the spec
    has them, the NATs; a step with nothing to change is skipped;
  - the comparison is written here: what the spec sets must match, what
    it leaves out is the API's default (Magic Modules: `keepaliveInterval`
    20, `identifierRange` from the API; Terraform's peer: BFD, `enable`,
    `ipAddress` computed), the four lists must have the same members in
    any order, route policy lists in order, links without their URL
    prefix;
  - MD5 key values come only from Secrets (`keyRef`) and are never
    logged or kept; they are sent only in the steps that write the keys
    (a live run: the API takes a key sent with its value as a change of
    the keys, so a NAT-only update carrying them was refused), and the
    other steps send the keys as read, by name, which keeps their values; the API never returns them
    (Terraform's peer flattener; patch 0027 makes the fake do the same),
    so `status.md5AuthenticationKeysVersion`, a digest of each key's
    Secret name, key and `resourceVersion` (not of the value), shows when
    a Secret changed and the keys are written again;
  - `privateIpAddress` and `redundantInterface` are the API's strings
    (upstream maps an address reference into a field that takes an IP);
    VPN tunnel and attachment references by name read the Config
    Connector object's `status.selfLink`; the network and
    `encryptedInterconnectRouter` are immutable (an error, not a write).
  Share of the ~3,450 non-test lines: about 52% generated (controller-gen's
  CRD and deepcopy, 48%; `controllerbuilder`, 4%), 27% copied unchanged,
  21% written or changed here. `ComputeRouterRoutePolicy`'s Config
  Connector router GVK variable is renamed `ConfigConnectorComputeRouterGVK`
  (the name is now our kind's). Not carried: `nccGateway` (a router for
  an NCC gateway spoke) and the input-only `params.resourceManagerTags`;
  `task coverage` counts 76 of the Router's 81 fields covered with ours,
  the MD5 key's value (ours is `keyRef`) and the NAT subnetworks' names
  (ours are references) among the five it does not match. Live: passes (run
  `cce-11100336-0ecc`) after four live runs that each found what the fake
  did not show: the one-feature rules for insert and update, and the
  key value counted as a change; each is now in patch 0027. The rotated
  key's value and the `MANAGED_BY_ATTACHMENT` path cannot be seen live
  either (the API returns no key values; the test makes no
  attachment).

- `IAMDenyPolicy` (`iam.gcp.hfl.dev/v1beta1`, decision 13, the first of
  the stable kinds for upstream's alpha-only foundation kinds). Upstream,
  rechecked 2026-11-10 (read-only): `IAMDenyPolicy` is `v1alpha1` with a
  direct controller on `master` (PR #4717, types, 2025-06; PR #8771,
  controller, 2026-05) and in `v1.160.0`, the latest release; no pull
  request promotes it; PR #7303 ("Add IAM deny policies support",
  Terraform-based, 2026-07-08) is open, review required, untouched since.
  **Copied** with as little change as possible: the types, identity
  (`policies/{attachment point}/denypolicies/{id}`, the attachment point
  the URL-escaped `cloudresourcemanager.googleapis.com/projects/{id}`,
  `folders/{id}` or `organizations/{id}`), the direct controller on IAM
  v2's REST client, the hand-written mappers and the fuzzer. Generated:
  the `DenyRule`, `PolicyRule` and `Expr` types (upstream's
  controllerbuilder output, unedited), deepcopy and the CRD. Changed: a
  CEL rule for exactly one of `projectRef`, `folderRef` and
  `organizationRef` (upstream fails in the controller when none is set,
  and takes the first of several); `status.externalRef` is written
  (upstream sets it on a typed copy that never reaches the object, so its
  objects have none); our group, version (`v1beta1`, which the charts
  allow; decision 13) and short names. Update is upstream's: a whole-policy
  `PUT` with the current etag when the spec's display name or rules differ
  from what was read, nothing otherwise. A changed parent or `resourceID`
  is refused by the identity check ("cannot change IAMDenyPolicy
  identity"). mockgcp serves IAM v2 deny policies unpatched (it checks the
  etag on update and stores a project's under its number, as the API
  answers). Upstream's own fixture recorded the real API
  (`pkg/test/resourcefixture/testdata/basic/iam/v1alpha1/iamdenypolicy/`,
  `_http.log`): create, get, a whole-policy `PUT` and delete, the policy
  returned as sent, with the project's number in its name.
  Live: pending, needs `iam.denypolicies.*` for the live-test service
  account, which only Deny Admin (`roles/iam.denyAdmin`) gives and which
  is granted on an organization; the test skips live unless
  `E2E_IAM_DENY_ADMIN=true` (`docs/live-runs.md`). A folder's and an
  organization's policy run on the fake only.
- `ComputeOrganizationSecurityPolicy` and
  `ComputeOrganizationSecurityPolicyAssociation`
  (`compute.gcp.hfl.dev/v1beta1`, decision 13: a hierarchical firewall
  policy under an organization or folder, and its attachment to an
  organization or folder). Upstream, rechecked 2026-11-10 (read-only):
  both are `v1alpha1` on `master` at the pin (`672414b`) and in
  `v1.160.0`, Terraform-based by default with an opt-in direct
  controller; `ComputeOrganizationSecurityPolicyRule` is Terraform-only
  (a closed PR, #9950, began its direct types); no open pull request
  promotes any of the three. **Copied** with as little change as
  possible: the types, identities and references, the direct
  controllers on the Compute Engine REST client, the hand-written
  mappers (upstream's `mappers.go`), the fuzzers and the association's
  mapper test. Changed: `displayName`, `parent` and `type` are immutable
  by CEL (upstream says so, but its update patches only the description,
  so a changed name would have been reported and never applied); the
  association takes exactly one of `policyId` and `securityPolicyRef`
  and its spec is immutable, both by CEL (upstream refuses these in the
  identity and the update); the reference names our policy kind; our
  group, version (`v1beta1`, as `IAMDenyPolicy`) and short names. The
  policy's ID is the server's: `resourceID` only acquires an existing
  policy; otherwise the first reconcile inserts one and keeps its ID in
  `status.policyId`, which later reconciles use (a status write lost
  after the insert would insert a second policy, as upstream). mockgcp
  serves both unpatched (`docs/fake-cloud.md`). Hand-written here: the
  end-to-end test only. Live: pending, needs the test folder (both act
  on an organization or folder; `docs/live-runs.md`). 2026-11-15 (the
  builder's RECPF answer, "a stable Cloud Armor association"): this kind is
  that association (fabric's `cloud_armor_policy` is Terraform's
  `google_compute_organization_security_policy_association`, attached to
  `projects/<number>` or `folders/<id>`); added `excludedFolders` and
  `excludedProjects` (v1 API fields), refused on a project's association
  by CEL as fabric refuses them, and a fake-cloud case of a `CLOUD_ARMOR`
  policy attached to a project and to a folder with exclusions
  (`docs/kinds/ComputeOrganizationSecurityPolicyAssociation.md`).
- `ComputeOrganizationSecurityPolicyRule` (`compute.gcp.hfl.dev/v1beta1`,
  decision 13: a rule of a hierarchical firewall policy, at its
  priority). Upstream, rechecked 2026-11-10 (read-only): `v1alpha1` at
  the pin with direct types, identity, reference, a mapper and a fuzzer,
  but reconciled only through Terraform (`static_config.go`); no open
  pull request adds a controller. **Copied:** the types, identity,
  reference and identity test. **Not copied:** upstream's mapper and
  fuzzer, which map the rule to `FirewallPolicyRule`, the message of
  `firewallPolicies`, not of this API. **Written here:** the controller,
  after Magic Modules' `OrganizationSecurityPolicyRule.yaml` (read, not
  copied) and our regional firewall policy rule: `getRule`, `addRule`,
  `patchRule` with every field (unset ones as their defaults: direction
  `INGRESS`, `versionedExpr` `FIREWALL`, `""`, `false`, `[]`) and
  `removeRule`, by priority; a rule the API answers 404 or 400 "does not
  contain a rule at priority" for is gone. A FIREWALL rule's
  `direction`, `enableLogging`, `targetResources`,
  `targetServiceAccounts`, `destIpRanges` and `layer4Configs` are only in
  the compute **beta** API (Magic Modules: `min_version: beta`; v1's
  `SecurityPolicyRule` is Cloud Armor's), so the controller uses the
  beta REST client (`google.golang.org/api/compute/v0.beta`, already a
  dependency), as Terraform does. Changed from upstream's schema:
  `securityPolicyRef` beside `policyId`, exactly one (the policy's ID is
  the server's, so `policyId` alone cannot name a policy made in the same
  apply; the association has the same pair); `policyId`,
  `securityPolicyRef` and `resourceID` (the priority, `metadata.name`
  when unset) immutable and the priority a number; `direction` and
  `versionedExpr` enums; at least one `layer4Config`; `match.description`
  refused, since the API's matcher has none (Magic Modules still lists
  it); `status.externalRef`
  (`locations/global/securityPolicies/{policy}/rules/{priority}`, ours)
  and `status.ruleTupleCount`. Generated: the CRD and deepcopy only
  (controller-gen); `hack/scaffold-kind.sh` does not fit a rule inside a
  policy (its header says so), and its types would be v1's Cloud Armor
  rule. Copied: 838 of the kind's 2,059 hand-kept lines (41%, the mock
  patch included); the rest is written here. mockgcp patch 0028
  (`docs/fake-cloud.md`). Live: pending, needs the test folder.
- `ComputeNetworkFirewallPolicyPacketMirroringRule`
  (`compute.gcp.hfl.dev/v1beta1` since 2026-11-16, decision 27; built at `v1alpha1`; foundation item 2: a packet mirroring
  rule of a global network firewall policy, at its priority). Upstream,
  checked 2026-11-10 (read-only): no kind, no direct or Terraform code and
  no pull request (searched `PacketMirroringRule`, `packet mirroring`);
  only generated types carry the policy's `packetMirroringRules` list.
  Terraform has `google_compute_network_firewall_policy_packet_mirroring_rule`
  (beta; Magic Modules' `NetworkFirewallPolicyPacketMirroringRule.yaml`,
  read). The API: v1 serves `networkFirewallPolicies.addPacketMirroringRule`,
  `getPacketMirroringRule`, `patchPacketMirroringRule` and
  `removePacketMirroringRule`; the hierarchical `firewallPolicies` has
  them in beta and alpha only, and `regionNetworkFirewallPolicies` has
  none in any version (its policy only lists `packetMirroringRules`), so
  the regional kind foundation item 2 names cannot be built on rule calls
  and the hierarchical one is left for its own pull request (live pending
  the test folder). **Written here**, the types, identity and controller
  adapted from our `ComputeNetworkFirewallPolicyRule` (same message, the
  policy's other list), with Terraform's fields: `priority` and
  `firewallPolicyRef` (Config Connector's `ComputeNetworkFirewallPolicyRef`)
  immutable, `action` `mirror`, `do_not_mirror` or `goto_next` (a CEL
  rule: `mirror` needs `securityProfileGroup`), `direction`,
  `match.srcIPRanges`, `destIPRanges` and `layer4Configs`, `ruleName`,
  `description`, `disabled`, `securityProfileGroup` (a string, as the
  API takes it: Terraform has no reference either) and
  `targetSecureTags`, but not Terraform's `tlsInspect`: the first live
  run's create was refused with 400 "TLS Inspection cannot be set for
  packet mirroring rule" for `tlsInspect: false`, so the field is gone and
  never sent (patch 0032 refuses it too); `status.externalRef`
  (`.../firewallPolicies/{policy}/packetMirroringRules/{priority}`, ours)
  and `status.ruleTupleCount`. The pinned `google.golang.org/api`
  compute/v1 client has no packet mirroring calls and the generated
  `cloud.google.com/go` client leaves an empty list out, so the patch is
  a plain JSON POST through the same transport, with an emptied list sent
  as `[]` (as the firewall rule's patch learned live), waiting on the
  global operation; add, get and remove use the generated client. A
  missing priority is taken from a 400 that says "does not contain a ...
  at priority" (assumed from the firewall rules; see the live row). The
  test runs a `do_not_mirror` and a `goto_next` rule live; a `mirror`
  rule needs a security profile group with a custom mirroring profile and
  a mirroring endpoint group, so it runs on the fake only. Generated: the
  CRD and deepcopy (controller-gen, 511 lines); `hack/scaffold-kind.sh`
  does not fit a rule inside a policy. Copied: none from upstream; the
  1,075 hand-kept lines (the mock patch included) are written here, 32% of
  the kind's lines generated. mockgcp patch 0032 (`docs/fake-cloud.md`).
  Live: create, resync, a second rule, delete and abandon pass; the
  patch does not yet (`docs/live-runs.md`, section 7 item 37).
- `ComputeNetworkFirewallPolicyRule` (`compute.gcp.hfl.dev/v1beta1`,
  decision 13: a rule of a global network firewall policy, at its
  priority). Upstream, rechecked 2026-11-10 (read-only): `v1alpha1` at
  the pin, Terraform-only (`static_config.go`), with no direct types,
  identity, mapper or controller in `apis/` or `pkg/controller/direct`;
  its schema is the Terraform-generated CRD (`projectRef`,
  `firewallPolicyRef`, the priority as `resourceID`, a string that
  defaults to `metadata.name`, `targetServiceAccountRefs`). No open pull
  request adds one. So nothing is copied: **written here**, the types,
  identity and controller adapted from our
  `ComputeRegionNetworkFirewallPolicyRule` (whose shape follows Config
  Connector's direct `ComputeFirewallPolicyRule`, the rule of an
  organization or folder policy, which upstream will most likely mirror
  when it promotes this kind), with upstream's
  `computefirewallpolicyrule_controller.go` read for the same four calls.
  Fields: the regional kind's, plus `match.srcNetworkContext` and
  `match.destNetworkContext` (upstream's alpha CRD has them), and an
  optional `projectRef`, refused when it is not the policy's project
  (upstream's alpha requires it; the policy already names its project).
  `firewallPolicyRef` is Config Connector's own
  `ComputeNetworkFirewallPolicyRef` (a `ComputeNetworkFirewallPolicy` by
  `name`, read from its `status.externalRef` or `status.selfLink`, or
  `external`, `projects/{p}/global/firewallPolicies/{policy}`), imported,
  not copied. `priority` (a number, as the regional kind and
  `ComputeFirewallPolicyRule` have it) and the policy are immutable.
  The controller is the regional one on the global calls
  (`networkFirewallPolicies.getRule`, `addRule`, `patchRule` through the
  Compute Engine REST client with emptied lists sent as `[]`, and
  `removeRule`; the patch waits on a global operation), sharing its
  missing-rule check (renamed `firewallPolicyRuleMissing`) and its
  comparison, to which the network contexts are added. Live showed that
  `patchRule` ignores a network context sent as `""` (it kept
  `INTERNET`); one is cleared by sending `UNSPECIFIED`, and `""` and
  `UNSPECIFIED` compare as unset. Not here yet, as
  in the regional kind: `securityProfileGroup`, `tlsInspect` and
  `targetResources`. Generated: the CRD and deepcopy only
  (controller-gen); `hack/scaffold-kind.sh` does not fit a rule inside a
  policy (the proto has no rule resource), so the generated share of the
  hand-kept lines is 0%. mockgcp serves a global policy's rules
  unpatched (`docs/fake-cloud.md`). Live: passes (`docs/live-runs.md`).
- `LoggingBillingAccountSink` (`logging.gcp.hfl.dev/v1beta1` since decision 27,
  first `v1alpha1`; decision
  15, foundation item 17: a billing account's log sink, Terraform's
  `google_logging_billing_account_sink`). Upstream, rechecked 2026-11-10
  (read-only): `LoggingLogSink` is `v1beta1` with a direct controller
  (opt-in; Terraform by default) whose spec takes `projectRef`,
  `folderRef` or `organizationRef`; its identity already parses
  `billingAccounts/{billingaccount}/sinks/{sink}`, but no field reaches
  it, on `master` (`56effb6`, 2026-11-09) either; no open pull request
  adds one (searched "LogSink", "billing sink"). **Copied** with as
  little change as possible: the types, identity, the direct controller
  and the sink's mapper functions; upstream's destination, BigQuery
  options and exclusion types are imported, not copied. Changed: a
  required, immutable `billingAccountRef` (Config Connector's
  `billing/v1alpha1` `BillingAccountRef`: `external` as
  `billingAccounts/{id}` or the bare ID, or `name` of a `BillingAccount`)
  in place of the three parents; `includeChildren` dropped (folders and
  organizations only); `uniqueWriterIdentity` dropped and always sent as
  true (a billing account's sink always has its own writer identity, and
  Terraform sends true); `status.externalRef`, checked by the identity
  so a changed `resourceID` is refused. Update is upstream's: a mask of
  the top-level fields that differ. Hand-written here: the end-to-end
  test only. Generated: the CRD and deepcopy (controller-gen);
  `hack/scaffold-kind.sh` was not run, since upstream's hand-written
  controller and mapper are the nearer code. Copied: 655 of the kind's
  1,008 hand-kept lines (65%; the mock patch and the test are the rest).
  `task coverage` counts 10 of the resource's 13 attributes as ours; the
  other three are `includeChildren` and `interceptChildren` (folders and
  organizations only) and the deprecated `outputVersionFormat`.
  mockgcp patch 0029 (`docs/fake-cloud.md`). Live: pending, needs
  billing account access (`E2E_BILLING_ACCOUNT`, `docs/live-runs.md`).
- `IAMPrincipalAccessBoundaryPolicy` (`iam.gcp.hfl.dev/v1beta1`,
  decision 13: an organization's principal access boundary policy, the
  resources its principals may use). Upstream, rechecked 2026-11-10
  (read-only): `v1alpha1` at the pin with a direct controller (the
  default in `static_config.go`), generated identity, reference, types,
  mapper and fuzzer; no open pull request promotes it. **Copied:** all
  of those, unchanged but for the package, our group, labels and short
  names, and CEL rules making `organizationRef`, `location` and
  `resourceID` immutable (decision 20; upstream only checks the identity
  against `status.externalRef`). The controller already writes status
  through `op.UpdateStatus` (the deny policy's did not). Generated: the
  CRD and deepcopy (controller-gen); upstream's identity, reference,
  types, mapper and fuzzer were themselves generated by controllerbuilder,
  so `hack/scaffold-kind.sh` was not run again. Copied: 941 of the
  kind's 1,528 hand-kept lines (62%, with our headers); written here (587):
  the end-to-end test and mockgcp
  patch 0030 (IAM v3 principal access boundary policies, which mockgcp
  lacked; `docs/fake-cloud.md`). Live: pending, needs organization-level
  access (a policy lives under an organization; `docs/live-runs.md`).
- `SecurityCenterMuteConfig` (`securitycenter.gcp.hfl.dev/v1beta1` since
  decision 27, first `v1alpha1`;
  decision 16, foundation item 20, the first of its Security Command
  Center kinds: a mute config of an organization, folder or project).
  Upstream, rechecked 2026-11-10 (read-only): on `master` at the pin
  (`672414b`) `SecurityCenterMuteConfig` is `v1alpha1` with a direct
  controller on the v1 API and takes `organizationRef` only, as do
  `SecurityCenterBigQueryExport` and the two custom module kinds;
  `SecurityCenterNotificationConfig` is Terraform-based. Open PR #7673
  ("direct SCC resources", one commit, last updated 2026-05-05, review
  required) does give mute configs, BigQuery exports and custom modules
  `organizationRef`, `folderRef` and `projectRef` (foundation.md said
  neither PR was known to; #7673 does), but it predates `master`'s own
  kinds, whose schemas and controllers differ, and has not moved since.
  #14181 (the health analytics custom module's controller) and #14206
  keep the organization only. **Copied** with as little change as
  possible: `master`'s types, identity, direct controller and mapper
  functions, and #7673's parent resolution
  (`apis/securitycenter/v1alpha1/parent.go`), so the schema is
  `master`'s plus #7673's two parent fields: exactly one of the three,
  each immutable (CEL). Changed: the identity is
  `{parent}/muteConfigs/{id}`; the update compares with the name the
  API returned (in case it names a project by number); export sets the
  parent it came from; delete of a missing mute config is done (upstream
  reports it not deleted). Hand-written here: the end-to-end test.
  Generated: the CRD and deepcopy (controller-gen); upstream's mapper is
  itself generated. Version `v1alpha1`, as foundation.md's build line
  says (decision 16 does not ask for a stable kind). mockgcp patch 0031
  (`docs/fake-cloud.md`). Live: the organization's and folder's are
  "live: pending, needs test folder"; a project's needs Security Command
  Center active for the project and `securitycenter.muteconfigs.*` for
  the live-test service account (`E2E_SECURITY_CENTER=true`,
  `docs/live-runs.md`).
- `DNSResponsePolicy` (`dns.gcp.hfl.dev/v1beta1`, decision 13: a Cloud
  DNS response policy, the DNS firewall a VPC network or GKE cluster
  consults before Cloud DNS; its rules are `DNSResponsePolicyRule`,
  next). Upstream, rechecked 2026-11-10 (read-only): `v1alpha1` at the
  pin with a **direct** controller, the default (`static_config.go`:
  direct, Terraform also supported), types, identity, reference, a
  generated mapper (`zz_generated.v1alpha1.mappers.go`, from
  `dev/tools/openapi-to-krm`) and a fuzzer. Merged #14530 (zero-diff
  takeover from the Terraform controller) and #14712 (direct the default,
  slated for 1.161) keep it `v1alpha1`; no open pull request promotes it
  (searched `DNSResponsePolicy`, `ResponsePolicyRule`). So it is
  **copied**, as `IAMDenyPolicy` was, into a new group
  `dns.gcp.hfl.dev` (version `v1beta1`). Changed: `projectRef` and
  `resourceID` are immutable by CEL rules (decision 20); the controller
  writes `status.externalRef`
  (`projects/{p}/locations/global/responsePolicies/{id}`, the identity's
  own form; upstream's status has no such field); and the object's
  `metadata.labels` are the policy's `labels`, sent on create and update
  and compared (upstream leaves `labels` out, the one field the coverage
  report counted uncovered besides output and name fields; decision 19).
  `networks[].networkUrl` stays a string, as upstream has it: a short
  path (`projects/{p}/global/networks/{n}`) or a full URL, which the
  controller normalizes to the full URL both ways; there is no
  `ComputeNetwork` reference. Delete unbinds the networks (a `patch` with
  an empty list) and then deletes, as upstream does (upstream unbinds
  first, which says the real API refuses to delete a bound policy;
  mockgcp does not refuse).
  Generated: the CRD and deepcopy (controller-gen, 376 lines); the mapper
  was generated upstream (`openapi-to-krm`) and is copied unedited.
  `hack/scaffold-kind.sh` was not run: upstream's direct code exists, and
  copying it keeps us on upstream's shape for the move back. Copied: 887
  of the kind's 1,359 hand-kept lines (65%); written here: the
  end-to-end test and its helpers (360) and the changes above. mockgcp
  serves response policies unpatched (`docs/fake-cloud.md`). Live:
  pending `roles/dns.admin` and `dns.googleapis.com` in
  the previous dev project (`docs/live-runs.md`).
- `ComputeVPNTunnel` (`compute.gcp.hfl.dev/v1beta1`, foundation item
  10, the first field-gap kind under decision 19 and the first to apply
  section 6.6's create-only pattern: an HA or Classic VPN tunnel with
  `cipherSuite`, the IKE phase 1 and 2 algorithms, which Config
  Connector's lacks). Upstream, rechecked 2026-11-10 (read-only):
  `ComputeVPNTunnel` is `v1beta1` and Terraform-only at the pin and on
  `master` (`static_config.go`), with direct types and a mapper (PR
  #10151, merged 2026-07-06) but no controller or identity; no pull
  request adds `cipherSuite` or a controller. **Copied**: the types and
  the mapper, changed for our group, with `cipherSuite`, `labels` (the
  one field `vpnTunnels.setLabels` changes in place) and
  `status.externalRef` added, `projectRef` optional beside the
  namespace's project annotation, every other field immutable by CEL
  (decision 20; exactly one of `vpnGatewayRef` and `targetVPNGatewayRef`,
  also CEL), and `routerRef` ours (our `ComputeRouter` or Config
  Connector's, by name, refused when both exist, as for
  `ComputeRouterRoutePolicy`; its resolution is now `ComputeRouterRef.Resolve`).
  The gateway references are Config Connector's types, resolved by
  `external` or by the named object's `status.selfLink`. **Written
  here**: the identity, the controller (insert, get, `setLabels` with the
  label fingerprint, delete; the shared secret read from its Secret on
  every reconcile, sent only by insert), and the end-to-end test. Live
  showed two things the fake did not: `vpnTunnels.insert` leaves the
  labels out (Create sets them after it, as Terraform's tunnel does), and
  a peer gateway on a reserved address (RFC 5735) is refused.
  Generated: the CRD and deepcopy (controller-gen);
  `hack/scaffold-kind.sh` was not run, since upstream's direct types and
  mapper already existed to copy. mockgcp patch 0033 (`docs/fake-cloud.md`):
  mockgcp had no VPN tunnel service. A tunnel moves to ours by acquiring
  the same name (`resourceID`) and abandoning Config Connector's object;
  ours cannot change an acquired tunnel's immutable fields, so a tunnel
  that needs `cipherSuite` is made new. Live: see `docs/live-runs.md`.
- `ComputeProjectMetadataItem` (`compute.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; rank 10,
  per-key metadata). **Written here** after Terraform's
  `google_compute_project_metadata_item` (read, not copied): `projectRef`,
  `resourceID` (the key; `metadata.name` by default), `value`. The
  controller reads the project's common instance metadata, changes the
  one key, writes it all back with the fingerprint it read, and reads
  again and retries on a 412 from another writer; delete removes only its
  key. Unlike Terraform, which refuses a key that is already there, a key
  already there is taken over, as Config Connector takes over existing
  resources. The API has no resource name for one key, so
  `status.externalRef` is ours: `projects/{project}/commonInstanceMetadata/{key}`.
  mockgcp already serves `projects.get` and `setCommonInstanceMetadata`;
  no patch.
  Live: passes, with `roles/iam.serviceAccountUser` on the project
  (`setCommonInstanceMetadata` checks `actAs` there; `docs/live-runs.md`).
- `IAMBinding` (`iam.gcp.hfl.dev/v1beta1`, rank 1; built as
  `IAMRoleBinding`, renamed by decision 12, moved from `v1alpha1` to
  `v1beta1` unchanged by decision 24). **Written here**:
  `resourceRef` (`kind`, `name` or `external`, shaped like
  `IAMPolicyMember`'s), `role`, `condition` (title, description,
  expression), `members` (strings as the API writes them) and
  `membersFrom[].serviceAccountRef`. It owns one role under one condition
  on one resource: members added out of band are removed at the next
  reconcile, every other role (and the same role under another condition),
  the audit configs and the etag are carried through unchanged, and delete
  removes only that binding (`deletion-policy: abandon` leaves it).
  `resourceRef`, `role` and `condition` are immutable (CEL). Two objects
  for the same role, condition and resource would overwrite each other, as
  two Terraform `google_*_iam_binding`s do.
  The controller does not use Config Connector's IAM client, which needs
  the Terraform provider and service mappings this manager does not load
  (section 2.1). `pkg/iampolicy` reads and writes a policy over HTTP: one
  table of resource types (`registry.go`) says, for each `kind`, the API
  host and version, whether the policy is read with GET or POST, the
  pattern of the resource's full name, how a bare ID becomes one
  (`external`) and how a referenced object becomes one (`name`: its
  `status.externalRef` when it sets it, else the project, `spec.location`
  and `spec.resourceID` or the object's name). Two protocols: the
  `google.iam.v1` mixin (`{resource}:getIamPolicy`, `:setIamPolicy`) and
  Cloud Storage's `GET`/`PUT .../iam`. The policy is read at version 3,
  changed, and written back with its etag; a 409 or 412 reads and tries
  again (five times). A member is compared as the API writes it (the
  address of a `user:`, `group:`, `serviceAccount:` or `domain:` member is
  lower-cased), in any order. A referenced object that is missing is "not
  found", one whose Ready condition is not True is "not ready", and the
  reconciler waits and retries; while the binding is being deleted, a
  missing object, or one being deleted without `deletion-policy: abandon`
  (2026-11-13; `docs/kinds/IAMBinding.md`), means there is nothing to
  remove. A kind that is not in
  the table is refused with the list of kinds.
  Kinds in the table (2026-11-09): `Project`, `Folder`, `Organization`,
  `IAMServiceAccount`, `StorageBucket`, `SecretManagerSecret` (global and
  regional; not in `IAMPolicyMember`), `ArtifactRegistryRepository`,
  `RunService`, `PubSubTopic`, `PubSubSubscription`, `KMSKeyRing`,
  `KMSCryptoKey`. A `Folder`, `Organization` or `KMSCryptoKey` is named by
  `external`; the others also by `name`. The table grows by one row per
  resource type, with no change to the kind. `membersFrom` takes
  `serviceAccountRef` or `logSinkRef` (2026-11-15, below at `IAMMember`);
  `sqlInstanceRef` and the other `memberFrom` sources of `IAMPolicyMember`
  are not offered; `members` takes the address.
  `CloudFunctionsFunction` (Config Connector's 1st gen function,
  `cloudfunctions.cnrm.cloud.google.com/v1beta1`) was added 2026-11-15
  (the builder's RECPF answer, decision 35): by `name` its project,
  `spec.region` (the resolver reads `spec.region` when a kind has no
  `spec.location`) and `resourceID`, or by `external`; the Cloud Functions
  v1 API's `:getIamPolicy` (GET) and `:setIamPolicy`, read from the v1 REST
  reference and Terraform's `google_cloudfunctions_function_iam_*` (not
  copied). `IAMPolicyMember` takes it too, so it is not a
  `NotInIAMPolicyMember` row. Tested on the fake cloud
  (`TestIAMOnCloudFunctionsFunction`: an `IAMBinding` and an `IAMMember`
  by name, set, read back and removed) and for its name and URL
  (`registry_test.go`); not run live: it needs a deployed 1st gen
  function, and the policy verbs are the mixin `IAMBinding`'s live run
  covers.
  Patch 0018: mockgcp serves Cloud Storage's `.../iam` paths from the
  store it already keeps for the `:getIamPolicy` verbs.
  Live: passes, on a secret and a bucket (`docs/live-runs.md`).
- `IAMMember` (`iam.gcp.hfl.dev/v1beta1` since decision 27, first `v1alpha1`; rank 2; built as
  `IAMRoleMember`, renamed by decision 12). **Written here**,
  the additive sibling of `IAMBinding`, shaped like Config Connector's
  `IAMPolicyMember`: `resourceRef` (the same type as `IAMBinding`'s),
  `role`, `condition`, and one of `member` or `memberFrom` (CEL: exactly
  one). Every field is immutable, as in `IAMPolicyMember`.
  `memberFrom` (and `IAMBinding.membersFrom`, the same type) takes exactly
  one of `serviceAccountRef` and, since 2026-11-15, `logSinkRef`: a
  `LoggingLogSink` whose `status.writerIdentity` is the member, shaped like
  `IAMPolicyMember.memberFrom.logSinkRef` (`name`, `namespace`) plus an
  optional `apiVersion` (an enum of `logging.gcp.hfl.dev/v1beta1`, ours,
  and `logging.cnrm.cloud.google.com/v1beta1`, Config Connector's; unset,
  ours is looked for first, then Config Connector's, as `resourceRef`
  does for a kind in both). Written here (`pkg/iampolicy/logsink.go`),
  after Config Connector's `iamclient.ResolveMemberIdentity`, which
  resolves its own `LoggingLogSink` only: the reason the catalog's folder
  and organization charts could not grant our sink's writer identity with
  `IAMPolicyMember`. A sink that is missing, not Ready or without a writer
  identity yet is a reference error: the object is `DependencyNotReady`
  and waits (a watch on the sink, then retries), never failing. While an
  `IAMMember` is being deleted, a `memberFrom` object that is gone or not
  ready (a sink deleted first, with its chart) does not hold the delete:
  the member comes from `status.externalRef`. Tested on the fake
  (`TestIAMMember`, "a regional secret"): an `IAMMember` made before its
  sink waits and names it, then grants the sink's writer identity once
  our `LoggingLogSink` is Ready; an `IAMBinding` takes the same source;
  the sink deleted first still lets the member remove itself; a Config
  Connector `LoggingLogSink` (its CRD is in `mockenv`, not reconciled)
  waits until its status has a writer identity; two sources or none are
  refused.
  It adds its member to the role's binding (`AddMembers`) and removes only
  that member on delete (`RemoveMembers`, and the binding if it was the
  last): other members of the role, other roles and the audit configs are
  carried through unchanged, so several objects can give one role to
  several members and anyone else's grants stay. `Find` is "the member is
  in the binding"; a member taken off out of band is put back at the next
  reconcile, a member somebody else added is left. `status.externalRef` is
  ours: `{resource}#{role}#{member}`, plus `#{condition title}`.
  It shares everything else with `IAMBinding` (`pkg/iampolicy`,
  `policy_common.go`), and the resource table grew by the rows
  `IAMPolicyMember` cannot reference, which both kinds accept (marked
  `NotInIAMPolicyMember` in `registry_more.go`: 31 rows, and the regional
  `SecretManagerSecret` that was already there; the catalog counts 32 by
  its own grouping, and a type missing is a row to add): `StorageManagedFolder`
  (Cloud Storage's own paths with the folder's name escaped whole and
  ending in a slash), `LoggingLogView`, `RunWorkerPool`, `PrivateCACAPool`,
  `CloudDeployDeliveryPipeline`, `CloudDeployTarget`, `DataformRepository`,
  `CloudFunctions2Function` (2026-11-12, Cloud Functions' v2 API),
  `DataplexLake`, `DataplexZone`, `DataplexDataScan`, `DataplexAspectType`,
  `WorkstationConfig`, `Workstation`, `BigQueryAnalyticsHubDataExchange`,
  `BigQueryAnalyticsHubListing`, `APIGatewayAPI`, `APIGatewayAPIConfig`,
  `APIGatewayGateway`, `BinaryAuthorizationAttestor`,
  `BigQueryConnectionConnection`, `SecureSourceManagerInstance`,
  `SecureSourceManagerRepository`, `DataCatalogTagTemplate`,
  `CloudBuildConnection`, `WorkbenchInstance`, `EndpointsService`,
  `IAPTunnelInstance`, `IAPWebCloudRunService`, and, with the third
  protocol, `ComputeBackendService` (global and regional) and
  `ComputeMachineImage`. The kinds Config Connector has can be named by
  `name` (the project, `spec.location` and the ID make the full name, or
  `status.externalRef` when it already is one); the others by `external`.
  **Compute Engine's protocol** is `GET {resource}/getIamPolicy?optionsRequestedPolicyVersion=3`
  and `POST {resource}/setIamPolicy` with `{"policy": ...}`, as Terraform
  sends it (the handoff's `{bindings, etag, policy}` body is the same
  request, whose `policy` field is the one used). Patch 0019: mockgcp
  serves those paths from the same store as the verbs.
  **Which rows are proven.** Three are exercised end to end, on the fake
  cloud and on a real project: a regional `SecretManagerSecret`, a
  `StorageManagedFolder` and a `ComputeBackendService`. The others are
  written from each service's REST reference and Terraform's matching
  `google_*_iam_*` resources (read, not copied) and checked only for their
  names and URLs (`registry_test.go`); none has been called against the
  real API. A wrong path, verb (GET or POST for the read) or version shows
  as the API's own error on the object's Ready condition, and is a
  one-row fix. They are listed as such so that nobody relies on them
  before a live run.
  Live: passes, on a regional secret, a managed folder and a backend
  service (`docs/live-runs.md`).
Below the top ten, each needed by one module (Looker instances, Endpoints
services, Cloud Build v2 repositories, regional instance templates, and
others in the inventory): not ranked here yet. Container Registry is
deprecated upstream and is not built; the catalog maps it to an Artifact
Registry `gcr.io` repository (decision 30).

### 6.6 Field gaps on large Terraform-based kinds (proposed 2026-11-10)

Decision 19 fills field gaps now instead of waiting for Config Connector.
The accepted pattern so far is a parallel kind of ours that owns the whole
object (decisions 11, 13 and 14). For a large Terraform-based kind
(`ContainerCluster`, `KMSCryptoKey`, `ComputeVPNTunnel`) most of that cost
is the update path: comparing and writing every field of the object, as
`ComputeRouter` does. How the API changes the missing field decides which
lighter pattern fits; check it per field before building.

1. **The API takes the field only at create: a create-only whole-object
   kind.** Nothing beside Config Connector's object can add such a field
   later, so the resource has to be made by our kind. But that kind is
   small: create, read, delete and labels, with every other field
   immutable by CEL (decision 20) and nothing compared or written after
   create. Upstream's direct types and mapper are copied where they
   exist. Examples: every field of a VPN tunnel but its labels
   (`cipherSuite`), a KMS key's `cryptoKeyBackend`, an interconnect
   attachment's IPv6 candidates. **Applied first in `ComputeVPNTunnel`**
   (section 6.5).
2. **The API changes the field by an update mask: a field kind beside
   Config Connector's object.** A small kind of ours names Config
   Connector's object (or the resource), reads the resource, and patches
   only its own fields with a mask that names them; Config Connector
   keeps owning the rest. It is safe only when Config Connector's update
   never sends the field: decision 11 rejected it for `RunService`
   because Terraform's Cloud Run update sends the whole service with no
   mask and resets the fields. So the kind's live test changes Config
   Connector's object after ours has set the field and checks that the
   field survives. Candidates: `KMSCryptoKey`'s
   `keyAccessJustificationsPolicy` (Terraform's update of a key names the
   changed fields in its mask; to be confirmed live), GKE cluster fields
   (`clusters.update` takes one `desired*` change at a time) and node pool
   `kubeletConfig` (`nodePools.update`). **Built 2026-11-16** as
   `KMSCryptoKeyFields`, `ContainerClusterFields` and
   `ContainerNodePoolFields` (decision 37, which also names the kind).
   A node pool's `kubeletConfig` is the partial case: the API takes it
   whole, so ours sends it merged, and Terraform's update of Config
   Connector's own kubelet fields can reset ours until our next reconcile.
3. **Otherwise** (the field is mutable and Config Connector's update would
   reset it), a whole-object kind as before.

One pattern does not fit all: a create-only field rules out the field
kind, a mask-less upstream update rules it out too, and a create-only
whole-object kind would take a mutable resource from Config Connector for
no reason.

**The field kinds' leftovers (2026-11-23).** Foundation items 11, 15 and
16 left fields no field kind can own. Upstream read again first
(read-only: `master` at `bc464f55`, releases `v1.161.0` and `v1.162.0`,
open pull requests by search, #12521's head `4e94076d`): none has them.
Items 11 and 16 rank above the security-adjacent items
(`docs/coverage/foundation.md`: importance first), and `hack/discokind`
generates a whole-object kind for each at little cost, so both were built
as pattern 3: `ComputeServiceAttachment` (`consumerAcceptLists[].networkUrl`,
`domainNames`, `metadata`, `natIpsPerEndpoint`) and `ComputeSubnetwork`
(the fields the API takes only at insert), section 2.4. Item 15's
leftovers were not built: its routing policy fields are not gaps
(`DNSRecordSet` has them under Terraform's names; the coverage now says
so), and the forwarding targets' `ipv6Address` and `domainName` need
hand-written whole-object `DNSManagedZone` and `DNSPolicy` kinds (copies
of upstream's opt-in direct controllers) that no known workload needs
yet (section 6), so they wait below the security-adjacent items.

## 7. What is left

Before config-connector-extensions can run on a real cluster:

1. Images are built locally by ko (`config/default` uses `ko://`
   references). A registry, signing and a release version scheme are not
   decided.
2. Done: install manifests (`config/default`) and namespaced identity
   matching Config Connector's (section 9).
3. Proven only on kind: Workload Identity itself (section 9.3).
4. Secret change tracking for `spec.password` (Config Connector's
   observed-secret-versions), and watching referenced Secrets.
5. Validation webhook parity: Config Connector's webhooks enforce
   immutability (`host`, `type`, `resourceID`); ours relies on the
   controller erroring. Either CEL `x-kubernetes-validations` in the CRD
   or a webhook.
6. Done: the managers' own Kubernetes service account, the default, with
   sharing Config Connector's as an opt-in (section 9.2).

To raise confidence in the kinds:

7. Port Config Connector's `SQLUser` fixtures (`sqluser`, `sqluser-full`)
   to our group and run its golden harness, recording `_http.log`.
8. Done (2026-11-09, `docs/live-runs.md`): against a real Cloud SQL
   Postgres 16 instance, `databaseRoles` on insert, adding a role and
   revoking one all match mockgcp, and `users.get` returns the roles. Not
   yet run live: an empty list (revoke everything).
9. Unit tests for `passwordPolicyEqual`, `sortedRoles` and identity
   parsing (Config Connector has an identity conformance helper,
   `apis/common/identity`).
10. Done 2026-11-10: fifteen mockgcp patches offered upstream, 0001 and
    0002 among them (UPSTREAM.md, "Pull requests we opened upstream").
    The comment on PR #7148 with the revoke semantics and the mockgcp
    change is drafted and waits for the builder's approval.

Housekeeping:

11. Done: the SQLUser test runs on `pkg/test/mockenv`, so it runs live too.
12. CI: `task check` on Linux (the 127.0.0.2 default then works without
    patch 0002, but the patch stays harmless).
13. `tests/operator/run.sh` does not clean the fake cloud on a reused kind
    cluster; run it on a fresh one.

To raise confidence in `ArtifactRegistryRepository`:

14. Done (2026-11-09, `docs/live-runs.md`): the API refuses a change of a
    custom upstream's URI ("Remote source configuartion cannot be
    updated"; patch 0016 makes mockgcp answer the same), so the object goes
    not Ready and the repository keeps its URI; Terraform replaces the
    repository instead. Done 2026-11-16 (decision 21, ARURI): ours
    replaces it too.
    The API also refuses, at creation, a remote upstream it cannot reach,
    unless `disableUpstreamValidation` is set; mockgcp does not check.
15. Watch PR #13517 (UPSTREAM.md, "Copied from open pull requests"); when
    it merges and a release ships it, retire ours (section 3.4).
16. The workload identity kinds against a real project: the API's
    errors for a missing pool, an ID still soft-deleted, and a namespace
    owned by a Google service (`ownerService`), none of which mockgcp
    models, and which `googleCloudResource` forms the API accepts in an
    attestation rule; then correct mockgcp.
17. `RunService` against a real project: which server defaults the real
    API fills in beyond mockgcp's (ports, startup probe, traffic), each of
    which must be taken from what was read or it shows as a difference on
    every resync; that the API accepts a top-level update mask for
    `template` and `invokerIamDisabled`; and a GPU service actually
    scheduling with `nodeSelector` and `gpuZonalRedundancyDisabled`.
18. `ArtifactRegistryTag` against a real project: a tag on a version that
    exists, the API's error for one that does not, and a package whose ID
    has a "/" (`%2F` in the name), which the fake test leaves out.
19. `IAMWorkloadIdentityPoolProviderKey` against a real project: the key's
    real certificate and validity window, its use from a SAML provider, the
    API's error for a provider that does not exist or is not SAML, and the
    soft-deleted ID's reuse; then correct mockgcp.
20. `ComputeRegionNetworkEndpoint` against a real project: the API's checks
    by group type (an IP address in an `INTERNET_FQDN_PORT` group, a VM in
    another zone), which mockgcp does not make, and the endpoint's identity
    when the group holds several with the same address and port.
21. `ComputeRouterRoutePolicy` against a real project: the API's checks of
    the CEL expressions and priorities, which mockgcp does not make, and a
    write with a stale fingerprint.
22. `StorageBucketObject`: watch the referenced Secret or ConfigMap so a
    change is uploaded at once, not at the next reconcile; and, against a
    real project, a resumable upload for content near the size limit
    (mockgcp serves media and multipart only).
23. `ComputeRegionNetworkFirewallPolicy` against a real project: whether a
    patch with a stale fingerprint is refused, whether `description: ""`
    clears the description, and deleting a policy that is still
    associated with a network (mockgcp checks none of these).
24. `LoggingSettings` against a real organization and folder: the
    service accounts' real names, the API's checks of the key (it must
    exist and the Log Router's account must be able to use it) and of the
    location, and `defaultSinkConfig` (newer than mockgcp's protos).
25. `ComputeProjectMetadataItem`: the 412 retry is not exercised by the
    fake (its writers do not race the way real ones can); the default
    network tier and default service accounts, rank 10's other parts.
26. `IAMBinding` and `IAMMember`: live runs of the rows in
    `pkg/iampolicy/registry_more.go` that have only been checked for
    their names and URLs (each service's quirks: GET or POST for the read,
    the API version, the policy version it accepts; a BigQuery dataset has
    no IAM policy, only `access`); `memberFrom` sources beyond a service
    account and a log sink; and a check that the policy's etag conflict is seen live (the
    fake serves 409; the API's real answer is 409 for most services and
    412 for Compute Engine).
27. `ComputeRegionNetworkFirewallPolicyRule` against a real project: the
    API's checks of a rule (`goto_next` with logging, secure tags together
    with target service accounts, FQDNs or region codes on the wrong
    direction, the policy's tuple limit), which mockgcp does not make;
    `targetResources`, the field this kind lacks (`securityProfileGroup`,
    `tlsInspect`, the network contexts and `match.srcNetworks` were added
    2026-11-23, fake only; the group case live needs
    `E2E_SECURITY_PROFILE_GROUP`); how long a changed rule stays "not
    ready".
28. `ComputeRouter`: a rotated MD5 key Secret is noticed at the next
    reconcile (Config Connector's resync interval), not when the Secret
    changes (no watch, as item 22); peers and interfaces the API manages
    for an interconnect attachment are kept on update but no test makes
    one (mockgcp never does); done (2026-11-10):
    `ComputeRouterRoutePolicy.routerRef.name` names ours as well as Config
    Connector's `ComputeRouter`, and refuses a name both have; done
    (2026-11-10): a CEL rule refuses an interface with more than one of
    `vpnTunnelRef`, `interconnectAttachmentRef` and `subnetworkRef`, as
    the API does; the Terraform-based
    `ComputeRouterInterface`, `ComputeRouterPeer` and `ComputeRouterNAT`
    on the same router would fight ours, so moving a router means
    abandoning all four official objects.

29. `IAMDenyPolicy`: a live run (it needs Deny Admin, granted on an
    organization: the test estate); the API's checks of principals and
    permissions (an unknown permission, a principal type a deny rule does
    not take), which mockgcp does not make; how long a new policy takes to
    apply; and a policy's `denialCondition`, which no test sets.
30. `ComputeOrganizationSecurityPolicy` and its association: a live run
    (the test folder: a policy under it, attached to it, and a
    `CLOUD_ARMOR` policy attached to a project by number, with and
    without exclusions on the folder); whether the API refuses exclusions
    on a project's association as fabric does (ours refuses them by CEL);
    what the real
    API defaults (`type`, rules it adds) and whether a resync then writes
    nothing; the API's checks of a parent or attachment the caller cannot
    use, which mockgcp does not make.
31. `ComputeOrganizationSecurityPolicyRule`: a live run (the test folder:
    a FIREWALL policy under it and its rules); whether the beta API still
    takes FIREWALL policies and rules (v1 has only Cloud Armor's); what
    `getRule` answers for a missing priority (assumed 400 "does not
    contain a rule at priority", as for firewall policies); whether
    `patchRule` keeps fields the body leaves out (the controller sends
    every field, so either way it converges); the default rules the API
    adds to a FIREWALL policy, and whether `targetResources` comes back
    as full URLs (compared either way).
32. `ComputeNetworkFirewallPolicyRule`: the API's checks of a rule that
    mockgcp does not make (as item 27 for the regional rule), whether
    `srcNetworkContext` is accepted for every value it lists, and the
    fields it lacks (`targetResources`, the network types).
    `securityProfileGroup`, `tlsInspect` and `match.srcNetworks` were
    added 2026-11-23 (foundation item 19, fake only): a live run of the
    group case needs an existing security profile group,
    `E2E_SECURITY_PROFILE_GROUP` (Cloud NGFW Enterprise); with it,
    `task test-live PROJECT=$GCP_PROJECT KINDS=compute
    LIVE_RUN=TestComputeNetworkFirewallPolicyRule`. To be seen live: the
    form the API answers for the group and a network (compared by path),
    and that `tlsInspect` and the group are cleared by `false` and `""`.
    The hierarchical policy's rule with the same fields is our
    `ComputeFirewallPolicyRule` (item 48).
33. `LoggingBillingAccountSink`: a live run (it needs a billing account
    the live-test service account administers logging on:
    `E2E_BILLING_ACCOUNT`); the writer identity's real address (the fake
    makes one up); whether the API checks the destination at create; and
    a sink named by a `BillingAccount` object, which no test makes.
34. `IAMPrincipalAccessBoundaryPolicy`: a live run (it needs
    `iam.principalaccessboundarypolicies.*` on an organization, which a
    live run never touches); what the API defaults
    (`enforcementVersion`), whether a resync then writes nothing, and
    the checks mockgcp does not make (resource names in rules, the 500
    resource limit, an `effect` other than `ALLOW`).
35. `SecurityCenterMuteConfig`: a live run (a project's needs Security
    Command Center active and `securitycenter.muteconfigs.*`; a folder's
    and an organization's the test folder); whether the API names a
    project's mute config by its number; the API's checks of a filter;
    and a dynamic mute config's `expiryTime`, which no test sets. The
    other Security Command Center kinds of decision 16 (BigQuery
    exports, notification configs, the two custom module kinds) are
    next, on the same parent.
36. `DNSResponsePolicy`: a live run (needs `dns.googleapis.com` on in
    `$GCP_PROJECT` and `roles/dns.admin` for `cce-live-tests`); whether
    the API returns `networkUrl` with the project ID or its number (the
    comparison normalizes the form, not the project); the API's refusal
    to delete a bound policy, which mockgcp does not make; `gkeClusters`,
    which no test sets (it needs a GKE cluster).
37. `ComputeNetworkFirewallPolicyPacketMirroringRule`: the live patch
    is done (run `cce-11120932-1df3`: the test had emptied an INGRESS
    rule's only source, which the API refuses; it now empties the
    destination, and the fake refuses an INGRESS rule without a source);
    left: a live `mirror`
    rule (a security profile group with a custom mirroring profile and a
    mirroring endpoint group); the hierarchical policy's packet mirroring
    rules, built 2026-11-23 as `ComputeFirewallPolicyPacketMirroringRule`
    (`compute.gcp.hfl.dev/v1alpha1`; `firewallPolicies.*PacketMirroringRule`
    are beta only and in neither pinned Go client, so the controller sends
    them as REST; mockgcp patch 0058), live pending the test folder; a
    regional kind, which the API has no rule
    calls for; `targetSecureTags`, which no test sets.
38. `EndpointsService`: a live run (needs `roles/servicemanagement.admin`
    for `cce-live-tests` and `servicemanagement.googleapis.com` on;
    `E2E_SERVICE_MANAGEMENT=true`; run `cce-11130544-c338` was refused
    the list); whether the API answers a get of a missing or deleted
    service with 404 or 403, which `Find` takes alike; a gRPC service (no test makes a
    descriptor set; only the OpenAPI path runs); the undelete of a name
    held by a service deleted within 30 days, which only the fake shows;
    rollouts that split traffic between configurations (Terraform has
    none either); the API's own reading of the configuration, which the
    fake does not parse.
39. `IAMPolicyBinding`: a live run (needs a principal access boundary
    policy, which lives under an organization, that `cce-live-tests` may
    bind, and `iam.policyBindings.*` on the project;
    `E2E_IAM_POLICY_BINDING_POLICY`); the API's checks of a principal set
    against the binding's parent and of a condition's attributes, which
    the fake does not make; `ACCESS` policies, which the fake does not
    serve.
40. `ChronicleRuleDeployment`, `ChronicleRule` and `ChronicleReferenceList`: a live run (needs a Google SecOps instance
    in or bound to the live-test project, `chronicle.googleapis.com` on
    and `roles/chronicle.editor` for `cce-live-tests`;
    `E2E_CHRONICLE_INSTANCE=<location>/<instance>`), and with it the
    sweeper's Chronicle section (the test deletes its rules itself; no
    live run has made any); whether `<location>-chronicle.googleapis.com`
    (Terraform's host) is the right host for every location; the API's
    coercion of legacy run frequencies, which only Terraform's diff
    suppression (copied) tells us; whether an archive and a disable in
    one patch would do (the controller sends them in two, disable
    first).
41. `AssetFeed`: a live run (needs `roles/cloudasset.owner` for
    `cce-live-tests`, then `E2E_CLOUD_ASSET=true`; the APIs are on and
    the account has `roles/pubsub.admin` for the test's topic); whether
    the real API takes `projects/<id>/feeds/<feed>` in a get and a delete
    as the fake does (the controller uses the recorded number once the
    feed exists); the sweeper does not list feeds (the test deletes its
    own); a folder's and an organization's feed live.
42. Low priority, not started (the builder's RECPF answer, 2026-11-15:
    "SCC v2 mute config and project SHA module kinds at low priority"),
    for the catalog's `scc_mute_configs` and `scc_sha_custom_modules`
    (its `variables.csv`, `project` and `folder` rows):
    - a Security Command Center v2 mute config,
      `{organizations|folders|projects}/{id}/locations/global/muteConfigs/{id}`
      (Terraform's `google_scc_v2_organization_mute_config`,
      `_folder_mute_config`, `_project_mute_config`; fabric's
      `scc_mute_configs`: `description`, `filter`, `type`). Our
      `SecurityCenterMuteConfig` is the v1 API's
      (`{parent}/muteConfigs/{id}`), so this is a new kind with one parent
      reference field (decision 18), probably generated by `hack/discokind`
      if the v2 collection has the standard shape;
    - a project's (and folder's) Security Health Analytics custom module,
      `{parent}/locations/global/securityHealthAnalyticsCustomModules`
      in `securitycentermanagement.googleapis.com` (Terraform's
      `google_scc_management_project_security_health_analytics_custom_module`
      and the folder and organization ones). Config Connector's
      `SecurityCenterManagementSecurityHealthAnalyticsCustomModule` is
      `v1alpha1` and takes an `organizationRef` only, so ours would be the
      stable equivalent with all three parents (decisions 16 and 26).
    Both need Security Command Center active for a live run, as
    `SecurityCenterMuteConfig` does (item 35).

33. `ComputeVPNTunnel`: a rotated shared secret is not applied (the API
    takes it only at insert; make a new tunnel); the API's checks of a
    cipher suite (which algorithm names and combinations it takes) and
    whether `vpnTunnels.get` returns the suite as sent; a live
    `params.resourceManagerTags` (added 2026-11-23, fake only: the
    live-test project has no tag to bind). Of the other fields of
    foundation item 10, `ComputeInterconnectAttachment`'s IPv6 candidates
    and `l2Forwarding` are in our generated kind (2026-11-13);
    `ComputeInterconnect`'s `applicationAwareInterconnect` and
    `aaiEnabled` need a stable `ComputeInterconnect` of ours (Config
    Connector's is alpha only; decision 26), not built: a real
    interconnect is a physical circuit no live run can order.

45. `PrivateCACAPoolFields` and `PrivateCACertificateTemplateFields`
    (foundation item 25, 2026-11-23): a live run, once the live-test
    account has Private CA (`E2E_PRIVATECA=true task test-live
    PROJECT=$GCP_PROJECT KINDS=privateca`): whether the API takes back a
    whole issuance policy or predefined values as read, and its checks of
    the name constraints and the backdate. Built 2026-11-23 from item 25:
    `PrivateCACertificateAuthority` (ours, section 2.3; fake only): a live
    run (`E2E_PRIVATECA=true task test-live PROJECT=$GCP_PROJECT
    KINDS=privateca LIVE_RUN=TestPrivateCACertificateAuthority`; a root
    authority in a DevOps tier pool, billed by the hour while it exists,
    deleted with `skipGracePeriod`): whether the API keeps a
    `subjectKeyId` sent or computes its own, the error for deleting an
    enabled authority (patch 0062 assumes FailedPrecondition), whether an
    update with the mask `labels,user_defined_access_urls` is taken, and
    whether the pool can be deleted at once after a delete without the
    grace period. Not built from item 25: a pool's `encryptionSpec`
    (CMEK): neither the pinned Go client (`cloud.google.com/go/security`
    v1.24.0, Config Connector's pin) nor mockgcp's protos have it, so a
    kind for it would send REST (`google.golang.org/api` has the field)
    and need an HTTP-layer fake that keeps a field the protos lack; wait
    for the pin's Go client to have it (a newer client than Config
    Connector's pin would be a dependency change, the builder's call);
    the authority's `subordinateConfig` (activation of a subordinate
    authority) and the subject's `rdnSequence`; and
    `allowRequesterSpecifiedNotBeforeTime` (not in the pinned Go client).

47. `ComputeSecurityPolicyFields` (foundation item 27, 2026-11-23): a
    live run (`E2E_CLOUD_ARMOR=true task test-live PROJECT=$GCP_PROJECT
    KINDS=compute LIVE_RUN=TestComputeSecurityPolicyFields`): whether the
    API merges the sub-object Terraform's update sends or replaces it
    (ours come back at the next reconcile either way), whether it takes
    the nested mask paths that clear, and whether threshold configs need
    Cloud Armor Enterprise. Not built from item 27: `userDefinedFields`,
    `ddosProtectionConfig` (regional network edge policies) and rule
    fields.

48. `ComputeFirewallPolicyRule` (ours, foundation item 19, 2026-11-23;
    section 2.3): a live run, pending the test folder (`task test-live
    PROJECT=$GCP_PROJECT KINDS=compute
    LIVE_RUN=TestComputeFirewallPolicyRule` with `E2E_TEST_FOLDER`; the
    security profile group case also needs `E2E_SECURITY_PROFILE_GROUP`, a
    group a rule under the folder may use, and skips without it; the
    secure tag case runs on the fake only). To be seen live: whether
    `patchRule` keeps a list the body leaves out (the controller sends
    `[]` either way), whether it takes the body without `priority` as
    upstream sends it, the form the API answers for a network, a group
    and a target network (compared by path), and its checks of a rule
    that mockgcp does not make. Not built from item 19: the network
    types, `targetForwardingRules` and `targetType`, a global or regional
    network rule's `targetResources` (the regional rule's `srcNetworks`,
    network contexts, group and TLS inspection were added the same day,
    item 27).

49. `ComputeServiceAttachment` (ours, generated; foundation item 11,
    2026-11-23): a live run (`task test-live PROJECT=$GCP_PROJECT
    KINDS=compute LIVE_RUN=TestComputeServiceAttachment`; its Setup makes
    two producers, each an internal passthrough load balancer and a PSC
    NAT subnet, free but for the forwarding rule's hourly price). To be
    seen live: an accept list entry by `networkUrl`, `domainNames` at
    create, the API's defaults (`propagatedConnectionLimit`) and whether a
    resync then writes nothing, and `metadata` and `natIpsPerEndpoint`,
    which mockgcp cannot show.

50. `ComputeSubnetwork` (ours, generated; foundation item 16,
    2026-11-23): a live run (`task test-live PROJECT=$GCP_PROJECT
    KINDS=compute LIVE_RUN=TestComputeSubnetwork`; the `insertonly` case
    skips live). To be seen live: whether `subnetworks.patch` takes the
    body the generated controller sends (only the fields that differ, the
    fingerprint and the name), the API's defaults for a new subnet
    (`stackType`, `privateIpv6GoogleAccess`, `logConfig`) and whether a
    resync then writes nothing; with a BYOIP sub-prefix and a tag in the
    project, the `insertonly` case. Not built: `expandIpCidrRange` and
    `setPrivateIpGoogleAccess`.

46. `AccessContextManagerAuthorizedOrgsDesc` (foundation item 21,
    2026-11-23; section 2.3): a live run needs an organization's access
    policy and Access Context Manager roles on it for the live-test
    account, which a live run does not touch (`live-skip`); and the move
    to Config Connector's kind once #14202 is released. Item 21's
    perimeter fields are in our generated
    `AccessContextManagerServicePerimeter`
    (`accesscontextmanager.gcp.hfl.dev/v1alpha1`, 2026-11-23): Config
    Connector's Terraform-based perimeter lacks, in both `status` and the
    dry-run `spec`, an egress rule's `sources` and `sourceRestriction`, the
    rules' `roles` and `title`, PSC endpoint sources and
    `vpcAccessibleServices.allowedServicePatterns`, and Terraform's update
    sends either configuration whole, so ours owns the whole perimeter.
    Its live run waits as this kind's does (`live-skip`); to be seen live:
    whether `servicePerimeters.patch` takes the top-level mask
    (`status`, `spec`) the controller sends, and whether the API answers
    rules as sent (order, `*`). It cannot be used beside Config
    Connector's perimeter or `AccessContextManagerServicePerimeterResource`
    for the same perimeter. The alpha-only user access binding's fields are
    not built: the API names a binding itself, which `hack/discokind` does
    not generate.

44. `IAMOauthClientCredential` and `IAMWorkforcePoolProviderScimToken`
    (decision 38, 2026-11-23): a live run. The credential's
    (`task test-live KINDS=iam LIVE_RUN=TestIAMOauthClientCredential`,
    which makes its own OAuth client) shows whether the API returns
    `clientSecret` on every read as its description says, and whether
    it refuses to delete an enabled credential (the controller disables
    it first either way; the fake refuses). The token's needs
    organization-level access and a SCIM tenant (Gemini Enterprise
    only), as the tenant's does (`live-skip`). Not built: a Secret that
    survives its object's abandon (the Secret is owned and goes with it;
    a new object for the same credential writes it again, and a token's
    cannot be).

43. A delete that still needs a reference (2026-11-16, `pkg/deleting`;
    updated 2026-11-17). Every kind now deletes by its recorded name and
    resolves nothing, and abandon resolves nothing, except two:
    `SQLAdminBackup` reads its `SQLInstance` object when `instanceRef`
    is by name (the backup's recorded name has no instance) and
    `ComputeOrganizationSecurityPolicyAssociation` its policy object
    when `securityPolicyRef` is by name; each finishes without a call if
    that object is gone. (`SQLUser`,
    `ComputeDiskResourcePolicyAttachment`,
    `ComputeRegionDiskResourcePolicyAttachment`,
    `ComputeNetworkEndpoint`, `ComputePerInstanceConfig`,
    `ComputeRegionPerInstanceConfig`, and `ArtifactRegistryRepository`
    gained `status.externalRef` and now delete from it without resolving
    references; `noRecord` is no longer needed on any kind in
    `pkg/controller`.)

## 8. Decisions

1. **Upstream's unmerged `SQLUser` (PR #7148)**: use it once it does what
   this kind needs, and keep it easy to update from upstream. It does not
   yet (`databaseRoles` is `/*NOTYET*/`), so ours stays;
   `hack/upstream-pr.sh` follows the PR's head and section 2 ("Switching
   to PR #7148") says how to switch.
2. **Roles replace.** The list is the whole set; a role taken off it is
   revoked.
3. **Status field name**: `status.appliedDatabaseRoles` stays until a run
   against real Cloud SQL shows whether `users.get` returns roles
   (section 7, item 8).
4. **Managers follow Config Connector's mode.** In cluster mode one manager
   watches the whole cluster, because Config Connector has one Google
   identity there. In namespaced mode there is one manager per
   `ConfigConnectorContext`, watching only its namespace (section 9).
5. **Field gaps on large existing kinds** wait for Config Connector's
   release, with the exception in section 6.1. **Reversed on
   2026-11-10: decision 19.** Never a fork. Exceptions
   made: decisions 11 and 14 (and 13, 16 for whole
   kinds). A parallel kind copied from an upstream migration pull
   request, owning the whole object, is now the accepted pattern when a
   foundation chart is blocked; a deprecation path for our kinds, for
   when upstream ships theirs, will be designed separately (decided
   2026-11-09) and is not built.
6. **The fake-cloud switch exists only in test builds.**
   - `--fake-cloud-url` and `pkg/fakecloud` are compiled into the manager
     only with `-tags fakecloud` (`cmd/manager/fakecloud.go`).
   - A production build has no such flag and refuses to start with it
     ("flag provided but not defined").
   - `hack/check-no-fake-cloud.sh`, run by `task check`, fails if the
     default build links `pkg/fakecloud` or shows the flag.
   - The kind test builds its images with `GOFLAGS=-tags=fakecloud`.
7. **Licence**: Apache-2.0 (`LICENSE`, `NOTICE`).
8. **`upstreamCredentials`: build it now** (decided 2026-11-09,
   answering section 10's question 1 of 2026-11-08). The remote
   repository's `upstreamCredentials` and `disableUpstreamValidation` are
   written here from the API and Terraform's behaviour (section 6.4).
9. **Rank 9: build it now** (decided 2026-11-09, answering section
   10's question 2): `IAMWorkloadIdentityPoolNamespace` first, copying PR
   #14209's types, a direct controller written here on
   `google.golang.org/api/iam/v1`, and a minimal mockgcp patch; then
   managed identities (#14418) if they follow cleanly.
10. **Then rank 8, and on down the list** (decided 2026-11-09): build in
    rank order as far as possible, as many kinds as can be done, with the
    aim of confidence in a working operator. This overrides decision 5's "wait for the release"
    for rank 8 (`RunService` `invokerIamDisabled` and GPUs), to be built
    the smallest workable way rather than as all of `RunService`; then the
    unranked candidates in section 6.3, top to bottom, skipping any that
    Config Connector already has.
11. **Rank 8 as a narrow direct `RunService` that owns the whole service**
    (2026-11-09; agreed through the coordinator). Two ways were weighed:
    - a field patch beside Config Connector's `RunService`, re-applying
      `invokerIamDisabled` (and the GPU fields) after Config Connector's
      updates. Rejected: Config Connector's `RunService` is Terraform-based,
      and Terraform's update sends the whole service with no update mask,
      so every change to it (every deploy) resets the fields until the
      patch runs again; for GPUs the template would flip between revisions
      with and without the accelerator, and a service whose GPU limit has
      no node selector may not be creatable at all;
    - **chosen:** our own `run.gcp.hfl.dev/v1alpha1` `RunService`, copied
      from Config Connector's direct `RunWorkerPool` (the nearest upstream
      code: same API family, a revision template with containers,
      volumes, VPC access, service account and `nodeSelector`), owning the
      whole service. It updates with an update mask of the top-level fields
      that differ, so fields it does not model are left alone. A workload
      that needs `invokerIamDisabled` or a GPU moves its service to this
      kind (acquire by `resourceID`, delete the Config Connector object with
      `deletion-policy: abandon`); others stay on Config Connector's.
12. **IAM kinds are named as Terraform's Google provider names them:
    `IAMBinding` and `IAMMember`** (decided 2026-11-09, through the
    coordinator: Terraform's Google provider is the model for naming).
    Terraform's
    `google_<resource>_iam_binding` is authoritative for one role and
    `google_<resource>_iam_member` is additive, and ours follow that.
    Each is one generic kind in `iam.gcp.hfl.dev/v1alpha1` with a resource
    reference (`IAMBinding` is `v1beta1` since decision 24). `IAMBinding` is one role, an optional condition and a list
    of members on one resource, authoritative for that role and
    condition. `IAMMember` is one member of one role on one resource,
    additive. The two were first built as `IAMRoleBinding` (first accepted
    as the right name) and `IAMRoleMember` (a
    working name, section 10 question 3). They were renamed before
    anything used them, so nothing needed migrating.
    `IAMPolicyBinding`, the catalog's suggestion, is not used: Config
    Connector's open PR #14133 gives that name to IAM v3's principal
    access boundary `PolicyBinding`. The non-authoritative form is
    preferred, with authoritative accepted where it is already built, so Config
    Connector's `IAMPolicyMember` stays the default in everything written
    here, and rank 2 is built additive (section 6.5).
13. **Alpha-only foundation kinds: ship our own stable kinds** (decided
    2026-11-09, answering `docs/coverage/foundation.md`). IAM deny policies, principal access
    boundary, hierarchical security policy, network firewall rules and
    DNS response policies: copy upstream's alpha types and controllers
    into our group as stable kinds. When upstream promotes, users move
    over by acquiring the same resource (as with `RunService`, decision
    11).
14. **Cloud Router: copy upstream's PR as our kind** (decided
    2026-11-09). Copy PR #13960's
    direct `ComputeRouter` into our group as a complete kind that owns
    the whole router (BGP peer fields, NAT, interfaces, MD5 keys). A
    workload that needs these fields moves its router to ours (acquire by
    name, abandon the official object); ours is retired when upstream
    releases.
15. **Billing account log sink: build `LoggingBillingAccountSink` now**
    (decided 2026-11-09). Its live test needs billing-account
    access the builder has not granted: mark it "live: pending, needs
    billing account access" and prove it on the fake.
16. **Security Command Center at folder and project level: build ours
    with all three parents** (decided 2026-11-09): organization, folder and project, accepting that
    it duplicates what upstream is building and widens the kinds to
    retire later.
17. **`ProjectDefaultServiceAccounts` acts on every reconcile and keeps
    conditional bindings** (the orchestrator, 2026-11-09, accepting the
    two choices section 6.5 records for the kind). Terraform's
    `google_project_default_service_accounts` acts once, at create, and
    its read does nothing. Ours acts on every reconcile, because a
    Kubernetes object is desired state held continuously, not a one-off
    command: a default account re-enabled by hand, or one that appears
    when App Engine is turned on later, is disabled, deleted or
    deprivileged again, and status only grows. `DEPRIVILEGE` reads and
    writes the project's IAM policy as version 3, where Terraform reads
    version 1; a version 1 read drops the conditions of conditional
    bindings, and writing that back would lose them, so ours keeps them.
18. **A new kind is named as Config Connector names it, else as
    Terraform's Google provider does** (decided 2026-11-09: Terraform's
    Google provider is the model for naming). A new kind
    takes Config Connector's name for the resource where Config Connector
    has one, otherwise the Terraform google provider's resource name in
    CamelCase, in our group. Parents (organization, folder, project) are
    reference fields on one kind, not separate kinds per parent, as
    Terraform's `parent` argument is. So organization address groups are
    `NetworkSecurityAddressGroup` in `networksecurity.gcp.hfl.dev`, taking
    an organization or a project as its parent as
    `google_network_security_address_group`'s `parent` does, not
    `NetworkSecurityOrganizationAddressGroup`, and the security profile
    kinds follow the same rule. This generalises decision 12 (the IAM
    kinds) and the folder and organization `ServiceIdentity`, which kept
    upstream's name with new parents.
19. **Field gaps: build them now, and keep up with upstream mechanically**
    (decided 2026-11-10, reversing decision 5: do not wait on a release;
    ship now, expect upstream to ship many of the same resources and
    attributes later, and keep up with upstream mechanically). Field gaps that
    `docs/coverage` lists for foundation kinds Config Connector already
    serves are built here, in `docs/coverage/foundation.md`'s order,
    without waiting for an upstream release or pull request. They use the
    accepted pattern (decisions 11, 14, 13): our own kind in our group,
    copied from upstream's nearest code, owning the whole object, which a
    workload moves to by acquiring the same resource. Where that pattern
    does not fit a gap, the first pull request for it proposes another in
    a short design note. Upstream will ship many of the same fields and
    resources after us; the means of keeping up are mechanical: the pin
    and patches (section 3), `task coverage` and `task upstream-pr`, and
    the retirement path (section 3.4).

20. **Immutability is enforced by CEL rules in the CRD, not a webhook**
    (decided 2026-11-10, G4): `self == oldSelf` transition rules on
    the immutable fields, as the kinds already do; no admission webhook
    is added for it.
21. **A remote `ArtifactRegistryRepository` whose upstream URI changes is
    deleted and made again** (confirmed 2026-11-10, G3; built 2026-11-16,
    ARURI). The builder, 2026-11-16, verbatim: "ARURI: as recommended
    (Replace it, as decision 21 (2026-11-10) already says and Terraform
    does: a remote repository holds only a cache of the upstream, so
    replacing loses nothing that can't be refetched. Build it in
    config-connector-extensions.)"
    Built in the controller (`pkg/controller/direct/artifactregistry/replace.go`),
    not the CRD: when the repository in the cloud is a remote one and
    the spec's `customRepository.uri` (Docker, Maven, npm or Python)
    differs from the cloud's, including a public upstream swapped for a
    custom one or back, the update deletes the repository, waits, and
    creates it from the spec with the same name, as Terraform's
    `ForceNew` on `custom_repository` does. Every other change (the
    description, the upstream credentials, cleanup policies) is still an
    update in place, and a standard or virtual repository is never
    replaced. If the create fails after the delete, the next reconcile
    finds no repository and creates it. The field stays mutable in the
    CRD: no CEL rule is added (decision 20 is for fields that cannot
    change at all).
    Proven on mockgcp and live (run `cce-11160906-a434`,
    `docs/kinds/ArtifactRegistryRepository.md`): the real API takes a
    create with the same name at once after the delete.
22. **`status.appliedDatabaseRoles` stays until upstream PR #7148**
    (confirmed 2026-11-10, G5; section 10 item 2's
    recommendation): the field and the comparison with what was applied
    are kept until `SQLUser` switches to the PR (section 2), then follow
    the PR.
23. **Organization custom roles are checked at the organization level**
    (confirmed 2026-11-10, G8): Config Connector's
    `IAMCustomRole` with the `cnrm.cloud.google.com/organization-id`
    annotation covers them (foundation item 3), proven against an
    organization once the test folder exists.

24. **`IAMBinding` is `v1beta1`** (decided 2026-11-10, K12: promote
    `IAMBinding` to `v1beta1` first, then make it the default in 0.4.0, so that the catalog's charts, which refuse alpha kinds, can
    render it by default). The kind moved from `iam.gcp.hfl.dev/v1alpha1`
    to `v1beta1` with its schema unchanged, and `v1alpha1` is no longer
    served: the CRD has one version, as every kind here does. Nothing
    was released at `v1alpha1` (the repository has no release or tag) and
    the catalog had not shipped it, so no stored object needs converting.
    No conversion webhook (a kind per version is preferred to a
    second API version with conversion, and decision 20 keeps webhooks
    out). The Go types are in `apis/iam/v1beta1` with their own copy of
    the resource reference and condition types; `IAMMember` stays
    `v1alpha1`. An install that has `v1alpha1` objects abandons them
    (`deletion-policy: abandon`), deletes the CRD, and re-creates them at
    `v1beta1`, which takes over the same binding (`CHANGELOG.md`).
    Whether `v1alpha1` should instead have stayed served, deprecated, for
    a window is section 10, question 4.

25. **Version slots use Config Connector's own annotations, plus an opt-in
    slot guard** (2026-11-12, for Bundt's hosted slots): release is
    `deletion-policy: abandon`, adopt is acquire by name, pause is
    `actuation-mode: Paused`, all upstream's and already honoured; the
    guard (section 2.5) adds `hfl.dev/slot` on the object and
    `hfl-dev-slot` on labelled resources, refuses writes to a resource
    another slot holds, and lets `hfl.dev/slot-takeover` hand a resource
    over. Config Connector's own lease (`cnrm.cloud.google.com/management-conflict-prevention-policy`)
    is not used: it covers only some kinds, and Bundt decided against it.

26. **Where Config Connector has only an alpha kind, ours is a stable
    (`v1beta1`) equivalent** (the coordinator's direction, 2026-11-12,
    answering the second half of section 10 question 6). Decision 13 made
    this so for alpha-only foundation kinds; it now holds for any
    Config Connector alpha-only kind a catalog module needs, in the order
    of the catalog's `docs/coverage/blocked-kinds.md`. The kind keeps
    Config Connector's name (decision 18) in our group, is generated by
    `hack/discokind` where the API has the shape and copied or written
    here where it does not, and retires as any other (section 3.4) once
    upstream promotes. Our own kinds for gaps Config Connector has no
    kind for at all still start at `v1alpha1`; when they move up is the
    first half of question 6, still open.

27. **Our own kinds are promoted to `v1beta1` once they pass live**
    (the builder, 2026-11-13, answering the first half of section 10
    question 6: "Q6 yes" and "promote those three"). A kind of ours for a
    gap Config Connector has no kind for (`ComputeRouterRoutePolicy`,
    `ComputeRegionNetworkEndpoint`, `StorageBucketObject`, `IAMMember`
    and the rest) moves from `v1alpha1` to `v1beta1` once it has passed
    live once and its schema has had no breaking change since, the same
    way as `IAMBinding` (decision 24). `ComputeRouter`, `LoggingSettings`
    and `SecurityCenterMuteConfig` are promoted first, to unblock the
    catalog's VPN, folder and organization charts.
    **Built 2026-11-15** for those three, as decision 24 did it: the
    types moved to `apis/compute/v1beta1`, `apis/logging/v1beta1` and
    `apis/securitycenter/v1beta1` with their schemas unchanged, each CRD
    serves `v1beta1` only (no conversion, no webhook), and the fake-cloud
    tests check that `v1alpha1` is refused; `CHANGELOG.md` has the upgrade.
    `ComputeRouter` passed live on 2026-11-10 (`1ff3750`); its one schema
    change since (`5a0daca`, a CEL rule refusing an interface with more
    than one link, which the API refuses too) is not breaking.
    `LoggingSettings` and `SecurityCenterMuteConfig` have not run live:
    each waits on a prerequisite (a test folder and KMS key; Security
    Command Center on the test project, `docs/live-runs.md`), so they
    were promoted on the builder's explicit "promote those three", ahead
    of the rule; their first live run is still owed. `ComputeRouterRoutePolicy`
    stays `v1alpha1` and finds our router at `v1beta1`.
    The rest of our `v1alpha1` kinds, checked 2026-11-15 against the
    ledger (`docs/live-runs.md` and `docs/kinds/`): these passed live and
    their CRDs are byte-for-byte unchanged since that run, so they qualify
    and were promoted the same way on 2026-11-15 (the
    `artifactregistry`, `networksecurity`, `sqladmin` and `storage`
    `v1alpha1` packages are gone; `compute` and `iam` keep only kinds not
    yet promoted; `pkg/controller` tests that each promoted CRD serves
    only `v1beta1`): `ArtifactRegistryTag` (`08fcc66`),
    `ComputeProjectMetadataItem` (`c459e36`), `ComputeRegionNetworkEndpoint`
    (`c459e36`), `ComputeRegionNetworkFirewallPolicy` (`c459e36`),
    `ComputeRegionNetworkFirewallPolicyAssociation` (`c7f163f`),
    `ComputeRegionNetworkFirewallPolicyRule` (`39ded9d`),
    `ComputeRouterRoutePolicy` (`afe91be`), `IAMMember` (`a8c6de4`),
    `IAMWorkloadIdentityPoolNamespace`, `IAMWorkloadIdentityPoolManagedIdentity`
    and `IAMWorkloadIdentityPoolProviderKey` (`c459e36`, again `78644ec`),
    `NetworkSecurityAddressGroup` (`a9a2026`), `NetworkSecuritySecurityProfile`
    (`b7eae2b`, project parent; the organization parent waits on the test
    folder) and `NetworkSecuritySecurityProfileGroup` (`c64a5b6`),
    `SQLAdminBackup` (`08fcc66`) and `StorageBucketObject` (`756c69f`).
    `RunService` (`aee1155`) passed too, but Config Connector has a
    `RunService`; ours is a field-gap kind (decision 19), which this
    decision's wording did not name (section 10, question 9); decision 34
    extends the rule to it, and it is `v1beta1` since 2026-11-15. The other
    `v1alpha1` kinds have no live pass yet, or wait on a prerequisite
    (`LoggingBillingAccountSink`, `ServiceIdentity` (passed live and
    promoted 2026-11-15, decision 36), `ComputeProjectDefaultNetworkTier`, `ProjectDefaultServiceAccounts`),
    or failed their last run (none now: `ComputeNetworkFirewallPolicyPacketMirroringRule`
    passed at `78644ec` and was promoted 2026-11-16).

28. **A service's generated kinds' tests get their own package** (the
    builder, 2026-11-13, answering section 10 question 8: "Q8 yes, split
    the tests"). `hack/discokind` writes them to
    `pkg/controller/direct/<service>/discokindtest`, fixtures beside them,
    so the generated and hand-written halves run in parallel; no timeout
    changes. Done 2026-11-15: the hand-written files that register a
    kind's kindtest setup or run a kindtest case moved with the fixtures
    (they need them and the generated tests' binary), and the generator
    fails while a service's old `zz_discokind_test.go` is left behind.
    `hack/live/run.sh` runs `./pkg/controller/direct/<k>/...`, live with
    `-p 1`. Measured on one offload VM (c2d-standard-16, mock, two runs
    each, same commit before and after): `compute` went from 510 s and
    503 s wall (one package) to 282 s and 275 s (`compute` 275 s and
    268 s beside `compute/discokindtest` 227 s and 223 s), with the same
    2,286 tests and subtests passing; `go test -list` names the same 172
    top-level tests across `pkg/controller/direct/...` and
    `pkg/discokind/...`, in 47 packages instead of 40.

29. **Every patch is numbered** (the builder, 2026-11-13: "patches: the
    guard wins (always numbered), fix CONTRIBUTING.md"). `task
    upstream:yaml-check` reads only numbered names, so a new patch takes
    the next number from `main`, whatever files it touches;
    `CONTRIBUTING.md` and `docs/handoff.md` say so.

30. **Container Registry maps to Artifact Registry; no kind is built**
    (the builder, 2026-11-13: "Container Registry: map to Artifact
    Registry"). Google's Container Registry is shut down. The catalog maps
    Terraform's `google_container_registry` to an Artifact Registry
    `gcr.io` repository (Config Connector's `ArtifactRegistryRepository`)
    rather than a kind here.

31. **A refusal for quota requeues and never marks an object failed**
    (the builder, 2026-11-14, after the catalog's live runs in
    hfl-dev-514205: Dataplex's 10 requests a minute per user and region
    refused `DataplexLake`, `DataplexZone` and `DataplexAsset`, an asset
    never went Ready and deletes stalled for more than 25 minutes; IAM's
    `GetAccessPolicy`, 5 a minute per project, refused
    `IAMAccessPolicy`). Patch 0047 makes the direct reconciler, for an
    HTTP 429 or gRPC `RESOURCE_EXHAUSTED` from any step (find, create,
    update, delete), leave the status alone, record a `QuotaExhausted`
    event and requeue after `QuotaWait`: 15 s doubling to 10 min, half of
    it fixed and half random (`pkg/controller`). The workqueue's own
    backoff started at 250 ms, which a per-minute quota refused again. A
    discokind operation is read less often as it goes on (2 s, half as
    long again each time, up to 30 s), and a read refused for quota does
    not end the wait. A quota that never comes back (a project's CPUs)
    is then only an event and a log line, never a failed condition, as
    the builder asked ("never mark the object failed").
32. **A 403 while deleting a `missingIs403` kind counts as gone only
    when a listing proves it** (2026-11-14). The catalog repository's
    live run `cat-11141549-4cb0` deleted its `biglake-catalog` chart at
    once: BigLake's catalog delete took the database and table along,
    and their reads and deletes then answered 403, which BigLake gives
    for a missing resource even to an admin. `Find` took that 403 for a
    denial while deleting, and both finalizers held for 30 minutes; a
    Bundt component's delete would do the same. Now, while deleting, a
    403 on the read before the delete or on the delete itself starts a
    proof (`GoneAfter403`, `pkg/discokind/gone403.go`): the resource's
    collection is listed, and the resource is gone if the list does not
    show it. When that listing answers 403 too (a database's tables
    after its catalog went), the parent's collection is listed, and so
    on up while the parent is itself a `missingIs403` resource of the
    same API; the resource is gone when an ancestor is missing from its
    collection's listing (the project's catalogs list without the
    catalog). It stays a denial, and the finalizer stays, when a
    listing still shows the resource or its ancestor, or when the
    listings answer 403 all the way up. An item with `deleteTime` set
    counts as gone. **Why a listing, not the parent's read:** a 403 on
    the parent's get is as ambiguous as the child's (BigLake answers 403
    for a missing catalog too), so "the parent also answers 403" would
    take an account denied every BigLake read for a parent gone, and
    drop finalizers over resources still there. A listing that succeeds
    is a positive answer from the API about what exists, from the same
    role (`roles/biglake.admin` and `roles/biglake.editor` hold the
    `list` permissions), and at the top it is the project's own
    collection. Simpler rules were rejected: taking every 403 while
    deleting for gone hides a real denial and leaks the resource when
    the delete permission is taken away; giving up the finalizer after
    a timeout leaks it too, only later. The cost is one to three list
    calls after a 403 while deleting, none otherwise. Tested on the
    fake, where a BigLake delete now takes the resources under its name
    along and a list under a gone parent answers 403:
    `TestBigLakeDeleteAllAtOnce` (fake and live; it fails in a test
    build with the fix off, `go test -tags cce_delete403_off`) and
    `TestBigLakeDeleteDenied` (fake only: a denied table its database
    still lists keeps its finalizer until the denial is lifted; a table
    deleted behind the controller's back, its database still there, is
    proven gone by its own collection).

33. **`LookerInstance`'s OAuth client secret comes from a Kubernetes
    Secret** (2026-11-15, RECLOOK). The builder, 2026-11-15, verbatim:
    "RECLOOK: as recommended (Yes. Add a secretKeyRef for
    oauthConfig.clientSecret, then the chart takes client_secret from a
    Kubernetes Secret, as bigquery-connection does.)"
    Built: `spec.oauthConfig.clientSecret` is Config Connector's
    `refs/v1beta1/secret.Legacy`, the shape of `SQLUser`'s `password`
    and `SQLInstance`'s root password: `value`, or
    `valueFrom.secretKeyRef` (`name`, `key`) of a Secret in the object's
    namespace. `hack/discokind` makes any field listed under `secrets`
    in `kinds.yaml` so; `pkg/discokind` reads the Secret before every
    write and sends the string. **The plain value is kept, not
    deprecated**, as `clientSecret.value`: Config Connector keeps
    `value` beside `valueFrom` on every such field, and this repository
    follows its shapes (section 2.3); the catalog's chart only ever
    writes `valueFrom`. Setting both is refused ("set one of value and
    valueFrom, not both"). The string form is gone, a breaking change
    allowed at `v1alpha1` (no object of the kind has run live or been
    made by a chart). The API never returns `oauthConfig.clientId` or
    `clientSecret` ("Input only"), so before this they were a
    difference on every resync and every reconcile patched
    `oauthConfig`; now they are left out of the comparison (`inputOnly`
    and `secrets`), and `status.inputVersion`, a digest of the client
    ID and of the Secret's name, key and `resourceVersion` (never its
    value, as `ComputeRouter`'s `md5AuthenticationKeysVersion`), says
    when to send them again: a changed client ID, another Secret, or
    the same Secret rotated (seen at the next reconcile; the controller
    does not watch Secrets). Tested on the fake, which now keeps
    input-only fields but never answers them, as the API does:
    `TestLookerInstanceOAuthClientSecret` (a missing Secret holds the
    create; the Secret's value reaches the cloud and never shows in a
    read; a resync writes nothing; a rotated Secret is sent again; a
    plain value works; value and valueFrom together are refused) and the
    kind's fixtures, which now read the secret from a Secret. Live:
    pending, as for the kind (`docs/live-runs.md`): an instance is
    billed by the year and takes about an hour, and the Looker API is
    not on in the live-test project.

34. **Decision 27 covers our field-gap kinds** (the builder, 2026-11-15,
    EXTQ6-1, answering section 10 question 9: "as recommended (Yes:
    promote field-gap kinds under the same rule. The catalog's charts
    refuse alpha kinds whatever gap a kind fills.)"). A kind of ours that
    fills a field gap on a Config Connector kind (decision 19) moves to
    `v1beta1` under decision 27's rule, as a kind for a missing resource
    does. **Built 2026-11-15:** `RunService` (passed live at `aee1155`; its
    CRD and `apis/run` are unchanged since, checked with `git log
    aee1155..` on both) moved to `apis/run/v1beta1` with its schema
    unchanged; the CRD serves `v1beta1` only (no conversion, no webhook),
    the fake-cloud test checks that `v1alpha1` is refused, `pkg/controller`
    tests its served version with the other promoted kinds, and
    `CHANGELOG.md` has the upgrade. No other field-gap kind of ours
    qualifies (checked 2026-11-15 against every `docs/kinds/` file of a
    kind still `v1alpha1`): `ComputeRegionInstanceTemplate` (Config
    Connector's template is global only) passed live at `ceda3f6`, but its
    CRD changed since (`43177af`), so it needs another live pass first.
    The same check found kinds that qualify under decision 27 itself but
    are missing from its 2026-11-15 list, which read the ledger in
    `docs/live-runs.md` rather than the kinds' own files: the hand-written
    `ComputeNetworkFirewallPolicyPacketMirroringRule` (it failed at
    `e24d9c6` and `64c9c42`, then passed at `78644ec`; CRD unchanged since)
    and six generated kinds (CRDs unchanged since their passes):
    `ComputeRegionHealthAggregationPolicy` and `LoggingSavedQuery`
    (`e8a920c`), `ComputeRegionBackendBucket` and `LoggingLink`
    (`81d7b24`), `GKEHubRolloutSequence` and `IAMOauthClient` (`e1e23f5`).
    **The six generated kinds were promoted 2026-11-15** (their CRDs and
    types rechecked unchanged since those commits with `git log`): the
    version in `hack/discokind/kinds.yaml`, regenerated into each group's
    `v1beta1` package (the types differ only in the package name;
    `apis/gkehub/v1beta1` is new), the fixtures' `apiVersion`, the
    `LoggingSavedQuery` slot-move test, `pkg/controller`'s served-version
    test, `docs/kinds/` and `CHANGELOG.md`.
    `ComputeNetworkFirewallPolicyPacketMirroringRule` waited at
    `v1alpha1` until the catalog's `net-firewall-policy` chart could move
    with a pin bump; **it was promoted 2026-11-16** (types and CRD
    rechecked unchanged since `78644ec` with `git log`): the types move to
    `apis/compute/v1beta1`, the CRD serves `v1beta1` only, the fake-cloud
    test checks `v1alpha1` is refused, and `versions_test.go` lists it.
    The catalog's chart renders it at `v1beta1` without `allow_alpha`.
    `ServiceIdentity`, a kind for a missing resource, passed live on
    2026-11-15 and was promoted under decision 27 (decision 36).

35. **IAMBinding and IAMMember take a `CloudFunctionsFunction`; AssetFeed's
    parent comparison is fixed; Cloud Armor gets a stable association;
    SCC v2 mute configs and project Security Health Analytics custom
    modules are low priority** (the builder, 2026-11-15, RECPF, the parts
    for this repository: "yes to config-connector-extensions' IAMBinding
    taking CloudFunctionsFunction, AssetFeed's parent comparison fix, and
    a stable Cloud Armor association; SCC v2 mute config and project SHA
    module kinds at low priority."). The asks came from the catalog's
    pull requests #215 and #216. **Built 2026-11-15** here: the IAM
    resource table's `CloudFunctionsFunction` row (section 6.5, rank 2).
    The `AssetFeed` fix is pull request #134 (branch
    `agent/r2-assetfeed-armor`) and the Cloud Armor association #136, with
    their notes there; `LookerInstance`'s `secretKeyRef` (RECLOOK) is decision 33. The SCC v2 mute config and the project SHA
    module kinds are not started.

36. **One served version; the catalog's folder, organization and logging
    gaps** (the builder, 2026-11-15, CCEQ, answering section 10 questions
    4 and 7: "as recommended (Q4 keep one served version (decision 24).
    Q7: our own LoggingLogSink (v1beta1); no Folder kind (deletion-policy
    abandon or Bundt's guard); our own OrgPolicyPolicy only if the catalog
    can't order deletes; build the service-agents config under decision
    18. Artifact Registry URI change: none written.)").
    - **Q4:** decision 24 stands as built: `IAMBinding`, and every kind
      promoted since, serves one version; `v1alpha1` comes back (served,
      not stored, deprecated) only if a cluster turns up with `v1alpha1`
      objects that cannot be abandoned and re-created.
    - **`LoggingLogSink`** (`logging.gcp.hfl.dev/v1beta1`, with
      `interceptChildren` and a folder, organization or project parent):
      built in its own pull request (#137, branch `agent/r2-logsink`).
    - **No `Folder` kind.** The catalog maps
      `deletion_protection = true` to
      `cnrm.cloud.google.com/deletion-policy: abandon`, or to Bundt's
      deletion guard for "refuse to delete".
    - **`OrgPolicyPolicy` is not built while the catalog can order
      deletes.** What the catalog does today (its `folder` and
      `organization` charts' READMEs, "Config Connector bugs", read
      2026-11-15): it does not order them. The `folder` chart's README
      says a component should delete its org policies before the folder;
      its fake-cloud case leaves org policies out, so they are only
      render-tested. The `organization` chart's policies name the
      organization by `organizationRef.external`, never an object, so
      they do not hit the bug (a policy on a custom constraint of the same
      component still needs the constraint until it is gone). So the
      order has to come from the catalog or from Bundt's delete order
      (waves); if neither can give it, this repository builds
      `OrgPolicyPolicy` (`orgpolicy.gcp.hfl.dev`, `v1beta1`, decision 19).
    - **The service-agents config is our `ServiceIdentity`** (decision
      18: Terraform's resource names the kind). Cloud Foundation Fabric's
      `modules/folder` and `modules/organization` (v60.0.0, the catalog's
      pin, `service-agents.tf`, read 2026-11-15) make one
      `google_folder_service_identity` or
      `google_organization_service_identity` per service in
      `service_agents_config.services` when `create_agents` (default
      true), and compute each agent's email from the module's own
      `service-agents.yaml` and the folder's or organization's number
      (the `service_agents` output). Those are exactly the resources
      `serviceusage.gcp.hfl.dev` `ServiceIdentity` was built for (folder
      or organization parent, `status.email`; section 6.5, rank 7), so
      no new kind is built. `ServiceIdentity` ran live for the first time
      on 2026-11-15 on the test folder: the first run
      (`cce-11152217-c987`) failed because the API answers an operation
      already done, named `operations/finished.DONE_OPERATION`, that
      `operations.get` refuses, and the controller polled it; fixed (a done
      operation is not read again; patch 0051 makes mockgcp answer the
      same), the second (`cce-11152310-b63f`) passed. With its schema
      unchanged, it moved to `v1beta1` under decision 27 the same day.
      What the catalog needs: in each chart, a `ServiceIdentity` per
      service with `folderRef` (the chart's `Folder`) or
      `organizationRef.external`, when `create_agents`; the
      `service_agents` output from `status.email` (or fabric's
      `service-agents.yaml` templates); and its `variables.csv` rows
      corrected (the organization row says "`ServiceIdentity` takes only
      a project", which is Config Connector's kind, not ours).
    - **Artifact Registry URI change:** nothing to build. The builder's
      note says "none written", as the item did. Decision 21 (confirmed
      2026-11-10) already says a remote `ArtifactRegistryRepository` whose
      upstream URI changes is deleted and made again; that replacement is
      not built (section 7, item 14: the object goes not Ready and the
      repository keeps its URI).

37. **A field kind is `<Config Connector kind>Fields`** (the builder,
    2026-11-16, CCEQ5, answering section 10 question 5: "as recommended
    (`<Config Connector kind>Fields` in our group, with a resourceRef to
    Config Connector's object and the API's field names, e.g.
    KMSCryptoKeyFields for keyAccessJustificationsPolicy.)"). A kind of
    ours that owns a few fields of a resource Config Connector's kind owns
    (section 6.6, pattern 2) is named after that kind with `Fields`, lives
    in our group for the service (`<service>.gcp.hfl.dev`), names the
    resource by `spec.resourceRef` (Config Connector's own reference type:
    `name` of its object, or `external`), and keeps the API's field names
    and nesting. Rules every field kind follows (`pkg/fieldkind`):
    - it owns only the fields its spec sets, and never sends another;
    - it never creates or deletes the resource: a missing one is waited
      for, not Ready, "does not exist yet";
    - delete gives the fields back where the API can unset them (cleared),
      and leaves them where it cannot (said per kind); `deletion-policy:
      abandon` always leaves them;
    - a second object of the same kind for the same resource in the
      namespace is refused (the oldest owns it);
    - a delete uses the resource in `status.externalRef`, so it does not
      need Config Connector's object to still exist; an object that never
      wrote its resource (no `status.externalRef`) goes without a call
      (`fieldkind.Gone`; review C1 on #143).
    **Built 2026-11-16** (`v1alpha1`, promoted under decision 27 once each
    passes live; the two container kinds are `v1beta1` since their live
    pass): `KMSCryptoKeyFields` (`kms.gcp.hfl.dev`;
    `keyAccessJustificationsPolicy`, `cryptoKeys.patch` with a one-path
    mask); `ContainerClusterFields` (`container.gcp.hfl.dev`;
    `secretManagerConfig`, `rbacBindingConfig`,
    `anonymousAuthenticationConfig`, `addonsConfig.statefulHaConfig`,
    `nodePoolAutoConfig.nodeKubeletConfig.insecureKubeletReadonlyPortEnabled`,
    one `clusters.update` per field; delete leaves them);
    `ContainerNodePoolFields` (`kubeletConfig`'s
    `insecureKubeletReadonlyPortEnabled`, `containerLogMaxSize`,
    `containerLogMaxFiles`, `allowedUnsafeSysctls`,
    `maxParallelImagePulls`; the API takes `kubeletConfig` whole, so ours
    reads it and sends it with only its fields changed, and Terraform's
    update of Config Connector's own kubelet fields can reset ours until
    our next reconcile). Their files in `docs/kinds/` have the details
    and the live runs. 2026-11-23 (foundation item 14):
    `ContainerNodePoolFields` also owns `kubeletConfig`'s soft eviction
    settings, `memoryManager`, `topologyManager` and
    `singleProcessOomKill` (no upstream field or pull request); fake
    tested, live pending. Patches 0052 and 0053 teach mockgcp the writes.
    Live (2026-11-16): `ContainerClusterFields` and
    `ContainerNodePoolFields` passed (run `cce-11160848-7b8d`, every
    case, on a real zonal cluster) with their CRDs unchanged since, so
    both are `container.gcp.hfl.dev/v1beta1` (decision 27). Cloud KMS (run `cce-11160523-1e83`) refuses any
    `keyAccessJustificationsPolicy` on a key outside Key Access
    Justifications enrollment (Assured Workloads), so `KMSCryptoKeyFields`
    cannot pass live in the live-test project; section 10 question 10.
    **Built 2026-11-23** (`v1alpha1`, live pending), for the field rows
    of `docs/coverage/foundation.md` (decision 19):
    `ComputeServiceAttachmentFields` (`compute.gcp.hfl.dev`, item 11;
    `reconcileConnections`, `propagatedConnectionLimit`,
    `serviceAttachments.patch` as a merge patch with only those fields
    and the fingerprint; Config Connector's DCL patch never sends them;
    delete leaves them; patch 0054); `LoggingLogBucketFields`
    (`logging.gcp.hfl.dev`, item 12; `indexConfigs`, `restrictedFields`,
    `cmekSettings`, `buckets.patch` with a mask of only those; Config
    Connector's DCL and direct updates mask only their own fields; delete
    clears the two lists and leaves the CMEK key, which the API cannot
    remove; patch 0055); `ComputeNetworkFields` and
    `ComputeSubnetworkFields` (`compute.gcp.hfl.dev`, item 16; a network's
    `routingConfig` BGP best path selection and a subnet's
    `allowSubnetCidrRoutesOverlap`, `networks.patch` and
    `subnetworks.patch` as merge patches with only those fields;
    Terraform's updates send only the fields that changed, a network's
    `routingConfig` with `routingMode` alone; delete leaves them; patch
    0056); `DNSPolicyFields` (`dns.gcp.hfl.dev`, item 15;
    `dns64Config`, `policies.patch`, a partial update, with only that
    field; Terraform's patch never sends it, but Config Connector's
    opt-in direct `DNSPolicy` sends `policies.update`, a whole replace,
    and would clear it; delete turns DNS64 off; patch 0057);
    `PrivateCACAPoolFields` and `PrivateCACertificateTemplateFields`
    (`privateca.gcp.hfl.dev`, item 25; a pool's
    `issuancePolicy.backdateDuration` and
    `issuancePolicy.baselineValues.nameConstraints`, a template's
    `maximumLifetime` and `predefinedValues.nameConstraints`;
    `caPools.patch` and `certificateTemplates.patch` with top-level
    masks, the issuance policy and predefined values sent as read with
    only ours changed; Config Connector's DCL and direct updates mask
    top-level fields of their own diffs, so its change of its issuance
    policy or predefined values clears ours until our next reconcile,
    the node pool's partial case; delete clears them; patch 0060);
    `ComputeSecurityPolicyFields` (`compute.gcp.hfl.dev`, item 27; a
    global Cloud Armor policy's Adaptive Protection `thresholdConfigs`
    and `requestBodyInspectionSize`, `securityPolicies.patch` as a merge
    patch of the sub-objects as read with ours changed, an update mask
    only to clear; Terraform's update sends only the sub-objects it
    changed, no mask; not for a policy on Config Connector's opt-in
    direct controller, which sends its whole object).
    Item 13 (`OrgPolicyPolicy.alternate`) is not built: the API marks
    `alternate` deprecated, and `dryRunSpec`, which Config Connector's
    kind has, replaces it.

38. **A secret the API returns goes to a Kubernetes Secret, never to
    status** (the builder, 2026-11-23, answering section 10 question 11:
    option (b), as recommended). A generated kind whose API returns a
    secret writes it to a Secret in the object's namespace, owned by the
    object, named by a new `spec.secretOutput.name` (default the object's
    name), holding the value under the field's name; never in status.
    The `hack/discokind` option is `secretOutputs` (top-level read-only
    string fields), the mirror of decision 33's `secrets`, with
    `createOnlyOutputs` for one the API returns only in the create's
    answer. The manager's ClusterRole gains `create` and `update` on
    Secrets (`config/operator/rbac.yaml`;
    `TestManagerRoleWritesSecretOutputs`). A token returned only at create
    is written once; if its Secret is lost, the token must be deleted and
    made again.
    **Built 2026-11-23** (`pkg/discokind/secretoutput.go`):
    - the Secret is made, empty, before the create, so a Secret of that
      name the object does not own (no owner reference to it) is left
      alone and stops the create before the API makes a secret it
      returns once; the object is not Ready and says why;
    - after the create, the values come from the create's answer (or its
      operation's response) and a read; on every later reconcile, from a
      read, so a lost Secret of a value the API reads back is written
      again, and one it does not (`createOnlyOutputs`) makes the object
      not Ready with what to do: delete the object, which deletes the
      resource, and make it again;
    - the generated types leave the fields out of `status.observedState`
      (and the CRD's schema would prune them if the controller wrote
      them); only the Secret's and the fields' names reach logs, events
      and errors;
    - the Secret goes with the object through garbage collection, also
      when it is abandoned (`deletion-policy: abandon` keeps the
      resource in the cloud, not the Secret).
    Kinds: `IAMOauthClientCredential` (`clientSecret`, returned on every
    read) and `IAMWorkforcePoolProviderScimToken` (`securityToken`,
    returned only by the create), foundation item 4, both
    `iam.gcp.hfl.dev/v1alpha1`. The token is named after Terraform's
    `google_iam_workforce_pool_provider_scim_token` (decision 18); item 4
    and question 11 called it `IAMWorkforcePoolProviderScimTenantToken`
    before Terraform had it. One more `hack/discokind` option came with
    the credential: `disableBeforeDelete` (the API's description says a
    credential "should first be disabled" before a delete; Terraform's
    example sets `disabled = true`), so the controller patches
    `disabled: true` before the delete. Upstream (2026-11-23): no Config
    Connector kind or pull request, open or closed, for either
    (searches for `OauthClientCredential`, `ScimToken`, `scim`).
    Tested on the fake (`pkg/discokind/fake` makes a random secret, and
    answers a create-only one only in the create's answer):
    `TestIAMOauthClientCredential` and
    `TestIAMWorkforcePoolProviderScimToken` (the generated kind tests,
    which now check every secret-output kind's Secret, its owner, and
    that no value is in the object) and `TestSecretOutputs` (a taken
    Secret name stops the create with no API call and the Secret
    untouched; a lost `clientSecret` is written again; a lost
    `securityToken` makes the object not Ready and no second token is
    made; delete disables the credential first; and no value is in any
    log line, at every verbosity, or any event). Live: pending (section
    7, item 44).

## 9. The operator: Config Connector's identity, per namespace

The goal: install the CRDs and run the managers so that a gap kind acts
with the same Google identity Config Connector uses in the same
namespace.

### 9.1 How Config Connector's operator does it (read at the pin)

- **Cluster mode, Workload Identity.** One controller manager in
  `cnrm-system` runs as the Kubernetes service account
  `cnrm-controller-manager`, which the operator annotates with
  `iam.gke.io/gcp-service-account: <ConfigConnector.spec.googleServiceAccount>`
  (`operator/pkg/controllers/configconnector`,
  `objectTransformForWorkloadIdentity`). The Google side is an IAM binding
  of `roles/iam.workloadIdentityUser` for
  `serviceAccount:<project>.svc.id.goog[cnrm-system/cnrm-controller-manager]`,
  which the user makes when installing Config Connector.
- **Cluster mode, a key.** `spec.credentialSecretName` names a Secret in
  `cnrm-system`, mounted as volume `gcp-service-account` at
  `/var/secrets/google`, with `GOOGLE_APPLICATION_CREDENTIALS` pointing at
  `key.json` (`config/installbundle/components/manager/authmodes/withsecretvolume`).
- **Namespaced mode.** For each `ConfigConnectorContext` in namespace N, a
  StatefulSet with `--scoped-namespace=N`, running as
  `cnrm-controller-manager-N`, annotated with that context's
  `googleServiceAccount`, in `cnrm-system`, or in the context's
  `spec.managerNamespace` under `--manager-namespace-isolation=dedicated`
  (`operator/channels/packages/configconnector/<v>/namespaced/per-namespace-components.yaml`,
  `pkg/controllers/configconnectorcontext/namespaced.go`). A context's
  `requestProjectPolicy` becomes `--user-project-override` and
  `--billing-project`.

### 9.2 What ours does

Workload Identity is bound to a namespace and a service-account name. Two
ways give "the same auth in the same namespace":

- **Its own service account (the default, `--identity=own`).** The manager
  runs in Config Connector's namespace as its own Kubernetes service
  account, which the operator creates with the same
  `iam.gke.io/gcp-service-account` annotation as Config Connector's
  (copied from it), and binds only to this project's roles. The Google
  service account is the same; the Kubernetes rights are ours alone. It
  costs one more Workload Identity binding per Google service account
  (below). With a key Secret it costs nothing: the same Secret is mounted
  the same way.
- **Config Connector's service account (opt-in, `config-connector`).** The
  manager runs as Config Connector's own service account. No extra
  binding, but the manager then holds every Kubernetes right Config
  Connector's account has (section 9.4). Chosen by `--identity=config-connector`
  on the operator, or per scope by the annotation
  `gcp.hfl.dev/identity: config-connector` on the `ConfigConnector` object
  (every manager) or on one `ConfigConnectorContext` (that namespace's
  manager). The annotation `gcp.hfl.dev/identity: own` opts a scope back. A
  context's annotation wins over the `ConfigConnector` object's, which wins
  over the flag; any other value is refused.

`pkg/operator`:

- watches `ConfigConnector`, `ConfigConnectorContext`, Config Connector's
  service accounts, its own service accounts and its own Deployments, and
  in one pass applies what should run and deletes what it made that
  should not;
- cluster mode: Deployment and service account
  `cnrm-system/config-connector-extensions-manager`; with a key, the same
  Secret mounted the same way;
- namespaced mode: Deployment and service account
  `config-connector-extensions-manager-N` with `--scoped-namespace=N` and
  the context's project policy flags, in `cnrm-system` or (dedicated) the
  context's manager namespace, annotated with the context's
  `googleServiceAccount` (copied from `cnrm-controller-manager-N`);
- without a `ConfigConnector` object (Config Connector from its install
  bundle): cluster mode, Workload Identity if `cnrm-controller-manager`
  carries the annotation, else the bundle's `gcp-key` Secret;
- waits for Config Connector's service account for the scope before it
  makes anything: until it exists Config Connector is not set up there,
  and there is no identity to copy;
- binds the manager's service account to the managers' roles
  (`config/operator/rbac.yaml`): `config-connector-extensions-manager` (our
  kinds, read Secrets, events, read the Config Connector kinds ours
  reference) cluster-wide in cluster mode or by RoleBinding in N alone,
  and `config-connector-extensions-manager-cluster` (CRDs,
  `ConfigConnector`, namespaces) cluster-wide;
- itself runs in `config-connector-extensions-system` as its own service
  account, holds no Google credentials, never changes Config Connector's
  service accounts, writes only the service accounts it labels as its own
  (RBAC cannot limit `create` by name, so its role allows service-account
  writes in every namespace), and may bind only those two roles (RBAC
  `bind` by name).

**The one step outside the cluster, for Workload Identity.** Allow the
manager's service account to act as the Google service account Config
Connector uses for the scope. `<cluster-project>` is the project that owns
the cluster's workload identity pool, `<gsa>` the Google service account:

```sh
# cluster mode
gcloud iam service-accounts add-iam-policy-binding <gsa> \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:<cluster-project>.svc.id.goog[cnrm-system/config-connector-extensions-manager]"

# namespaced mode, for the context in namespace N (its googleServiceAccount)
gcloud iam service-accounts add-iam-policy-binding <gsa> \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:<cluster-project>.svc.id.goog[cnrm-system/config-connector-extensions-manager-N]"
```

Under dedicated isolation the namespace in the member is the context's
`spec.managerNamespace`. The operator logs the member it needs when it
first creates a manager's service account.

**The symptom when it is missing.** The operator cannot see it (it holds
no Google credentials), and the manager starts normally. Each gap object
then fails to reconcile: its `Ready` condition is `False` with a message
from the GKE metadata server that it could not get an access token for
the Google service account (`iam.serviceAccounts.getAccessToken`
permission denied, HTTP 403). Config Connector's own objects in the same
namespace keep working, which is the tell. Add the binding; the next
reconcile succeeds.

A static bundle was the alternative to an operator: cheaper, but it can
only cover cluster mode, because namespaced mode's managers depend on which
contexts exist.

### 9.3 What was proven (2026-11-07)

`pkg/operator` against envtest (`task test`): cluster mode with Workload
Identity (its own service account with Config Connector's annotation,
bound to our roles; Config Connector's service account left unchanged)
and with a key (the same Secret, no annotation); the switch to namespaced
mode (the cluster manager, its service account and bindings go, one
manager per context comes, with its own service account carrying the
context's Google service account and its RoleBinding only in its
namespace, and the billing-project flags); a context removed takes its
manager, service account and bindings; the opt-in to share Config
Connector's service account, on the `ConfigConnector` object and back; the
`--identity=config-connector` flag with one context opting back to its
own; an unknown identity refused; no manager before Config Connector's
service account exists; Config Connector without its operator.

`tests/operator/run.sh` (`task operator-e2e`) on kind 0.33 (Kubernetes
1.37) with the in-cluster fake cloud (`cmd/fakegcp`; managers get
`--fake-cloud-url`, which loads no Google credentials and since section
9.4 exists only in builds with `-tags fakecloud`), images built by ko
0.19.1 for linux/arm64, on a fresh cluster, 2026-11-08:

```text
ok: CRDs installed: secretmanagersecrets, secretmanagersecretversions, sqladminbackups, sqlusers (*.gcp.hfl.dev)
== cluster mode: one manager in cnrm-system as its own service account with Config Connector's identity
ok: manager pod runs as cnrm-system/config-connector-extensions-manager, mapped like Config Connector's to kcc@mock-project.iam.gserviceaccount.com
ok: the manager may manage gap kinds, and may not create Secrets, change ConfigConnector or create webhooks
ok: Config Connector's service account has no rights on gap kinds
== cluster mode: a regional secret and its version become Ready
ok: Ready: projects/mock-project/locations/us-central1/secrets/app-db-password
== the opt-in: share Config Connector's service account, then back
ok: shared: the manager runs as cnrm-controller-manager, its own service account is gone
ok: back to its own service account; Config Connector's has no rights on gap kinds again
== namespaced mode: Config Connector's operator moves to one manager per context
ok: team-a's manager runs as cnrm-system/config-connector-extensions-manager-team-a, mapped like Config Connector's to cnrm-system-team-a@mock-project.iam.gserviceaccount.com, with --scoped-namespace=team-a
ok: team-a's manager may manage gap kinds in team-a and not in team-b; Config Connector's team-a account has no rights on them
ok: team-a/app-config Ready through team-a's manager
ok: team-b/app-config untouched: no context, no manager
all checks passed
```

Found on the way: the manager had never been started as a binary (only
its controllers, in tests), and it panicked at start ("flag redefined:
alsologtostderr"): Config Connector's dependencies already register klog's
flags, so `klog.InitFlags` must not be called again. Fixed in
`cmd/manager` and `cmd/fakegcp`. mockgcp also needs a writable `/tmp`
(it links controller-runtime's envtest helpers), and ko must run with an
empty Docker config on a Mac (Docker Desktop's credential helper blocked
on the keychain).

Not proven, and cannot be on kind: Workload Identity itself (the GKE
metadata server exchanging the pod's token for the Google service
account's). What is proven instead is everything on our side of it: the
manager pod runs as its own service account in Config Connector's
namespace, carrying exactly Config Connector's Google service account for
the scope, with only this project's Kubernetes rights, so on GKE one more
binding (section 9.2) gives it Config Connector's identity. With the
opt-in it runs as Config Connector's service account itself, which needs
no binding.
Config Connector's own operator was not installed; its footprint (the
namespace, its service accounts with the annotation, `ConfigConnector`,
`ConfigConnectorContext`) was made by hand as its code says it makes them.

### 9.4 Why not share Config Connector's Kubernetes service account by default

On the Google side, sharing changes nothing: the same Google service
account, through the same Workload Identity binding. On the Kubernetes
side the two directions differ, read from Config Connector's install
bundle at the pin (`config/installbundle/components/clusterroles/` and
`releases/scopes/*`):

- **What our manager would gain by running as Config Connector's account.**
  Cluster mode (`cnrm-controller-manager`, all by ClusterRoleBinding):
  - `cnrm-admin`: create, update and delete on every
    `*.cnrm.cloud.google.com` group (180 groups), cluster-wide;
  - `cnrm-manager-cluster-role`: read CRDs and namespaces; full control of
    `validatingwebhookconfigurations` and of every
    `core.cnrm.cloud.google.com` resource;
  - `cnrm-manager-ns-role`: full control of Secrets, ConfigMaps, Services
    and events in every namespace.

  Namespaced mode (`cnrm-controller-manager-N`): the same roles, but
  `cnrm-admin` and `cnrm-manager-ns-role` only in N (and
  `cnrm-manager-ns-role` in `cnrm-system`); `cnrm-manager-cluster-role`
  stays cluster-wide.
- **What our manager needs:** our six groups, read Secrets, write
  events, and read the ten Config Connector groups ours reference
  (`config-connector-extensions-manager` and `config-connector-extensions-manager-cluster`,
  `config/operator/rbac.yaml`).
- **What Config Connector's pods would gain from our binding, when
  shared:** only full control of our six groups (`sql`, `sqladmin`,
  `secretmanager`, `artifactregistry`, `iam` and `run.gcp.hfl.dev`). Everything else in
  `config-connector-extensions-manager` they already hold.

So sharing barely widens Config Connector's side, and widens ours a lot:
a bug or a compromise in the manager would act with Config Connector's
full Kubernetes rights, including writing every Secret in the cluster in
cluster mode. That is why the default is the manager's own service
account (section 9.2), with only our roles, bound to the same Google
service account by one more `roles/iam.workloadIdentityUser` binding per
Google service account. Sharing stays as an opt-in for a quick trial where
nobody can change IAM. With the default, Config Connector's pods gain
nothing from this project.

## 10. Open questions

A question for the builder goes here, with a recommendation; the answer
moves to section 8.

1. Answered 2026-11-09: the kind is `IAMBinding`, first built as
   `IAMRoleBinding` (decision 12).

2. Answered 2026-11-10 (decision 22): keep the field until PR #7148.
   The question was: **`status.appliedDatabaseRoles` after the live run** (2026-11-09).
   Decision 3 kept the field "until a run against real Cloud SQL shows
   whether `users.get` returns roles". It does (`docs/live-runs.md`).
   Upstream PR #7148 compares the spec with the roles `users.get` returns.
   Recommendation: keep the field and the comparison with what was
   applied until we switch to PR #7148 (section 2), since changing it now
   changes the CRD for no user-visible gain; then follow the PR.

3. Answered 2026-11-09: the additive kind is `IAMMember`, and the
   authoritative one is renamed with it to `IAMBinding`. Both mirror
   Terraform's `google_<resource>_iam_member` and `_iam_binding`
   (decision 12). The question had recommended keeping `IAMRoleMember`.

4. Answered 2026-11-15 (CCEQ, decision 36): keep one served version
   (decision 24). The question was:
   **Should `IAMBinding` `v1alpha1` stay served, deprecated, beside
   `v1beta1`?** (2026-11-10, after decision 24). Decision 24 moved the
   kind and stopped serving `v1alpha1`. The alternative is a CRD with
   both versions (identical schemas, `conversion: None`, `v1beta1`
   stored, `v1alpha1` marked `deprecated` with a warning) for a support
   window, so an object created at `v1alpha1` keeps working. That is a
   second API version of the same kind, which is to be
   avoided, and nothing outside test clusters holds a `v1alpha1` object.
   Recommendation: keep decision 24 as built (one served version); serve
   both only if a cluster turns up with `v1alpha1` objects that cannot be
   abandoned and re-created. Reversing is cheap: add the version back to
   the CRD with `served: true`, `storage: false`, `deprecated: true`.
5. Answered 2026-11-16 (CCEQ5, decision 37): "as recommended
   (`<Config Connector kind>Fields` in our group, with a resourceRef to
   Config Connector's object and the API's field names, e.g.
   KMSCryptoKeyFields for keyAccessJustificationsPolicy.)" The question was:
   **The name of a field kind** (section 6.6, pattern 2; 2026-11-10).
   A kind of ours that owns a few fields of a resource Config Connector's
   kind owns has no name in Config Connector or Terraform (decision 18).
   Recommendation: `<Config Connector kind>Fields` in our group, with a
   `resourceRef` to Config Connector's object, for example
   `KMSCryptoKeyFields` for `keyAccessJustificationsPolicy`; the fields
   keep the API's names. Not needed until the first such PR.
6. Answered 2026-11-13: our own `v1alpha1` kinds move to `v1beta1`
   once they pass live, and `ComputeRouter`, `LoggingSettings` and
   `SecurityCenterMuteConfig` are promoted first (decision 27; the second
   half was decision 26). Status 2026-11-15: the three are `v1beta1`
   (decision 27, "Built"); `LoggingSettings` and `SecurityCenterMuteConfig`
   still owe their first live run.

7. Answered 2026-11-15 (CCEQ, decision 36): our own `LoggingLogSink`
   (`v1beta1`); no `Folder` kind; our own `OrgPolicyPolicy` only if the
   catalog cannot order deletes; the service-agents config under decision
   18 (it is our `ServiceIdentity`). The question was:
   **Field gaps on Config Connector kinds the catalog's folder,
   organization and logging ports found** (2026-11-13). Decision 19 says a
   field gap is our own kind in our group, owning the whole object.
   - `LoggingLogSink.interceptChildren` (a folder's or organization's sink
     that intercepts its children's logs). Config Connector's kind lacks
     it. Recommendation: our own `LoggingLogSink` at `v1beta1` (decisions
     19 and 26, as `NetworkServicesGateway`) with a folder, organization or
     project parent, hand-written after `LoggingBillingAccountSink` (a
     sink's create takes the name in the body and `uniqueWriterIdentity`
     as a parameter, which is not a `hack/discokind` shape).
   - `Folder.deletionProtection`. This is not an API field: Terraform's
     `google_folder.deletion_protection` is a client-side guard.
     Recommendation: no kind; the catalog maps it to
     `cnrm.cloud.google.com/deletion-policy: abandon` (which keeps the
     folder) or, for "refuse to delete", to Bundt's own deletion guard.
   - Config Connector's `OrgPolicyPolicy` cannot be deleted once the
     folder or organization object it names is gone (the parent-gone rule
     of PRs #109 and #111). It is Config Connector's controller, which
     this repository does not run and does not patch (patches change the
     pinned code we build, not the operator Config Connector runs).
     Recommendation: our own `OrgPolicyPolicy` (`orgpolicy.gcp.hfl.dev`,
     `v1beta1`, decision 19) only if the catalog cannot order deletes; until
     then, the catalog deletes policies before their parents, and the
     catalog's notes say so.
   - A "service-agents config" for folders and organizations: our
     `ServiceIdentity` already takes a folder or organization. If the
     catalog means something else (a resource that configures service
     agents), it needs naming: recommendation, the catalog names the
     Terraform resource and it is built here under decision 18.

8. Answered 2026-11-13: yes, split the generated kinds' tests into
   their own package (decision 28).

9. Answered 2026-11-15 (EXTQ6-1, decision 34): yes, and `RunService`
   is `v1beta1`. The question was:
   **Does decision 27 cover our field-gap kinds?** (2026-11-15.)
   Decision 27 promotes "a kind of ours for a gap Config Connector has no
   kind for". `RunService` (`run.gcp.hfl.dev/v1alpha1`) is ours for a
   field gap: Config Connector has a `RunService`, and ours owns the
   whole service to add `invokerIamDisabled` and GPUs (decision 19). It
   passed live (`aee1155`) and its CRD has not changed since.
   Recommendation: yes, promote field-gap kinds under the same rule, so
   every kind of ours reaches `v1beta1` the same way; the catalog's charts
   refuse alpha kinds whichever gap the kind fills.

10. **Where can `KMSCryptoKeyFields` run live?** (2026-11-16, CCEQ8.)
    Cloud KMS refuses a key's `keyAccessJustificationsPolicy`, set or
    cleared, unless the key is enrolled in Key Access Justifications with
    policy enforcement, which comes from an Assured Workloads folder (run
    `cce-11160523-1e83`: 400 "This key is not enrolled in Key Access
    Justifications with policy enforcement"). The live-test project is not
    enrolled, so the kind cannot get its live row, and decision 27 keeps it
    `v1alpha1`. Recommendation: do not enroll for tests now (Assured
    Workloads with Key Access Justifications is a paid, organization-level
    setup); keep the kind fake-tested and `v1alpha1`, let a catalog chart
    render it only behind `allow_alpha`, and run it live (`E2E_KMS_KAJ=true`)
    in the first enrolled project a customer or the builder has.

    **Answered** 2026-11-17 (CCEQ8): "CCEQ8: as recommended (Don't enroll a
    project just for tests. Keep KMSCryptoKeyFields tested on the fake cloud
    and at v1alpha1, let charts render it only behind allow_alpha, and run
    it live (E2E_KMS_KAJ=true) in the first enrolled project a customer or
    you have.)" No project is enrolled for tests; `KMSCryptoKeyFields` stays
    `v1alpha1` (decision 27) and fake-tested, and its live row waits for the
    first enrolled project.

11. **Where does a generated kind put a secret the API returns?**
    (2026-11-19, foundation item 4's last two kinds.) Two whole-resource
    gaps are standard collections `hack/discokind` could generate today,
    but each returns a secret as an output field:
    `IAMOauthClientCredential` (`projects.locations.oauthClients.credentials`,
    `clientSecret`, returned on every read) and
    `IAMWorkforcePoolProviderScimTenantToken`
    (`...providers.scimTenants.tokens`, `securityToken`, returned only by
    the create). The generator puts every output field in
    `status.observedState`, so the secret would sit in the object's status,
    readable by anyone who can read the object, as Config Connector's
    `IAMServiceAccountKey.status.privateKey` does. foundation.md's build
    line says the token is "secret output, like a service account key",
    which does not settle where it goes, and decision 33 settled only
    secrets we send (from a Kubernetes Secret), not ones we receive.
    Options: (a) status, as Config Connector's service account key;
    (b) a Kubernetes Secret in the object's namespace, owned by the
    object, named by a new `spec.secretOutput.name` (default the object's
    name), holding the value under the field's name, never in status;
    a token the API returns only at create is written once and, if the
    Secret is lost, the token must be deleted and made again.
    Recommendation: (b), as a `hack/discokind` option (`secretOutputs`,
    the mirror of decision 33's `secrets`), so both kinds and any later
    one get it without hand-written code; the manager's ClusterRole
    already writes Secrets for nothing else, so it gains `create` and
    `update` on Secrets. Until answered, neither kind is built.

    **Answered** 2026-11-23: option (b), as recommended: a generated kind
    writes a secret the API returns into a Kubernetes Secret, never into
    status, in the object's namespace, owned by the object, named by
    `spec.secretOutput.name` (default the object's name), the value under
    the field's name; a `hack/discokind` option `secretOutputs`; the
    manager's ClusterRole gains create and update on Secrets; a token
    returned only at create is written once, and if its Secret is lost it
    must be deleted and made again. Decision 38; both kinds are built (the
    token as `IAMWorkforcePoolProviderScimToken`, decision 18).
