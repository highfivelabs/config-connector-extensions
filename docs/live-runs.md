# Live runs

Every kind is built and tested against the fake cloud (Config Connector's
mockgcp, `docs/fake-cloud.md`). The fake is only as good as its likeness to
Google Cloud, so the same tests are also run against a real project: at
least once for every kind before it is called done, and again when a
kind or its mockgcp patch changes in a way the fake might get wrong. This
page says how, and keeps the ledger of every live run.

## The switch

The end-to-end tests are built on `pkg/test/mockenv`, which reads
`E2E_GCP_TARGET`, the variable Config Connector's own harness uses
(`config/tests/samples/create/harness.go`):

| `E2E_GCP_TARGET` | Google Cloud is | Set by |
| --- | --- | --- |
| unset or `mock` | mockgcp, in process, with our patches | `task test`, `task test-mock`, `task check` (which force `mock`) |
| `real` | the project in `GCP_PROJECT_ID`, with Application Default Credentials | `task test-live` only |

The test code is the same for both. What differs is data, all in
`pkg/test/mockenv/target.go`:

- `mockenv.Project`: `mock-project`, or the real project.
- `mockenv.Name(base)`: `base`, or `<run ID>-base`, so runs never collide
  and every resource a run makes can be found by its name.
- `mockenv.Labels()`: none, or `cce-e2e=true` and `cce-run=<run ID>` on every
  object (Config Connector copies `metadata.labels` to the resource).
- `mockenv.Timeout()`: 60 s, or 15 min (a real Cloud SQL backup takes minutes).
- `env.SQLInstance()`: an instance made in mockgcp, or the run's shared one.
- `mockenv.ServiceAccount(t)`: a made-up email, or the run's own service
  account (a Cloud SQL IAM user must be a real principal).

Requests are recorded in both modes, so checks on what the controller sent
(the regional endpoint, the update mask) hold against the real API too.

## Running it

```sh
task test-mock                                  # the default; what task check runs
task test-live PROJECT=$GCP_PROJECT           # every built kind, mock then live
task test-live PROJECT=$GCP_PROJECT KINDS=sql # one kind (a package under pkg/controller/direct)
task live-sweep PROJECT=$GCP_PROJECT          # delete leftovers older than 2h
task live-sweep PROJECT=$GCP_PROJECT RUN=cce-11091530-a3f2   # everything of one run
task live-list PROJECT=$GCP_PROJECT           # list every live-test resource
```

