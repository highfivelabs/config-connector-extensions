# The fake cloud

A Kubernetes cluster where Config Connector's resources, and ours, become
Ready against a fake Google Cloud, so a tool that applies Config Connector
resources can be tested end to end, reading their status back, without a
real project, credentials or cost. Proven 2026-11-07 on an Apple-silicon Mac (Docker 20.10, kind
0.33).

```sh
task upstream          # once: the pinned Config Connector checkout
task fake-cloud-up     # kind cluster, CRDs, bin/fakecloud on the host
task fake-cloud-test   # SQLInstance, SQLDatabase, IAMServiceAccount, SQLUser: all Ready, then all deleted
task fake-cloud-down   # stop it and delete the cluster
```

`export KUBECONFIG=bin/fake-cloud/kubeconfig` to use the cluster; the
manager's log is `bin/fake-cloud/fakecloud.log`.

## How it works

Config Connector has no switch that points a deployed manager at mockgcp:
mockgcp is an in-process `http.RoundTripper` and gRPC server, not a network
endpoint, and Config Connector's own e2e tests run it in mock mode by
building the manager in the test process
(`config/tests/samples/create/harness.go`). `cmd/fakecloud` does the same
outside a test:

- `kccmanager.New`, Config Connector's real manager with every controller
  (direct, Terraform-based and DCL-based), given mockgcp as its HTTP client
  and gRPC connection, the same hooks the harness sets (including the
  Terraform provider's HTTP client transformers);
- this project's controllers (`SQLUser` with `databaseRoles`) in the
  same manager;
- Config Connector's deletion defender, which in a real install runs as
  its own process (`cmd/deletiondefender`); without it Config Connector's
  kinds never finish deleting, because their reconcilers wait for it to
  drop its finalizer. Ours have their own defenders in the manager;
- mockgcp's in-memory store, with the projects named by `--projects`
  (default `mock-project`) created at start.

It runs on the host against the kind cluster's API server. The cluster
holds only CRDs: by default the `sql`, `iam`, `resourcemanager` and
`compute` groups (105 CRDs); `CRD_GROUPS=all` installs all ~800.

## Timings (2026-11-07)

| Step | Time |
| --- | --- |
| `go build ./cmd/fakecloud`, first time (Config Connector compiles with it) | about 3 min of CPU, 25 s wall on 6 cores |
| `task fake-cloud-up`: kind cluster, 105 CRDs, manager started | 41 s |
| `task fake-cloud-test`: 4 resources applied and Ready | 2 s |
| `task fake-cloud-down` | about 2 s |

## What does not work, or differs from the real thing

- **The fake cloud forgets.** Its store is in memory: restarting
  `bin/fakecloud` loses every fake resource, while the Kubernetes objects
  remain and are recreated in the fake cloud on their next reconcile.
- **No Config Connector webhooks.** The admission webhook that copies a
  namespace's `cnrm.cloud.google.com/project-id` annotation onto each
  resource does not run, so every resource must carry the annotation
  itself (`hack/fake-cloud/sample.yaml` does). Defaulting and
  validation webhooks are absent too.
- **mockgcp's limits.** Cloud SQL knows Postgres up to 16, not 17 (the
  sample uses 16), and does not refuse a user on an
  instance that does not exist yet, so our `SQLUser` can be Ready before
  its `SQLInstance` (upstream's ref resolver does not wait on readiness
  either; real Cloud SQL would fail and retry). Services mockgcp does not
  implement fail as they would against an unknown host.
