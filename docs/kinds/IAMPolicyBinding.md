---
kinds: [IAMPolicyBinding]
apiVersion: iam.gcp.hfl.dev/v1alpha1
---
# IAMPolicyBinding

`iam.gcp.hfl.dev/v1alpha1`

**Gap it fills.** an IAM v3 policy binding: a principal access boundary policy bound to the principals of an organization, folder or project. Terraform's `google_iam_projects_policy_binding` (rank 57 of the catalog's `blocked-kinds.md`, proposed there as `IamProjectsPolicyBinding`; the `geap-agent-gateway` module), `google_iam_folders_policy_binding` and `google_iam_organizations_policy_binding`. Config Connector has no released kind.

**Source.** hand-written, not generated: IAM v3 has no Discovery document (at the pinned discovery-artifact-manager commit or its master, 2026-11-13), so `hack/discokind` cannot express it. The types are copied from Config Connector's open PR #14133 (head `dcb10c53`, types, identity, reference and mapper, no controller); the identity, mapper and controller are written here in the shape of our `IAMPrincipalAccessBoundaryPolicy` (`UPSTREAM.md`).

## Notes

The name is Config Connector's (PR #14133, decision 18); decision 12
already kept `IAMPolicyBinding` for this resource. It starts at
`v1alpha1` because Config Connector has released no kind for it
(decision 26's last sentence). When upstream releases its kind, ours
retires (design.md section 3.4); upstream's takes an organization only.

The spec differs from the PR's in four ways:

- **The parent** is exactly one of `organizationRef`, `folderRef` and
  `projectRef` (decision 18, as Terraform's three resources), not an
  organization only. A CEL rule refuses none or two.
- **The policy** is `policyRef`: an `IAMPrincipalAccessBoundaryPolicy`
  of ours by `name` (its `status.externalRef` is used, so it must be
  Ready), or a policy's resource name as `external`. The PR takes a
  string.
- **etag** is in `status.observedState`, with the policy kind the API
  set; the PR has etag in the spec.
- **`location`** is optional and defaults to `global`, the only location
  the API has.

`target`, `policyKind`, `policyRef`, the parent, `location` and
`resourceID` are immutable (CEL). `displayName`, `annotations` and
`condition` change in place: the controller patches with an update mask
of the fields that differ, sending the binding's etag, so a resync
writes nothing. `policyKind` left out is the API's to set and is not
compared.

mockgcp served no policy bindings; patch 0044 adds them for the three
parents (`docs/fake-cloud.md`).

**Live: skips.** A binding needs a principal access boundary policy,
which lives under an organization (a live run never touches one), and
the right to bind it (`iam.principalaccessboundarypolicies.bind` on the
policy, `iam.policyBindings.*` on the project), neither of which
`cce-live-tests` has. With them, `E2E_IAM_POLICY_BINDING_POLICY=organizations/{org}/locations/global/principalAccessBoundaryPolicies/{id}`
runs the project's binding live: it binds that policy to the run's
project with a condition no real principal meets
(`principal.subject == 'nobody@example.com'`), so the boundary changes
nothing for the project's principals; the folder's binding runs on the
fake only. The sweeper deletes the project's bindings whose ID carries a
run ID, behind the same switch.

**An access policy, bound to a resource** (2026-11-13, for the catalog's
`geap-agent-gateway` module). `target.resource` binds an access policy to
the resource it applies to: an organization, folder or project
(`//cloudresourcemanager.googleapis.com/projects/...`) or a resource in
it, such as an agent gateway. `target` takes exactly one of
`principalSet` and `resource` (CEL). With `policyKind: ACCESS`,
`policyRef.name` names an `IAMAccessPolicy`; otherwise an
`IAMPrincipalAccessBoundaryPolicy`. `policyRef.external` takes either
policy's resource name. A binding being deleted takes its name from
`status.externalRef` and resolves neither its parent nor its policy, so a
policy deleted with it does not hold it (and the policy's delete waits for
the binding). `TestIAMPolicyBindingOfAnAccessPolicy` binds a project's
policy to the project and to an agent gateway and deletes all three at
once; it skips live (no access policy role for the live-test account).

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
