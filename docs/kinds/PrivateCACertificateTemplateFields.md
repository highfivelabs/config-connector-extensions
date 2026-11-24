---
kinds: [PrivateCACertificateTemplateFields]
apiVersion: privateca.gcp.hfl.dev/v1alpha1
---
# PrivateCACertificateTemplateFields

`privateca.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Certificate Authority Service certificate template's `maximumLifetime` and `predefinedValues.nameConstraints`, which Config Connector's `PrivateCACertificateTemplate` lacks (foundation item 25); a field kind (`docs/design.md` section 6.6, pattern 2, and decision 37) that sets only those fields on a template Config Connector's `PrivateCACertificateTemplate` (or anything else) made

**Source.** written here from the Certificate Authority Service API; the reference (`resourceRef`) and the template's identity are Config Connector's `PrivateCACertificateTemplateRef` and `PrivateCACertificateTemplateIdentity` (`apis/privateca/v1beta1`); the name constraints type is Config Connector's generated one (`apis/privateca/v1beta1/types.generated.go`, commented out there as unreachable)

## Notes

2026-11-23: built (foundation item 25, decision 19). `spec.resourceRef`
names Config Connector's `PrivateCACertificateTemplate` by `name`
(Config Connector's reference resolves it from the spec once the object
is Ready) or any template by `external`
(`projects/{project}/locations/{location}/certificateTemplates/{template}`).
`spec.maximumLifetime` (seconds, such as `2592000s`) and
`spec.predefinedValues.nameConstraints` keep the API's names; at least
one is set, and a field left out is not owned.

The controller reads the template with `certificateTemplates.get` and
writes with `certificateTemplates.patch` and a mask of the owned fields
that differ: `maximum_lifetime`, and `predefined_values` sent as the
template has them with only the name constraints changed (the API's
masks name top-level fields). Config Connector's
`PrivateCACertificateTemplate` masks top-level fields too and knows
neither: `maximumLifetime` is never in its mask; when it changes its own
predefined values, its update clears our name constraints until this
kind's next reconcile (the partial case of section 6.6). The test sends
such an update and checks both.

Delete clears the fields it owns; with `deletion-policy: abandon` they
stay. A second object for the same template in the namespace is refused.
Patch 0060 makes mockgcp's done-at-once operations (which its templates
use) answer with their result, as the Go clients need.

The test (`TestPrivateCACertificateTemplateFields`) makes the run's own
template in us-central1 (no cost) and deletes it. Live: pending, and
skipped unless `E2E_PRIVATECA=true`: `E2E_PRIVATECA=true task test-live
PROJECT=$GCP_PROJECT KINDS=privateca
LIVE_RUN=TestPrivateCACertificateTemplateFields`.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