`PROJECT` defaults to `HFL_GCP_TEST_PROJECT`, then `GCP_PROJECT`, the
live-test project; neither has a default in the tree. Set them in the
environment or in a git-ignored `.env` (`.env.example` names every
variable; `task` loads `.env`). On an offload VM they reach the run only
through `env NAME=...` on the command line, as below.
`GCP_LIVE_FALLBACK_PROJECT` (optional, never the default) allows the previous dev project as a fallback until the live-test project's quota matches it.
On 2026-11-11 the first sampled run after the grants (GC1) ran there
(run `cce-11112128-d3db`, rows in the kinds' files): an offload VM cannot
yet run as `$GCP_LIVE_SA`, because the person who starts offload lacks
`iam.serviceAccounts.actAs` on it ("The user does not have access to
service account"). The builder's fix:
`gcloud iam service-accounts add-iam-policy-binding "$GCP_LIVE_SA" --project "$GCP_PROJECT" --member "user:<the offload user>" --role roles/iam.serviceAccountUser`
(and, while the VMs live in `$OFFLOAD_PROJECT`, cross-project service
account use allowed for `$GCP_PROJECT`).
The persistent test estate (`factory/estate/gcp/README.md`) writes that
variable, with the folder, the scratch project and the Log Router key
(`E2E_TEST_FOLDER`, `E2E_LOGGING_FOLDER`, `E2E_LOGGING_KMS_KEY`,
`E2E_TEST_FOLDER_PROJECT`), to one env file per generation;
`hack/live/projects.sh` allows its projects when
`GCP_LIVE_PROJECT_PATTERN` matches them (each generation's project and
its scratch project). Until the run scripts impersonate the
generation's own live-test account, runs stay in
`$GCP_PROJECT`: load the env file only after that lands.

Live runs happen on a short-lived Spot VM that runs as a dedicated
service account, never on a laptop and never with a downloaded key:

```sh
../offload/offload run --sa "$GCP_LIVE_SA" --hours 3 -- \
  env GCP_PROJECT="$GCP_PROJECT" bash -c 'task upstream && task test-live'
```

`hack/live/run.sh` (what `task test-live` runs):

1. refuses any project not in `hack/live/projects.sh` and any credentials
   that are not a service account;
2. sweeps leftovers older than 2h;
3. makes a run ID, `cce-<UTC month day hour minute>-<4 hex>`;
4. if a SQL kind is in the run, creates one Cloud SQL instance for the run
   (POSTGRES_16, Enterprise edition, `db-f1-micro`, zonal, 10 GB HDD, no
   automated backups, `cloudsql.iam_authentication=on`), shared by the SQL
   kinds because a real one takes minutes to make;
5. for each kind, in `docs/design.md` section 6.3's order, runs its package's
   tests with `E2E_GCP_TARGET=mock` and then `real`;
6. deletes the instance; then, pass or fail, sweeps everything of the run
   and leftovers older than 2h, and lists what is left;
7. writes `offload-artifacts/live/<run ID>/summary.md` and each kind's mock
   and live log (uploaded with the offload run's log).

Each test deletes what it makes, including what it abandons on purpose
(`deletion-policy: abandon`). The sweeper (`hack/live/sweep.sh`) is the
backstop for a test that fails midway or a VM that is preempted, and also
refuses any project but the ones listed.

### The service account

`$GCP_LIVE_SA`, recreated 2026-11-10
with the roles the live-test account in the previous dev project was given on 2026-11-09
(factory `docs/gcp-org-migration.md`):

| Role | On | For |
| --- | --- | --- |
| `roles/artifactregistry.admin` | project | `ArtifactRegistryRepository` |
| `roles/secretmanager.admin` | project | `SecretManagerSecret`, `SecretManagerSecretVersion` |
| `roles/cloudsql.admin` | project | `SQLUser`, `SQLAdminBackup`, the run's instance |
| `roles/compute.instanceAdmin.v1` | project | the offload VM (deletes itself) |
| `roles/logging.logWriter` | project | the offload VM |
| `roles/storage.objectAdmin` | bucket `$OFFLOAD_BUCKET` | the offload VM (checkout in, log out) |
| `roles/iam.serviceAccountUser` | itself | the offload VM |
| `roles/storage.admin` | project | `StorageBucketObject` (the test's bucket and its objects) |
| `roles/compute.networkAdmin` | project | `ComputeRegionNetworkEndpoint`, `ComputeRouterRoutePolicy` (the tests' networks, groups and routers); `NetworkSecurityAddressGroup` (the role has `networksecurity.addressGroups.*`); `NetworkSecuritySecurityProfile` and `NetworkSecuritySecurityProfileGroup` (a project's needed no other role, runs `cce-11092339-c43e` and `cce-11092345-9b77`) |
| `roles/compute.securityAdmin` | project | `ComputeRegionNetworkFirewallPolicy`, `ComputeRegionNetworkFirewallPolicyRule`, `ComputeRegionNetworkFirewallPolicyAssociation`, `ComputeNetworkFirewallPolicyRule` (and the test's global policy) |
| `roles/dns.admin` | project | `DNSResponsePolicy` (not granted yet: the test skips live without `E2E_DNS_ADMIN=true`) |
| `roles/apigee.admin` | project | `ApigeeNATAddress` (not granted, and no Apigee organization exists: the test skips live without `E2E_APIGEE_INSTANCE`) |
| `roles/spanner.admin` | project | `SpannerInstanceConfig` (not granted yet, and `spanner.googleapis.com` is off: the test skips live without `E2E_SPANNER=true`) |
| `roles/datacatalog.admin` | project | `DataCatalogTag` (not granted yet, and `datacatalog.googleapis.com` is off: the test skips live without `E2E_DATA_CATALOG=true`) |
| `roles/chronicle.editor` | project | `ChronicleRule`, `ChronicleRuleDeployment` (not granted, and the project has no Google SecOps instance: the test skips live without `E2E_CHRONICLE_INSTANCE`) |
| `roles/aiplatform.user` | project | `AIPlatformReasoningEngine` (not granted yet, and `aiplatform.googleapis.com` is off: the test skips live without `E2E_VERTEX_AI=true`) |
| `roles/servicemanagement.admin` | project | `EndpointsService` (not granted: run `cce-11130544-c338` was refused `servicemanagement.services.list`; the test skips live without `E2E_SERVICE_MANAGEMENT=true`) |
| `roles/compute.securityAdmin` | project | `ComputeRegionNetworkFirewallPolicy`, `ComputeRegionNetworkFirewallPolicyRule`, `ComputeRegionNetworkFirewallPolicyAssociation`, `ComputeNetworkFirewallPolicyRule`, `ComputeNetworkFirewallPolicyPacketMirroringRule` (and the tests' global policies) |
| `roles/iam.workloadIdentityPoolAdmin` | project | `IAMWorkloadIdentityPoolNamespace`, `IAMWorkloadIdentityPoolManagedIdentity`, `IAMWorkloadIdentityPoolProviderKey` (the tests' pools and providers) |
| `roles/run.admin` | project | `RunService` |
| `roles/iam.serviceAccountUser` | the compute default service account | `RunService` (a service runs as it) |
| `roles/iam.serviceAccountUser` | project | `ComputeProjectMetadataItem` (`setCommonInstanceMetadata` checks `actAs` on the project) |

`IAMMember` on a backend service needed no new role: the account's
`roles/compute.networkAdmin` and `roles/compute.securityAdmin` were enough
(run `cce-11091851-bd92`).

APIs enabled for it: `artifactregistry`, `secretmanager`, `sqladmin`,
`cloudresourcemanager`, `run`, `networksecurity` (2026-11-09, for
`NetworkSecurityAddressGroup`; the sweeper lists address groups, so it
needs the API on); `compute`, `iam` and `storage` were already on. A new kind that needs another role or API adds it
here, and to the account, in the same change.

### The slot move, live

`TestSlotMoveSecretManagerSecret` (`pkg/test/slotmove`, section 2.5 of
`docs/design.md`) runs two managers as two slots against the project and
moves one regional secret from one to the other; it makes one secret and
deletes it. Run only it:

```sh
../offload/offload run --sa "$GCP_LIVE_SA" --hours 1 -- \
  env GCP_PROJECT="$GCP_PROJECT" LIVE_RUN=TestSlotMove bash -c 'task upstream && task test-live KINDS=secretmanager'
```

## Adding a kind

A kind is not done until it has one live run recorded. From 2026-11-11 a
run's rows go in the kind's own file, `docs/kinds/<Kind>.md` ("Live
runs"), so pull requests do not conflict here; the ledger below keeps the
runs before then.

1. Its end-to-end test uses `pkg/test/mockenv` and takes every name from
   `mockenv.Name`, labels from `mockenv.Labels()`, the project from
   `mockenv.Project`, and deletes everything it makes, abandoned
   resources included.
2. The sweeper knows its resource type (`hack/live/sweep.sh`), and the
   service account has the role it needs. `task check:static` parses
   every shell script with `bash -n` and refuses conflict markers
   (`hack/check-shell.sh`): run `bash -n hack/live/sweep.sh` after any
   conflict merge, since nothing else runs the sweeper before a live run.
3. `task test-live PROJECT=$GCP_PROJECT KINDS=<package>` on an offload VM.
4. A row in the kind's `docs/kinds` file. A difference between mock and live is fixed where
   it belongs: in our controller (with a test), or in mockgcp (a patch in
   `patches/k8s-config-connector/`), or written down in `docs/fake-cloud.md`
   when the fake cannot show it.

## Ledger

One row per kind per live run. Mock and live ran from the same checkout,
on the same VM, minutes apart.

| Date | Commit | Kind | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-11-09 | `8a13722`+ | SQLAdminBackup | PASS | FAIL | after `backupRuns.delete` completes, `backupRuns.get` still returns the run, status `DELETED`; mockgcp answered 404 | instance `cce-11090729-0d21-pg` (db-f1-micro, ~30 min; swept by hand after the VM was preempted), 2 on-demand backups (deleted) |
| 2026-11-09 | `8a13722`+ | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | 1 regional and 1 global secret, 1 version (deleted) |
| 2026-11-09 | `8a13722`+ | SQLUser | PASS | PASS | none: `users.get` returns `databaseRoles`, and insert, add and revoke match patch 0001 | 1 IAM user in the shared instance (deleted) |
| 2026-11-09 | `8a13722`+ | ArtifactRegistryRepository | PASS | none | the VM was preempted during the live test | none left (checked) |
| 2026-11-09 | `8a13722`+ | SQLAdminBackup | PASS | FAIL | the same as above, now with the status printed: `DELETED` | instance `cce-11090817-870d-pg` (~40 min; swept by hand), 2 backups |
| 2026-11-09 | `8a13722`+ | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | SQLUser | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | ArtifactRegistryRepository | PASS | FAIL | the API refuses a remote upstream it cannot reach ("Failed to validate remote upstream", `registry.example.com/v2/`); mockgcp never checks. Run stopped by hand | none created |
| 2026-11-09 | `8a13722`+ | SQLAdminBackup | PASS | PASS | none, with the fix (Find takes `DELETED` as gone) and patch 0015 (numbered 0010 then) | instance `cce-11090854-9525-pg` (~25 min), 3 backups (deleted) |
| 2026-11-09 | `8a13722`+ | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | SQLUser | PASS | PASS | none | as above |
| 2026-11-09 | `8a13722`+ | ArtifactRegistryRepository | PASS | FAIL | a changed custom upstream URI: the API refuses it with "Remote source configuartion cannot be updated", mockgcp with "not supported by mockgcp" (fixed by patch 0016, numbered 0011 then); the upstreamCredentials test passed live | 6 remote repositories, 1 secret with 2 versions (deleted) |
| 2026-11-09 | `f2b89cd` | SQLAdminBackup | PASS | PASS | none | instance `cce-11090920-6dcf-pg` (~25 min), 3 backups (deleted) |
| 2026-11-09 | `f2b89cd` | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | 2 secrets, 1 version (deleted) |
| 2026-11-09 | `f2b89cd` | SQLUser | PASS | PASS | none | 1 IAM user (deleted with the instance) |
| 2026-11-09 | `f2b89cd` | ArtifactRegistryRepository | PASS | PASS | none | 6 repositories, 1 secret (deleted) |
| 2026-11-09 | `01afd2d`+ | ArtifactRegistryTag | PASS | PASS | none: tags on two pushed images, moved, deleted, abandoned | 1 Docker repository with 2 minimal images (deleted) |
| 2026-11-09 | `01afd2d`+ | ArtifactRegistryRepository | PASS | PASS | none | 6 repositories, 1 secret (deleted) |
| 2026-11-09 | `08fcc66` | SQLAdminBackup | PASS | PASS | none | instance `cce-11091348-ef59-pg` (db-f1-micro, ~25 min), 3 on-demand backups (deleted) |
| 2026-11-09 | `08fcc66` | SecretManagerSecret, SecretManagerSecretVersion | PASS | PASS | none | 2 secrets, 1 version (deleted) |
| 2026-11-09 | `08fcc66` | SQLUser | PASS | PASS | none | 1 IAM user (deleted with the instance) |
| 2026-11-09 | `08fcc66` | ArtifactRegistryRepository, ArtifactRegistryTag | PASS | PASS | none | 7 repositories, 2 minimal images, 1 secret (deleted) |
| 2026-11-09 | pending | ComputeRegionNetworkEndpoint | PASS | pending, needs `roles/compute.networkAdmin` | not run | none |
| 2026-11-09 | pending | ComputeRouterRoutePolicy | PASS | pending, needs `roles/compute.networkAdmin` | not run | none |
| 2026-11-09 | pending | ComputeRegionNetworkFirewallPolicy | PASS | pending, needs `roles/compute.securityAdmin` | not run | none |
| 2026-11-09 | pending | LoggingSettings | PASS | pending, needs a test folder with `roles/logging.admin` and a KMS key its Log Router can use | not run | none |
| 2026-11-09 | `756c69f` | StorageBucketObject | PASS | PASS | none: media and multipart uploads, a new generation for new content, content from a ConfigMap and a Secret, delete and abandon all match patch 0013 | 1 bucket (US) with 4 small objects (deleted with the bucket) |
| 2026-11-09 | `117f4ee` | ComputeProjectMetadataItem | PASS | FAIL | `projects.setCommonInstanceMetadata` needs `iam.serviceAccounts.actAs` on the project ("Required 'iam.serviceAccounts.actAs' permission for 'projects/$GCP_PROJECT'"); the service account has actAs only on the compute default service account. mockgcp checks no permissions. Pending `roles/iam.serviceAccountUser` on the project | none (the first write was refused; the sweep matched 0, `make live-list` empty) |
| 2026-11-09 | `2b9fc73` | ComputeRegionNetworkEndpoint | PASS | PASS | none | 1 network, 1 regional internet NEG with 3 endpoints (deleted) |
| 2026-11-09 | `2b9fc73` | ComputeRegionNetworkFirewallPolicy | PASS | PASS | none | 2 regional firewall policies (deleted) |
| 2026-11-09 | `2b9fc73` | ComputeRouterRoutePolicy | PASS | FAIL | `routers.getRoutePolicy` answers a missing policy with 400 "Invalid value for field 'policy': ... The policy does not exist", not 404, so Find took it for an error and the policy was never made; mockgcp answered 404. Fixed in the controller and in patch 0012 | 1 network, 1 Cloud Router (deleted) |
| 2026-11-09 | `2b9fc73` | IAMWorkloadIdentityPoolNamespace | PASS | PASS | none | 1 workload identity pool (TRUST_DOMAIN) with 3 namespaces (deleted; kept 30 days, deleted) |
| 2026-11-09 | `2b9fc73` | IAMWorkloadIdentityPoolProviderKey | PASS | PASS | none, against a real SAML provider with a fresh self-signed certificate | 1 pool, 1 SAML provider, 2 keys (deleted with the pool) |
| 2026-11-09 | `2b9fc73` | IAMWorkloadIdentityPoolManagedIdentity | PASS | FAIL | the API refuses attestation rules naming a project outside the pool's organization ("must be within same organization as the WorkloadIdentityPool"); the test's made-up project 123 is now the test project's number live; mockgcp does not check | 1 pool with 1 namespace (deleted) |
| 2026-11-09 | `2b9fc73` | RunService | PASS | FAIL | no Cloud Run GPU quota without zonal redundancy in the project; the test now runs without a GPU live (`E2E_RUN_GPU=true` where there is quota) | none made |
| 2026-11-09 | `c459e36` | ComputeProjectMetadataItem and the rest of compute, iam, run | PASS | none | the VM was preempted (run `cce-11091605-8e55`) after the metadata item's first six subtests passed with the new project-level actAs; a resume then failed for want of `make upstream` (see `docs/handoff.md`) | 3 metadata keys left; swept by `make live-sweep RUN=cce-11091605-8e55` |
| 2026-11-09 | `c459e36`+ | ComputeProjectMetadataItem | PASS | PASS | none (each write takes about 20 s live) | 5 metadata keys (removed) |
| 2026-11-09 | `c459e36`+ | ComputeRegionNetworkEndpoint, ComputeRegionNetworkFirewallPolicy, ComputeRouterRoutePolicy | PASS | PASS | none, with the route policy fix | 2 networks, 1 NEG, 1 router, 2 firewall policies (deleted) |
| 2026-11-09 | `c459e36`+ | IAMWorkloadIdentityPoolNamespace, IAMWorkloadIdentityPoolManagedIdentity, IAMWorkloadIdentityPoolProviderKey | PASS | PASS | none, with the attestation rules in the test project | 3 pools, 1 SAML provider (deleted) |
| 2026-11-09 | `c459e36`+ | RunService | PASS | FAIL | without a GPU, create, a new image, delete and abandon pass; a resync with an unchanged spec sent a `template` update, and turning invoker IAM back on carried `template` too | 2 services (deleted) |
| 2026-11-09 | `58d6ade` | RunService | PASS | FAIL | the same, now with the cloud's template printed: the API gives each container an http1 port 8080 and a TCP startup probe the spec leaves out. Fixed in the controller and in patch 0008 | 2 services (deleted) |
| 2026-11-09 | `aee1155` | RunService | PASS | PASS | none, with the default port and startup probe taken from what was read (no GPU live: no quota) | 2 services (deleted); `make live-list` empty |
| 2026-11-09 | `a3b6b51` | IAMRoleBinding | PASS | PASS | the project's organization refuses `projectOwner:` and the like as members of a secret's policy ("invalid due to membership type restrictions"; mockgcp accepts any member), so the test binds the project's service accounts; secret policy (global), bucket policy (Cloud Storage JSON API), a conditional binding and the same role without it, a member added out of band removed, delete and abandon all match patch 0018 and the fake | 1 secret, 1 bucket (deleted); `task live-list` empty |
| 2026-11-09 | `e8dfd1b` | IAMRoleMember | PASS | PASS | none: a regional secret (the regional host's `:getIamPolicy`/`:setIamPolicy`), a managed folder `team/app/` (Cloud Storage's `.../managedFolders/team%2Fapp%2F/iam`, escaped whole, accepted as sent) and a global backend service (Compute Engine's `getIamPolicy`/`setIamPolicy` with `{"policy": ...}`) all match patches 0018 and 0019; the member added by another writer stayed, a removed one was put back, delete and abandon touched only the object's own member, and a conditional member was a binding of its own. The 28 other rows of the resource table are not called live | 1 regional secret, 1 bucket with 1 managed folder, 1 health check and 1 backend service (deleted); `task live-list` empty |
| 2026-11-09 | `a8c6de4` | IAMBinding | PASS | PASS | none: the rename of `IAMRoleBinding` (design.md decision 12), rerun whole | run `cce-11092235-4dca`; `task live-list` empty |
| 2026-11-09 | `a8c6de4` | IAMMember | PASS | PASS | none: the rename of `IAMRoleMember` (decision 12), rerun whole (regional secret, managed folder, backend service) | the same run |

| 2026-11-09 | `39ded9d`+ | ComputeRegionNetworkFirewallPolicyRule | PASS | PASS | after three live runs that found: `patchRule` keeps a list the body leaves out (fixed: the emptied list is sent as `[]`, through the Compute Engine REST client); the unset strings and booleans must still be sent (`""`, false); a rule just changed answers a patch with 400 "The rule at priority N is not ready" (the test's out-of-band patch retries; the reconciler's retry absorbs it). The missing-priority answer is the 400 the code assumed: "Invalid value for field 'priority': '1000'. The firewall policy does not contain a rule at priority 1000." | 2 regional firewall policies and their rules (deleted); `task live-list` showed only the network of a concurrent run |
| 2026-11-09 | `c7f163f`+ | ComputeRegionNetworkFirewallPolicyAssociation | PASS | PASS | after one run that failed only on the draft's `status.firewallPolicyId` (the API returns an association with only `attachmentTarget`, as a full URL, and `name`; the field was dropped): create, resync, a second network's association, a name taken for another network (an error, not a move), an association removed out of band made again, delete, abandon. A missing association is the 400 "An association with that name does not exist." Mock fixed to match (patch 0021) | 2 networks, 1 regional firewall policy with up to 2 associations (deleted); `task live-list` empty |
| 2026-11-09 | pending | ServiceIdentity | PASS | pending, needs the test folder (`E2E_TEST_FOLDER`) | not run | none |
| 2026-11-09 | pending | ComputeProjectDefaultNetworkTier | PASS | pending, needs a test project under the test folder (`E2E_TEST_FOLDER_PROJECT`) with `compute.projects.setDefaultNetworkTier` for `cce-live-tests` | not run (a live `compute` run skips it) | none |
| 2026-11-09 | pending | ProjectDefaultServiceAccounts | PASS | pending, needs a test project under the test folder (`E2E_TEST_FOLDER_PROJECT`) whose Compute Engine API is on | not run (`resourcemanager` is not a default live kind) | none |
| 2026-11-09 | `a9a2026` | NetworkSecurityAddressGroup (project parent) | PASS | PASS | none for a project's address group: create, a resync with no patch (the API's default `purpose` and item order are no diff), one patch for an item and description change, the CRD refusing a capacity change and a missing or double parent, delete and abandon all match patch 0025 and the fake. Run `cce-11092303-4b16`; the first run (`cce-11092258-c4e8`, `e244bd5`) passed live but not on the fake, which refused to start until patch 0025 named the organization service in mockgcp's HTTP mux | 2 global address groups (deleted, the abandoned one by the test); `task live-list` empty |
| 2026-11-09 | pending | NetworkSecurityAddressGroup (organization parent) | PASS | pending, needs test folder (an organization's address group needs organization access; a live run never touches the organization) | not run (the subtest skips live) | none |
| 2026-11-09 | `b7eae2b` | NetworkSecuritySecurityProfile (project parent) | PASS | PASS | none: create (threat prevention, severity overrides out of the API's order), a resync with no patch, one patch for an added override and a description change, a URL filtering profile whose resync writes nothing, the CRD refusing a type change, a missing or double parent and two profiles, delete and abandon all match patch 0026 and the fake. Run `cce-11092339-c43e` | 3 global security profiles (deleted, the abandoned one by the test); `task live-list` empty |
| 2026-11-09 | pending | NetworkSecuritySecurityProfile (organization parent) | PASS | pending, needs test folder (an organization's profile needs organization access; a live run never touches the organization) | not run (the subtest skips live) | none |
| 2026-11-09 | `c64a5b6` | NetworkSecuritySecurityProfileGroup (project parent) | PASS | PASS | none: create naming a threat prevention profile by object name, a resync with no patch (the API returns the profile reference as sent), one patch for an added URL filtering profile, the CRD refusing a missing or double parent, delete and abandon all match patch 0026 and the fake. Run `cce-11092345-9b77` | 2 global groups and 2 profiles (deleted, the abandoned group by the test, groups before profiles); `task live-list` empty |
| 2026-11-09 | pending | NetworkSecuritySecurityProfileGroup (organization parent) | PASS | pending, needs test folder (an organization's group needs organization access; a live run never touches the organization) | not run (the subtest skips live) | none |
| 2026-11-10 | `ffc7fc9` | ComputeRouter | PASS | FAIL | `routers.insert` of a router with interfaces, a BGP peer, a NAT and an MD5 key: 400 "Creating a router with multiple features is currently unsupported."; mockgcp accepted it (fixed: insert bare, then update; patch 0027) | none made (checked: 0 leftovers) |
| 2026-11-10 | `8648114` | ComputeRouter | PASS | FAIL | `routers.update` with the four lists at once: 400 "Updating multiple Router features is currently unsupported." (fixed: one feature per update; patch 0027) | a bare router (deleted) |
| 2026-11-10 | `86ca6c8` | ComputeRouter | PASS | FAIL | the interfaces, peer and key steps passed; the NAT-only update carrying the unchanged MD5 key values was refused as two features (fixed: keys by name outside the key steps; patch 0027) | a router with an interface, peer and key (swept) |
| 2026-11-10 | `d171981` | ComputeRouter | PASS | FAIL | create, a resync with no write, the peer and NAT change, delete and abandon pass; two subtests read the cloud before the sequential live writes landed (test fixed to wait on the cloud and status) | 2 routers, a NAT (deleted) |
| 2026-11-10 | `1ff3750` | ComputeRouter | PASS | PASS | none left: run `cce-11100336-0ecc` (the first attempt, `cce-11100319-ed80`, was preempted and swept by ID); the API's defaults (BFD, peer IP, NAT timeouts, `identifierRange`) leave a resync without a write | 2 routers with an interface, a BGP peer, an MD5 key and a Cloud NAT (deleted) |
| 2026-11-10 | `afe91be` | ComputeRouterRoutePolicy (and the rest of `compute`) | PASS | PASS | none: a policy on our `ComputeRouter` named by `routerRef.name`, and the name refused while a Config Connector `ComputeRouter` of the same name exists, as on the fake (design.md section 7 item 28). Run `cce-11100428-c1fb` | the compute package's networks, routers (one of ours), NEG and firewall policies (deleted); `task live-list` empty |
| 2026-11-10 | pending | IAMDenyPolicy | PASS | pending, needs `iam.denypolicies.*` (Deny Admin, `roles/iam.denyAdmin`, granted on an organization) for `cce-live-tests`; then `E2E_IAM_DENY_ADMIN=true` | not run (the test skips live without it) | none |
| 2026-11-10 | pending | IAMPrincipalAccessBoundaryPolicy | PASS | pending, needs organization-level access (a policy lives under an organization) | not run (the test skips live) | none |
| 2026-11-10 | pending | ComputeOrganizationSecurityPolicy, ComputeOrganizationSecurityPolicyAssociation | PASS | pending, needs the test folder (both act on an organization or folder) | not run (the test skips live) | none |
| 2026-11-10 | pending | ComputeOrganizationSecurityPolicyRule | PASS | pending, needs the test folder (a rule of a policy under an organization or folder) | not run (the test skips live) | none |
| 2026-11-10 | pending | LoggingBillingAccountSink | PASS | pending, needs billing account access: a billing account on which `cce-live-tests` has `roles/logging.configWriter`, passed as `E2E_BILLING_ACCOUNT=billingAccounts/{id}` (design.md decision 15) | not run (the test skips live without it) | none |
| 2026-11-10 | `18addd0` | ComputeNetworkFirewallPolicyRule (and the rest of `compute`) | PASS | PASS | after one run (`cce-11100614-922f`) that found: `patchRule` does not clear `srcNetworkContext` when sent `""` (it kept INTERNET, and the reconciler patched on every resync); fixed by sending `UNSPECIFIED`, with `""` and `UNSPECIFIED` compared as unset. The missing-priority answer is the regional one, 400 "The firewall policy does not contain a rule at priority 1000." Run `cce-11100702-7115` | 2 global network firewall policies and their rules, and the compute package's other resources (deleted); `task live-list` empty |
| 2026-11-10 | pending | SecurityCenterMuteConfig (project parent) | PASS | pending, needs Security Command Center active for `$GCP_PROJECT` and `securitycenter.muteconfigs.*` (`roles/securitycenter.muteConfigsEditor`) for `cce-live-tests`; then `E2E_SECURITY_CENTER=true` | not run (the test skips live without it) | none |
| 2026-11-10 | pending | SecurityCenterMuteConfig (folder and organization parents) | PASS | pending, needs test folder | not run (the subtest skips live) | none |
| 2026-11-10 | pending | DNSResponsePolicy | PASS | pending, needs `dns.googleapis.com` on in the previous dev project and `roles/dns.admin` for `cce-live-tests`; then `E2E_DNS_ADMIN=true` | not run (the test skips live without it) | none |
| 2026-11-10 | `e24d9c6` | ComputeNetworkFirewallPolicyPacketMirroringRule | PASS | FAIL | `addPacketMirroringRule` refused `tlsInspect: false`: 400 "TLS Inspection cannot be set for packet mirroring rule" (mockgcp accepted it; fixed: the field is gone from the kind and patch 0032 refuses it). Run `cce-11100835-60b4` | 1 policy (swept by run ID in the next run) |
| 2026-11-10 | `64c9c42` | ComputeNetworkFirewallPolicyPacketMirroringRule | PASS | FAIL | create, a resync with no write, a second rule, delete and abandon pass; the patch (new description, rule name, `destIpRanges`, `srcIpRanges` emptied) did not reach the cloud in 15 minutes, cause not logged (the test now prints the cloud rule and the object's Ready message on timeout). `ComputeNetworkFirewallPolicyRule` failed its network-context subtest, as on `main` (#51 fixes it). Run `cce-11100936-3a4c` | the compute package's resources (swept by run ID) |
| 2026-11-10 | `81ac634` | ComputeVPNTunnel | PASS | FAIL | `vpnTunnels.insert`: 400 "peer_ip may not be a reserved IP (RFC5735)" for the peer gateway on 203.0.113.10, which mockgcp accepted (fixed: the test reserves an external address of its project for the peer; `docs/fake-cloud.md`). Run `cce-11100852-72bb`; an earlier attempt, `cce-11100848-13c6`, stopped at its first sweep (`externalVpnGateways.list` refuses a `name~` filter; the sweeper now matches names itself) | the network, HA and external VPN gateways and router (deleted by the test) |
| 2026-11-10 | `23fa22b` | ComputeVPNTunnel | PASS | FAIL | "a resync writes nothing" saw one write: `vpnTunnels.insert` leaves the labels out, so the first reconcile set them (fixed: Create sets them after insert, as Terraform's tunnel does; patch 0033's insert leaves them out too). The other five subtests passed. Run `cce-11101009-a46a` | 2 tunnels, an address, the gateways, router and network (deleted); `task live-list` empty |
| 2026-11-10 | `edf63e7` | ComputeVPNTunnel | PASS | PASS | none: an HA VPN tunnel to an external peer on the test's own address, with a phase 1 and 2 cipher suite the API returned as sent, labels set after insert, a resync with no write, a label change by `setLabels`, the CRD refusing a cipher suite change, delete and abandon. Run `cce-11101104-1658` (the package's `TestComputeNetworkFirewallPolicyRule` failed as in #51, which fixes it) | 2 tunnels, an address, 2 VPN gateways, a router, a network (deleted); `task live-list` empty |

`+` means the commit plus the uncommitted changes that became the next
commit. Runs `cce-11090718-8cc1` (gcloud's GA track has no `--labels` for
Cloud SQL; nothing made) and `cce-11090806-613a` (preempted while making
the instance; swept by hand) reached no test.

Packages `compute`, `iam` and `run` run live with the roles granted on
2026-11-09 (the service account table). Their tests take every cloud
name from `mockenv.Name` and delete what they make, abandoned resources
included: `compute` makes a network, a regional network endpoint group, a
router and firewall policies; each `iam` test makes its own workload
identity pool (its ID is kept 30 days after delete, so it is never
reused), and the key test a SAML provider with a fresh self-signed
certificate; `run` makes Cloud Run services from Cloud Run's public
sample image. The sweeper finds all of these by the run's name prefix.
The test project has no Cloud Run GPU quota without zonal redundancy, so
live `RunService` runs without a GPU and skips the GPU checks
(`E2E_RUN_GPU=true` where there is quota); the fake cloud keeps the GPU.

`LoggingSettings` (package `logging`) is ready to run live but waits on
the builder for a test folder and a key; it never changes an
organization's settings live (that case runs in the fake cloud only). It
needs: a test folder, passed as `E2E_LOGGING_FOLDER=folders/<id>`, with
`roles/logging.admin` on it for the live-test service account; and a
Cloud KMS key, passed as `E2E_LOGGING_KMS_KEY`, on which the folder's
Log Router service account (`kmsServiceAccountId` in the folder's
settings) has `roles/cloudkms.cryptoKeyEncrypterDecrypter`. Settings
cannot be deleted, so the test puts the folder's back as it found them;
there is nothing for the sweeper. Then:
`E2E_LOGGING_FOLDER=... E2E_LOGGING_KMS_KEY=... task test-live PROJECT=$GCP_PROJECT KINDS=logging`.

`ServiceIdentity` (package `serviceusage`) also waits on the test folder,
passed as `E2E_TEST_FOLDER=folders/<id>`; it never uses the
organization (that case runs in the fake cloud only). The live-test
service account needs `serviceusage.services.use` on the folder or the
quota project (assumed from the Service Usage reference, not yet seen).
A service identity cannot be deleted, so there is nothing for the
sweeper; the run leaves the folder's service agent in place, as
Terraform does. Then:
`E2E_TEST_FOLDER=... task test-live PROJECT=$GCP_PROJECT KINDS=serviceusage`.

`ComputeOrganizationSecurityPolicy`, its association and its rule
(package `compute`) wait on the test folder: a live run skips them, and the
sweeper has nothing of theirs to find in a project.

`IAMDenyPolicy` (package `iam`) waits on Deny Admin for the live-test
service account: deny policies need `iam.denypolicies.*`, which only
`roles/iam.denyAdmin` has, and it is granted on an organization (the
test estate). Until then a live `iam` run skips the test, and the
sweeper leaves deny policies alone (listing them needs the same role).
With it: `E2E_IAM_DENY_ADMIN=true task test-live PROJECT=$GCP_PROJECT
KINDS=iam`. The test's policy is on the run's project and denies
`iam.googleapis.com/roles.delete` (and then `roles.undelete`) to everyone
but the run's own service account for a minute or two; folder and
organization policies run on the fake only. The sweeper deletes
policies whose ID carries a run ID.

`IAMPolicyBinding` (package `iam`) waits on a principal access boundary
policy: one lives under an organization (a run never makes one), and
`cce-live-tests` needs `iam.principalaccessboundarypolicies.bind` on it
and `iam.policyBindings.*` on the project. Until then a live `iam` run
skips the test and the sweeper leaves policy bindings alone. With them:
`E2E_IAM_POLICY_BINDING_POLICY=organizations/{org}/locations/global/principalAccessBoundaryPolicies/{id}
task test-live PROJECT=$GCP_PROJECT KINDS=iam`. The test binds that policy
to the run's project with a condition no real principal meets, so the
boundary changes nothing; the sweeper deletes the project's bindings
whose ID carries a run ID.

`LoggingBillingAccountSink` (package `logging`) waits on billing account
access: the live-test service account needs `roles/logging.configWriter`
(or `logging.sinks.*`) on a billing account, which only the builder can
grant; until then a live `logging` run skips the test and the sweeper
leaves billing account sinks alone. With it:
`E2E_BILLING_ACCOUNT=billingAccounts/<id> task test-live
PROJECT=$GCP_PROJECT KINDS=logging` (the variable must reach the
sweeper too). The test's sinks send to a Pub/Sub topic path in the run's
project that need not exist (assumed: the API does not check the
destination at create); their names carry the run ID, and the sweeper
deletes them from that billing account.

`SecurityCenterMuteConfig` (package `securitycenter`) waits on Security
Command Center for the run's project: it must be active for the project
(or its organization), and the live-test service account needs
`securitycenter.muteconfigs.*` there (`roles/securitycenter.muteConfigsEditor`;
assumed from the Security Command Center reference, not yet seen). Until
then a live run skips the test and the sweeper leaves mute configs
alone. With it: `E2E_SECURITY_CENTER=true task test-live
PROJECT=$GCP_PROJECT KINDS=securitycenter`. A folder's and an
organization's mute configs run on the fake only (the test folder).

`DNSResponsePolicy` (package `dns`) waits on Cloud DNS in the project:
`dns.googleapis.com` is not on in `$GCP_PROJECT` and `cce-live-tests`
has no DNS role (checked 2026-11-10). Until both are, a live `dns` run
skips the test and the sweeper leaves response policies alone (listing
them needs the API and the role). With them:
`E2E_DNS_ADMIN=true task test-live PROJECT=$GCP_PROJECT KINDS=dns`.
The test binds a policy to two networks of its own, unbinds one,
deletes it (unbinding first) and abandons and then acquires another; the
sweeper unbinds and deletes policies whose name carries a run ID.
`DNSResponsePolicyRule` (the same package) waits on the same switch; its
test makes a policy of its own through the API, with rules that answer a
name, switch to `bypassResponsePolicy` and are abandoned, and the sweeper
deletes a policy's rules before the policy.

`ApigeeNATAddress` (package `apigee`) waits on Apigee: a NAT address
belongs to an Apigee instance, which needs a provisioned Apigee
organization (billed, and an hour or more to make), and the live-test
service account needs `roles/apigee.admin`; `$GCP_PROJECT` has neither
(`apigee.googleapis.com` off, checked 2026-11-12). No test makes an
organization or an instance. With one: `E2E_APIGEE_INSTANCE=organizations/O/instances/I
task test-live PROJECT=$GCP_PROJECT KINDS=apigee`; the sweeper then
deletes that instance's NAT addresses whose name carries a run ID.

`SpannerInstanceConfig` (package `spanner`) waits on Spanner in the
project: `spanner.googleapis.com` is not on in `$GCP_PROJECT` and
`cce-live-tests` has no Spanner role (checked 2026-11-12). Until both are
(the API, and `roles/spanner.admin`), a live `spanner` run skips the test
and the sweeper leaves instance configurations alone. With them:
`E2E_SPANNER=true task test-live PROJECT=$GCP_PROJECT KINDS=spanner`.
The test makes user-managed configurations only (no instance), which cost
nothing; the sweeper deletes those whose ID carries a run ID after
`custom-`.
`DataCatalogTag` (package `datacatalog`) waits on Data Catalog in the
project: `datacatalog.googleapis.com` is not on in `$GCP_PROJECT` and
`cce-live-tests` has no Data Catalog role (checked 2026-11-12). Until both
are (the API, and `roles/datacatalog.admin`), a live `datacatalog` run
skips the test and the sweeper leaves Data Catalog alone. With them:
`E2E_DATA_CATALOG=true task test-live PROJECT=$GCP_PROJECT KINDS=datacatalog`.
The test makes a tag template, an entry group and an entry of its own
(free), then the tag; the sweeper deletes entry groups (with their entries
and tags) and tag templates whose ID carries a run ID.
`AIPlatformReasoningEngine` (package `aiplatform`) waits on Vertex AI in
the project: `aiplatform.googleapis.com` is not on in `$GCP_PROJECT` and
`cce-live-tests` has no Vertex AI role (checked 2026-11-12). Until both
are (the API, and `roles/aiplatform.user`), a live `aiplatform` run skips
the test and the sweeper leaves reasoning engines alone. With them:
`E2E_VERTEX_AI=true task test-live PROJECT=$GCP_PROJECT KINDS=aiplatform`.
The test makes an engine with no code (sessions and memory only), which
costs nothing while idle; the sweeper deletes engines in `us-central1`
whose display name carries a run ID.

`EndpointsService` (package `servicemanagement`) waits on Service
Management for the account: run `cce-11130544-c338` (2026-11-13) was
refused `servicemanagement.services.list` on `$GCP_PROJECT` (403), so
`cce-live-tests` has no Service Management role, and whether
`servicemanagement.googleapis.com` is on was not reached. Until the
account has `roles/servicemanagement.admin` (and the API is on), a live
`servicemanagement` run skips the test, and the sweeper takes a 403 on
the list as nothing to sweep. With them:
`E2E_SERVICE_MANAGEMENT=true task test-live PROJECT=$GCP_PROJECT KINDS=servicemanagement`.
The test makes two services, `<run ID>-echo` and `<run ID>-kept`
`.endpoints.$GCP_PROJECT.cloud.goog` (free; a deleted service holds its
name for 30 days, and every name carries the run ID); the sweeper deletes
services whose name carries a run ID.

`ChronicleRule` and `ChronicleRuleDeployment` (package `chronicle`) wait on a Google SecOps
(Chronicle) instance, which `$GCP_PROJECT` does not have (an instance is
provisioned with a SecOps subscription, not by an API call), and on
`roles/chronicle.editor` for `cce-live-tests`. Until both exist, a live
`chronicle` run skips the test, makes nothing, and the sweeper needs no
Chronicle section. With them:
`E2E_CHRONICLE_INSTANCE=<location>/<instance ID> task test-live PROJECT=$GCP_PROJECT KINDS=chronicle`.
The test makes two rules of its own (named after the run ID, matching
nothing) and deletes them; before the first live run the sweeper should
learn to delete rules whose name carries a run ID.

`ComputeVPNTunnel` (package `compute`) needs nothing new: the account's
`roles/compute.networkAdmin` has `compute.vpnTunnels.*`,
`compute.vpnGateways.*` and `compute.externalVpnGateways.*`. The test
makes a network, an HA VPN gateway, an external address of its own, an
external peer gateway on that address (the API refuses a reserved
address such as 203.0.113.10 for a peer, run `cce-11100852-72bb`; nothing
answers on ours, so the tunnel never comes up) and a Cloud Router through
the API, then two tunnels; the sweeper deletes tunnels, then HA and
external VPN gateways and addresses, by name. A
tunnel bills by the hour while it exists; the test keeps each for a few
minutes.

`GKEHubRolloutSequence` (package `gkehub`) needs a fleet in the run's
project: a rollout sequence's stages name fleet host projects, and
`rolloutSequences.create` answers 404 "fleet not found" without one. A
fleet is free and a project has at most one (`fleets/default`), so the
live test makes it when the project has none (labelled with the run,
`pkg/controller/direct/gkehub/fleet_test.go`), deletes it after its own
resources, and leaves alone a fleet the project already had; the
sweeper deletes a fleet that carries a live run's labels. It needs
`roles/gkehub.admin` (`gkehub.fleet.*`, `gkehub.rolloutsequences.*`) and
the GKE Hub API on. A changed rollout sequence took about seven minutes
to read back changed (run `cce-11120814-5d2a`).

`IAMMember`'s BigQuery dataset case (package `iam`) waits on BigQuery
for the live-test account: making a dataset needs
`bigquery.datasets.create` and changing its access list
`bigquery.datasets.update` (`roles/bigquery.admin`, or
`roles/bigquery.dataOwner` with `roles/bigquery.user`; assumed from the
BigQuery reference, not yet seen), which `cce-live-tests` has in neither
project. Until then a live `iam` run skips the case and the sweeper
leaves datasets alone. With it: `E2E_BIGQUERY_ADMIN=true task test-live
PROJECT=$GCP_PROJECT KINDS=iam LIVE_RUN=TestIAMMember`. The case makes
one dataset (US, empty, labelled with the run) and deletes it; the
sweeper deletes datasets whose ID is the run's name with underscores.

`ComputeRegionBackendBucket` (package `compute`) and `LoggingLink`
(package `logging`) make what their fixtures name in a live `Setup`
(`regionbackendbucket_test.go`, `link_test.go`): two regional Cloud
Storage buckets, and two global log buckets with Log Analytics on. The
account's `roles/storage.admin`, `roles/compute.loadBalancerAdmin` and
`roles/logging.admin` (in the fallback project) cover them; the sweeper
deletes log buckets, links first, by the run's name, where the account
may list them. A deleted log bucket is kept seven days and its ID is not
reused.

`ComputeRegionNotificationEndpoint` skips live (its fixtures'
`live-skip`): `regionNotificationEndpoints.insert` answers 400
`invalidResourceUsage` "HealthCheck as a Service feature is not
available for this project" (run `cce-11112128-d3db`). Health Check as a
Service is gated per project, with no public way to turn it on; the kind
waits for a project Google has allowlisted.

`ComputeProjectDefaultNetworkTier` changes a whole project, so it runs
live only in a test project under the test folder, passed as
`E2E_TEST_FOLDER_PROJECT=<project ID>`; without it a live `compute` run
skips the test, and the test refuses `$GCP_PROJECT` itself. It needs
`compute.projects.setDefaultNetworkTier` on that project for the
live-test service account (no role it has includes it; the Terraform
spike in catalog `docs/terraform-leverage.md` got a 403 in
`$GCP_PROJECT`), and the test project in `hack/live/projects.sh`. The
test puts the project's tier back as it found it; there is nothing for
the sweeper.

`ProjectDefaultServiceAccounts` (package `resourcemanager`) disables,
deletes and deprivileges a project's default service accounts, so it too
runs live only in a test project under the test folder
(`E2E_TEST_FOLDER_PROJECT`), never in `$GCP_PROJECT`, whose compute
default account the offload VMs and `RunService` run as. The project's
Compute Engine API must be on (so its default account exists). The
live-test service account needs `iam.serviceAccounts.list`, `disable`,
`enable`, `delete` and `undelete` and the project's `getIamPolicy` and
`setIamPolicy` there (assumed from the IAM reference, not yet seen). The
test leaves the accounts enabled and present; it does not give back the
roles `DEPRIVILEGE` removed (never reverted, as Terraform's), so the test
project's default accounts end without their project roles. Nothing for
the sweeper. Then:
`E2E_TEST_FOLDER_PROJECT=... task test-live PROJECT=$GCP_PROJECT KINDS=resourcemanager`.

Every run ended with `task live-list` empty (run `cce-11091348-ef59`:
"0 live-test resource(s) in $GCP_PROJECT"). Cost of all the runs above,
roughly: five db-f1-micro instances for 25 to 40 minutes each (well under
$0.50 together), Spot VMs of 4 to 8 vCPUs for about 4 VM-hours (about
$0.30), and Artifact Registry and Secret Manager use of a few cents.

