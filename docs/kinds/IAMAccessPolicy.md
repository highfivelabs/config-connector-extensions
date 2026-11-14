---
kinds: [IAMAccessPolicy]
apiVersion: iam.gcp.hfl.dev/v1beta1
---
# IAMAccessPolicy

`iam.gcp.hfl.dev/v1beta1`

**Gap it fills.** an IAM v3 access policy of an organization, folder or project: Terraform's `google_iam_projects_access_policy` (and its organization and folder forms), which the catalog's `geap-agent-gateway` module uses for an agent gateway's access rules. Config Connector has `IAMAccessPolicy` at `v1alpha1` with an organization parent only and no controller. Ours keeps the name (decision 18) at `v1beta1` (decision 26).

**Source.** types copied from Config Connector's `apis/iam/v1alpha1/iamaccesspolicy_types.go` with every parent; upstream's `AccessPolicyDetails` and observed state are imported. The identity and controller are written here after `IAMPolicyBinding`'s identity and `IAMPrincipalAccessBoundaryPolicy`'s controller, on `cloud.google.com/go/iam/apiv3` (v1.14.0, the first with access policies).

## Notes

The spec: exactly one of `organizationRef`, `folderRef` and `projectRef`
(CEL), `location` (`global` when left out) and `resourceID`, all
immutable; `displayName` (at most 63 characters), `annotations` and
`details.rules`, each with `effect` (`DENY` or `ALLOW`), `principals`,
`excludedPrincipals`, `operation.permissions` and
`operation.excludedPermissions`, and `conditions`.

The API's update has no mask: it replaces the policy. The controller
compares the display name, annotations and rules with the policy it reads
and, when any differs, sends the whole spec with the read etag. Nothing
differing, nothing is sent.

Delete deletes the policy without `force`: the API refuses a policy a
binding names, so the delete is retried until the binding is gone (an
`IAMPolicyBinding` deleted with it). A delete takes the policy's name from
`status.externalRef`, so a parent object already gone does not hold it.
`deletion-policy: abandon` keeps the policy.

`status` has `externalRef`, `etag` and `observedState` (`uid`,
`createTime`, `updateTime`).

mockgcp patch 0045 serves the policies (`docs/fake-cloud.md`). What the
fake cannot show: whether principals, permissions and conditions are
valid, what a policy denies, and permissions.

IAM allows `GetAccessPolicy` 5 times a minute per project. A refusal for
quota (429) requeues the policy after a growing wait and never marks it
failed (`docs/design.md` decision 31; `TestIAMAccessPolicyRefusedForQuota`
on the fake).

Live is pending: the live-test account has no access policy role
(`roles/iam.accessPolicyAdmin`), so the test skips live.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-14 | `64f375e` | PASS | SKIP | `TestIAMAccessPolicyRefusedForQuota` passes on the fake (four refused reads, then Ready; two refused reads on delete, then gone; no UpdateFailed or DeleteFailed); live skipped: no access policy role (run `cce-11140351-e031`) | none |