- **Artifact Registry in mockgcp.** It stores a repository as sent, so a
  remote repository's `customRepository` round-trips, but it does not
  reach the upstream: the real API refuses, at creation, an upstream it
  cannot reach ("Failed to validate remote upstream", seen live) unless
  `disableUpstreamValidation` is set. Its update accepts only
  `description`, `labels` and the cleanup policies; changing
  `remoteRepositoryConfig` (for example the custom upstream's URI) is
  refused, and patch 0016 gives the real API's error for it ("Remote
  source configuartion cannot be updated", seen live). Patch 0005 lets
  it change a remote repository's upstream username and password version
  alone, as the real API and Terraform do; it never reads the secret
  version, so a password that does not exist or cannot be read by
  Artifact Registry's service agent passes in the fake and fails in the
  real cloud.
- **IAM policies in mockgcp** (patch 0018, ours, for `IAMBinding`).
  Every `:getIamPolicy` and `:setIamPolicy` is served from one store keyed
  by host and path, for any resource name, existing or not: a policy on a
  secret that does not exist reads as empty and can be set. The etag is
  checked on write (409); permissions are not, nor are the members' forms
  (`user:` with no address is accepted) or whether a role exists.
  Cloud Storage's own `.../iam` paths are served from the same store.
  Compute Engine's `GET .../getIamPolicy` and `POST .../setIamPolicy`
  (patch 0019, ours, for `IAMBinding` and `IAMMember`) are served
  from it too. The store's etag check and write are atomic (patch 0019; before it, two
  writers that read the same policy both passed the check and the later one
  replaced the earlier). What the store cannot show: a role that does not apply to
  the resource (`roles/viewer` on a managed folder), a member the
  resource's organization refuses, a managed folder that does not exist,
  and each service's own choice of GET or POST for the read, its API
  version and its etag error (409 or 412), which is why the rows of the
  resource table beyond the three with a live run are unproven.
- **Global network firewall policy rules in mockgcp** (upstream's,
  unpatched, for `ComputeNetworkFirewallPolicyRule`). `getRule`,
  `addRule`, `patchRule` and `removeRule` by priority; a priority with no
  rule answers the real API's 400 "does not contain a rule at
  priority N". What it cannot show is what the regional rules' note
  below says of them: `addRule` does not refuse a priority in use, no
  rule's fields are checked (including `srcNetworkContext` against
  `srcNetworks`), `patchRule` replaces the rule whole where the real API
  keeps a list the body leaves out (and a network context sent as `""`;
  the controller sends `UNSPECIFIED`), and no rule is ever "not ready".
- **Regional network firewall policies in mockgcp** (patch 0014, adapted
  from its global ones; their rules are patch 0020). The fingerprint is
  not checked on patch, a policy is deleted even when associated, and
  associations are patch 0021. A rule is stored as sent: mockgcp
  checks none of the API's rules about a rule (a `goto_next` rule with
  logging, secure tags together with target service accounts, a layer 4
  protocol with ports it cannot have, FQDNs and region codes on the wrong
  direction, the policy's limit on rule tuples), and it reports a rule
  tuple count of 2 for every rule, whatever it matches. Its `patchRule`
  replaces the whole rule; the real API keeps a list the body leaves out
  (the controller sends `[]` for it), and answers 400 "The rule at
  priority N is not ready" to a patch soon after another change, which
  the fake never does.
  An association (patch 0021) is stored as sent: mockgcp does not check
  that the network exists, nor that a network has no policy attached
  from another policy (it refuses only a repeated name or network inside
  one policy), nor what the API fills in itself.
- **Folder and organization service identities in mockgcp** (patch
  0022, ours). Any folder or organization ID is accepted, and every
  service gets an identity of the form
  `service-folder-{id}@gcp-sa-{service}.iam.gserviceaccount.com` (or
  `service-org-...`); the real API gives one only to services that have a
  service agent at that level, and some agents have other domains. Not
  yet compared with the real API: its live run waits for the test folder.
- **Service accounts listed, undeleted and named by unique ID in
  mockgcp** (patch 0024, ours). `list` returns one page; a deleted
  account is kept for ever (the API keeps it 30 days, and undelete can
  fail for other reasons); `undelete` takes only a unique ID and refuses
  an account whose email is in use again. The fake project has no default
  accounts: the test makes accounts with the defaults' display names
  (their emails are not the real defaults' `{number}-compute@developer...`
  and `{project}@appspot...`). mockgcp keeps an IAM policy per request
  path, so a project's policy through cloudresourcemanager v1 and v3 are
  two policies there (one in the real API); the test uses v3, as the
  controller does. Not yet compared with the real API: its live run waits
  for a test project.
