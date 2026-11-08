# gaps-gcp

Kubernetes kinds for Google Cloud that
[Config Connector](https://github.com/GoogleCloudPlatform/k8s-config-connector)
(KCC) does not have yet, or has without a field we need. It is the GCP
member of a set of gap-filling providers: `gaps-gcp`, `gaps-aws` (beside
ACK) and `gaps-azure` (beside Azure Service Operator).

The kinds:

| Kind | Gap it fills | Source |
| --- | --- | --- |
| `sql.gcp.hfl.dev/v1beta1` `SQLUser` | `spec.databaseRoles`: Config Connector's `SQLUser` cannot grant a Cloud SQL user its database roles (`User.databaseRoles`, Terraform's `google_sql_user.database_roles`) | ours, on upstream's `SQLSSLCert` controller; upstream PR #7148 is watched |
| `sqladmin.gcp.hfl.dev/v1alpha1` `SQLAdminBackup` | an on-demand Cloud SQL backup (`backupRuns`); upstream master has the types but no controller | upstream PR #12553 |
| `secretmanager.gcp.hfl.dev/v1beta1` `SecretManagerSecret`, `SecretManagerSecretVersion` | `spec.location`: a regional secret (Terraform's `google_secret_manager_regional_secret`), in no Config Connector release or on master | master's direct controllers plus upstream PR #6651 |

## Where it sits

| | Config Connector | gaps-gcp |
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
  moved from Terraform to direct) changed for users. See `docs/plan.md`.
- Any change to upstream code we need (so far only the mockgcp fake) is a
  numbered patch in `patches/k8s-config-connector/`, applied to the
  checkout by `hack/upstream.sh`, and is meant to go upstream.

## Relation to Bundt

Bundt treats each gaps repository as a Provider in its catalog: a provider
bundle pins the upstream controller (Config Connector) version, the
`gaps-gcp` version, and the shared API model commit, because the two must
agree on the API surface. A Package that needs `SQLUser.databaseRoles`
requires the provider that serves `sqlusers.sql.gcp.hfl.dev`. `gcp`, `aws`
and `azure` are reserved organisation names in Bundt so that
`<service>.gcp.hfl.dev` never collides with a Package's
`<team>.<org>.hfl.dev`. When upstream ships the field, the bundle that
carries both versions carries the migration, and our kind is deprecated
for a support window.

The motivating case is in the bundt repository, `charts/infra/gcp`:
bntd's service account is an IAM database user, which starts with no
privileges; today someone runs a `GRANT` once by hand.

## Installing it

Beside Config Connector, with its operator or its install bundle:

```sh
ko apply -k config/default    # CRDs, the operator (gaps-gcp-system)
```

The operator (`cmd/operator`, `pkg/operator`) starts gaps-gcp's managers
where Config Connector runs its own and as Config Connector's own
Kubernetes service accounts, so a gap kind acts as the same Google
identity Config Connector would for the same namespace: in cluster mode one
manager in `cnrm-system` as `cnrm-controller-manager`; in namespaced mode one
per `ConfigConnectorContext`, watching only that namespace, as
`cnrm-controller-manager-<namespace>`. With a key Secret instead of Workload
Identity, it mounts the same Secret the same way. It never makes a service
account or holds a Google credential. `docs/plan.md` section 9.

## Developing it

```sh
make upstream   # fetch Config Connector at the pin (~650 MB) and apply patches/
make check      # pin check, generated files current, gofmt, vet, build, tests
make build      # bin/manager, bin/operator, bin/fakegcp, bin/fakecloud
make operator-e2e  # the operator on kind with the fake cloud (tests/operator/run.sh)
```

`make test` runs each kind end to end: a real kube-apiserver (envtest),
Config Connector's direct reconciler, our adapters, and Config Connector's
mockgcp as a fake Google Cloud (`pkg/test/mockenv`); and the operator
against envtest. No real Google Cloud and no recordings.

```yaml
apiVersion: sql.gcp.hfl.dev/v1beta1
kind: SQLUser
metadata:
  name: bundt-bntd
  annotations:
    cnrm.cloud.google.com/deletion-policy: abandon
spec:
  resourceID: bntd@my-project.iam
  type: CLOUD_IAM_SERVICE_ACCOUNT
  instanceRef:
    name: bundt            # a sql.cnrm.cloud.google.com SQLInstance
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
| `pkg/operator/`, `cmd/operator/` | runs the managers as Config Connector's service accounts |
| `pkg/test/mockenv/` | the envtest + mockgcp harness the kinds' tests share |
| `pkg/fakecloud/`, `cmd/fakegcp/` | tests only: mockgcp as a network service, and the manager's `--fake-cloud-url` (compiled in only with `-tags fakecloud`) |
| `cmd/manager/` | the controller manager |
| `cmd/fakecloud/` | Config Connector's manager and ours against mockgcp, on the host (`docs/fake-cloud.md`) |
| `tests/operator/` | the operator on kind |
| `patches/k8s-config-connector/` | our changes to the upstream checkout, in order |
| `hack/` | fetch and check upstream, generate, regenerate mock protos |
| `UPSTREAM.md` | the pinned commits |
| `docs/plan.md` | architecture, sync, testing, gap ranking, what is left |

## Licence

None. The repository is private and High Five Labs attaches no licence to
its own work here (builder, 2026-11-07). Code copied from Config Connector
stays under Config Connector's Apache-2.0: each copied file keeps Google's
copyright and licence header and says how it was changed, as that licence
requires; see `NOTICE`. Nothing is copied from the Terraform provider
(MPL-2.0); its behaviour is followed, not its code.
