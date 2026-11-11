---
kinds: [LoggingBillingAccountSink]
apiVersion: logging.gcp.hfl.dev/v1alpha1
---
# LoggingBillingAccountSink

`logging.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a billing account's log sink (Terraform's `google_logging_billing_account_sink`): its audit logs to a bucket, dataset, topic or log bucket, with a filter and exclusions; Config Connector's `LoggingLogSink` takes a project, folder or organization only (`docs/design.md` decision 15)

**Source.** Config Connector's direct `LoggingLogSink` (types, identity, mapper, controller) with `billingAccountRef`

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
