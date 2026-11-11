---
kinds: [ComputeRouter]
apiVersion: compute.gcp.hfl.dev/v1alpha1
---
# ComputeRouter

`compute.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a whole Cloud Router: BGP (with `identifierRange`), interfaces, BGP peers (BFD, custom learned ranges, IPv4 and IPv6 next hops, route policies), Cloud NATs and MD5 authentication keys whose values come from Secrets; Config Connector's direct `ComputeRouter` has only the BGP basics and leaves the rest to Terraform-based kinds without these fields (`docs/design.md` decision 14)

**Source.** copied from Config Connector's direct `ComputeRouter`, NAT and interface mappers; the peers and keys written here

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
