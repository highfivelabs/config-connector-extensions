# Upstream

config-connector-extensions builds on Config Connector at one commit of `master`.
`hack/upstream.sh` reads the commit from the table below (the "Commit"
row), fetches it into `third_party/k8s-config-connector` and applies
`patches/k8s-config-connector/*.patch` in order; `hack/upstream.sh --check`
fails if the checkout is anything else.

`upstream.yaml` says the same as this file's tables in machine-readable
form (pins, pull requests, patches, copied files), for tools such as the
factory's daily watch. Change both together; `task upstream:yaml-check`
(part of `task check`) fails when they disagree.

| | |
| --- | --- |
| Repository | `github.com/GoogleCloudPlatform/k8s-config-connector` |
| Commit | `672414b8191fc2d2c12fff53f6faccf5ae291eef` |
| Commit date | 2026-11-07T03:04:32Z ("Greenfield: Triage judgement queue and finalize direct KRM types and identity for DataplexGlossaryTerm (#15158)") |
| Branch | `master` |
| Nearest release | `v1.160.0` = `c38b031c7f5e5657b6ac6afc18da95e039066ad8` (2026-10-29); the pin is ahead of it |
| Pinned on | 2026-11-07 |
| How | `git fetch --depth 1 origin <commit>`; used through `replace` in `go.mod`, never vendored or committed |

## What is copied, from where

All paths are in Config Connector at the commit above unless noted.

