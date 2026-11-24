---
kinds: [PrivateCACertificateAuthority]
apiVersion: privateca.gcp.hfl.dev/v1alpha1
---
# PrivateCACertificateAuthority

`privateca.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Certificate Authority Service authority with the fields the API takes only at create and Config Connector's `PrivateCACertificateAuthority` lacks: `config.x509Config.nameConstraints` and `aiaOcspServers`, `config.subjectKeyId`; and `userDefinedAccessUrls` (foundation item 25; `docs/design.md` section 2.3)

**Source.** Config Connector's direct `PrivateCACertificateAuthority` types, identity, mapper and controller; enabling and disabling after Terraform's `google_privateca_certificate_authority` (read, not copied)

## Which kind to use

Config Connector's `privateca.cnrm.cloud.google.com/v1beta1`
`PrivateCACertificateAuthority` for an authority that needs none of the
fields above; this one for a new authority that does. The spec is Config
Connector's plus those fields and `skipGracePeriod`. The API takes an
authority's config only when it is made, so an existing authority cannot
gain them: make a new one with this kind. Never manage one authority with
both kinds.

## Notes

2026-11-23: built (foundation item 25), `docs/design.md` section 6.6
pattern 1 (a create-only whole-object kind, as `ComputeVPNTunnel`). Every
field but `metadata.labels` and `userDefinedAccessUrls` is immutable (CEL);
nothing else is compared after create, so an update sends
`certificateAuthorities.patch` with a mask of only `labels` and
`user_defined_access_urls`, when they differ. A self-signed authority is
enabled after create (the API leaves a new one staged) and when found
staged; a subordinate one is left awaiting activation, which this kind
does not do (`subordinateConfig` is not built). A delete disables an
enabled authority first, then deletes it, with the 30-day grace period
unless `spec.skipGracePeriod`. An authority in its grace period (state
`DELETED`) counts as gone. A delete of an authority that was made
resolves nothing (`status.externalRef`). Patch 0062 makes mockgcp's update
honour the mask and refuse to delete an enabled authority.

The test (`TestPrivateCACertificateAuthority`) makes the run's DevOps tier
pool in the cloud, a root authority with the new fields (and checks them
in the cloud, enabled), a resync with no write, a labels and URLs change
in one update, the CEL refusals (a key spec with neither `algorithm` nor
`cloudKmsKeyVersionRef`, and a change of `config`, `lifetime`, `keySpec`,
`type` or `caPoolRef`), and a delete that disables first. Live: pending
(`E2E_PRIVATECA=true task test-live PROJECT=$GCP_PROJECT KINDS=privateca
LIVE_RUN=TestPrivateCACertificateAuthority`); a DevOps tier authority is
billed while it exists (by the hour), and the test deletes it without the
grace period. What to look for: whether the API keeps the `subjectKeyId`
sent (the test checks it on the fake only), the real error for deleting
an enabled authority, and whether the pool deletes at once afterwards.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
