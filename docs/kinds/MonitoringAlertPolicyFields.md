---
kinds: [MonitoringAlertPolicyFields]
apiVersion: monitoring.gcp.hfl.dev/v1alpha1
---
# MonitoringAlertPolicyFields

`monitoring.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud Monitoring alert policy's `documentation.subject` (the notification's subject line), `documentation.links` (playbooks, dashboards) and `alertStrategy.notificationPrompts` (notify on close too), which Config Connector's `MonitoringAlertPolicy` has no field for (application queue item 3, `docs/coverage/application.md`)

**Source.** a field kind written here (decision 37); its field types copied from Config Connector's generated, unreachable Monitoring types

## Usage

The policy stays Config Connector's `MonitoringAlertPolicy`; this object
names it and sets only the fields it holds.

```yaml
apiVersion: monitoring.gcp.hfl.dev/v1alpha1
kind: MonitoringAlertPolicyFields
metadata:
  name: cpu-high-fields
spec:
  resourceRef:
    name: cpu-high        # a MonitoringAlertPolicy, or external: projects/{{projectID}}/alertPolicies/{{policyID}}
  documentation:
    subject: "CPU high on ${resource.label.instance_id}"
    links:
    - displayName: playbook
      url: https://example.com/playbooks/cpu
  alertStrategy:
    notificationPrompts: [OPENED, CLOSED]
```

A field the spec sets is owned: it is compared on every reconcile and
patched back when it differs. A field the spec leaves out is not sent;
taking one off the spec leaves it on the policy as it is (delete the
object to clear what it set). One object per policy.

## Notes

2026-11-23: built. Upstream read first (read-only: master `a5456d80`,
releases to `v1.162.0`, the heads of the open pull requests from #13500
to #16337): master's `MonitoringAlertPolicy` (Terraform by default, an
opt-in direct controller) has none of these fields (its generated types
hold them as unreachable), and no pull request adds them.

The controller reads the policy with `alertPolicies.get` and writes it
with `alertPolicies.patch`. This is the partial case, as a node pool's
`kubeletConfig` is: Terraform's `google_monitoring_alert_policy` update
(read at the pin's fork, whose schema lacks these fields) names
`documentation` and `alertStrategy` whole in its update mask when it
changes them, and the opt-in direct controller diffs top-level fields the
same way, so ours sends `documentation` and `alertStrategy` as read with
its fields set, under those masks. A Config Connector change to the
policy's documentation or alert strategy drops ours until this object's
next reconcile puts them back; between the two, a notification has the
default subject and no links, and no close notification is sent. Delete
clears what it owns and keeps the policy; abandon keeps them. The policy
is never made or deleted: a missing one is waited for.

Not built: `conditions[].conditionPrometheusQueryLanguage.disableMetricValidation`
(inside a list Config Connector owns). `userLabels` is not a gap: Config
Connector writes the object's `metadata.labels` there.

mockgcp patch 0067 lets `alertPolicies.patch` take the `alertStrategy`
mask path (it refused it, though Terraform sends it). The test
(`TestMonitoringAlertPolicyFields`) makes a policy with a CPU threshold
condition and no notification channel, names it through a Config
Connector `MonitoringAlertPolicy` object, sets the fields, checks that a
resync writes nothing, that a patch like Terraform's drops them and the
next reconcile puts them back without touching Terraform's content and
auto-close, changes them, checks the CRD's refusals, the one-object rule
and the wait for a missing policy, and clears the fields on delete and
keeps them on abandon.

Live: pending. `task test-live PROJECT=$GCP_PROJECT KINDS=monitoring
LIVE_RUN=TestMonitoringAlertPolicyFields` (one alert policy with no
notification channel: free; needs `monitoring.googleapis.com` on and
`roles/monitoring.alertPolicyEditor` or `roles/monitoring.editor` for
the live-test account). To be seen live: that the API reads back the
links and prompts as sent, and what it reads for cleared prompts.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