- **Organization address groups in mockgcp** (patch 0025, ours). An
  organization's address groups are served by the project service's
  server, which takes any parent: mockgcp does not check that the
  organization exists or that the caller may use it, nor the API's
  limits (a capacity it refuses, more items than the capacity, an item
  that is not a range of the group's type), nor that a group is in use
  by a firewall rule when it is deleted. It answers a capacity change
  like any other update; the CRD refuses one first (immutable, as
  Magic Modules marks it). A project's address group runs live
  (`docs/live-runs.md`); an organization's is not yet compared with the
  real API, since a live run never touches the organization.
- **Organization security profiles in mockgcp** (patch 0026, ours). An
  organization's security profiles and profile groups are served by a
  server of ours with its own name parsing (the project service's
  parses only project names and looks the project up). Neither mockgcp
  server checks that the organization exists or that the caller may use
  it, that `type` matches the profile set, that a threat ID or URL
  matcher is valid, or that a profile is in use by a group when it is
  deleted; an update replaces the whole profile, whatever the update
  mask; and neither adds the default URL filter (priority 2147483647)
  that Terraform's flattener says the real API adds to a URL filtering
  profile, so on the fake the comparison that ignores it is not
  exercised; live, a resync of a URL filtering profile writes nothing.
  Neither checks that a group's profile exists, is of the right type, or
  is under the same parent. A project's profile and group run live
  (`docs/live-runs.md`); an organization's are not yet compared with the
  real API.
- **IAM principal access boundary policies in mockgcp** (patch 0030,
  ours, for `IAMPrincipalAccessBoundaryPolicy`). Create, get, list,
  update with a mask and delete under any organization and location;
  the REST paths are added by hand (mockgcp has no IAM v3 gateway), and
  every write answers a finished operation. It sets
  `enforcementVersion` to `1` when unset (the real API's latest version
  is assumed), checks no permissions, and stores any rule: no check of
  the resource names, the 500-resource limit or the effect.
- **IAM deny policies in mockgcp** (upstream's, unpatched, for
  `IAMDenyPolicy`). It serves create, get, list, a whole-policy update
  that must carry the current etag, and delete, under a project's number
  as the API answers, and a folder's or organization's under any ID. It
  checks nobody's permissions (the API needs Deny Admin, granted on an
  organization), and none of the rule's contents: any principal or
  permission string and any `denialCondition` is stored. Nothing it
  stores is enforced.
- **DNS response policies in mockgcp** (upstream's, unpatched, for
  `DNSResponsePolicy`). It serves create (a new numeric ID), get, a
  whole-policy update and patch (both replace the stored policy with the
  body: a patch of `networks` alone drops the description and labels,
  where the real API keeps them), and delete. It does not check that a
  bound network or GKE cluster exists, does not refuse to delete a
  policy still bound to a network (upstream's controller unbinds first,
  as ours does), and its list answers empty. Labels are stored as sent.
- **Packet mirroring rules of a global network firewall policy in
  mockgcp** (patch 0032, ours, for
  `ComputeNetworkFirewallPolicyPacketMirroringRule`). It serves add, get,
  patch and remove by priority in the policy's `packetMirroringRules`; a
  missing priority is a 400 worded after the firewall rules' answer, not
  seen from this API; a patch replaces the rule whole, where the real API
  is assumed to keep lists the body leaves out (the controller sends
  `[]`). It refuses `tlsInspect`, even `false`, as the real API did in a
  live run, and an `INGRESS` rule that names no source (no `srcIpRanges`
  or other `src*` matcher), with the real API's 400 (run
  `cce-11120814-5d2a`). Otherwise it checks nothing in the rule: a `mirror` rule's security
  profile group need not exist, and the API's other rules for which
  matchers and directions a packet mirroring rule takes are not modelled. It adds
  no default packet mirroring rules to a new policy.
