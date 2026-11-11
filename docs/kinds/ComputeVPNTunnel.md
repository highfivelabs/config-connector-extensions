---
kinds: [ComputeVPNTunnel]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeVPNTunnel

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** an HA or Classic VPN tunnel with `cipherSuite` (the IKE phase 1 and 2 algorithms), its shared secret from a Secret; Config Connector's Terraform-based `ComputeVPNTunnel` has no cipher suite. Every field but `labels` is set once, when the tunnel is made (`docs/design.md` 6.6)

**Source.** Config Connector's direct `ComputeVPNTunnel` types and mapper; the controller written here

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
