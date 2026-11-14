---
kinds: [ChronicleRule]
apiVersion: chronicle.gcp.hfl.dev/v1beta1
---
# ChronicleRule

`chronicle.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Google SecOps (Chronicle) detection rule in an instance: Terraform's `google_chronicle_rule`, which the catalog's `secops-rules` module uses (rank 28 of its `blocked-kinds.md`). Config Connector 1.160.0 has `ChronicleRule` at `v1alpha1` only, and with no instance field: its identity needs `{instance}` and nothing sets it, so every object fails with "cannot resolve instance for ChronicleRule". Ours keeps the name (decision 18) at `v1beta1` (decision 26) and carries the instance.

**Source.** copied from Config Connector's `apis/chronicle/v1alpha1/chroniclerule_types.go` with `instance` added, `etag` moved to status and Terraform's `deletionPolicy`; upstream's `ChronicleRuleIdentity` and observed state are imported. The controller is written here after `ChronicleRuleDeployment`'s (REST JSON to the location's host).

## Notes

The spec: `projectRef`, `location` (the instance's, such as `us`) and
`instance` (the SecOps instance's ID), all immutable (CEL); `text`, the
YARA-L rule; `scope`, a data access scope's name; `resourceID`, the rule's
`ru_...` ID, only to acquire a rule that exists; `deletionPolicy`,
`DEFAULT` or `FORCE`.

The API assigns a rule's ID. With no `resourceID` and no
`status.externalRef`, the controller creates the rule and records its
name in `status.externalRef`; after that it reads and patches that rule.
`resourceID` names a rule that exists, which is acquired, never created
(the ID cannot be chosen): a missing one leaves the object not Ready with
"spec.resourceID acquires a rule that exists". It can be set later to the
ID in `status.externalRef`, and is immutable once set.

Update patches `text` and `scope` when the spec sets them and they
differ, with an update mask of those; nothing differing, nothing is sent.
A new text makes a new revision (`status.observedState.revisionID`). The
text is compared as written: if the API rewrites a rule's text, every
resync would patch it; the fake keeps it as sent, and live has not shown
yet.

Delete deletes the rule, and the API deletes its deployment with it. A
rule with retrohunts or detections is refused unless `deletionPolicy` is
`FORCE`, which deletes them too (`force=true`); with `DEFAULT` the object
stays not deleted, with the API's message, until they are gone.
`deletion-policy: abandon` keeps the rule. A delete takes the rule's name
from `status.externalRef` and does not resolve `projectRef`, so a Project
object deleted first does not hold it.

`status` has `externalRef`, `etag` and `observedState` (upstream's:
`revisionID`, `displayName`, `author`, `severity`, `metadata`,
`createTime`, `revisionCreateTime`, `compilationState`, `type`,
`referenceLists`, `allowedRunFrequencies`, `compilationDiagnostics`,
`nearRealTimeLiveRuleEligible`, `inputsUsed`).

A `ChronicleRuleDeployment` whose `ruleRef` names a rule by `name` finds
this kind first, then Config Connector's; it waits (not Ready) until the
rule has a `status.externalRef`. Deleting the rule and its deployment at
once works in either order: the deployment's delete needs no rule.

mockgcp has no Chronicle, so its fake is
`pkg/controller/direct/chronicle/fake` (`docs/fake-cloud.md`). What it
cannot show: SecOps instances, the rule language (it never compiles a
rule), data access scopes, retrohunts and detections, and permissions.

Live is pending, as `ChronicleRuleDeployment`'s: it needs a Google SecOps
instance, which the live-test project does not have, and
`roles/chronicle.editor` for `cce-live-tests`. The test skips live unless
`E2E_CHRONICLE_INSTANCE=<location>/<instance ID>` (`docs/live-runs.md`).

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
