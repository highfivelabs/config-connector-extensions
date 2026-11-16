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

To confirm live: that `desiredAddonsConfig` naming one add-on leaves the
others (the test checks HTTP load balancing stays off), and how the API
answers a whole-`kubeletConfig` update without these fields. The live
test skips unless `E2E_GKE=true` (a zonal cluster with one `e2-small`
node and a one-node pool, made and deleted by the test; the sweeper
deletes leftover clusters by label).

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
