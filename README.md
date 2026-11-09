# config-connector-extensions

Kubernetes kinds for Google Cloud that
[Config Connector](https://github.com/GoogleCloudPlatform/k8s-config-connector)
(KCC) does not have yet, or has without a field a workload needs. Install
it beside Config Connector; it runs Config Connector's own reconciler for
a few extra kinds, in its own API groups, and retires each kind when
Config Connector ships it.

config-connector-extensions is not affiliated with or endorsed by Google.
"Config Connector" names the project it extends.

The kinds:

| Kind | Gap it fills | Source |
| --- | --- | --- |
| `sql.gcp.hfl.dev/v1beta1` `SQLUser` | `spec.databaseRoles`: Config Connector's `SQLUser` cannot grant a Cloud SQL user its database roles (`User.databaseRoles`, Terraform's `google_sql_user.database_roles`) | ours, on upstream's `SQLSSLCert` controller; upstream PR #7148 is watched |
| `sqladmin.gcp.hfl.dev/v1alpha1` `SQLAdminBackup` | an on-demand Cloud SQL backup (`backupRuns`); upstream master has the types but no controller | upstream PR #12553 |
| `secretmanager.gcp.hfl.dev/v1beta1` `SecretManagerSecret`, `SecretManagerSecretVersion` | `spec.location`: a regional secret (Terraform's `google_secret_manager_regional_secret`), in no Config Connector release or on master | master's direct controllers plus upstream PR #6651 |
| `artifactregistry.gcp.hfl.dev/v1beta1` `ArtifactRegistryRepository` | `remoteRepositoryConfig.<format>Repository.customRepository.uri`: a remote repository that pulls through an upstream of your choosing (Docker, Maven, npm, Python), not only the public ones Artifact Registry names, with `upstreamCredentials` (username and Secret Manager password version) for one that needs a login; in no Config Connector release or on master | master's direct controller plus upstream PR #13517; `upstreamCredentials` written here |
| `iam.gcp.hfl.dev/v1alpha1` `IAMWorkloadIdentityPoolNamespace` | a namespace in a workload identity pool (managed workload identities, Terraform's `google_iam_workload_identity_pool_namespace`); Config Connector has types for it only in a pull request, and no controller | upstream PR #14209's types; controller and the pool reference written here |
| `iam.gcp.hfl.dev/v1alpha1` `IAMWorkloadIdentityPoolManagedIdentity` | a managed identity in a pool namespace, with the attestation rules that say which workloads may receive it (Terraform's `google_iam_workload_identity_pool_managed_identity`); types only in a Config Connector pull request, no controller | upstream PR #14418's types; controller, the namespace reference and `attestationRules` written here |
| `run.gcp.hfl.dev/v1alpha1` `RunService` | a Cloud Run service with `invokerIamDisabled` (public without IAM, where an organization policy refuses `allUsers`) and GPUs (`template.nodeSelector`, `gpuZonalRedundancyDisabled`), which Config Connector's Terraform-based `RunService` cannot express; a narrow direct kind that owns the whole service | Config Connector's direct `RunWorkerPool`, for a service; the service fields written here |
| `iam.gcp.hfl.dev/v1alpha1` `IAMWorkloadIdentityPoolProviderKey` | a key for a workload identity pool provider, whose public half (in status) encrypts the provider's SAML assertions; in no Config Connector code, pull request or Terraform resource | written here from the IAM v1 API |
| `storage.gcp.hfl.dev/v1alpha1` `StorageBucketObject` | a small object in a bucket (a configuration file, a script), its content inline or from a Secret or ConfigMap; in no Config Connector code or pull request | written here from the Cloud Storage JSON API |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeRouterRoutePolicy` | a Cloud Router route policy (import or export terms, as CEL), as HA VPN and interconnect setups use; in no Config Connector code or pull request | written here from the Compute Engine API |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeRegionNetworkEndpoint` | an endpoint (internet address, domain name or VM port) in a regional network endpoint group, such as a regional load balancer's backend; Config Connector has zonal and global endpoints only | written here after Config Connector's zonal `ComputeNetworkEndpoint` |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeRegionNetworkFirewallPolicy` | a regional network firewall policy, for firewall rules that apply to a VPC in one region; Config Connector has only the global policy | Config Connector's opt-in direct `ComputeNetworkFirewallPolicy`, for a region |
| `logging.gcp.hfl.dev/v1alpha1` `LoggingSettings` | an organization's or folder's Log Router settings: the CMEK key, the default log bucket location, the `_Default` sink off for new projects; in no Config Connector code (one closed scaffold PR) | written here from the Cloud Logging API |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeProjectMetadataItem` | one key of a project's common instance metadata (for example `enable-oslogin`), leaving the other keys alone; Config Connector's `ComputeProjectMetadata` owns the whole metadata | written here after Terraform's `google_compute_project_metadata_item` |
| `artifactregistry.gcp.hfl.dev/v1alpha1` `ArtifactRegistryTag` | a tag on a package's version (for example `prod` on an image digest), moved by changing `spec.version`; in no Config Connector code, pull request or Terraform resource | written here from the Artifact Registry API |
| `iam.gcp.hfl.dev/v1alpha1` `IAMRoleBinding` | one role on one resource with its members, authoritative for that role (Terraform's `google_*_iam_binding`): members added out of band are removed; Config Connector's `IAMPolicyMember` is additive and `IAMPartialPolicy` keeps what it lists | written here (`pkg/iampolicy`), reading and writing the policy over HTTP |
| `iam.gcp.hfl.dev/v1alpha1` `IAMRoleMember` | one member of one role on one resource, additive (Terraform's `google_*_iam_member`), for the resource types Config Connector's `IAMPolicyMember` cannot reference: regional secrets, managed folders, log views, Cloud Run worker pools, backend services and about two dozen more; the name is a working one (`docs/design.md` section 10) | written here, over the same `pkg/iampolicy` as `IAMRoleBinding` |

## Where it sits

| | Config Connector | config-connector-extensions |
| --- | --- | --- |
| API groups | `<service>.cnrm.cloud.google.com` | `<service>.gcp.hfl.dev` |
| Example | `sql.cnrm.cloud.google.com/v1beta1` `SQLUser` | `sql.gcp.hfl.dev/v1beta1` `SQLUser` |
| Process | KCC's controller manager | our own manager (`cmd/manager`), beside KCC |
| Reconciler | Terraform, DCL or direct, per kind | KCC's direct reconciler (`directbase`), imported unchanged |

The groups never clash with Config Connector's, so both can be installed
on one cluster, and a kind can move from our group to theirs when upstream
fills the gap. Config Connector will not reconcile a kind in its own group
that it does not know (its registration controller skips CRDs missing from
its static table), which is why a gap kind lives in our group, in our
process.

A gap kind references Config Connector kinds the same way KCC kinds
reference each other: `SQLUser.spec.instanceRef.name` names a
`sql.cnrm.cloud.google.com` `SQLInstance` in the same namespace, or
`instanceRef.external` gives `projects/<p>/instances/<i>`.

Annotations mean what they mean in Config Connector, because the reconciler
is Config Connector's: `cnrm.cloud.google.com/deletion-policy: abandon`,
`cnrm.cloud.google.com/project-id`, `.../reconcile-interval-in-seconds`,
`.../state-into-spec` and the actuation mode from `ConfigConnector` /
`ConfigConnectorContext`. An uninstall abandons cloud resources rather
than deleting them (KCC's deletion defender, also imported unchanged).

## The copy-upstream rule

> Where the official one already has a resource on master, copy their code
> as the implementation, changing as little as possible.

So:

- Config Connector is pinned to one commit of `master` (`UPSTREAM.md`) and
  used as a Go module from a local checkout. Its framework (the direct
  reconciler, references, identity, config, mockgcp) is imported, never
  edited in this tree.
- Per-kind code is copied from the nearest upstream source with its licence
  header kept and a "Modified by High Five Labs" note saying what changed.
  For `SQLUser`, upstream master reconciles it with Terraform, so there is
  no direct controller to copy: the Go types come from KCC's generated
  `SQLUser` types (same schema), and the controller is KCC's direct
  `SQLSSLCert` controller (the sibling Cloud SQL sub-resource that already
  moved from Terraform to direct) changed for users. See `docs/design.md`.
- Any change to upstream code we need (so far only the mockgcp fake) is a
  numbered patch in `patches/k8s-config-connector/`, applied to the
  checkout by `hack/upstream.sh`, and is meant to go upstream.

## Versions and retiring kinds

A config-connector-extensions release names the Config Connector release it is installed
beside: the two must agree on the API surface (references, annotations,
`ConfigConnector` and `ConfigConnectorContext`). When a Config Connector
release ships a kind or field that config-connector-extensions fills, our
kind is deprecated for a support window, with a documented move to Config
Connector's group (`docs/design.md` section 3.4).

The first case: an application's Google service account signs in to
Cloud SQL as an IAM database user, which starts with no privileges.
Without `databaseRoles`, someone runs a `GRANT` once by hand.

## Installing it

Beside Config Connector, with its operator or its install bundle:

```sh
ko apply -k config/default    # CRDs, the operator (config-connector-extensions-system)
```

The operator (`cmd/operator`, `pkg/operator`) starts the managers where
Config Connector runs its own, so a gap kind acts as the same Google
identity Config Connector would for the same namespace: in cluster mode
one manager in `cnrm-system`; in namespaced mode one per
`ConfigConnectorContext`, watching only that namespace. It holds no
Google credential.

Each manager runs as **its own Kubernetes service account**, next to
Config Connector's, carrying the same Workload Identity annotation and
only this project's rights. With Workload Identity that needs one more
binding per Google service account, the same command you ran for Config
Connector with our service account's name:

```sh
gcloud iam service-accounts add-iam-policy-binding <gsa> \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:<cluster-project>.svc.id.goog[cnrm-system/config-connector-extensions-manager]"
# namespaced mode: ...[cnrm-system/config-connector-extensions-manager-<namespace>]
```

Without it, gap objects stay not Ready with a 403 from the metadata server
while Config Connector's own objects work. With a key Secret instead of
Workload Identity, the same Secret is mounted the same way and nothing
more is needed.

For a quick trial where you cannot change IAM, the managers can share
Config Connector's own service account instead: `--identity=config-connector`
on the operator, or the annotation `gcp.hfl.dev/identity: config-connector`
on the `ConfigConnector` object or one `ConfigConnectorContext`. The
manager then holds all of Config Connector's Kubernetes rights, including
writing every Secret in cluster mode. `docs/design.md` sections 9.2 and
9.4.

## Developing it

```sh
task upstream   # fetch Config Connector at the pin (~650 MB) and apply patches/
task check      # pin check, generated files current, gofmt, vet, build, tests
task build      # bin/manager, bin/operator, bin/fakegcp, bin/fakecloud
task operator-e2e  # the operator on kind with the fake cloud (tests/operator/run.sh)
task test-live PROJECT=<project>  # the same tests against a real project (docs/live-runs.md)
```

`task test` (also `task test-mock`) runs each kind end to end: a real
kube-apiserver (envtest), Config Connector's direct reconciler, our
adapters, and Config Connector's mockgcp as a fake Google Cloud
(`pkg/test/mockenv`); and the operator against envtest. No real Google
Cloud and no recordings. With `E2E_GCP_TARGET=real`, which only
`task test-live` sets, the same tests run against a real project; every
kind has been run that way at least once (the ledger in
`docs/live-runs.md`).

```yaml
apiVersion: sql.gcp.hfl.dev/v1beta1
kind: SQLUser
metadata:
  name: app-runtime
  annotations:
    cnrm.cloud.google.com/deletion-policy: abandon
spec:
  resourceID: app-runtime@my-project.iam
  type: CLOUD_IAM_SERVICE_ACCOUNT
  instanceRef:
    name: example          # a sql.cnrm.cloud.google.com SQLInstance
  databaseRoles:
  - cloudsqlsuperuser
```

`databaseRoles` behaves as Terraform's `database_roles`: the roles are sent
when the user is created, and whenever the list changes the controller
calls `users.update` with `revokeExistingRoles`, so the user ends with
exactly the listed roles, whoever granted the others. Roles granted out of
band while the list is unchanged are not detected (the controller compares
against what it last applied, `status.appliedDatabaseRoles`).

## Layout

| Path | What |
| --- | --- |
| `apis/<service>/<version>/` | each kind's types and identity (copied, see headers) |
| `config/crds/` | generated CRDs (`hack/generate.sh`) |
| `config/operator/`, `config/default/` | the operator, its RBAC and the managers' roles; the install |
| `config/fake-cloud/` | tests only: the install with the in-cluster fake cloud |
| `pkg/controller/direct/<service>/` | each kind's adapter and its mockgcp test |
| `pkg/controller/` | registers our kinds on KCC's direct reconciler and deletion defender |
| `pkg/operator/`, `cmd/operator/` | runs the managers beside Config Connector's, as their own service accounts with its identity |
| `pkg/test/mockenv/` | the envtest + mockgcp harness the kinds' tests share |
| `pkg/fakecloud/`, `cmd/fakegcp/` | tests only: mockgcp as a network service, and the manager's `--fake-cloud-url` (compiled in only with `-tags fakecloud`) |
| `cmd/manager/` | the controller manager |
| `cmd/fakecloud/` | Config Connector's manager and ours against mockgcp, on the host (`docs/fake-cloud.md`) |
| `tests/operator/` | the operator on kind |
| `patches/k8s-config-connector/` | our changes to the upstream checkout, in order |
| `hack/` | fetch and check upstream, generate, regenerate mock protos |
| `hack/live/` | live runs against a real project and the sweeper (`docs/live-runs.md`) |
| `UPSTREAM.md` | the pinned commits |
| `docs/design.md` | architecture, sync, testing, choosing gaps, what is left |
| `docs/fake-cloud.md` | Config Connector's manager and ours against mockgcp on kind |
| `docs/live-runs.md` | the mock/live switch, the live-test service account, the ledger of live runs |

## Contributing

See `CONTRIBUTING.md`. Commits are signed off under the Developer
Certificate of Origin.

## Licence

Apache-2.0 (`LICENSE`). Code copied from Config Connector stays under its
Apache-2.0 licence with Google's copyright: each copied file keeps its
header and says how it was changed, and `NOTICE` names the sources. Binaries
also link code that Config Connector depends on, including MPL-2.0
packages from HashiCorp's Terraform plugin SDK (`NOTICE`). Nothing is
copied from the Terraform Google provider; its behaviour is followed, not
its code.