| Here | From | Changed |
| --- | --- | --- |
| `apis/sql/v1beta1/groupversion_info.go` | `apis/sql/v1beta1/groupversion_info.go` | group `sql.gcp.hfl.dev` |
| `apis/sql/v1beta1/sqluser_types.go` | `pkg/clients/generated/apis/sql/v1beta1/sqluser_types.go` (the Go types of the Terraform-based `SQLUser` CRD) | group, labels and short names; `instanceRef` and `password` use the shared direct-controller types (same schema); new `spec.databaseRoles`, `status.appliedDatabaseRoles` |
| `apis/sql/v1beta1/sqluser_identity.go` | `apis/sql/v1beta1/sqlsslcert_identity.go` | identity is `projects/{project}/instances/{instance}/users/{name}`, name user-chosen, MySQL host carried beside it |
| `pkg/controller/direct/sql/client.go` | `pkg/controller/direct/sql/client.go` | only the users and operations clients |
| `pkg/controller/direct/sql/sqluser_controller.go` | `pkg/controller/direct/sql/sqlsslcert_controller.go` | changed for users; `databaseRoles` as Terraform's `google_sql_user` does it |
| `apis/sqladmin/v1alpha1/groupversion_info.go` | `apis/sqladmin/v1alpha1/groupversion_info.go` | group `sqladmin.gcp.hfl.dev` |
| `apis/sqladmin/v1alpha1/sqladminbackup_types.go` | `apis/sqladmin/v1alpha1/sqladminbackup_types.go` | group, labels, short names |
| `apis/sqladmin/v1alpha1/sqladminbackup_identity.go` | `apis/sqladmin/v1alpha1/sqladminbackup_identity.go`, plus PR #12553's change to it | the backup ID is Cloud SQL's, so it comes from `status.externalRef` (as in the PR) |
| `pkg/controller/direct/sqladmin/sqladminbackup_controller.go` | PR #12553 (head `9611a6de`) `pkg/controller/direct/sqladmin/sqladminbackup_controller.go` | the sqladmin v1beta4 REST client instead of `cloud.google.com/go/sql` (not in the pin's go.mod; adding it moves ~65 shared modules); Update compares the fields the spec sets; Find takes a run with status `DELETED` as gone (seen live) |
| `pkg/controller/direct/sqladmin/sqladminbackup_mapping.go` | PR #12553 `mapper.generated.go` and `mapper.go` | the same fields, mapped to the REST types |
| `apis/secretmanager/v1beta1/groupversion_info.go` | `apis/secretmanager/v1beta1/groupversion_info.go` | group `secretmanager.gcp.hfl.dev` |
| `apis/secretmanager/v1beta1/secretmanagersecret_types.go` | `apis/secretmanager/v1beta1/secretmanagersecret_types.go` | group, labels, short names; `spec.location` from PR #6651 (head `9aab204d`) |
| `apis/secretmanager/v1beta1/secretmanagersecret_identity.go`, `secretmanagersecret_reference.go` | the same files | PR #6651: a secret's parent carries its location; regional external references parse |
| `apis/secretmanager/v1beta1/secretmanagersecretversion_types.go` | the same file | group, labels, short names; `secretRef` typed `SecretManagerSecretRef`, not its alias (controller-gen v0.16.5 panics on aliases under Go 1.27; same schema) |
| `apis/secretmanager/v1beta1/secretmanagersecretversion_identity.go`, `secretmanagersecretversion_reference.go` | the same files | PR #6651: a version's parent is its secret's identity; regional version references parse |
| `apis/secretmanager/v1beta1/types.generated.go` | the same file | no |
| `pkg/controller/direct/secretmanager/secretmanagersecret_controller.go` | the same file | PR #6651: a regional secret uses `secretmanager.<location>.rep.googleapis.com`; a `CloudLabels` method for the slot guard (`pkg/slotguard`) |
| `pkg/controller/direct/secretmanager/secretmanagersecretversion_controller.go` | the same file | PR #6651: the same endpoint for its versions; ours: an unset `spec.enabled` means true (upstream dereferences it unchecked; there the direct controller is opt-in) |
| `pkg/controller/direct/secretmanager/mapper.generated.go`, `secret_mapping.go` | the same files | our KRM types |
| `apis/artifactregistry/v1beta1/groupversion_info.go` | `apis/artifactregistry/v1beta1/groupversion_info.go` | group `artifactregistry.gcp.hfl.dev` |
| `apis/artifactregistry/v1beta1/artifactregistryrepository_types.go` | the same file, plus PR #13517 (head `eb2774ce`) | group, labels, short names; `customRepository` under `remoteRepositoryConfig.{docker,maven,npm,python}Repository` from the PR; written here: the `upstreamCredentials` and `disableUpstreamValidation` fields (their types are ours, `upstreamcredentials_types.go`) |
| `apis/artifactregistry/v1beta1/artifactregistryrepository_identity.go` | the same file | no |
| `apis/artifactregistry/v1beta1/artifactregistryrepository_reference.go` | the same file | the reference names our kind (`ArtifactRegistryRepositoryGVK` is in `artifactregistry.gcp.hfl.dev`) |
| `apis/artifactregistry/v1beta1/types.generated.go` | the same file, plus PR #13517 | no (the PR changes only comments) |
| `pkg/controller/direct/artifactregistry/artifactregistryrepository_controller.go` | the same file | our KRM types; written here: resolve the upstream password's secret version, ignore the input-only `disableUpstreamValidation`, narrow the update mask for credentials (`upstreamcredentials_mapping.go`, ours); a `CloudLabels` method for the slot guard (`pkg/slotguard`) |
| `pkg/controller/direct/artifactregistry/mapper.go` | the same file | our KRM types |
| `pkg/controller/direct/artifactregistry/mapper.generated.go` | the same file, plus PR #13517 | only the `ArtifactRegistryRepository` (v1beta1) functions; our KRM types; the PR's `customRepository` mapping; written here: the remote config maps `upstreamCredentials` and `disableUpstreamValidation` |
| `pkg/controller/direct/artifactregistry/artifactregistryrepository_fuzzer_test.go` | `pkg/controller/direct/artifactregistry/artifactregistryrepository_fuzzer.go`, plus PR #13517 | a test file, run by its own test function instead of registered with Config Connector's fuzz runner |
| `apis/artifactregistry/v1alpha1/groupversion_info.go` | `apis/artifactregistry/v1alpha1/groupversion_info.go` | group `artifactregistry.gcp.hfl.dev`; the version's only kind, `ArtifactRegistryTag`, is written here |
| `apis/storage/v1alpha1/groupversion_info.go` | `apis/storage/v1alpha1/groupversion_info.go` | group `storage.gcp.hfl.dev`, with the group name marker upstream keeps in `doc.go`; its kind, `StorageBucketObject`, is written here |
| `apis/compute/v1alpha1/groupversion_info.go` | `apis/compute/v1alpha1/groupversion_info.go` | group `compute.gcp.hfl.dev`; its kinds `ComputeRegionNetworkEndpoint`, `ComputeRouterRoutePolicy`, `ComputeProjectMetadataItem`, `ComputeProjectDefaultNetworkTier` and `ComputeRegionNetworkFirewallPolicyRule`, `ComputeRegionNetworkFirewallPolicyAssociation` (with the `ComputeRegionNetworkFirewallPolicyRef` it and the association share) are written here |
| `apis/compute/v1alpha1/computeregionnetworkfirewallpolicy_types.go` | `apis/compute/v1beta1/computenetworkfirewallpolicy_types.go` | `ComputeNetworkFirewallPolicy` becomes `ComputeRegionNetworkFirewallPolicy`; group, labels, short names; projectRef typed as our other kinds'; written here: `spec.region`, `status.regionNetworkFirewallPolicyId` (Terraform's name) |
| `apis/compute/v1alpha1/computeregionnetworkfirewallpolicy_identity.go` | `apis/compute/v1beta1/computenetworkfirewallpolicy_identity.go` | `projects/{project}/regions/{region}/firewallPolicies/{firewallPolicy}`; the region from `spec.region` |
| `pkg/controller/direct/compute/regionnetworkfirewallpolicy_controller.go` | `pkg/controller/direct/compute/computenetworkfirewallpolicy_controller.go`, `computenetworkfirewallpolicy_mapper.go` | the regional client and the region in each request; our KRM types; written here: the spec mapping (description only) and an unset description sent as "" on update |
| `apis/compute/v1alpha1/computerouter_types.go` | `apis/compute/v1beta1/computerouter_types.go` (open PR #13960 does not change it) | the whole router (decision 14): written here, `bgp.identifierRange`, `md5AuthenticationKeys` (values from Secrets only), `interfaces` (upstream's `ComputeRouterInterfaceSpec` fields), `bgpPeers` (the generated `RouterBgpPeer` field set, Config Connector's `ComputeRouterPeer` names), `nats` (upstream's `ComputeRouterNATSpec` fields and types, plus four), `status.md5AuthenticationKeysVersion`; group, labels, short names |
| `apis/compute/v1alpha1/computerouter_identity.go` | `apis/compute/v1beta1/computerouter_identity.go` | no (the package) |
| `apis/compute/v1alpha1/types.generated.go`, `pkg/controller/direct/compute/mapper.generated.go` | generated by `hack/scaffold-kind.sh` (`google.cloud.compute.v1`, `ComputeRouter:Router`, as v1alpha2 since upstream has compute v1alpha1) | only `RouterBgpPeerBfd`, `RouterBgpPeerCustomLearnedIpRange` and `RouterNatSubnetworkToNat64` and their functions; the package |
| `pkg/controller/direct/compute/router_mapper.go` | `pkg/controller/direct/compute/computerouter_mapper.go`, `computerouternat_mapper.go`, `computerouterinterface_mapper.go`; the generated `RouterBgpPeer` mapper | our types; the NAT's and interface's name and new fields; `privateIpAddress` and `redundantInterface` as strings; written here: the peer's names, `enable` as a boolean, the router appliance reference, MD5 keys by name |
| `pkg/controller/direct/compute/router_controller.go` | `pkg/controller/direct/compute/computerouter_controller.go` | `routers.update` with the read router's other fields kept, a subset comparison, MD5 key values from Secrets, immutable network and `encryptedInterconnectRouter`, VPN tunnel and attachment references by name, our REST client |
| `apis/iam/v1alpha1/groupversion_info.go` | `apis/iam/v1alpha1/groupversion_info.go` | group `iam.gcp.hfl.dev` |
| `apis/logging/v1alpha1/groupversion_info.go` | `apis/logging/v1alpha1/groupversion_info.go` | group `logging.gcp.hfl.dev`; its kind `LoggingSettings` is written here |
| `apis/logging/v1alpha1/loggingbillingaccountsink_types.go` | `apis/logging/v1beta1/logginglogsink_types.go` | `LoggingBillingAccountSink` (decision 15): a required, immutable `billingAccountRef` (Config Connector's `billing/v1alpha1` reference) instead of the three parents; `includeChildren` and `uniqueWriterIdentity` dropped; upstream's destination, BigQuery options and exclusion types reused; `status.externalRef`; group, labels, short names |
| `apis/logging/v1alpha1/loggingbillingaccountsink_identity.go` | `apis/logging/v1beta1/logginglogsink_identity.go` | only the `billingAccounts/{billingaccount}/sinks/{sink}` format (upstream's identity has it already); the parent from `billingAccountRef`; checked against `status.externalRef` |
| `pkg/controller/direct/logging/billingaccountsink_controller.go` | `pkg/controller/direct/logging/logginglogsink_controller.go` | names; `uniqueWriterIdentity` always true; `status.externalRef` |
| `pkg/controller/direct/logging/billingaccountsink_mapper.go` | `pkg/controller/direct/logging/mapper.go` | only the `LoggingLogSinkSpec` and `Status` functions, named for the kind, without `includeChildren` |
| `apis/resourcemanager/v1alpha1/groupversion_info.go` | `apis/resourcemanager/v1beta1/groupversion_info.go` | group `resourcemanager.gcp.hfl.dev`, version `v1alpha1`; its kind, `ProjectDefaultServiceAccounts`, is written here |
| `apis/serviceusage/v1alpha1/serviceidentity_types.go` | generated by `hack/scaffold-kind.sh` (controllerbuilder `generate-types`), then edited after `apis/serviceusage/v1beta1/serviceidentity_types.go` | `folderRef` or `organizationRef` in place of the scaffold's `projectRef` and `location`; group, labels, short names; status `email` as upstream's, `externalRef` |
| `apis/serviceusage/v1alpha1/doc.go`, `groupversion_info.go`, `types.generated.go`, `pkg/controller/direct/serviceusage/mapper.generated.go` | generated by `hack/scaffold-kind.sh` (controllerbuilder `generate-types`, `generate-mapper`) | no (the script rewrites the group and import paths and adds the SPDX line) |
| `apis/serviceusage/v1alpha1/serviceidentity_identity.go` | written here, in the shape of `apis/serviceusage/v1beta1/serviceidentity_identity.go` | `folders/{id}/services/{service}` or `organizations/{id}/services/{service}` |
| `pkg/controller/direct/serviceusage/serviceidentity_controller.go` | `pkg/controller/direct/serviceusage/serviceidentity_controller.go` | a folder or organization parent; status `externalRef` and `observedState` (through the generated mapper); no export |
| `hack/scaffold/serviceusage/overlay.proto` | written here, in the form of `apis/batch/v1alpha1/overlay.proto` | `ServiceIdentity`'s fields output-only |
| `apis/networksecurity/v1alpha1/groupversion_info.go`, `doc.go` | the same files | group `networksecurity.gcp.hfl.dev` |
| `apis/networksecurity/v1alpha1/networksecurityaddressgroup_types.go` | the same file | `organizationRef` beside `projectRef`, exactly one (CEL); `capacity` immutable (CEL, Magic Modules' `AddressGroup.yaml`); group, labels, short names |
| `apis/networksecurity/v1alpha1/networksecurityaddressgroup_identity.go`, `networksecurityaddressgroup_identity_test.go` | the same files | an organization's address group, `organizations/{organization}/locations/{location}/addressGroups/{addressgroup}`, beside a project's |
| `apis/securitycenter/v1alpha1/groupversion_info.go` | the same file | group `securitycenter.gcp.hfl.dev` |
| `apis/securitycenter/v1alpha1/parent.go` | PR #7673 (head `423fea9f`) `apis/securitycenter/v1alpha1/identity.go` | `folderRef` typed without its alias; `ParseParent` returns the rest of the name |
| `apis/securitycenter/v1alpha1/securitycentermuteconfig_types.go` | the same file | `folderRef` and `projectRef` beside `organizationRef` (as PR #7673), exactly one and immutable (CEL); group, labels, short names |
| `apis/securitycenter/v1alpha1/securitycentermuteconfig_identity.go` | the same file | `{parent}/muteConfigs/{muteConfig}` with an organization, folder or project parent |
| `pkg/controller/direct/securitycenter/muteconfig_controller.go` | `pkg/controller/direct/securitycenter/muteconfig/securitycentermuteconfig_controller.go` | package and model name; the identity's parent; the update compares with the name the API returned; export sets the parent it came from; delete of a missing mute config is done |
| `pkg/controller/direct/securitycenter/muteconfig_mapper.go` | `pkg/controller/direct/securitycenter/mapper.generated.go` | only the `SecurityCenterMuteConfig` functions; our KRM types |
| `pkg/controller/direct/networksecurity/addressgroup_controller.go` | `pkg/controller/direct/networksecurity/networksecurityaddressgroup_controller.go` | the organization service's client for an organization parent; our KRM types; returned to our manager, not registered; a `CloudLabels` method for the slot guard (`pkg/slotguard`) |
| `pkg/controller/direct/networksecurity/mapper.generated.go` | the same file (generated upstream by controllerbuilder) | only the `NetworkSecurityAddressGroup`, `NetworkSecuritySecurityProfile` and `NetworkSecuritySecurityProfileGroup` functions; our KRM types (the intercept and mirroring endpoint group references stay Config Connector's) |
| `pkg/controller/direct/networksecurity/addressgroup_fuzzer_test.go` | `pkg/controller/direct/networksecurity/networksecurityaddressgroup_fuzzer.go` | a test file, run by its own test function |
| `apis/networksecurity/v1alpha1/networksecuritysecurityprofile_types.go` | the same file | `organizationRef` beside `projectRef`, exactly one (CEL); `type` required, an enum and immutable, at most one profile (CEL; Magic Modules' `SecurityProfile.yaml`); group, labels, short names; the endpoint group references are Config Connector's types |
| `apis/networksecurity/v1alpha1/networksecuritysecurityprofile_identity.go`, `networksecuritysecurityprofile_identity_test.go`, `networksecuritysecurityprofile_reference.go` | the same files | an organization's profile, `organizations/{organization}/locations/{location}/securityProfiles/{securityprofile}`, beside a project's; the reference names our kind |
| `pkg/controller/direct/networksecurity/securityprofile_controller.go` | `pkg/controller/direct/networksecurity/networksecuritysecurityprofile_controller.go` | the REST clients (`SecurityProfileGroupClient`, `OrganizationSecurityProfileGroupClient` for an organization parent) in place of upstream's gRPC connection, which our manager does not configure; an unchanged object still gets its status; the comparison ignores list order and the URL filter the API adds (Magic Modules' `SecurityProfile.yaml`); no export; our KRM types; returned to our manager, not registered; a `CloudLabels` method for the slot guard (`pkg/slotguard`) |
| `pkg/controller/direct/networksecurity/securityprofile_fuzzer_test.go` | `pkg/controller/direct/networksecurity/networksecuritysecurityprofile_fuzzer.go` | a test file, run by its own test function |
| `apis/networksecurity/v1alpha1/networksecuritysecurityprofilegroup_types.go` | the same file | `organizationRef` beside `projectRef`, exactly one (CEL); the profile references name our `NetworkSecuritySecurityProfile`; group, labels, short names |
| `apis/networksecurity/v1alpha1/networksecuritysecurityprofilegroup_identity.go`, `networksecuritysecurityprofilegroup_identity_test.go` | the same files | an organization's group, `organizations/{organization}/locations/{location}/securityProfileGroups/{securityprofilegroup}`, beside a project's |
| `pkg/controller/direct/networksecurity/securityprofilegroup_controller.go` | `pkg/controller/direct/networksecurity/networksecuritysecurityprofilegroup_controller.go` | as `securityprofile_controller.go`: the REST clients, the organization service's for an organization parent; an unchanged object still gets its status; no export; our KRM types; returned to our manager, not registered; a `CloudLabels` method for the slot guard (`pkg/slotguard`) |
| `pkg/controller/direct/networksecurity/securityprofilegroup_fuzzer_test.go` | `pkg/controller/direct/networksecurity/networksecuritysecurityprofilegroup_fuzzer.go` | a test file, run by its own test function |
| `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_types.go` | PR #14209 (head `a5e8f5a9`) `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_types.go` | group, labels, short names; written here: `spec.workloadIdentityPoolRef` (the PR has no field for the pool) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_identity.go` | PR #14209 `iamworkloadidentitypoolnamespace_identity.generated.go` | written here: the pool from `workloadIdentityPoolRef` (the PR's is a stub that always fails) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolnamespace_reference.go` | PR #14209 `iamworkloadidentitypoolnamespace_reference.generated.go` | the reference names our kind |
| `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_types.go` | PR #14418 (head `b080f5dc`) `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_types.go` | group, labels, short names; written here: `spec.workloadIdentityPoolNamespaceRef` and `spec.attestationRules` (the PR has neither) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_identity.go` | PR #14418 `iamworkloadidentitypoolmanagedidentity_identity.generated.go` | written here: the pool and namespace from `workloadIdentityPoolNamespaceRef` (the PR's are stubs that always fail) |
| `apis/iam/v1alpha1/iamworkloadidentitypoolmanagedidentity_reference.go` | PR #14418 `iamworkloadidentitypoolmanagedidentity_reference.generated.go` | the reference names our kind |
| `apis/iam/v1alpha1/types.generated.go` | PR #14209 `apis/iam/v1alpha1/types.generated.go` | only `OwnerServiceObservedState`, the type the PR adds for this kind |
| `apis/iam/v1beta1/groupversion_info.go` | `apis/iam/v1beta1/groupversion_info.go` | group `iam.gcp.hfl.dev`: our stable IAM kinds (decision 13) |
| `apis/iam/v1beta1/doc.go` | `apis/iam/v1alpha1/doc.go` | the package |
| `apis/iam/v1beta1/iamdenypolicy_types.go` | `apis/iam/v1alpha1/iamdenypolicy_types.go` | a stable kind (decision 13); exactly one of `projectRef`, `folderRef`, `organizationRef` (CEL); upstream's commented-out fields dropped; `folderRef` typed without its alias; group, labels, short names |
| `apis/iam/v1beta1/iamdenypolicy_identity.go`, `iamdenypolicy_identity_test.go` | the same files in `apis/iam/v1alpha1/` | the package |
| `apis/iam/v1beta1/types.generated.go` | `apis/iam/v1alpha1/types.generated.go` (generated upstream by controllerbuilder) | only `DenyRule`, `PolicyRule`, `Expr`, `PrincipalAccessBoundaryPolicyDetails` and `PrincipalAccessBoundaryPolicyRule`, unedited; the package |
| `pkg/controller/direct/iam/denypolicy_controller.go` | `pkg/controller/direct/iam/iamdenypolicy/iamdenypolicy_controller.go` | `status.externalRef` written (upstream's never reaches the object), also when nothing changed; our KRM types; returned to our manager, not registered |
| `pkg/controller/direct/iam/denypolicy_mapper.go` | `pkg/controller/direct/iam/mappers.go` | only the `IAMDenyPolicy` functions and `Expr`'s, named for the kind; our KRM types |
| `pkg/controller/direct/iam/denypolicy_fuzzer_test.go` | `pkg/controller/direct/iam/iamdenypolicy_fuzzer.go` | a test file, run by its own test function |
| `apis/iam/v1beta1/iamprincipalaccessboundarypolicy_types.go` | `apis/iam/v1alpha1/iamprincipalaccessboundarypolicy_types.go` | a stable kind (decision 13); `organizationRef`, `location` and `resourceID` immutable (CEL, decision 20); group, labels, short names |
| `apis/iam/v1beta1/iamprincipalaccessboundarypolicy_identity.go`, `iamprincipalaccessboundarypolicy_reference.go`, `iamprincipalaccessboundarypolicy_identity_test.go` | `apis/iam/v1alpha1/iamprincipalaccessboundarypolicy_identity.generated.go`, `..._reference.generated.go`, `..._identity_generated_test.go` (generated upstream by controllerbuilder) | the package; the generated-code marker dropped |
| `pkg/controller/direct/iam/principalaccessboundarypolicy_controller.go` | `pkg/controller/direct/iam/iamprincipalaccessboundarypolicy_controller.go` | our KRM types |
| `pkg/controller/direct/iam/principalaccessboundarypolicy_mapper.go` | `pkg/controller/direct/iam/iamprincipalaccessboundarypolicy_mapper.go` | our KRM types |
| `pkg/controller/direct/iam/principalaccessboundarypolicy_fuzzer_test.go` | `pkg/controller/direct/iam/iamprincipalaccessboundarypolicy_fuzzer.generated.go` | a test file, run by its own test function |
| `apis/compute/v1beta1/groupversion_info.go` | `apis/compute/v1beta1/groupversion_info.go` | group `compute.gcp.hfl.dev`: our stable compute kinds (decision 13) |
| `apis/compute/v1beta1/doc.go` | `apis/compute/v1alpha1/doc.go` | the package |
| `apis/compute/v1beta1/computeorganizationsecuritypolicy_types.go` | `apis/compute/v1alpha1/computeorganizationsecuritypolicy_types.go` | a stable kind (decision 13); `displayName`, `parent` and `type` immutable (CEL); group, labels, short names |
| `apis/compute/v1beta1/computeorganizationsecuritypolicyassociation_types.go` | `apis/compute/v1alpha1/computeorganizationsecuritypolicyassociation_types.go` | a stable kind (decision 13); exactly one of `policyId` and `securityPolicyRef`, the spec immutable (CEL); group, labels, short names |
| `apis/compute/v1beta1/computeorganizationsecuritypolicy_identity.go`, `apis/compute/v1beta1/computeorganizationsecuritypolicy_identity_test.go`, `apis/compute/v1beta1/computeorganizationsecuritypolicy_reference.go`, `apis/compute/v1beta1/computeorganizationsecuritypolicyassociation_identity.go`, `apis/compute/v1beta1/computeorganizationsecuritypolicyassociation_identity_test.go`, `apis/compute/v1beta1/computeorganizationsecuritypolicyassociation_reference.go` | the same files in `apis/compute/v1alpha1/` | the package; the test objects carry our group and version |
| `pkg/controller/direct/compute/organizationsecuritypolicy_controller.go`, `pkg/controller/direct/compute/organizationsecuritypolicyassociation_controller.go` | `pkg/controller/direct/compute/computeorganizationsecuritypolicy_controller.go`, `pkg/controller/direct/compute/computeorganizationsecuritypolicyassociation_controller.go` | our KRM types and mapper names; returned to our manager, not registered |
| `apis/compute/v1beta1/computeorganizationsecuritypolicyrule_types.go` | `apis/compute/v1alpha1/computeorganizationsecuritypolicyrule_types.go` | a stable kind (decision 13); `securityPolicyRef` beside `policyId`, exactly one (CEL); `policyId`, `securityPolicyRef` and `resourceID` (the priority) immutable (CEL); `direction` and `versionedExpr` enums; `match.description` refused (the API has none); at least one `layer4Config`; `status.externalRef`, `status.ruleTupleCount`; group, labels, short names |
| `apis/compute/v1beta1/computeorganizationsecuritypolicyrule_identity.go`, `apis/compute/v1beta1/computeorganizationsecuritypolicyrule_identity_test.go`, `apis/compute/v1beta1/computeorganizationsecuritypolicyrule_reference.go` | the same files in `apis/compute/v1alpha1/` | the package; the priority must be a number; the policy from `securityPolicyRef` as the association resolves it; a changed identity refused; the test's objects in our group, its error cases with a numeric priority, and our cases |
| `apis/compute/v1beta1/computevpntunnel_types.go` | the same file | a field-gap kind (decision 19, `docs/design.md` 6.6): `cipherSuite`, `labels`, `projectRef`, `status.externalRef` added; immutable fields by CEL; exactly one of `vpnGatewayRef` and `targetVPNGatewayRef` (CEL); `routerRef` ours; group, labels, short names |
| `pkg/controller/direct/compute/vpntunnel_mapper.go` | `pkg/controller/direct/compute/computevpntunnel_mapper.go` | our types; the spec-to-proto and status-from-proto directions only; the name and shared secret passed in; `cipherSuite` and `labels` |
| `pkg/controller/direct/compute/organizationsecuritypolicy_mapper.go` | `pkg/controller/direct/compute/mappers.go` | only the policy's and the association's functions; our KRM types, `v1beta1` in their names |
| `apis/compute/v1beta1/computenetworkendpoint_types.go` | `apis/compute/v1alpha1/computenetworkendpoint_types.go` | a stable kind (decision 26); the spec immutable (CEL); group, labels, short names |
| `apis/compute/v1beta1/computenetworkendpoint_identity.go`, `apis/compute/v1beta1/computenetworkendpoint_identity_test.go`, `apis/compute/v1beta1/computenetworkendpoint_reference.go` | the same files in `apis/compute/v1alpha1/` | the package |
| `pkg/controller/direct/compute/networkendpoint_controller.go`, `pkg/controller/direct/compute/networkendpoint_mapper.go` | `pkg/controller/direct/compute/computenetworkendpoint_controller.go`, `pkg/controller/direct/compute/computenetworkendpoint_mapper.go` | our KRM types; returned to our manager, not registered; the REST client from the controller config's options; helpers and mapper functions named for this package; the mapper's unused status functions dropped; a detach never sends an empty `instance` (the API refuses it for hybrid endpoints, run `cce-11121922-5984`) |
| `apis/aiplatform/v1beta1/groupversion_info.go`, `apis/aiplatform/v1beta1/doc.go` | the same files in `apis/aiplatform/v1alpha1/` | group `aiplatform.gcp.hfl.dev`, version `v1beta1`: our stable Vertex AI kinds (decision 26) |
| `apis/aiplatform/v1beta1/aiplatformreasoningengine_types.go` | `apis/aiplatform/v1alpha1/aiplatformreasoningengine_types.go` | a stable kind (decision 26); upstream's leaf types imported, not copied; `projectRef`, `location` and `resourceID` immutable (CEL); group, labels, short names |
| `apis/aiplatform/v1beta1/aiplatformreasoningengine_identity.go`, `apis/aiplatform/v1beta1/aiplatformreasoningengine_identity_test.go` | the same files in `apis/aiplatform/v1alpha1/` | the package |
| `pkg/controller/direct/aiplatform/reasoningengine_controller.go` | `pkg/controller/direct/aiplatform/aiplatformreasoningengine_controller.go` | our KRM types; returned to our manager, not registered; the JSON REST API through `pkg/discokind`'s `API` with the protos as JSON (the Go client has no REST transport for reasoning engines); `CloudLabels` for the slot guard; no export |
| `pkg/controller/direct/aiplatform/reasoningengine_mapper.go` | `pkg/controller/direct/aiplatform/aiplatformreasoningengine_mapper.go`, and the functions it calls in `mapper.generated.go` and `aiplatformschedule_mapper.go` | our KRM types for the kind, upstream's for the leaf types; only the functions this kind uses |
| `pkg/controller/direct/compute/organizationsecuritypolicy_fuzzer_test.go`, `pkg/controller/direct/compute/organizationsecuritypolicyassociation_fuzzer_test.go`, `pkg/controller/direct/compute/organizationsecuritypolicyassociation_mapper_test.go` | `pkg/controller/direct/compute/computeorganizationsecuritypolicy_fuzzer.go`, `pkg/controller/direct/compute/computeorganizationsecuritypolicyassociation_fuzzer.go`, `pkg/controller/direct/compute/computeorganizationsecuritypolicyassociation_mapper_test.go` | test files, the fuzzers run by their own test functions |
| `apis/dns/v1beta1/groupversion_info.go` | `apis/dns/v1alpha1/groupversion_info.go` | group `dns.gcp.hfl.dev`, version `v1beta1`: our stable DNS kinds (decision 13) |
| `apis/dns/v1beta1/doc.go` | `apis/dns/v1alpha1/doc.go` | the package |
| `apis/dns/v1beta1/dnsresponsepolicy_types.go` | `apis/dns/v1alpha1/dnsresponsepolicy_types.go` | a stable kind (decision 13); `projectRef` and `resourceID` immutable by CEL (decision 20); `status.externalRef`; group, labels, short names |
| `apis/dns/v1beta1/dnsresponsepolicy_identity.go`, `dnsresponsepolicy_identity_test.go` | the same files in `apis/dns/v1alpha1/` | the package |
| `apis/dns/v1beta1/dnsresponsepolicy_reference.go` | `apis/dns/v1alpha1/dnsresponsepolicy_reference.go` | the package; the reference names our kind |
| `pkg/controller/direct/dns/responsepolicy_controller.go` | `pkg/controller/direct/dns/dnsresponsepolicy_controller.go` | `status.externalRef` written; `metadata.labels` sent as the policy's labels and compared (decision 19); our KRM types; returned to our manager, not registered; a `CloudLabels` method for the slot guard (`pkg/slotguard`) |
| `pkg/controller/direct/dns/responsepolicy_mapper.go` | `pkg/controller/direct/dns/zz_generated.v1alpha1.mappers.go` (generated upstream by `dev/tools/openapi-to-krm`) | only the `DNSResponsePolicy` functions, unedited; our KRM types |
| `pkg/controller/direct/dns/responsepolicy_fuzzer_test.go` | `pkg/controller/direct/dns/dnsresponsepolicy_fuzzer.go` | a test file, run by its own test function |
| `apis/run/v1alpha1/groupversion_info.go` | `apis/run/v1alpha1/groupversion_info.go` | group `run.gcp.hfl.dev` |
| `apis/run/v1alpha1/runservice_types.go` | `apis/run/v1alpha1/runworkerpool_types.go` | `RunWorkerPool` becomes `RunService` (Cloud Run v2 `Service`), its template `RevisionTemplate`; no instance splits; group, labels, short names; secret references typed without their aliases; written here: the service's `ingress`, `invokerIamDisabled`, `defaultUriDisabled`, `scaling`, the template's scaling, timeout, concurrency, execution environment, session affinity, health check and GPU zonal redundancy fields, the URIs in status |
| `apis/run/v1alpha1/servicescaling_types.go` | written here | `ServiceScaling`, `RevisionScaling` from the Cloud Run v2 API |
| `apis/run/v1alpha1/runservice_identity.go`, `runservice_reference.go` | `apis/run/v1alpha1/runworkerpool_identity.go`, `runworkerpool_reference.go` | a service (`.../services/{service}`) instead of a worker pool; the reference names our kind |
| `apis/run/v1alpha1/types.generated.go` | the same file | without `InstanceSplit`, `InstanceSplitStatus`, `WorkerPoolScaling` |
| `pkg/controller/direct/run/runservice_controller.go` | `pkg/controller/direct/run/runworkerpool_controller.go` | the Services client instead of WorkerPools; no instance splits; written here: a service's server defaults taken from what was read; a `CloudLabels` method for the slot guard (`pkg/slotguard`) |
| `pkg/controller/direct/run/mapper.generated.go` | the same file | only the v1alpha1 functions and three Condition reason helpers; our KRM types; the worker pool's template mapping for a `RevisionTemplate`, plus (written here) the fields a service template has |
| `pkg/controller/direct/run/runservice_mapper.go` | `pkg/controller/direct/run/runworkerpool_mapper.go` | the spec and observed state for a `Service`; written here: the spec's `FromProto`, the service fields, the scaling mappings |
| `pkg/controller/direct/run/resolverefs.go` | the same file | `ResolveRunServiceRefs` only; no v1beta1 cases |
| `apis/iam/v1beta1/iambinding_types.go`, `iambinding_identity.go`, `iamresourceref_types.go`, `apis/iam/v1alpha1/iammember_types.go`, `iammember_identity.go`, `iamresourceref_types.go`, `pkg/iampolicy/`, `pkg/controller/direct/iam/binding_controller.go`, `member_controller.go`, `policy_common.go` | written here | `IAMBinding` and `IAMMember`: no Config Connector code (its IAM kinds run on the Terraform provider, which this manager does not load); `membersFrom` reuses `apis/iam/iamrefs.IAMServiceAccountRef`; Terraform's `google_*_iam_binding` read, not copied |
| `hack/boilerplate.go.txt` | `hack/boilerplate.go.txt` | an SPDX line after Google's header, so generated files carry it |
| `hack/generate.sh` | `dev/tasks/generate-crds` (the controller-gen and crd-tools steps) | our paths; no version annotation, no alpha backport |
| `hack/regen-mocksql-users.sh` | `mockgcp/Makefile` (`gen-proto`), `mockgcp/apply-proto-patches.sh`, `mockgcp/fixup-third-party.sh` | one proto file only; our extra fields |

The CRD schema in `config/crds/` has the same property paths as upstream's
`config/crds/resources/apiextensions.k8s.io_v1_customresourcedefinition_sqlusers.sql.cnrm.cloud.google.com.yaml`
plus `spec.databaseRoles` and `status.appliedDatabaseRoles` (checked by
flattening both).
`config/crds/artifactregistry.gcp.hfl.dev_artifactregistryrepositories.yaml`
has the same property paths as PR #13517's
`apiextensions.k8s.io_v1_customresourcedefinition_artifactregistryrepositories.artifactregistry.cnrm.cloud.google.com.yaml`,
which is the pin's plus the four `customRepository.uri` fields (checked by
flattening all three, 2026-11-08).

## Patches to the checkout

| Patch | What | Upstream candidate |
| --- | --- | --- |
| `0001-mockgcp-sql-users-database-roles.patch` | mockgcp's Cloud SQL `User` gains `database_roles`, and `users.update` gains `database_roles` / `revoke_existing_roles` (googleapis master's field numbers); regenerated with `hack/regen-mocksql-users.sh`, which reproduces upstream's committed files byte for byte when run with `ONLY_UPSTREAM=1` | offered: GoogleCloudPlatform/k8s-config-connector#15278 |
| `0002-mockgcp-listen-address-env.patch` | `MOCKGCP_LISTEN_ADDRESS` overrides mockgcp's `127.0.0.2:0` (absent on macOS) | offered: GoogleCloudPlatform/k8s-config-connector#15277 |
| `0003-mockgcp-sql-backup-runs.patch` | mockgcp serves Cloud SQL `backupRuns` insert, get and delete: PR #12553's `mocksql/sqlbackup.go` and its registration, unchanged | it is upstream's; drop it when the PR merges and the pin passes it |
| `0004-mockgcp-secretmanager-regional.patch` | mockgcp serves regional secrets (PR #6651's `secrets.go` and `service.go` change, applied by hand) and, ours, versions under a regional secret | yes, with the PR |
| `0005-mockgcp-artifactregistry-upstream-credentials.patch` | written here: mockgcp's Artifact Registry accepts the update mask paths for a remote repository's upstream username and password version, and does not store the input-only `disableUpstreamValidation` | offered: GoogleCloudPlatform/k8s-config-connector#15281 |
| `0006-mockgcp-iam-workload-identity-pool-namespaces.patch` | mockgcp serves workload identity pool namespaces: PR #14209's `mockgcp/apis/google/iam/v1/workload_identity_pool_namespace.proto` as package `mockgcp.iam.v1`, its Go generated by `hack/regen-mockiam-workload-identity.sh` with mockgcp's toolchain (reproducible byte for byte), and written here: `mockiam/workloadidentitypoolnamespaces.go` (get, create, patch, soft delete, undelete, list, as long-running operations) and its registration | yes, with the PR |
| `0007-mockgcp-iam-workload-identity-pool-managed-identities.patch` | on top of 0006: mockgcp serves managed identities: PR #14418's proto as package `mockgcp.iam.v1`, without the operation metadata 0006 already defines, and, written here, with `setAttestationRules` and `listAttestationRules`; its Go generated by the same script; and, written here, `mockiam/workloadidentitypoolmanagedidentities.go` (get, create in an active namespace, patch, soft delete, undelete, list, attestation rules) | yes, with the PR |
| `0008-mockgcp-run-service-update-mask.patch` | mockgcp's Cloud Run `services.patch` changes exactly the masked top-level fields (written here), and keeps `invoker_iam_disabled` without a mask (PR #14417's line, head `9297537a`) | not offered (2026-11-10): with it, eight of upstream's RunService fixtures fail against mockgcp (their golden logs lack the container defaults); needs golden updates and the update-mask part split out |
| `0009-mockgcp-artifactregistry-tags.patch` | written here: mockgcp serves Artifact Registry package tags (get, create, patch `version`, delete) in an existing repository | offered: GoogleCloudPlatform/k8s-config-connector#15282 |
| `0010-mockgcp-iam-workload-identity-pool-provider-keys.patch` | on top of 0006 and 0007, written here: mockgcp serves workload identity pool provider keys (get, create with a placeholder PEM, soft delete, undelete, list), from a proto written from the IAM v1 discovery document and generated by `hack/regen-mockiam-workload-identity.sh` | yes |
| `0011-mockgcp-compute-region-network-endpoints.patch` | written here: mockgcp attaches, detaches and lists a regional network endpoint group's endpoints, adapted from its zonal ones | offered: GoogleCloudPlatform/k8s-config-connector#15279 |
| `0012-mockgcp-compute-router-route-policies.patch` | written here: mockgcp serves a Cloud Router's route policies (get, update, patch, delete, list) | offered: GoogleCloudPlatform/k8s-config-connector#15280 |
| `0013-mockgcp-storage-object-media.patch` | written here: mockgcp stores Cloud Storage objects with their content (media and multipart uploads, `alt=media` reads, get, delete), and its request prefilter leaves upload bodies, which are not JSON, alone; its objects were stubs | offered: GoogleCloudPlatform/k8s-config-connector#15290 |
| `0014-mockgcp-compute-region-network-firewall-policies.patch` | adapted from mockgcp's global network firewall policies: regional policies (get, list, insert, patch, delete) | offered: GoogleCloudPlatform/k8s-config-connector#15287 |
| `0015-mockgcp-sql-backup-runs-deleted.patch` | written here, on 0003: `backupRuns.delete` keeps the run with status `DELETED`, as real Cloud SQL does (a live run, `docs/live-runs.md`) | yes, with PR #12553 |
| `0016-mockgcp-artifactregistry-remote-source-immutable.patch` | written here, on 0005: an update of `remoteRepositoryConfig` fails with the API's own error, "Remote source configuartion cannot be updated" (a live run, `docs/live-runs.md`), not mockgcp's "not supported" | yes |
| `0017-mockgcp-logging-settings.patch` | written here: mockgcp serves Log Router settings (get, update with a mask) for organizations and folders | offered: GoogleCloudPlatform/k8s-config-connector#15291 |
| `0018-mockgcp-storage-iam-policy.patch` | written here: mockgcp serves Cloud Storage's `GET`/`PUT /storage/v1/b/{bucket}/iam` (and a managed folder's) from the store it keeps for the `:getIamPolicy` verbs | not offered (2026-11-10): with it, upstream's storageinsightsreportconfig-parquet fixture fails against mockgcp (its golden log records the old bucket IAM path); needs a golden update |
| `0019-mockgcp-compute-iam-policy.patch` | written here, on 0018: mockgcp serves Compute Engine's `GET {resource}/getIamPolicy` and `POST {resource}/setIamPolicy` from the store it keeps for the `:getIamPolicy` verbs, and makes the store's etag check and write atomic (two racing writers lost an update) | yes |
| `0020-mockgcp-compute-region-network-firewall-policy-rules.patch` | on top of 0014, adapted from mockgcp's global network firewall policies: a regional policy's rules (getRule, addRule, patchRule, removeRule), addressed by priority | yes |
| `0021-mockgcp-compute-region-network-firewall-policy-associations.patch` | on top of 0014 and 0020, written here: a regional policy's associations (getAssociation, addAssociation, removeAssociation) | yes |
| `0022-mockgcp-serviceusage-hierarchy-identity.patch` | written here: `services.generateServiceIdentity` (v1beta1) for `folders/{id}/services/{service}` and `organizations/{id}/services/{service}` | offered: GoogleCloudPlatform/k8s-config-connector#15284 |
| `0023-mockgcp-compute-project-default-network-tier.patch` | written here: `projects.setDefaultNetworkTier` (`PREMIUM` or `STANDARD`) | offered: GoogleCloudPlatform/k8s-config-connector#15288 |
| `0024-mockgcp-iam-service-accounts-list-undelete.patch` | written here: `serviceAccounts.list` and `undelete`; delete, disable and enable by unique ID as well as email; a deleted account kept for undelete | offered: GoogleCloudPlatform/k8s-config-connector#15283 |
| `0025-mockgcp-networksecurity-organization-address-groups.patch` | written here: mockgcp serves the `OrganizationAddressGroupService` (create, get, update, delete of `organizations/{id}/locations/{l}/addressGroups/{a}`) with the project service's server | follow GoogleCloudPlatform/k8s-config-connector#4711 (our #15285 closed as its duplicate; see "Watched upstream change") |
| `0026-mockgcp-networksecurity-organization-security-profiles.patch` | written here: mockgcp serves the `OrganizationSecurityProfileGroupService` (create, get, update, delete of `organizations/{id}/locations/{l}/securityProfiles/{p}` and `.../securityProfileGroups/{g}`) with its own server, since the project service's parses only project names | offered: GoogleCloudPlatform/k8s-config-connector#15289 |
| `0027-mockgcp-compute-routers-as-the-api.patch` | written here: `routers.get` returns each MD5 authentication key by name, without its value, and `routers.insert` and `routers.update` refuse more than one feature (interfaces, peers with keys, NATs) at a time, as the real API does | offered: GoogleCloudPlatform/k8s-config-connector#15286 |
| `0028-mockgcp-compute-organization-security-policy-rules.patch` | written here: `organizationSecurityPolicies.addRule`, `getRule`, `patchRule` and `removeRule` (rules by priority in the policy), and the beta-only fields of a FIREWALL rule (direction, logging, targets, destination ranges, layer 4 configs), which the v1 messages drop, kept beside the rule by the HTTP layer | the rule methods yes; the beta fields a stopgap |
| `0029-mockgcp-logging-billing-account-sinks.patch` | written here: log sinks under `billingAccounts/{id}`, with a writer identity of their own; `UpdateSink` takes `destination`, `disabled`, `exclusions`, `includeChildren` and `bigqueryOptions` (it took `description` and `filter` only); a folder parent's name fixed | yes |
| `0030-mockgcp-iam-principal-access-boundary-policies.patch` | written here, for `IAMPrincipalAccessBoundaryPolicy`: IAM v3 principal access boundary policies (create, get, list, update with a mask, delete; finished operations; etag), on REST paths added to the mux by hand since mockgcp has no v3 gateway | yes |
| `0031-mockgcp-securitycenter-mute-config-parents.patch` | written here: mute configs under `folders/{id}` and `projects/{id}` as well as `organizations/{id}` | yes |
| `0032-mockgcp-compute-network-firewall-policy-packet-mirroring-rules.patch` | written here: `networkFirewallPolicies.addPacketMirroringRule`, `getPacketMirroringRule`, `patchPacketMirroringRule` and `removePacketMirroringRule` (rules by priority in the policy's `packetMirroringRules`), in a new file; 0029 and 0030 are taken by open sibling pull requests | yes |
| `0033-mockgcp-compute-vpn-tunnels.patch` | written here: mockgcp serves `vpnTunnels` (insert, get, `setLabels` with the label fingerprint, delete); the shared secret is kept out of reads and its hash returned; VPN tunnel and HA VPN gateway writes answer with a regional operation (the gateways answered with a global one, which a regional client cannot wait on) | yes |
| `0034-mockgcp-serviceusage-public-services.patch` | written here: Service Usage knows every public Google API (the services of the pinned Discovery documents), so enabling iam, logging, monitoring, artifactregistry and the rest works in the fake; numbered after the patches open in #55 and #56 | yes |

## Pull requests we opened upstream

Our mockgcp patches offered upstream from the builder's fork (approved
2026-11-10), one pull request per patch, each tested with mockgcp's build,
vet and unit tests and upstream's related e2e fixtures against mockgcp.
Not offered: 0003 (upstream's own PR #12553), 0004, 0006, 0007, 0010 and
0015 (built on open upstream pull requests), 0016, 0019, 0020 and 0021
(built on our patches above; offer each when its base merges), 0028
(upstream is changing the same mock, PR #14796; its beta fields are a
stopgap), 0008 and 0018 (break upstream fixtures, see the patch table).
When one merges and the pin passes it, drop the patch.

| PR | Head | Patch | State (2026-11-10) |
| --- | --- | --- | --- |
| GoogleCloudPlatform/k8s-config-connector#15278 | `8dc3b755` | `0001-mockgcp-sql-users-database-roles.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15277 | `d84c6359` | `0002-mockgcp-listen-address-env.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15281 | `946eabfc` | `0005-mockgcp-artifactregistry-upstream-credentials.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15282 | `d053028c` | `0009-mockgcp-artifactregistry-tags.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15279 | `5544aa3e` | `0011-mockgcp-compute-region-network-endpoints.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15280 | `08965b0d` | `0012-mockgcp-compute-router-route-policies.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15290 | `eefae9c1` | `0013-mockgcp-storage-object-media.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15287 | `a8bf5103` | `0014-mockgcp-compute-region-network-firewall-policies.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15291 | `406951b3` | `0017-mockgcp-logging-settings.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15284 | `b3b495d9` | `0022-mockgcp-serviceusage-hierarchy-identity.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15288 | `b0d0a022` | `0023-mockgcp-compute-project-default-network-tier.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15283 | `368552c1` | `0024-mockgcp-iam-service-accounts-list-undelete.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15285 | `112e5e3a` | `0025-mockgcp-networksecurity-organization-address-groups.patch` | closed 2026-11-10: duplicate of #4711 |
| GoogleCloudPlatform/k8s-config-connector#15289 | `6e0ae4e4` | `0026-mockgcp-networksecurity-organization-security-profiles.patch` | open |
| GoogleCloudPlatform/k8s-config-connector#15286 | `0d5ce7b4` | `0027-mockgcp-compute-routers-as-the-api.patch` | open |

## Other sources consulted, not copied

| Source | Commit | Used for |
| --- | --- | --- |
| `github.com/googleapis/googleapis` | `731d7f2ab6e4e2ea15030c95039e2cb66174d4fb` (mockgcp's pin, `mockgcp/git.versions`) | the protos mockgcp is generated from |
| `github.com/googleapis/googleapis` | `bf54c1031efddde9cdfee0f89254315d0116a5f9` (`master`, 2026-11-07) | field numbers of `User.database_roles` (15) and `SqlUsersUpdateRequest.database_roles` / `revoke_existing_roles` (5, 6) |
| `github.com/googleapis/googleapis` | `1765b559c42386788ff0c6412491277b4791107a` (Config Connector's `apis/git.versions` at the pin) | the protos `hack/scaffold-kind.sh` generates from (`controllerbuilder`'s `generate-proto.sh`) |
| `github.com/hashicorp/terraform-provider-google-beta` | `42eb358f50f54450f5433a359517c42edf10e63f` (`main`, 2026-11-07), `google-beta/services/sql/resource_sql_user.go` | the behaviour of `database_roles` (MPL-2.0; read, not copied) |
| `github.com/hashicorp/terraform-provider-google-beta` | `24c6f5fd13ec6b5809877eb7804089dd0d1fe7a4` (`main`, 2026-11-09), `google-beta/services/resourcemanager/resource_folder_service_identity.go`, `resource_organization_service_identity.go` | the request path of a folder's and organization's service identity (MPL-2.0; read, not copied) |
| `github.com/hashicorp/terraform-provider-google-beta` | `24c6f5fd13ec6b5809877eb7804089dd0d1fe7a4` (`main`, 2026-11-09), `google-beta/services/resourcemanager/resource_google_project_default_service_accounts.go` | the behaviour of `ProjectDefaultServiceAccounts`: the defaults by display name, the three actions, the restore policies (MPL-2.0; read, not copied) |
| `github.com/GoogleCloudPlatform/magic-modules` | `aad42de6b6564e69c864647145979e60c9eaece1` (`main`, 2026-11-09), `mmv1/products/networksecurity/AddressGroup.yaml` | the behaviour of address groups: an organization or project parent, `capacity` and the parent immutable (Apache-2.0; read, not copied) |
| `github.com/GoogleCloudPlatform/magic-modules` | `aad42de6b6564e69c864647145979e60c9eaece1` (`main`, 2026-11-09), `mmv1/products/networksecurity/SecurityProfile.yaml`, `templates/terraform/custom_flatten/network_security_security_profile_url_filters_flatten.go.tmpl` | the behaviour of security profiles: an organization or project parent, `type` required and immutable, one profile, lists as sets, the API's default URL filter (Apache-2.0; read, not copied) |
| `github.com/GoogleCloudPlatform/magic-modules` | `aad42de6b6564e69c864647145979e60c9eaece1` (`main`, 2026-11-09), `mmv1/products/networksecurity/SecurityProfileGroup.yaml` | the behaviour of security profile groups: an organization or project parent, the parent, location and name immutable, no other field behaviour (Apache-2.0; read, not copied) |
| `github.com/GoogleCloudPlatform/magic-modules` | `aad42de6b6564e69c864647145979e60c9eaece1` (`main`, 2026-11-09), `mmv1/products/compute/Router.yaml`, `RouterNat.yaml`, `mmv1/third_party/terraform/services/compute/resource_compute_router_peer.go.tmpl`, `resource_compute_router_interface.go.tmpl` (read as data) | the behaviour of `ComputeRouter`: PATCH and send-empty fields, `bgp.identifierRange` defaulted by the API, the MD5 key value never read back, a peer's `enable` as `"TRUE"`/`"FALSE"`, an interface needing only an IP range |
| `github.com/GoogleCloudPlatform/magic-modules` | `aad42de6b6564e69c864647145979e60c9eaece1` (`main`, 2026-11-09), `mmv1/products/compute/NetworkFirewallPolicyPacketMirroringRule.yaml` | the fields of `ComputeNetworkFirewallPolicyPacketMirroringRule` (a beta resource there: match ranges and layer 4 configs, `mirror`/`do_not_mirror`/`goto_next`, `securityProfileGroup`, `targetSecureTags`, the priority and policy immutable; its `tlsInspect` the API refuses) and its four calls (Apache-2.0; read, not copied) |

## Watched upstream change

Upstream's own direct `SQLUser` is an open pull request. It is not copied:
at the head below it declares `databaseRoles` but leaves the field, its
diff and its mapping commented out (`/*NOTYET*/`), so it does not do what
we need. `hack/upstream-pr.sh` (`task upstream-pr`) fetches the PR's head
by `refs/pull/<n>/head`, says whether it moved, and whether `databaseRoles`
is live yet; when it is, follow "Switching to PR #7148" in `docs/design.md`.

| | |
| --- | --- |
| Watched PR | #7148 "feat(sql): migrate SQLUser from Terraform to Direct controller" |
| PR head | `eb5d176ab176d4afa5b7d1878d8f035281a7a072` |
| Read on | 2026-11-07 (open, review required, last updated 2026-06-04) |
| Verdict | not adequate: `databaseRoles` commented out |

Upstream's mockgcp organization address groups are an open draft pull
request, #4711 (its direct controller is draft #4719). We closed our #15285
as its duplicate (builder 2026-11-10: where upstream work is in progress,
close ours and follow it). Patch 0025 keeps carrying the feature here; when
#4711 merges and the pin passes it, drop 0025.

| | |
| --- | --- |
| Watched PR | #4711 "mockGCP: Add mocks for NetworkSecurityAddressGroup" |
| PR head | `efd3714a62b5264a530d98073ea6255a820ac971` |
| Read on | 2026-11-10 (open draft, review required, last updated 2025-06-22) |
| Verdict | follow; patch 0025 stays until it merges |

## Updating the pin

1. Change the "Commit" row (and date, release) above.
2. `hack/upstream.sh`. If a patch no longer applies, rebase it (or drop it
   if upstream took it) and regenerate it from `git diff` in the checkout.
3. `hack/check-go-mod.sh`; copy any changed `replace` pins from the
   checkout's `go.mod` into ours. Then `go mod tidy`.
4. Re-read the upstream files in the table above at the new commit and
   carry over their changes (`git diff <old>..<new> -- <path>` in a full
   clone); keep our changes as small as before.
5. `task check`.
6. If upstream now has a kind or field we fill, follow "Retiring a gap
   kind" in `docs/design.md`.

## Copied from open pull requests

Code copied from pull requests that have not merged. `hack/upstream-pr.sh`
reads this table and says, for each, whether its head moved since the row
was written and whether it merged. When one moves, re-read its diff for the
copied files; when it merges and the pin passes it, retire our copy
(`docs/design.md` section 3.4) and drop its mockgcp patch.

| PR | Head | Read on | Copied for |
| --- | --- | --- | --- |
| #12553 | `9611a6de02e08aca9a624affaf59bc104d1cb88f` | 2026-11-07 (open, review required, last updated 2026-09-04) | `SQLAdminBackup`, patch 0003 |
| #6651 | `9aab204dd9d87716863b966c724b7a12664fd85f` | 2026-11-07 (open, review required, last updated 2026-03-17) | regional `SecretManagerSecret` / `SecretManagerSecretVersion`, patch 0004 |
| #14417 | `9297537aa0479a6457f736f96101190cba876fcb` | 2026-11-09 (open, review required, no reviews) | one mockgcp line in patch 0008 (`invokerIamDisabled` kept on update); the field itself is written here on our `RunService` |
| #14418 | `b080f5dcfae5e5be5acd2361a7afd5c60e33568f` | 2026-11-09 (open, review required, no reviews) | `IAMWorkloadIdentityPoolManagedIdentity` types and identity; the mockgcp proto in patch 0007 |
| #14209 | `a5e8f5a9e3648a13d3346cfa8cf07cf26e64ec09` | 2026-11-09 (open, changes requested on its CI files, not on the types) | `IAMWorkloadIdentityPoolNamespace` types and identity; the mockgcp proto in patch 0006 |
| #7673 | `423fea9f0ba9d40ee7cd32d28cf197554f0983ee` | 2026-11-10 (open, review required, last updated 2026-05-05; its organization, folder and project parents never reached `master`, whose Security Command Center kinds take an organization only) | the parent of `SecurityCenterMuteConfig` (and the later Security Command Center kinds), `apis/securitycenter/v1alpha1/parent.go` |
| #13517 | `eb2774cef2776e8a1dc2f766d6cf491f0931f8b7` | 2026-11-08 (open, review required, no reviews, paused by upstream's bot for inactivity on 2026-10-13) | `ArtifactRegistryRepository` with `customRepository`; no mockgcp patch needed |
