---
kinds: [IAMBinding]
apiVersion: iam.gcp.hfl.dev/v1beta1
---
# IAMBinding

`iam.gcp.hfl.dev/v1beta1`

**Gap it fills.** one role on one resource with its members, authoritative for that role (Terraform's `google_*_iam_binding`): members added out of band are removed; Config Connector's `IAMPolicyMember` is additive and `IAMPartialPolicy` keeps what it lists

**Source.** written here (`pkg/iampolicy`), reading and writing the policy over HTTP

## Notes

Notes from before 2026-11-11 are in `docs/design.md` (section 6 and
section 7, under the kind's name); new ones go here.

BigQuery datasets: see `IAMMember.md` (the same `pkg/iampolicy` row, 2026-11-12).

A parent being deleted (2026-11-13): when everything is deleted at once (a
namespace, a release), the object a binding names (an `APIGatewayAPI`, a
secret) often goes first and sits with a deletion timestamp, not Ready,
while Config Connector deletes its resource. The binding's delete no
longer waits on "reference ... is not ready": a parent that is gone, or
being deleted without `deletion-policy: abandon`, takes the policy with it,
so the delete succeeds without a call. A parent being deleted with
abandon keeps its resource, so the binding is still removed from it.
`pkg/controller/direct/iam/parentgone_mockgcp_test.go` deletes the parent
and the binding in both orders and together, for `IAMBinding` and
`IAMMember`.

## Live runs

Runs before 2026-11-11 are in the ledger in `docs/live-runs.md`; new rows
go here, one per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-12 | `e1e23f5` | PASS | PASS | none (run `cce-11120814-5d2a`, the fallback project) | 1 secret, 1 bucket (deleted); `task live-list` empty |
