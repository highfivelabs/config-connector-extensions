---
kinds: [ChronicleReferenceList]
apiVersion: chronicle.gcp.hfl.dev/v1beta1
---
# ChronicleReferenceList

`chronicle.gcp.hfl.dev/v1beta1`

**Gap it fills.** a Google SecOps (Chronicle) reference list in an instance: Terraform's `google_chronicle_reference_list`, which the catalog's `secops-rules` module uses (rank 27 of its `blocked-kinds.md`). Config Connector 1.160.0 has `ChronicleReferenceList` at `v1alpha1` only, and with no instance field: its identity needs `{instance}` and nothing sets it, so every object fails with "cannot resolve instance". Ours keeps the name (decision 18) at `v1beta1` (decision 26) and carries the instance.

**Source.** copied from Config Connector's `apis/chronicle/v1alpha1/chroniclereferencelist_types.go` with `instance` added; upstream's `ChronicleReferenceListIdentity`, `ReferenceListEntry`, `ScopeInfo` and observed state are imported. The controller is written here after `ChronicleRule`'s (REST JSON to the location's host).

## Notes

The spec: `projectRef`, `location` (the instance's, such as `us`) and
`instance` (the SecOps instance's ID), immutable (CEL); `resourceID`, the
list's ID, which is also its display name and how a rule names it
(`%id`), `metadata.name` when unset, immutable; `description`; `entries`
(each a `value`); `syntaxType`
(`REFERENCE_LIST_SYNTAX_TYPE_PLAIN_TEXT_STRING`, `_REGEX` or `_CIDR`); and
`scopeInfo`.

The ID starts with a letter and has only letters, digits and underscores
(the API's rule; the CRD checks `resourceID` with a pattern). A
`metadata.name` cannot have an underscore and usually has a hyphen, so set
`resourceID` unless the name is one word.

Create sends the list with `referenceListId`. Reads use
`REFERENCE_LIST_VIEW_FULL`, the only view with the entries. Update patches
`description`, `entries`, `syntaxType` and `scopeInfo` when they differ
(entries in the spec's order, which the API keeps), with an update mask of
those; nothing differing, nothing is sent.

The API has no delete for a reference list, so deleting the object leaves
the list as it is, as Terraform does; `deletion-policy: abandon` changes
nothing. A new object with the same ID finds the list and takes it over
(it patches what differs).

`status` has `externalRef` and `observedState` (upstream's: `displayName`,
`revisionCreateTime`, `rules`, `ruleAssociationsCount`).

mockgcp has no Chronicle, so its fake is
`pkg/controller/direct/chronicle/fake` (`docs/fake-cloud.md`). What it
cannot show: SecOps instances, whether entries suit the syntax type (a
regex or CIDR is not checked), scopes, which rules use a list, and
permissions.

Live is pending, as `ChronicleRule`'s: it needs a Google SecOps instance
and `roles/chronicle.editor` for `cce-live-tests`; the test skips live
unless `E2E_CHRONICLE_INSTANCE=<location>/<instance ID>`. A live run
leaves its list (the API cannot delete it), named after the run.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
