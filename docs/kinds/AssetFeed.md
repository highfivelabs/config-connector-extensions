---
kinds: [AssetFeed]
apiVersion: asset.gcp.hfl.dev/v1beta1
---
# AssetFeed

`asset.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Cloud Asset Inventory feed of an organization, folder or project (Terraform's `google_cloud_asset_organization_feed`, `_folder_feed`, `_project_feed`) that can be reconciled and deleted after it is made; Config Connector's `AssetFeed` (`v1beta1`) refuses every reconcile of a project's feed after the create, so the feed can never be updated or deleted (`docs/design.md` decision 19; the builder, 2026-11-15, RECPF)

**Source.** Config Connector's direct `AssetFeed`, with its parent comparison fixed

## Notes

2026-11-15, for the catalog's `project` and `organization` charts (catalog
PR #215). Google Cloud names a project's feed `projects/<number>/feeds/<id>`
whatever the create named, and Config Connector's identity
(`apis/asset/v1beta1/assetfeed_identity.go` at the pin) compares that number
with the project ID the `projectRef` resolves to on every reconcile after the
create: "spec.projectRef changed, expect 886048300, got catalog-mock-prj".
With the project named by a `Project` object, the delete also waits on the
object ("referenced Project catalog-project/mock not found") when it goes
first. Upstream, checked 2026-11-15 (read-only): the same code at the pin and
on `master`; no open pull request touches the comparison.

Ours keeps upstream's schema and controller with three changes:

- the identity treats a project's number and its ID as the same project:
  when they differ, it reads the ID's number with Resource Manager's
  `projects.get` (`resourcemanager.projects.get`, which any user of the
  project has) and compares; another project is still refused;
- a delete names the feed from `status.externalRef` and does not resolve
  the parent (the rule `pkg/discokind` and `IAMBinding` follow), and a
  delete of a feed never made whose parent is gone makes no call;
- the update compares with the name the API returned, so the number is no
  difference.

Exactly one parent, and the parent and `resourceID` immutable, by CEL
(decision 20; upstream refused a change in the identity). `v1beta1`, as
`NetworkServicesGateway` (decisions 19 and 26): Config Connector's kind is
`v1beta1`, and the catalog's charts refuse alpha kinds.

`apis/asset/v1beta1/assetfeed_identity_test.go` reproduces the bug against
upstream's own identity (it fails if upstream fixes it: then this kind
retires, section 3.4) and checks ours. The end-to-end test
(`pkg/controller/direct/asset/assetfeed_mockgcp_test.go`) makes a project's
feed by project ID, changes it after the create (one `PATCH`), resyncs
(no write), deletes it, deletes a feed whose `Project` object goes first,
abandons one, and makes a folder's and an organization's feed on the fake.
mockgcp serves feeds unpatched and names a project's feed by its number, as
the API does. The catalog's charts can now name the project by ID or by its
`Project` object, so `asset_feeds` works on a new project too.

A live run needs `cloudasset.feeds.*` for the live-test service account
(`roles/cloudasset.owner`; it has `roles/cloudasset.viewer` and
`roles/pubsub.admin`, and `cloudasset` and `pubsub` are on in the project,
checked 2026-11-15); the test skips live unless `E2E_CLOUD_ASSET=true`
(`docs/live-runs.md`). The folder's and the organization's feeds are fake
only.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-15 | `c419344` | PASS | pending, needs `roles/cloudasset.owner` for `cce-live-tests`; then `E2E_CLOUD_ASSET=true` (run `cce-11152322-2857` skipped live) | not run | none |
| 2026-11-18 | `23e425b`+ | AssetFeed | PASS | FAIL | `TestAssetFeed` live: 403 `Cloud Resource Manager API has not been used in project 50660598795` (`hfl-testing-8347a2`, the impersonated account's home project, where tf-infra lists `cloudresourcemanager.googleapis.com` but it is not enabled). Whether `live-tests@` holds `pubsub.admin` in `hfl-dev-514205` for the test's topic is not yet seen. Run `cce-11181655-f14f` and `cce-11181701-a6f8` | none made |
