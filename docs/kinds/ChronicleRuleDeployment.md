---
kinds: [ChronicleRuleDeployment]
apiVersion: chronicle.gcp.hfl.dev/v1alpha1
---
# ChronicleRuleDeployment

`chronicle.gcp.hfl.dev/v1alpha1`

**Gap it fills.** the deployment of a Google SecOps (Chronicle) detection rule: whether the rule runs continuously, whether its detections are alerts, whether it is archived, and how often it runs. Config Connector has `ChronicleRule` (alpha) but no kind for its deployment. Terraform's `google_chronicle_rule_deployment`, which the catalog's `secops-rules` module uses (rank 30 of its `blocked-kinds.md`). It is named as Terraform names it (decision 18) and starts at `v1alpha1` (decision 26's last sentence).

**Source.** hand-written, not generated: the Chronicle API has no Discovery document at the pinned discovery-artifact-manager commit (nor at its head), and the deployment is a singleton under its rule with get and patch only. Written on Config Connector's direct reconciler after `EndpointsService`'s controller; the behaviour is Terraform's (Magic Modules' `chronicle/RuleDeployment.yaml`, its custom create, pre-update and run frequency diff suppression). `ruleRef` is Config Connector's `ChronicleRuleRef`, imported.

## Notes

The spec: `ruleRef` (a `ChronicleRule` by `name` once it has
`status.externalRef`: ours, `chronicle.gcp.hfl.dev/v1beta1`, first, then
Config Connector's; or `external`
`projects/{project}/locations/{location}/instances/{instance}/rules/{rule}`,
without a revision), immutable (CEL); then `enabled`, `alerting`,
`archived`, `runFrequency` (`LIVE`, `HOURLY`, `DAILY`, `LIVE_CUSTOMIZABLE`,
`HOURLY_CUSTOMIZABLE`) and `scheduleCustomizations`
(`ensureEnrichmentCompleteness`, `lateArrivingDataAdjustment`), each left
as the deployment has it when the spec leaves it out. CEL refuses an
archived rule that is enabled or alerting (the API needs a rule disabled
to archive it, and turns alerting off when it does).

The deployment exists with its rule; the API has no create or delete.
Create and update read it and patch the fields the spec sets that differ,
with an update mask of those fields; nothing differing, nothing is sent.
`runFrequency` and `scheduleCustomizations` share the `runFrequency` mask,
so a change to either sends both. An archive goes in a patch after the
other changes (disable first), an unarchive in one before them (an
archived rule's `enabled`, `alerting` and `runFrequency` cannot change).
The API stores some legacy run frequencies as others (`LIVE` as
`LIVE_CUSTOMIZABLE`, `HOURLY` or `DAILY` as `HOURLY_CUSTOMIZABLE` for a
multi-event rule, as `LIVE` for a single-event one); as Terraform does,
those count as matching, and `status.observedState.runFrequency` holds
the stored value. A rule that does not exist has no deployment: the
object is not Ready, with "does the rule exist?". Delete leaves the
deployment as it is, and needs no rule (a rule object deleted with it does
not hold it), as Terraform's `exclude_delete` does; it does not
disable the rule first. `status.observedState` has `runFrequency`,
`archiveTime`, `executionState`, `producerRules`, `consumerRules` and
`lastAlertStatusChangeTime`.

The controller speaks REST JSON (`pkg/discokind`'s `API`) to
`https://<location>-chronicle.googleapis.com/v1/`, Terraform's host,
rather than `cloud.google.com/go/chronicle`: v1.5.0, the version in
`go.mod`, has neither the customizable run frequencies nor
`scheduleCustomizations`.

mockgcp has no Chronicle, so its fake is
`pkg/controller/direct/chronicle/fake`, in front of mockgcp
(`docs/fake-cloud.md`). What the fake cannot show: SecOps instances, the
rule language, the API's coercion of legacy run frequencies, chained
rules, detections and permissions.

Live is pending: it needs a Google SecOps instance, which the live-test
project does not have (one comes with a SecOps subscription, not an API
call), and `roles/chronicle.editor` for `cce-live-tests`. The test skips
live unless `E2E_CHRONICLE_INSTANCE=<location>/<instance ID>`
(`docs/live-runs.md`). Nothing is made, so the sweeper has no Chronicle
section yet; it needs one (rules whose name carries a run ID) before the
first live run.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
