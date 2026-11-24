---
kinds: [PrivateCACAPoolFields]
apiVersion: privateca.gcp.hfl.dev/v1alpha1
---
# PrivateCACAPoolFields

`privateca.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Certificate Authority Service CA pool's `issuancePolicy.backdateDuration` and `issuancePolicy.baselineValues.nameConstraints`, which Config Connector's `PrivateCACAPool` lacks (foundation item 25); a field kind (`docs/design.md` section 6.6, pattern 2, and decision 37) that sets only those fields on a pool Config Connector's `PrivateCACAPool` (or anything else) made

**Source.** written here from the Certificate Authority Service API; the reference (`resourceRef`) and the pool's identity are Config Connector's `PrivateCACAPoolRef` and `PrivateCACAPoolIdentity` (`apis/privateca/privatecarefs`); the name constraints type and `backdateDuration`'s description are Config Connector's generated ones (`apis/privateca/v1beta1/types.generated.go`, where the type is commented out as unreachable); the group file is copied (`UPSTREAM.md`)

## Notes

2026-11-23: built (foundation item 25, decision 19). `spec.resourceRef`
names Config Connector's `PrivateCACAPool` by `name` (resolved from its
`status.externalRef`, or from its spec when the DCL controller wrote
none) or any pool by `external`
(`projects/{project}/locations/{location}/caPools/{pool}`).
`spec.issuancePolicy.backdateDuration` (seconds, such as `3600s`; the API
takes up to 48 hours) and `spec.issuancePolicy.baselineValues.nameConstraints`
(`critical` and the permitted and excluded DNS names, IP ranges, email
addresses and URIs, each list owned whole and compared as a set) keep the
API's names; at least one is set, and a field left out is not owned.

The API's update mask names top-level fields, so the controller reads the
pool with `caPools.get` and, when an owned field differs, sends
`caPools.patch` with a mask of `issuance_policy` and the pool's issuance
policy as read, with only its own fields changed. Config Connector's
`PrivateCACAPool` (the DCL by default, a direct controller as an opt-in)
masks top-level fields too, and knows neither field, so it never sees a
diff on them. When Config Connector changes its own issuance policy
fields, its update sends an issuance policy without ours and the API
clears them; this kind puts them back at its next reconcile (the partial
case of section 6.6, as `ContainerNodePoolFields`' `kubeletConfig`). The
test sends such an update and checks both that Config Connector's change
stays and that ours comes back.

Delete clears the fields it owns and leaves the rest of the issuance
policy; with `deletion-policy: abandon` they stay. A second object for
the same pool in the namespace is refused; a missing pool is waited for,
never made.

Not built (item 25): `issuancePolicy.allowRequesterSpecifiedNotBeforeTime`
is in neither the Go client at the pin (`cloud.google.com/go/security`
v1.24.0) nor mockgcp's protos; `encryptionSpec.cloudKmsKey` is taken only
when the pool is made, so it needs a kind that makes the pool (section
6.6, pattern 1).

The test (`TestPrivateCACAPoolFields`) makes the run's own DevOps-tier
pool in us-central1 with no certificate authority (no cost) and deletes
it. Live: pending, and skipped unless `E2E_PRIVATECA=true`
(`privateca.googleapis.com` on and `roles/privateca.admin` for the
live-test service account): `E2E_PRIVATECA=true task test-live
PROJECT=$GCP_PROJECT KINDS=privateca LIVE_RUN=TestPrivateCACAPoolFields`.
What to look for: whether the API takes the whole issuance policy back
as read (or refuses a field it returned), and how it answers a backdate
over 48 hours.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
