---
kinds: [ContainerClusterFields, ContainerNodePoolFields]
apiVersion: container.gcp.hfl.dev/v1beta1
---
# ContainerClusterFields, ContainerNodePoolFields

`container.gcp.hfl.dev/v1beta1` (since 2026-11-16, decision 27; first `v1alpha1`)

**Gap it fills.** GKE cluster and node pool fields Config Connector's `ContainerCluster` and `ContainerNodePool` lack, set beside them as field kinds (`docs/design.md` section 6.6, pattern 2, and decision 37): on a cluster, `secretManagerConfig`, `rbacBindingConfig`, `anonymousAuthenticationConfig`, `addonsConfig.statefulHaConfig` and `nodePoolAutoConfig.nodeKubeletConfig.insecureKubeletReadonlyPortEnabled`; on a node pool, `config.kubeletConfig`'s `insecureKubeletReadonlyPortEnabled`, `containerLogMaxSize`, `containerLogMaxFiles`, `allowedUnsafeSysctls` and `maxParallelImagePulls`, and (2026-11-23) `evictionSoft`, `evictionSoftGracePeriod`, `evictionMinimumReclaim`, `evictionMaxPodGracePeriodSeconds`, `memoryManager`, `topologyManager` and `singleProcessOomKill`

**Source.** written here from the GKE API; the references are Config Connector's `ContainerClusterRef` and `ContainerNodePoolRef`, and the operation wait is copied from Config Connector's direct `ContainerNodePool`

## Notes

2026-11-16: built after the builder named the pattern (section 10
question 5, decision 37). These are the fields the catalog's
`gke-cluster-standard` and `gke-nodepool` charts refuse today for want of
a field.

`ContainerClusterFields` sends one `clusters.update` per owned field that
differs, each with a single `desired*` field (the API takes one change at
a time): `desiredSecretManagerConfig`, `desiredRbacBindingConfig`,
`desiredAnonymousAuthenticationConfig`, `desiredAddonsConfig` naming only
the Stateful HA add-on, and `desiredNodePoolAutoConfigKubeletConfig`.
Terraform's cluster update sends only the `desired*` fields of what it
changed, none of these, so Config Connector's updates leave them alone.
Delete leaves the fields as they are: the API has no "unset" for them,
only other values.

`ContainerNodePoolFields`: `nodePools.update` takes `kubeletConfig` whole,
so the controller reads the node pool and sends its `kubeletConfig` with
only the owned fields changed; Config Connector's kubelet fields
(`cpuCfsQuota`, `podPidsLimit`, ...) go back as read. The other way round
is not safe: when one of Config Connector's own kubelet fields changes,
Terraform sends the whole `kubeletConfig` from its schema, which lacks
these fields, and can reset them; this object sets them again on its
next reconcile (the test makes that update and checks). Delete unsets the
owned fields (GKE's defaults apply again) and keeps the rest; with
`deletion-policy: abandon` they stay.

Both never create or delete the cluster or node pool (a missing one is
waited for), and refuse a second object of the same kind for the same
resource in the namespace. Patch 0053 makes mockgcp's `clusters.update`
take the five `desired*` fields and merge `desiredAddonsConfig` (it
replaced the whole add-ons config).

Both passed live on 2026-11-16 (run `cce-11160848-7b8d`); with their
CRDs unchanged they can move to `v1beta1` under decision 27 (not done in
this change). The live test skips unless `E2E_GKE=true` (a zonal cluster with one
`e2-small` node and a one-node pool, made and deleted by the test; the
sweeper deletes leftover clusters by label) and takes about an hour: run
it with `LIVE_TIMEOUT=150m`.

What live taught (2026-11-16):
- The Secret Manager add-on needs Workload Identity: 400
  "Secret_manager_config.enabled cannot be true if
  workload_identity_config.workload_pool is not set" (run
  `cce-11160547-aae3`). The test's cluster has a pool, mockgcp refuses
  the same, and a refused field no longer stops the others: the
  controller tries every owned field and reports the refusals together.
- Each `clusters.update` takes about 7 minutes (five took 20); the test
  waits up to 75 minutes live (run `cce-11160623-50f0`).
