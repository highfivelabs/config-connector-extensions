---
kinds: [ComputeOrganizationSecurityPolicyAssociation]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeOrganizationSecurityPolicyAssociation

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** attaches a hierarchical firewall policy to an organization or folder, as a stable kind; Config Connector's is alpha only (decision 13)

**Source.** Config Connector's direct `ComputeOrganizationSecurityPolicyAssociation`

## Notes

2026-11-15, the builder's RECPF answer ("a stable Cloud Armor
association"). Fabric's `cloud_armor_policy` (the `project` and `folder`
modules, v60.0.0) is Terraform's
`google_compute_organization_security_policy_association`: name (default
`project-<number>` or `folder-<number>`), `attachment_id`
(`projects/<number>`, `folders/<id>`), `policy_id`, and
`excluded_folders` and `excluded_projects` (refused by fabric on a
project). That is this kind, already stable here (decision 13); the
catalog's charts rendered Config Connector's `v1alpha1` one behind
`allow_alpha`. Added: `excludedFolders` and `excludedProjects`, the v1
API's fields (in the compute v1 REST and proto clients at the pin), mapped
both ways and immutable with the rest of the spec, and refused by CEL on a
`projects/` attachment, as fabric refuses them. The end-to-end test now
attaches a `CLOUD_ARMOR` policy to `projects/<number>` and to a folder
with exclusions, and checks the refusal; mockgcp stores exclusions as sent
and accepts any attachment (it checks none). Live: pending, as the policy
(`ComputeOrganizationSecurityPolicy` lives under an organization or
folder; `docs/design.md` section 7, item 30): the live-test account acts on
the live-test project only.

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
