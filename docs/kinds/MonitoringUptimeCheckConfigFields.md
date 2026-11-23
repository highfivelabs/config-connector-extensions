---
kinds: [MonitoringUptimeCheckConfigFields]
apiVersion: monitoring.gcp.hfl.dev/v1alpha1
---
# MonitoringUptimeCheckConfigFields

`monitoring.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud Monitoring uptime check's `userLabels`, and its HTTP check's `customContentType`, `acceptedResponseStatusCodes`, `pingConfig` and `serviceAgentAuthentication` (an OIDC token for the Monitoring service agent), which Config Connector's DCL-based `MonitoringUptimeCheckConfig` has no field for (application queue item 3, `docs/coverage/application.md`)

**Source.** a field kind written here (decision 37); its field types copied from Config Connector's generated, unreachable Monitoring types

## Usage

The check stays Config Connector's `MonitoringUptimeCheckConfig`; this
object names it and sets only the fields it holds.

```yaml
apiVersion: monitoring.gcp.hfl.dev/v1alpha1
kind: MonitoringUptimeCheckConfigFields
metadata:
  name: checkout-fields
spec:
  resourceRef:
    name: checkout        # a MonitoringUptimeCheckConfig, or external: projects/{{projectID}}/uptimeCheckConfigs/{{checkID}}
  userLabels:
    team: payments
  httpCheck:              # the MonitoringUptimeCheckConfig's spec.httpCheck makes the HTTP check
    acceptedResponseStatusCodes:
    - statusClass: STATUS_CLASS_2XX
    - statusValue: 404
    pingConfig:
      pingsCount: 2
    serviceAgentAuthentication:
      type: OIDC_TOKEN    # for a Cloud Run service that requires authentication
```

A field the spec sets is owned: it is compared on every reconcile and
patched back when it differs. A field the spec leaves out is not sent;
taking one off the spec leaves it on the check as it is (delete the
object to clear what it set). One object per check.

## Notes

2026-11-23: built. Upstream read first (read-only: master `a5456d80`,
releases to `v1.162.0`, the heads of the open pull requests from #13500
to #16337): `MonitoringUptimeCheckConfig` is DCL-based, on master too (no
direct controller), its generated direct types have these fields only as
unreachable, and no pull request touches its types.

The controller reads the check with `uptimeCheckConfigs.get` and writes
it with `uptimeCheckConfigs.patch`, whose update mask names only the
owned fields that differ, the HTTP check's by their nested paths
(`httpCheck.pingConfig`), so the rest of the HTTP check is left alone.
DCL's update sends a mask of the paths of the fields it changed
(`dcl.UpdateMask`, nested: `httpCheck.path`), never these, so its
updates leave them alone; to be confirmed live. Delete clears the owned
fields and keeps the check; abandon keeps them. The check is never made
or deleted: a missing one is waited for. HTTP check fields on a TCP
check are refused (not Ready, nothing sent).

Not built: `checkerType` (the API takes it only at create; Terraform's
`checker_type` is `ForceNew`), `syntheticMonitor` (a check's target, set
at create), `contentMatchers[].jsonPathMatcher` (inside a list Config
Connector owns), and `disabled` and `logCheckFailures` (newer than the
pinned Monitoring protos, which the fake serves; a fake of our own would
be needed to test them).

mockgcp patch 0066 lets `uptimeCheckConfigs.patch` clear a masked field
the request leaves unset (it panicked on a list). The test
(`TestMonitoringUptimeCheckConfigFields`) makes an HTTP uptime check of
example.com, names it through a Config Connector
`MonitoringUptimeCheckConfig` object, sets the fields, checks that a patch
like DCL's keeps them and a resync writes nothing, changes them and puts
back one cleared outside, checks the CRD's refusals, the one-object rule,
a TCP check refused and a missing check waited for, and clears the fields
on delete and keeps them on abandon.

Live: pending. `task test-live PROJECT=$GCP_PROJECT KINDS=monitoring
LIVE_RUN=TestMonitoringUptimeCheckConfigFields` (two uptime checks of
example.com, every 15 minutes for the minutes they exist: free at that
volume; needs `monitoring.googleapis.com` on and
`roles/monitoring.uptimeCheckConfigEditor` or `roles/monitoring.editor`
for the live-test account). To be seen live: that the API takes
`userLabels` in an update mask (its description lists the updatable
fields without it), that it takes the nested `httpCheck.*` paths, and
that DCL's patch leaves the fields alone.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
