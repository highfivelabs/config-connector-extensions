---
kinds: [ContainerClusterFields, ContainerNodePoolFields]
apiVersion: container.gcp.hfl.dev/v1alpha1
---
# ContainerClusterFields, ContainerNodePoolFields

`container.gcp.hfl.dev/v1alpha1`

**Gap it fills.** GKE cluster and node pool fields Config Connector's `ContainerCluster` and `ContainerNodePool` lack, set beside them as field kinds (`docs/design.md` section 6.6, pattern 2, and decision 37): on a cluster, `secretManagerConfig`, `rbacBindingConfig`, `anonymousAuthenticationConfig`, `addonsConfig.statefulHaConfig` and `nodePoolAutoConfig.nodeKubeletConfig.insecureKubeletReadonlyPortEnabled`; on a node pool, `config.kubeletConfig`'s `insecureKubeletReadonlyPortEnabled`, `containerLogMaxSize`, `containerLogMaxFiles`, `allowedUnsafeSysctls` and `maxParallelImagePulls`

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

The live test skips unless `E2E_GKE=true` (a zonal cluster with one
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

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-16 | `7be2586` | fail (an update count the fake's defaults break; fixed) | skipped (no `E2E_GKE`) (`cce-11160523-1e83`) | none | none |
| 2026-11-16 | `4b15567` | pass | fail: the Secret Manager add-on needs Workload Identity (`cce-11160547-aae3`) | the fake took it; now refuses | one zonal cluster (1 `e2-small` node) and a one-node pool, deleted |
| 2026-11-16 | `815f129` | pass | fail: each update about 7 minutes, the test's 15-minute wait ran out; the cluster's delete refused during an update, swept by the next run (`cce-11160623-50f0`) | none | one zonal cluster (1 `e2-small` node) and a one-node pool, deleted |
| 2026-11-16 | `eefbc0b` | pass | fail on one case only: the Terraform-reset case read the node pool before the resync's update ran (a test bug, fixed); every other case passed, cluster fields and node pool fields (`cce-11160727-382a`) | none | one zonal cluster (1 `e2-small` node) and a one-node pool, deleted |
