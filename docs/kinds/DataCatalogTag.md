---
kinds: [DataCatalogTag]
apiVersion: datacatalog.gcp.hfl.dev/v1beta1
---
# DataCatalogTag

`datacatalog.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Data Catalog tag (a tag template's fields filled in on one entry, or one column of it), as a stable kind; Config Connector's `DataCatalogTag` is alpha only (decision 26). Terraform's `google_data_catalog_tag`, which the catalog's `data-catalog-tag` module uses (rank 12 of its `blocked-kinds.md`)

**Source.** hand-written, not generated: copied from Config Connector's direct `DataCatalogTag` (types, identity, mapper, controller; `UPSTREAM.md`). The API assigns the tag's ID at create and has no get for a tag (it is found by listing the entry's tags), so `hack/discokind` cannot express it.

## Notes

The spec keeps upstream's fields, so an object moves to upstream's kind
unchanged when Config Connector promotes it: `entryRef` (Config
Connector's `DataCatalogEntry`, or `external:
projects/P/locations/L/entryGroups/G/entries/E`), `templateRef` (Config
Connector's `DataCatalogTagTemplate`, or its name), `column`, `fields`,
and `resourceID`, which is only for acquiring an existing tag by its ID.
A new tag's ID is the API's: the controller keeps the created tag's name
in `status.externalRef`, where the next reconcile finds it. `entryRef`,
`templateRef`, `column` and `resourceID` are immutable (CEL); only
`fields` is updated (`updateMask=fields`).

Two changes from upstream's controller, found reading it:
an update now writes `status.externalRef` (upstream's status after an
update has none, and without it the next reconcile would not know the
tag's ID and would make a second tag), and fields are compared without
the display name and order the API copies from the template (upstream
compared the protos whole with `reflect.DeepEqual`, which a read tag with
display names never equals, so every resync would write).

Live is pending: `$GCP_PROJECT` has `datacatalog.googleapis.com` off and
the live-test service account lacks `roles/datacatalog.admin` (read
2026-11-12), so the test skips live unless `E2E_DATA_CATALOG=true`. What
the fake cannot show: whether the API lists a tag's name with the project
ID (as `status.externalRef` has it) or its number; the API's checks of
fields against the template (mockgcp checks none, and gives every tag the
same ID); and whether an update keeps fields the request leaves out
(mockgcp's update ignores the mask).

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
