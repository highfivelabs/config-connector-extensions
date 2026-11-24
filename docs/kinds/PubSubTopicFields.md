---
kinds: [PubSubTopicFields]
apiVersion: pubsub.gcp.hfl.dev/v1alpha1
---
# PubSubTopicFields

`pubsub.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Pub/Sub topic's `ingestionDataSourceSettings` (an import topic: Amazon Kinesis, Cloud Storage, Azure Event Hubs, Amazon MSK or Confluent Cloud, with platform logs) and `messageTransforms` (JavaScript UDFs), which Config Connector's `PubSubTopic` has no field for (application queue item 1, `docs/coverage/application.md`)

**Source.** a field kind written here (decision 37); its field types copied from Config Connector's generated, unreachable Pub/Sub types

## Usage

The topic stays Config Connector's `PubSubTopic`; this object names it and
sets only the fields it holds.

```yaml
apiVersion: pubsub.gcp.hfl.dev/v1alpha1
kind: PubSubTopicFields
metadata:
  name: orders-fields
spec:
  resourceRef:
    name: orders          # a PubSubTopic, or external: projects/{{projectID}}/topics/orders
  messageTransforms:
  - javascriptUdf:
      functionName: redact
      code: |
        function redact(message, metadata) {
          const data = JSON.parse(message.data);
          delete data.email;
          return {data: JSON.stringify(data), attributes: message.attributes};
        }
  ingestionDataSourceSettings:
    cloudStorage:
      bucket: orders-export
      textFormat: {}
      matchGlob: "**.jsonl"
    platformLogsSettings:
      severity: WARNING
```

A field the spec sets is owned: it is compared on every reconcile and
patched back when it differs. A field the spec leaves out is not sent;
taking one off the spec leaves it on the topic as it is (delete the object
to clear what it set). One object per topic.

## Notes

2026-11-23: built. Upstream read first (read-only: master `bc464f55`,
releases to `v1.162.0`, the pull request heads from #13500 to #16331):
master's direct `PubSubTopic` (opt-in; Terraform is the default)
generates `IngestionDataSourceSettings` and `MessageTransform` as
unreachable, its CRD has neither, and no pull request adds them.

The controller reads the topic with `topics.get` and writes it with
`topics.patch`, whose update mask names only the owned fields that differ
(compared with unset, `""`, `false` and `[]` the same, and each source's
output-only `state` left out). Config Connector's Terraform-based
`PubSubTopic` patches with a mask of the fields it changed (read at the
pin's fork), so it leaves these alone; to be confirmed live. Delete clears
the owned fields and keeps the topic; abandon keeps them. The topic is
never made or deleted: a missing one is waited for.

mockgcp patch 0064 lets `topics.patch` take the two mask paths. The test
(`TestPubSubTopicFields`) makes a topic, names it through a Config
Connector `PubSubTopic` object, sets a transform, checks that a patch
like Terraform's keeps it and a resync writes nothing, adds a disabled
transform and puts back one cleared outside, sets a Cloud Storage import
(fake only), checks the CRD's refusals (two sources, no field) and the
one-object rule, waits for a missing topic, and clears the fields on
delete and keeps them on abandon.

Live: pending. `task test-live PROJECT=$GCP_PROJECT KINDS=pubsub
LIVE_RUN=TestPubSubTopicFields` (one topic, free with no messages; needs
`pubsub.googleapis.com` on and `roles/pubsub.editor` for the live-test
account); the import case skips live. To be seen live: that Terraform's
patch and the opt-in direct controller's leave the fields alone, whether
the API answers a transform's code as sent, and the topic's `state`.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
