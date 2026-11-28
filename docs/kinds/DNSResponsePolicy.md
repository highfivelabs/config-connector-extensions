---
kinds: [DNSResponsePolicy]
apiVersion: dns.gcp.hfl.dev/v1beta1
---
# DNSResponsePolicy

`dns.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Cloud DNS response policy (the DNS firewall a VPC network or GKE cluster consults before Cloud DNS), bound to networks and clusters, with the object's labels, as a stable kind the charts allow; Config Connector's `DNSResponsePolicy` is alpha only (`docs/design.md` decision 13)

**Source.** Config Connector's direct `DNSResponsePolicy`, its status and labels written

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

References (2026-11-28, `docs/design.md` 2.4, "References"): a network's `networkUrlRef` (Config Connector's `ComputeNetworkRef`) and a cluster's `gkeClusterNameRef` (`ContainerClusterRef`) sit beside the strings, as Upjet resolves them; one of each pair is required. Upstream's `common.NormalizeReferences` resolves them and the controller copies the value into the string, which it then sends as a full URL as before. `TestDNSResponsePolicy` adds its second network by reference.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-18 | `23e425b`+ | DNSResponsePolicy | PASS | FAIL | the test's network: 403 `compute.networks.create` on `projects/hfl-dev-514205` for `live-tests@` (it holds `dns.admin` there, no Compute network role). Run `cce-11181733-4513` | none made |
