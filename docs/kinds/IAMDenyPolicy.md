---
kinds: [IAMDenyPolicy]
apiVersion: iam.gcp.hfl.dev/v1beta1
---
# IAMDenyPolicy

`iam.gcp.hfl.dev/v1beta1`

**Gap it fills.** an IAM deny policy on a project, folder or organization (who may not use which permissions, with exceptions and a tag condition), as a stable kind the charts allow; Config Connector's `IAMDenyPolicy` is alpha only (`docs/design.md` decision 13)

**Source.** Config Connector's direct `IAMDenyPolicy`, its status written

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

**Deleted with its parent (2026-11-13).** The delete takes the policy's
name from `status.externalRef` and does not resolve `projectRef`,
`folderRef` or `organizationRef` again, so a Project, Folder or
Organization object deleted first (everything deleted at once) does not
hold it on "referenced Folder ... not found"; `deletion-policy: abandon`
still keeps the policy. An object being deleted that never had a policy
(no `status.externalRef`) and whose parent cannot be resolved goes
without a call. The generated kinds' rule (`pkg/discokind`), applied to
this hand-written controller. `TestIAMDenyPolicy` deletes a policy whose
Folder object went first, deleted and abandoned (fake only: a live run
never touches a folder).

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-18 | `23e425b`+ | IAMDenyPolicy | PASS | PASS | after one run (`cce-11181733-4513`) whose only failure was a 429 `Policy Read Requests per minute per user` on the abandon step (the test's `get` now waits and retries). Create, resync with no write, one update for a changed rule and display name, the one-parent CRD rule, delete and abandon pass with `roles/iam.denyAdmin` on the organization (`E2E_IAM_DENY_ADMIN=true`, as `live-tests@`). Run `cce-11181742-20cc`. Already `v1beta1` | 1 deny policy on `hfl-dev-514205` (deleted); `task live-list` empty |
