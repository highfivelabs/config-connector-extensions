---
kinds: [ComputeServiceAttachmentFields]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeServiceAttachmentFields

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Private Service Connect service attachment's `reconcileConnections` and `propagatedConnectionLimit`, which Config Connector's `ComputeServiceAttachment` lacks (foundation item 11); a field kind (`docs/design.md` section 6.6, pattern 2, and decision 37) that sets only those fields on a service attachment Config Connector's `ComputeServiceAttachment` (or anything else) made

**Source.** written here from the Compute Engine API; the reference (`resourceRef`) is Config Connector's `ComputeServiceAttachmentRef` (`apis/refs/v1beta1`), resolved by its `ResolveComputeServiceAttachment`

## Notes

2026-11-23: built (foundation item 11, decision 19). `spec.resourceRef`
names Config Connector's `ComputeServiceAttachment` by `name` (resolved
from its `status.selfLink`) or any service attachment by `external`
(`projects/{project}/regions/{region}/serviceAttachments/{name}`, or a
Compute Engine URL). `spec.reconcileConnections` and
`spec.propagatedConnectionLimit` keep the API's names; at least one is
set, and a field left out is not owned (never read or written). The
controller reads the service attachment with `serviceAttachments.get` and
writes with `serviceAttachments.patch`, whose body has only the owned
fields that differ and the fingerprint just read: the API's patch is a
JSON merge patch, so nothing else is touched.

Why the field kind is safe beside Config Connector's object: Config
Connector's `ComputeServiceAttachment` runs the DCL (the only controller
it supports at the pin), and the DCL's patch body
(`newUpdateServiceAttachmentPatchRequest`) carries `description`,
`connectionPreference`, `natSubnets`, `consumerRejectLists`,
`consumerAcceptLists`, the fingerprint and the name, never these two
fields. The test sends that patch and checks the fields survive. To be
confirmed live: that the real API keeps fields a merge patch leaves out
(it says so for every Compute Engine `patch`).

Not in this kind, and why: `consumerAcceptLists[].networkUrl` (the DCL
sends the whole accept list, so a field kind would fight it; it needs a
whole-object kind); `domainNames` (immutable after create: a create-only
whole-object kind); `metadata`, `natIpsPerEndpoint` and
`producerForwardingRule` (not in the API's GA reference at the time, or
create-only; not checked live).

The object never creates or deletes the service attachment: a missing one
is waited for (not Ready, "does not exist yet"). Delete and
`deletion-policy: abandon` both leave the fields as they are: a merge
patch can only set a value, not give a field back to its default. A
second object for the same service attachment in the namespace is
refused (the oldest owns it). Patch 0054 lets mockgcp's compute patches
replace a bool or a number (it answered 500 for any scalar but a string).

The test (`TestComputeServiceAttachmentFields`) makes an internal
passthrough load balancer (network, two subnets, a TCP health check, a
backend service with no backends, a forwarding rule) and a service
attachment publishing it, all named by the run and deleted at the end;
the sweeper now also deletes service attachments and forwarding rules by
name. Live: pending (`task test-live KINDS=compute
LIVE_RUN=TestComputeServiceAttachmentFields`); what to look for: whether
the API defaults `propagatedConnectionLimit` (250) and `reconcileConnections`
on a new service attachment, and whether a patch without the fingerprint
would do.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