- **Organization security policies in mockgcp** (upstream's,
  unpatched, for `ComputeOrganizationSecurityPolicy` and its
  association). It serves insert (a new numeric ID, the parent as sent,
  a default rule), get, list, a patch of the description only, delete,
  and add, get and remove of an association. It checks no permissions
  on the parent or the attachment, does not refuse a second association
  to the same organization or folder, and does not default `type`.
- **A billing account's log sinks in mockgcp** (patch 0029, ours, for
  `LoggingBillingAccountSink`). Sinks under `billingAccounts/{id}`, with
  a writer identity the fake makes up
  (`serviceAccount:service-billing-{id}@gcp-sa-logging.iam.gserviceaccount.com`;
  the real address has not been seen). `UpdateSink` applies the mask's
  `description`, `filter`, `destination`, `disabled`, `exclusions`,
  `includeChildren` and `bigqueryOptions`, and refuses any other path. It
  checks no billing account (any ID is one), no destination (a topic or
  bucket that does not exist is accepted) and no filter, does not
  require `uniqueWriterIdentity`, and keeps no exclusion's timestamps.
- **Security Command Center mute configs in mockgcp** (patch 0031, ours,
  for `SecurityCenterMuteConfig`). A folder's and a project's beside an
  organization's, each stored and named under its parent as given (a
  project by the ID it was named with; whether the real API answers
  with its number is not known). It checks no parent (any folder or
  organization ID is one), no filter syntax, and no expiry of a dynamic
  mute config.
- **Spanner instance configurations keep labels** (patch 0036, ours, for
  `SpannerInstanceConfig`). Create keeps the labels and leader options
  it is sent, as the API does. mockgcp still checks nothing about the
  replicas (that they hold the base configuration's and add one of its
  optional replicas), fills in no leader options, and answers every
  configuration as `READY` at once.
- **VPN tunnels in mockgcp** (patch 0033, ours, for `ComputeVPNTunnel`).
  Insert, get, `setLabels` (refused on a stale label fingerprint) and
  delete of `projects/{p}/regions/{r}/vpnTunnels/{t}`; insert leaves
  the labels out, as the API does (live run `cce-11101009-a46a`); a read never
  returns the shared secret, only a hash of it, and a new tunnel reads as
  `FIRST_HANDSHAKE`. It checks only that the secret is set and that
  exactly one of the HA and Classic gateways is named: not that the
  gateways, the peer or the router exist, are in the tunnel's region or
  network, or that the interfaces match; not that the peer's address is
  public (the API refuses a reserved one, RFC 5735); not the cipher suite's
  algorithm names or combinations; and it keeps the references as sent
  (the real API answers with full URLs). The patch also makes HA VPN
  gateway writes answer with a regional operation, as the API does:
  upstream's mock answered with a global one, which the regional client's
  `Wait` could not find (the test's gateway failed with a 404 on it).
- **Organization security policy rules in mockgcp** (patch 0028, ours,
  for `ComputeOrganizationSecurityPolicyRule`). `addRule`, `getRule`,
  `patchRule` and `removeRule` by priority in the policy's rules;
  `addRule` refuses a missing priority or action and a priority in use;
  a priority with no rule answers 400 "The security policy does not
  contain a rule at priority N" (the firewall policies' wording, assumed
  here); `patchRule` replaces the rule whole. The beta-only fields of a
  FIREWALL rule (direction, logging, targets, destination ranges, layer
  4 configs), which the fake's v1 messages drop, are kept beside the
  rule by the HTTP layer and put back into `getRule` and the policy's
  rules. It does not refuse a FIREWALL rule on a Cloud Armor policy or
  the reverse, checks no field's values (a protocol, a range, a target
  network), computes no `ruleTupleCount`, and adds Cloud Armor's default
  rule (priority 2147483647) to every new policy, whatever its type.
