---
kinds: [AccessContextManagerGCPUserAccessBinding]
apiVersion: accesscontextmanager.gcp.hfl.dev/v1beta1
---
# AccessContextManagerGCPUserAccessBinding

`accesscontextmanager.gcp.hfl.dev/v1beta1`

**Gap it fills.** the stable equivalent of Config Connector's alpha-only `AccessContextManagerGCPUserAccessBinding` (decision 26), with the fields it lacks: `dryRunAccessLevels`, `sessionSettings` (Google Cloud session length), `restrictedClientApplications` and `scopedAccessSettings` (foundation item 21)

**Source.** Config Connector's direct `AccessContextManagerGCPUserAccessBinding` types, identity and controller, on `google.golang.org/api`'s REST client; the added fields from the API's v1 Discovery document

## Usage

A binding is created under an organization and named by the server: the
ID it gets is written back to `spec.resourceID`. `groupKey` (a Google
Group's ID, not its email) and the organization are immutable.

```yaml
apiVersion: accesscontextmanager.gcp.hfl.dev/v1beta1
kind: AccessContextManagerGCPUserAccessBinding
metadata:
  name: contractors
spec:
  organizationRef:
    external: organizations/123456789012
  groupKey: 01d520gv4vjcrht
  accessLevels:
  - accessPolicies/1111/accessLevels/corp_devices
  sessionSettings:
    sessionLengthEnabled: true
    sessionLength: 43200s
    sessionReauthMethod: SECURITY_KEY
```

## Which kind to use

Config Connector's `accesscontextmanager.cnrm.cloud.google.com/v1alpha1`
kind takes only `groupKey` and `accessLevels`, and is alpha. This one
takes the same and the fields above, as `v1beta1`. Never both for one
binding. To move one: create this kind's object with the same
`organizationRef` and `groupKey` and `spec.resourceID` set to the
binding's ID (or without it: an object with no ID finds the
organization's binding of its group), wait for it to be Ready, then delete
Config Connector's object with `cnrm.cloud.google.com/deletion-policy:
abandon`.

## Notes

2026-11-23: built (foundation item 21). Upstream read first (read-only:
master `bc464f55`, releases to `v1.162.0`, every pull request head from
#13500 to #16331): Config Connector's direct controller and types are
unchanged since the pin but for a label, and no pull request adds the
fields; its pinned Go client (`cloud.google.com/go/accesscontextmanager`
v1.14.0) and mockgcp's protos have only `groupKey` and `accessLevels`, so
this kind sends REST (`google.golang.org/api/accesscontextmanager/v1`,
already a dependency), and its fake is an HTTP fake of our own in front of
mockgcp (`pkg/controller/direct/accesscontextmanager/fake`). Not in the
kind: `principal` (in the API's newer Discovery documents, not in the
pinned REST client).

The controller copies upstream's calls: create with the name left out,
the server's ID written back to `spec.resourceID`; patch with an update
mask of the fields that differ (unset, `""`, `false` and `[]` are the
same); delete. Written here: an object with no ID first lists the
organization's bindings for its group key and takes that one over (the
API keeps one binding per group, so a create retried after its ID was
lost would fail); a delete of a binding that was made uses
`status.externalRef` and resolves nothing. CEL refuses a changed
`groupKey`, organization or `resourceID`, and
`restrictedClientApplications` together with `scopedAccessSettings`.

The test (`TestAccessContextManagerGCPUserAccessBinding`) makes a binding
with access levels, a dry-run level and session settings, checks the ID
is written back and a resync writes nothing, sets scoped access settings
and takes the dry run and the session off, checks the CRD's refusals,
takes over a binding made outside by its group key without a create,
abandons one and deletes the other.

Live: skips (an organization, a Google Group and an access policy's
levels, which a live run does not touch), as
`AccessContextManagerAuthorizedOrgsDesc` does. To be seen live: whether
the API answers durations as sent ("43200s"), whether a patch's mask in
snake_case is taken, and its checks of which fields go together.

## Live runs

One row per run.

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
