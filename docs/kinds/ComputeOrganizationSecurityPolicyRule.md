---
kinds: [ComputeOrganizationSecurityPolicyRule]
apiVersion: compute.gcp.hfl.dev/v1beta1
---
# ComputeOrganizationSecurityPolicyRule

`compute.gcp.hfl.dev/v1beta1`

**Gap it fills.** a rule of a hierarchical firewall policy (direction, ranges, protocols and ports, targets, logging), as a stable kind; the policy by reference or by ID; Config Connector's is alpha and Terraform-based only (decision 13)

**Source.** Config Connector's alpha types, identity and reference; the controller written here on the compute beta API

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