- **Cloud Routers in mockgcp** (patch 0027, ours, for `ComputeRouter`).
  `routers.get` returns an MD5 key by name only, as the real API does;
  the fake keeps the value, so a test cannot see what value was sent
  (the real API cannot be asked either). `routers.insert` refuses a
  router with more than one of interfaces, BGP peers, NATs and MD5 keys,
  with the API's own 400 (seen live with all four); which pairs the real
  API allows is not known, so the fake refuses any two; `routers.update`
  likewise refuses a change of more than one of the interfaces, the
  peers with the keys, and the NATs (the API's "Updating multiple Router
  features is currently unsupported"; whether the base fields or the
  keys alone count as features is not known, and a live `routers.patch`
  that added an interface and a peer together was accepted, so the real
  rule is narrower than the fake's; the controller never sends two). Otherwise `routers.update`
  stores the body as sent: it does not check that a peer's interface,
  its MD5 key name, or a key's single use exist; it does not fill in a
  peer's `ipAddress`, BFD, `enable` or `managementType`, a NAT's
  timeouts, port counts or `endpointTypes`, or `bgp.identifierRange`, so
  the comparison that leaves such API defaults alone is exercised only
  live; it never makes a peer or interface `MANAGED_BY_ATTACHMENT`, so
  keeping those on update is not exercised at all; and it does not check
  that one network and region has at most one NAT for all subnetworks.
  Its `routers.patch` appends to `bgp.advertisedIpRanges` and the MD5
  keys rather than replacing them (proto merge); the controller writes
  with `routers.update`, so only a test's own patch meets that.
- **A project's default network tier in mockgcp** (patch 0023, ours).
  `setDefaultNetworkTier` takes `PREMIUM` or `STANDARD` and refuses
  anything else with a message of our own; what the real API answers for
  `FIXED_STANDARD` is not known (the CRD refuses it first). Not yet
  compared with the real API: its live run waits for a test project.
- **Log Router settings in mockgcp** (patch 0017, ours). Any
  organization or folder ID is accepted, the key is not checked, and the
  service account IDs only follow the documented form.
- **Cloud Storage objects in mockgcp** (patch 0013, ours). Before it,
  objects were stubs. Uploads are media or multipart, not resumable; an
  object's ACLs, holds, retention and storage class are not modelled.
- **Router route policies in mockgcp** (patch 0012, ours). The CEL in
  terms is stored, not parsed, and the fingerprint is not checked on write.
  A missing policy is answered with the real API's 400 ("The policy does
  not exist"), not a 404, as a live run showed.
- **Regional network endpoints in mockgcp** (patch 0011, ours). Any
  endpoint is accepted in any group: the group's type (internet IP, domain
  name, VM) is not checked against the endpoint, nor is the VM.
- **Artifact Registry tags in mockgcp** (patch 0009, ours). A tag must be
  in an existing repository; the fake holds no packages, so a tag on a
  version that was never pushed is accepted.
- **Workload identity pool namespaces in mockgcp** (patch 0006, ours). It
  has no workload identity pools, so a namespace is created whether or not
  its pool exists; the real API refuses one in a missing pool. Delete is a
  soft delete, as documented, and the ID cannot be reused; the fake never
  purges it. It never sets `ownerService`.
- **Workload identity pool provider keys in mockgcp** (patch 0010, ours).
  The public key is a placeholder PEM, not a certificate, and its validity
  window is made up; the provider is not checked.
- **Managed identities in mockgcp** (patch 0007, ours). One must be in an
  existing, active namespace. An attestation rule's `googleCloudResource`
  is only checked to be non-empty; the real API checks its form and
  refuses one naming a project outside the pool's organization (a live
  run), so against a real project the test names the test project.
- **Cloud Run services in mockgcp.** Patch 0008 makes `services.patch`
  honour an update mask and give a container the API's default port and
  startup probe, as a live run showed. The fake still fills in fewer
  server defaults than the real API (no traffic), does not schedule
  anything, so a GPU `nodeSelector` is stored but never checked against
  the region's GPUs or quota, and `invokerIamDisabled` changes nothing it
  serves.
