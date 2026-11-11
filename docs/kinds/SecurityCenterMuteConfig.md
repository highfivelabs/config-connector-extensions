---
kinds: [SecurityCenterMuteConfig]
apiVersion: securitycenter.gcp.hfl.dev/v1alpha1
---
# SecurityCenterMuteConfig

`securitycenter.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Security Command Center mute config of an organization, folder or project (Terraform's `google_scc_mute_config`, `google_scc_folder_mute_config`, `google_scc_project_mute_config`); Config Connector's `SecurityCenterMuteConfig` is alpha and takes an organization only (`docs/design.md` decision 16)

**Source.** Config Connector's direct `SecurityCenterMuteConfig`, with the parents of upstream PR #7673

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
