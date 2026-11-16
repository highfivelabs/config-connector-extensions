---
kinds: [LoggingLogSink]
apiVersion: logging.gcp.hfl.dev/v1beta1
---
# LoggingLogSink

`logging.gcp.hfl.dev/v1beta1`

**Gap it fills.** a project's, folder's or organization's log sink, owning the whole sink, with what Config Connector's `LoggingLogSink` lacks: `interceptChildren` (Terraform's `intercept_children`: a folder's or organization's sink that keeps the logs it routes from its children's sinks) and a Cloud project destination, which an intercepting sink needs (`docs/design.md` decisions 19 and 26; section 10 question 7, answered 2026-11-15)

**Source.** Config Connector's direct `LoggingLogSink` (types, identity, mapper, controller), on the Logging REST client, with `interceptChildren`, `destination.projectRef` and one CEL-checked parent

## Notes

Built 2026-11-15 (`docs/design.md` section 6.4 has what was copied and
changed, and the upstream check).

**The spec is Config Connector's**, field for field, plus
`interceptChildren` and `destination.projectRef`, so a manifest moves by
changing its `apiVersion` (and naming its parent: ours takes no fall back
to the namespace's project). An existing sink moves to ours by acquiring
it (`resourceID`) and abandoning Config Connector's object
(`cnrm.cloud.google.com/deletion-policy: abandon`).

**An intercepting sink** needs a `folderRef` or `organizationRef`,
`includeChildren: true` and `destination.projectRef`; the CRD refuses
anything else, as the API does. `interceptChildren` and `includeChildren`
cannot change (a new sink is a new object).

**The writer identity** is in `status.writerIdentity`. A folder's or
organization's sink always has its own. The catalog's charts grant it on
the destination with our `IAMMember`'s `memberFrom.logSinkRef`
(2026-11-15), which reads this kind's (and Config Connector's) writer
identity; Config Connector's `IAMPolicyMember.memberFrom.logSinkRef`
reads only Config Connector's `LoggingLogSink`.

**Destination forms** are `LoggingBillingAccountSink`'s (a bucket by bare
name, `gs://`, `storage.googleapis.com/` and the rest); a project as its ID,
`projects/<id>`, or a `Project` object by `name`.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-15 | `e464b10` | PASS | PASS (project parent) | none: create (with `uniqueWriterIdentity=true`), a resync with no write, one update with the mask `disabled,exclusions`, the CRD's refusals, delete and abandon match the fake; run `cce-11152237-3412`, `LIVE_RUN=TestLoggingLogSink` | 2 project sinks to the project's `_Default` bucket, matching no log (deleted, the abandoned one by the test); `task live-list` empty |
| 2026-11-15 | pending | PASS | pending (folder parent, `interceptChildren`): needs `E2E_LOGGING_FOLDER` with `logging.sinks.*` (`roles/logging.configWriter`) for `$GCP_LIVE_SA`; the test estate grants it to the generation's own account, which the run scripts do not impersonate yet | not run (the test skips live) | none |
| 2026-11-15 | n/a | PASS | never (organization parent: a live run never uses an organization) | not run | none |