- **Found by live runs** (`docs/live-runs.md`). Where mockgcp and the
  real API were seen to differ, the fake was patched to match: a deleted
  Cloud SQL backup run is kept with status `DELETED` (patch 0015), and
  the Artifact Registry error above (patch 0016), a missing route
  policy's 400 (patch 0012), and a Cloud Run container's default port and
  startup probe (patch 0008). The same tests run
  against a real project with `task test-live`.
- **Not in the cluster.** The manager runs on the host, not as a pod; a
  Target's agent in the cluster cannot reach it. An in-cluster image is
  the way to make the fake cloud reachable from inside: `cmd/fakegcp` and
  `config/fake-cloud/`, used by `task operator-e2e`.
- **Not the operator.** Config Connector's operator (`ConfigConnector`,
  `ConfigConnectorContext` installation, per-namespace managers) is not
  run; the manager watches the whole cluster. `task operator-e2e` covers
  the operator (`docs/design.md` section 9).

## Found on the way

Google requires a service account ID of 6 to 30 characters, and mockgcp
refuses a shorter one as real Google Cloud would; the sample uses
`app-runtime`.

## The generated kinds' fake (`pkg/discokind/fake`)

The kinds `hack/discokind` generates are not served by mockgcp but by
`pkg/discokind/fake`, an in-memory store in front of it (in
`pkg/test/mockenv` and both fake-cloud binaries) that serves every such
kind's collection from the kind's descriptor: create with the ID
parameter (409 when it exists), get, list, patch by update mask, delete
(or, for a soft-deleting API such as OAuth clients, the deleted state),
and finished operations where the API answers one. It sets `name`,
`createTime`, `updateTime`, `uid` and `etag` when the API has them, and a
kind's `fakeDefaults`. It does not check required fields, enum values,
cross-field rules or permissions, and fills in no other defaults: that is
the live run's job (`docs/testing.md`).

A custom method on an object (`name:verb`) is not the collection's: the
fake passes it to mockgcp, which serves `:getIamPolicy` and
`:setIamPolicy` for every resource (patch 0018). Before 2026-11-13 the
fake took `.../lakes/l:getIamPolicy` for an object called
`l:getIamPolicy` and answered 404, so `IAMBinding` and `IAMMember` on
Cloud Build v2 connections, API Gateway configs and gateways and the
Dataplex kinds never became Ready on the fake cloud
(`TestDiscokindPolicy*` in `pkg/controller/direct/iam`).

`NetworkServicesGateway` (2026-11-13): the fake keeps what a gateway's
spec says and checks none of what the real API does for a Secure Web
Proxy: that the network, the subnet, an active proxy-only subnet in the
region and the gateway security policy exist, that `routingMode` is set,
and which fields go with which type. It makes no Cloud Router or Cloud
NAT, and a gateway is ready at once, where a real one takes minutes. The
live run shows these.

## Service Management's fake (`pkg/controller/direct/servicemanagement/fake`)

mockgcp does not serve Service Management (`servicemanagement.googleapis.com`),
which `EndpointsService` calls, so a small in-memory fake does, in front of
mockgcp and behind the generated kinds' fake (in `pkg/test/mockenv` and both
fake-cloud binaries). It serves services (create, get, list, delete,
undelete), `configs:submit`, configurations (get, list), rollouts (create)
and the active configuration (`services/{name}/config`); every operation is
finished at once. A deleted service answers 404 and holds its name (a
create answers 409) until it is undeleted, as the API keeps a deleted
service for 30 days. A configuration's ID is `<date>r<n>`, its `title` is
the OpenAPI document's `info.title` and its `endpoints` are the document's
`x-google-endpoints`. It does not show: the API's reading of the
configuration (no OpenAPI or gRPC validation, no check that the document's
`host` is the service's name, no `apis`), DNS, permissions, rollouts that
split traffic (it takes one configuration at 100%), and how long a rollout
takes. A new fake was chosen over a mockgcp patch because mockgcp has no
Service Management protos or service to extend, as the generated kinds'
fake was. As there, a custom IAM method on a service (`:getIamPolicy`,
`:setIamPolicy`, `:testIamPermissions`) goes on to mockgcp, which serves
it for every resource (patch 0018).
