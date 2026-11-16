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

2026-11-16, live (run `cce-11160523-1e83`): the API refuses a policy on a
key that is not enrolled in Key Access Justifications, setting and
clearing alike: 400, "This key is not enrolled in Key Access
Justifications with policy enforcement". Enrollment comes from Assured
Workloads (a folder with Key Access Justifications), which the live-test
project is not in. The controller reports the API's message (not Ready,
`UpdateFailed`) and tries again; nothing else of the key was written (the
test's key kept its labels and has no policy). The live test now skips
unless `E2E_KMS_KAJ=true`, for a run against an enrolled project; whether
to enroll one is a question for the builder (CCEQ8). The mask behaviour
(only the policy is written; Config Connector's label update leaves it)
is proven on the fake only.

## Live runs

| Date | Commit | Mock | Live | Differences seen | Cost-relevant resources made and deleted |
| --- | --- | --- | --- | --- | --- |
| 2026-11-16 | `7be2586` | pass (9 cases) | fail: 400 "This key is not enrolled in Key Access Justifications with policy enforcement" on the first patch (run `cce-11160523-1e83`); now skips without `E2E_KMS_KAJ=true` | the API needs Key Access Justifications enrollment; mockgcp does not check it | one key with no version (`cce-11160523-1e83-kaj-key`, free; Cloud KMS never deletes keys) in key ring `cce-e2e-fields` |
