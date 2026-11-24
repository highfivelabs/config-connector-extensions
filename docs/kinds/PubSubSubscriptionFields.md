---
kinds: [PubSubSubscriptionFields]
apiVersion: pubsub.gcp.hfl.dev/v1alpha1
---
# PubSubSubscriptionFields

`pubsub.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Pub/Sub subscription's `messageTransforms` (JavaScript UDFs), its `bigtableConfig` (an export to a Bigtable table), and fields of its BigQuery and Cloud Storage exports (`serviceAccountEmail`, `filenameDatetimeFormat`, `maxMessages`, `avroConfig.useTopicSchema`), which Config Connector's `PubSubSubscription` has no field for (application queue item 1, `docs/coverage/application.md`)

**Source.** a field kind written here (decision 37), `PubSubTopicFields`' twin; the export fields' comments copied from Config Connector's generated Pub/Sub types

## Usage

The subscription stays Config Connector's `PubSubSubscription`; this
object names it and sets only the fields it holds.

```yaml
apiVersion: pubsub.gcp.hfl.dev/v1alpha1
kind: PubSubSubscriptionFields
metadata:
  name: orders-archive-fields
spec:
  resourceRef:
    name: orders-archive  # a PubSubSubscription, or external: projects/{{projectID}}/subscriptions/orders-archive
  messageTransforms:
  - javascriptUdf:
      functionName: redact
      code: |
        function redact(message, metadata) {
          const data = JSON.parse(message.data);
          delete data.email;
          return {data: JSON.stringify(data), attributes: message.attributes};
        }
  cloudStorageConfig:     # the PubSubSubscription's spec.cloudStorageConfig makes the export
    serviceAccountEmail: archive-writer@my-project.iam.gserviceaccount.com
    maxMessages: 10000
    filenameDatetimeFormat: YYYY-MM-DD/hh_mm_ssZ
```

A Bigtable export goes on a pull subscription (one whose
`PubSubSubscription` sets no `pushConfig.pushEndpoint`, `bigqueryConfig`
or `cloudStorageConfig`: a subscription delivers to one place):

```yaml
spec:
  resourceRef:
    name: orders-to-bigtable
  bigtableConfig:
    table: projects/my-project/instances/events/tables/orders
    appProfileId: pubsub  # single-cluster routing
    writeMetadata: true
```

A field the spec sets is owned: it is compared on every reconcile and
patched back when it differs. A field the spec leaves out is not sent;
taking one off the spec leaves it on the subscription as it is (delete
the object to clear what it set). One object per subscription.

## Notes

2026-11-23: built. Upstream read first (read-only: master `a5456d80`,
releases to `v1.162.0`, the heads of the open pull requests from #13500
to #16337): master's `PubSubSubscription` has none of these fields (its
direct types generate `MessageTransform` as unreachable and have no
Bigtable type), and the only pull request that touches its types,
#13712, adds `bigqueryConfig.useTableSchema`, which is therefore left
to it (section 6.1: wait for the release).

The controller reads the subscription with `subscriptions.get` and
writes it with `subscriptions.patch`, with an update mask naming only the
fields that differ:

- `messageTransforms` and `bigtableConfig` are top-level fields that
  neither Config Connector path sends: Terraform's
  `google_pubsub_subscription` update (the default) builds its mask from
  the top-level fields it changed, and the opt-in direct controller's
  from its spec's, and neither has these. They are ours alone.
- The export fields are the partial case (as a node pool's
  `kubeletConfig` is): the API takes `bigqueryConfig` and
  `cloudStorageConfig` whole, so ours sends the export as read with its
  fields set. When Config Connector changes the same export, it sends it
  without them, and they are off until this object's next reconcile puts
  them back; between the two, the export writes as the Pub/Sub service
  agent (when `serviceAccountEmail` is ours). Set them on a subscription
  whose export Config Connector seldom changes, or change both together.
- A Bigtable export on a subscription that pushes or exports elsewhere is
  refused (not Ready, nothing sent): Config Connector would put its own
  delivery back. An export field without its export (or
  `avroConfig.useTopicSchema` on a text export) is refused too.

Delete clears the owned fields (no transforms, no Bigtable export, the
export fields unset) and keeps the subscription; abandon keeps them. The
subscription is never made or deleted: a missing one is waited for.

Not built: `bigtableConfig.columnFamilyMapping` (newer than the pinned
client and mockgcp's protos), the `aiInference` and `compression`
transforms (as for `PubSubTopicFields`), `pushConfig.pubsubWrapper` and
`cloudStorageConfig.textConfig` (empty messages that are the defaults),
`tags` (taken only at create) and `detached` (set by the detach call).

mockgcp patch 0065 lets `subscriptions.patch` take the two new mask
paths and keeps an Avro export's format (it set text on every update).
The test (`TestPubSubSubscriptionFields`) makes a topic and a pull
subscription, names it through a Config Connector `PubSubSubscription`
object, sets a transform, checks that a patch like Terraform's keeps it
and a resync writes nothing, adds a disabled transform and puts back one
cleared outside, sets a Bigtable export (fake only), checks the CRD's
refusals, the one-object rule and the wait for a missing subscription,
clears the fields on delete and keeps them on abandon; then, on export
subscriptions of their own (fake only), sets a BigQuery export's service
account and puts it back after a Terraform-like patch of the export,
refuses a Bigtable export where it exports to BigQuery and Cloud Storage
fields where it has no such export, and sets and clears a Cloud Storage
Avro export's fields.

Live: pending. `task test-live PROJECT=$GCP_PROJECT KINDS=pubsub
LIVE_RUN=TestPubSubSubscriptionFields` (one topic and one pull
subscription, free with no messages; needs `pubsub.googleapis.com` on and
`roles/pubsub.editor` for the live-test account); the Bigtable, BigQuery
and Cloud Storage cases skip live (a Bigtable instance costs money; the
exports need a table or bucket the project's Pub/Sub service agent can
write to). To be seen live: that Terraform's patch leaves the transforms
alone, whether the API answers a transform's code as sent, and the
subscription's `state`.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
