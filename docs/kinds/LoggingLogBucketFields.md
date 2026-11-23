---
kinds: [LoggingLogBucketFields]
apiVersion: logging.gcp.hfl.dev/v1alpha1
---
# LoggingLogBucketFields

`logging.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud Logging bucket's `indexConfigs`, `restrictedFields` and `cmekSettings`, which Config Connector's `LoggingLogBucket` lacks (foundation item 12); a field kind (`docs/design.md` section 6.6, pattern 2, and decision 37) that sets only those fields on a bucket Config Connector's `LoggingLogBucket` (or anything else) made

**Source.** written here from the Cloud Logging API; the reference (`resourceRef`) and the bucket's identity are Config Connector's `LoggingLogBucketRef` and `LogBucketIdentity` (`apis/logging/v1beta1`); the group file is copied (`UPSTREAM.md`)

## Notes

2026-11-23: built (foundation item 12, decision 19). `spec.resourceRef`
names Config Connector's `LoggingLogBucket` by `name` (resolved from its
`status.externalRef`, or from its spec when the DCL controller wrote
none) or any bucket by `external` (`{parent}/locations/{location}/buckets/{bucket}`,
the parent a project, folder, organization or billing account).
`spec.indexConfigs` (a list of `fieldPath` and `type`),
`spec.restrictedFields` and `spec.cmekSettings.kmsKeyName` keep the API's
names; at least one is set, and a field left out is not owned. The two
lists are owned whole and compared as sets. The controller reads the
bucket with `buckets.get` and writes with `buckets.patch` and an update
mask naming only the owned fields that differ. `status.cmekServiceAccountID`
is the Log Router account the API reports for the CMEK key.

Why the field kind is safe beside Config Connector's object: Config
Connector's `LoggingLogBucket` runs the DCL by default, whose
`updateMask` comes from its own field diffs (`description`,
`retentionDays`, `locked`, `analyticsEnabled`), and optionally a direct
controller, whose mask comes from a round trip through its spec, which has
none of these fields. The test sends a description update with a mask of
`description` and checks the fields survive.

Delete clears the index configs and restricted fields it owns (a mask
with the path and an empty body), and leaves the CMEK key: the API cannot
remove a bucket's CMEK, only change the key. With `deletion-policy:
abandon` everything stays. A bucket the API has deleted (kept for 7 days
as `DELETE_REQUESTED`) counts as missing. A second object for the same
bucket in the namespace is refused. Patch 0055 teaches mockgcp's
`buckets.patch` the three paths.

The test (`TestLoggingLogBucketFields`) makes the run's own bucket in
us-central1 (no logs routed to it, no cost) and deletes it. Its CMEK case
runs live only with `E2E_LOGGING_BUCKET_KMS_KEY`, a us-central1 key the
project's Log Router service account may use, because a bucket keeps its
CMEK for good. Live: pending (`task test-live KINDS=logging
LIVE_RUN=TestLoggingLogBucketFields`); what to look for: the API's
answer for a field path it cannot index, whether `restrictedFields` needs
the bucket to be upgraded to Log Analytics, and the real mask spelling
the API accepts (the REST client sends camelCase).

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
