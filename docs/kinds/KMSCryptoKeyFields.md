---
kinds: [KMSCryptoKeyFields]
apiVersion: kms.gcp.hfl.dev/v1alpha1
---
# KMSCryptoKeyFields

`kms.gcp.hfl.dev/v1alpha1`

**Gap it fills.** a Cloud KMS key's `keyAccessJustificationsPolicy`, which Config Connector's `KMSCryptoKey` lacks; a field kind (`docs/design.md` section 6.6, pattern 2, and decision 37) that sets only that field on a key Config Connector's `KMSCryptoKey` (or anything else) made

**Source.** written here from the Cloud KMS API; the reference (`resourceRef`) is Config Connector's `KMSCryptoKeyRef`

## Notes

2026-11-16: built after the builder named the pattern (section 10
question 5, decision 37). `spec.resourceRef` names Config Connector's
`KMSCryptoKey` by `name` (resolved from its `status.externalRef` or
`status.selfLink`) or any key by `external`; `spec.keyAccessJustificationsPolicy.allowedAccessReasons`
is the API's list, by the API's enum names (an empty list allows no
access). The controller reads the key with `cryptoKeys.get` and writes
with `cryptoKeys.patch` and `updateMask=keyAccessJustificationsPolicy`
alone, so no other field of the key is sent. Terraform's update of a key
(Config Connector's `KMSCryptoKey` runs Terraform) also masks only the
fields it changed; the test makes that update (labels, mask `labels`)
and checks the policy survives. The object never creates or deletes the
key: a missing key is waited for (not Ready, "does not exist yet").
Delete clears the policy (a mask with the path and no policy); with
`deletion-policy: abandon` it stays. A second object for the same key in
the namespace is refused (the oldest owns it). Patch 0052 makes mockgcp's
`cryptoKeys.patch` take the path.

The test's key is the run's own, made with no key version (no cost); Cloud
KMS never deletes a key or key ring, so keys stay in the shared key ring
`cce-e2e-fields` (us-central1) with their policy cleared.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
