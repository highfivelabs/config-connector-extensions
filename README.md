# gaps-gcp

Kubernetes kinds for Google Cloud that
[Config Connector](https://github.com/GoogleCloudPlatform/k8s-config-connector)
(KCC) does not have yet, or has without a field we need. It is the GCP
member of a set of gap-filling providers: `gaps-gcp`, `gaps-aws` (beside
ACK) and `gaps-azure` (beside Azure Service Operator).

The first kind is `SQLUser` with `spec.databaseRoles`: Config Connector's
`SQLUser` cannot grant a Cloud SQL user its database roles (the Cloud SQL
Admin API's `User.databaseRoles`, Terraform's `google_sql_user.database_roles`).

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

## Using it

```sh
make upstream   # fetch Config Connector at the pin (~650 MB) and apply patches/
make check      # pin check, generated files current, gofmt, vet, build, tests
make build      # bin/manager
```

`make test` runs the controller end to end: a real kube-apiserver
(envtest), Config Connector's direct reconciler, our adapter, and
Config Connector's mockgcp as a fake Cloud SQL Admin API. No real Google
Cloud and no recordings.

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
against what it last applied, `status.appliedDatabaseRoles`). It is not yet
installable as a release: there is no image,
Helm chart or RBAC yet (`docs/plan.md`, "What is left").

## Layout

| Path | What |
| --- | --- |
| `apis/sql/v1beta1/` | `SQLUser` types and identity (copied, see headers) |
| `config/crds/` | generated CRDs (`hack/generate.sh`) |
| `pkg/controller/direct/sql/` | the `SQLUser` adapter and its test |
| `pkg/controller/` | registers our kinds on KCC's direct reconciler and deletion defender |
| `cmd/manager/` | the controller manager |
| `patches/k8s-config-connector/` | our changes to the upstream checkout, in order |
| `hack/` | fetch and check upstream, generate, regenerate mock protos |
| `UPSTREAM.md` | the pinned commits |
| `docs/plan.md` | architecture, sync, testing, gap ranking, what is left |

## Licence

Apache-2.0 (`LICENSE`). Most code here is copied from Config Connector,
which is Apache-2.0, Copyright Google LLC; each copied file keeps Google's
header and says how it was changed. See `NOTICE`. Nothing is copied from
the Terraform provider (MPL-2.0); its behaviour is followed, not its code.