- A cluster refuses a delete while an update runs (400 "Cluster is
  running incompatible operation"); the test's cleanup and the sweeper
  try again for up to 30 minutes. That run's cluster was left and swept
  by the next run.
- `desiredAddonsConfig` naming only Stateful HA leaves the other add-ons
  (HTTP load balancing stayed off), and `nodePools.update` replaces the
  whole `kubeletConfig` (Terraform's update reset this object's fields
  until its next reconcile), both as the fake now does (run
  `cce-11160727-382a`).

2026-11-16: `ContainerNodePoolFields`' `resourceRef.name` resolves the
node pool through its `clusterRef` here (`nodepoolref.go`), not through
Config Connector's `ContainerNodePoolRef.Normalize`. The Terraform-based
`ContainerNodePool` has no `status.externalRef`, and upstream's fallback
parses `clusterRef.external` as a full cluster id only; a bare cluster
name (Terraform's `cluster` field, and what the catalog's `gke-nodepool`
chart writes, because a full id breaks Config Connector's delete of the
node pool) left the object "reference not ready" for good (found in
catalog #240's fake-cloud run). Now:

| The node pool's `clusterRef` | The node pool resolved as |
| --- | --- |
| `external: <cluster>` (bare name) | `projects/<the node pool's cnrm.cloud.google.com/project-id annotation>/locations/<its spec.location>/clusters/<cluster>/nodePools/<its resourceID or name>` |
| `external: projects/<p>/locations/<l>/clusters/<c>` (or `zones/<z>`) | that cluster; location the node pool's `spec.location`, else the cluster's |
| `name: <ContainerCluster>` | through the `ContainerCluster` object, as before |

A node pool's `status.externalRef`, if one is ever set, is used first. The
project comes only from the annotation (Config Connector's admission
webhook always sets it on its objects; the managers do not read
namespaces). The fake-cloud test covers the bare name and the full id.
No live run: the change reads Kubernetes objects only and makes no new
API call.

2026-11-23 (foundation item 14): `ContainerNodePoolFields` gains seven
more kubelet settings with no Config Connector field and no upstream pull
request (pull requests searched for `eviction`, `memoryManager` and
`kubeletConfig`: only #14187, `allowedUnsafeSysctls`, which this kind
already has): soft eviction (`evictionSoft`, `evictionSoftGracePeriod`,
`evictionMinimumReclaim`, each owned whole, and
`evictionMaxPodGracePeriodSeconds`), the memory and topology managers,
and `singleProcessOomKill`. They go in the same whole-`kubeletConfig`
write and are cleared on delete. Tested on the fake
(`TestContainerFields`, "eviction and manager fields are written, the
others kept": the spec gains them, the node pool holds them with this
object's earlier fields and Config Connector's `podPidsLimit` kept, a
resync writes nothing, delete clears them). The `v1beta1` CRD gains
optional fields only. Live pending (`E2E_GKE=true`): the API's checks
of the values (a soft threshold without its grace period, the
topology manager's policies), and whether it answers a cleared message
as unset or empty (the comparison takes the two as one). The rest of
item 14 is recorded in `hack/coverage/foundation.yaml`.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-16 | `7be2586` | fail (an update count the fake's defaults break; fixed) | skipped (no `E2E_GKE`) (`cce-11160523-1e83`) | none | none |
| 2026-11-16 | `4b15567` | pass | fail: the Secret Manager add-on needs Workload Identity (`cce-11160547-aae3`) | the fake took it; now refuses | one zonal cluster (1 `e2-small` node) and a one-node pool, deleted |
| 2026-11-16 | `815f129` | pass | fail: each update about 7 minutes, the test's 15-minute wait ran out; the cluster's delete refused during an update, swept by the next run (`cce-11160623-50f0`) | none | one zonal cluster (1 `e2-small` node) and a one-node pool, deleted |
| 2026-11-16 | `eefbc0b` | pass | fail on one case only: the Terraform-reset case read the node pool before the resync's update ran (a test bug, fixed); every other case passed, cluster fields and node pool fields (`cce-11160727-382a`) | none | one zonal cluster (1 `e2-small` node) and a one-node pool, deleted |
| 2026-11-16 | `d59760e` | not run: the VM's `/tmp` filled while building (the same code passed the fake in `cce-11160727-382a` and in `task ci` on `d59760e`) | **pass**, every case of both kinds: five cluster fields set one update each (about 7 minutes per update), a changed field written alone, delete leaves them; node pool fields set with Config Connector's kubelet fields kept, Terraform's whole-config update resets them and the next reconcile sets them again, delete unsets them, abandon keeps them (`cce-11160848-7b8d`, 58 minutes) | none | one zonal cluster (1 `e2-small` node) and a one-node pool, deleted; sweep found nothing left |
