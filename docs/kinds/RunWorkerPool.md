---
kinds: [RunWorkerPool]
apiVersion: run.gcp.hfl.dev/v1beta1
---
# RunWorkerPool

`run.gcp.hfl.dev/v1beta1`

**Gap it fills.** a stable Cloud Run worker pool: Config Connector's `RunWorkerPool` is `v1alpha1` only (decision 26), and lacks the revision template's labels and `binaryAuthorization.policy` (application item 6)

**Source.** Config Connector's direct `RunWorkerPool` types, identity, mappers, reference resolution and controller; its container, volume and VPC access types are our `RunService`'s (the same upstream types)

## Which kind to use

This one where an alpha kind is refused (the catalog's charts) or the
template's labels or a named Binary Authorization policy are needed;
otherwise either. The spec is Config Connector's `RunWorkerPool`'s plus
`template.labels` and `binaryAuthorization.policy`. To move a worker
pool: create this kind's object with `spec.resourceID` naming it (it is
acquired, then updated to the spec), then delete Config Connector's object
with `cnrm.cloud.google.com/deletion-policy: abandon`. Never manage one
worker pool with both kinds. When Config Connector promotes its kind with
these fields, move back the same way (`docs/design.md` section 3.4).

## Notes

2026-11-23: built (application item 6's worker pool half,
`docs/design.md` sections 2.3 and 6.7), as a decision 26 stable copy.
Upstream read first (read-only): master `5d27e1c6`'s `RunWorkerPool` is the
pin's, still `v1alpha1` and direct only, and no open pull request from
#13500 touches it. The controller is upstream's: it compares the spec's
top-level fields with the worker pool as read (server defaults taken from
it) and patches with an update mask of the ones that differ. CEL:
`projectRef`, `location` and `resourceID` immutable; `binaryAuthorization`
takes one of `useDefault` and `policy` (the same rule and field now on
`RunService`, whose Binary Authorization type this kind shares). A delete
of a worker pool that was made resolves nothing (`status.externalRef`).

Not built: the template's `gpuZonalRedundancyDisabled` (in the pinned Go
client but not in mockgcp's worker pool protos, so the fake drops it;
`nodeSelector` is built, as upstream has it), `template.client` and
`clientVersion` (newer than the pinned Go client), `volumeMounts[].subPath`
(not in mockgcp's protos), and the preview build and source fields.

The test (`TestRunWorkerPool`) checks that `v1alpha1` is not served, refuses
a default with a named policy, makes a worker pool with template labels
and a named policy scaled to no instances (and checks them in the cloud),
a resync with no write, a template labels change sent with the mask
`template`, a policy change sent alone, the immutable location, delete and
abandon. Live: pending, `task test-live PROJECT=$GCP_PROJECT KINDS=run
LIVE_RUN=TestRunWorkerPool` (two worker pools of no instances on Cloud
Run's public sample job image: free; the named policy only with
`E2E_RUN_BINAUTHZ_POLICY`, and the policy change skips live). The
account's `roles/run.admin` and `roles/iam.serviceAccountUser` on the
compute default service account cover it. What to look for: the server
defaults the real API gives a worker pool beyond mockgcp's, and whether
it takes `binary_authorization` in an update mask.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
