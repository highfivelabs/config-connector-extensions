# Handoff

This is where an agent that is new to the repository starts. It says what
is done, what is left (in order), and the working conventions that are not
obvious from the code. Written 2026-11-09 by the agent that built the
seed-catalog kinds (`docs/design.md` 6.5), updated by the ones that built
`IAMRoleMember` (now `IAMMember`, design.md decision 12), the folder and
organization `ServiceIdentity`, `ProjectDefaultServiceAccounts`,
`NetworkSecurityAddressGroup`, the security profile kinds, `ComputeRouter`,
`IAMDenyPolicy`, `ComputeOrganizationSecurityPolicyRule` and
`DNSResponsePolicy`,
`ComputeNetworkFirewallPolicyPacketMirroringRule` and `ComputeVPNTunnel`. Read
`CLAUDE.md`, `README.md`, `docs/design.md` and `docs/live-runs.md` first;
this file only adds to them.

## Where things stand

After the VPN tunnel PR (foundation item 10's `ComputeVPNTunnel`, the
first field gap under decision 19): thirty-eight kinds (`docs/kinds/`,
`task kinds`),
mockgcp patches 0001 to 0035 (`UPSTREAM.md` and `upstream.yaml`, which
must agree: `task upstream:yaml-check`; every patch is numbered, taking the next number from `main`, since `task
upstream:yaml-check` reads only numbered names: design.md decision 29,
the builder, 2026-11-13), the live-test switch and the Taskfile (`docs/live-runs.md`,
factory's `taskfiles/README.md` for the shared `go:*` tasks; use `task`,
not `make`). Every built kind passes
live but `LoggingSettings`, `ServiceIdentity`,
`ComputeProjectDefaultNetworkTier` and `ProjectDefaultServiceAccounts`,
which wait on the test folder, as do `ComputeOrganizationSecurityPolicy`,
its association and its rule (organization and folder only), and `IAMDenyPolicy`,
which waits on Deny Admin (granted on an organization; `E2E_IAM_DENY_ADMIN=true`),
and `LoggingBillingAccountSink`, which waits on billing account access
(`E2E_BILLING_ACCOUNT`),
and `DNSResponsePolicy`, which waits on Cloud DNS in `$GCP_PROJECT`
(`dns.googleapis.com` and `roles/dns.admin`; `E2E_DNS_ADMIN=true`).

`ComputeRegionNetworkFirewallPolicyRule` (rank 6) passes live. What the
real API did that the fake cannot show is in design.md 6.5 and
`docs/fake-cloud.md`: a missing priority is a 400, `patchRule` keeps a
list the body leaves out (so the controller sends `[]` through the
Compute Engine REST client), and a just-changed rule is "not ready".
It adds `ComputeRegionNetworkFirewallPolicyRef`, which the association
kind reuses. Config Connector's reconciler watches a missing reference
with a watch that outlives the object and keeps kube-apiserver from
stopping in an envtest: a test that leaves a reference missing at its
end fails with "timeout waiting for process kube-apiserver to stop";
make the referenced object in the test.

`task operator-e2e` goes through `hack/operator-e2e.sh`, which fails
unless `tests/operator/run.sh` is intact (more than 200 lines, errexit,
the closing `echo "all checks passed"`), exits 0 and ends with that line.

**Generating a kind from Discovery (`discokind`, first choice).** If the
gap is a standard collection (create with an ID query parameter, get,
delete, optionally patch with an update mask), add it to
`hack/discokind/kinds.yaml`, run `go run ./hack/discokind` and
`hack/generate.sh`, and write its two fixtures
(`pkg/controller/direct/<service>/testdata/discokind/<Kind>/create.yaml`,
`update.yaml`; `live-skip` for organization-level kinds). No controller,
mockgcp patch, test code or `controllers.go` edit; several kinds per pull
request. `docs/throughput.md` says why and what it reaches (178 of the
494 whole-resource gaps), `docs/testing.md` how such a kind is tested.
Run one kind live with `LIVE_RUN=TestLoggingSavedQuery task test-live
PROJECT=$GCP_PROJECT KINDS=logging`.

**Generating a kind.** `hack/scaffold-kind.sh` runs Config Connector's
`controllerbuilder` (types, mapper, controller) on an offload VM and brings
the files back as the run's artifacts (`gcloud storage cp -r
$OFFLOAD_BUCKET/runs/<run>/offload-artifacts/scaffold <dir>
--project $OFFLOAD_PROJECT`); its header says how. It was first used for
`ServiceIdentity`. What it taught: a proto without `field_behavior` puts
output fields in the spec, so field behaviour goes in
`hack/scaffold/<service>/overlay.proto` (from Magic Modules' product YAML
where one exists: `docs/prior-art.md` in `highfivelabs/factory`, 6.1);
a proto without `google.api.resource` gets a `projectRef` and `location`
to replace by hand; and `generate-controller` writes nothing for a service
upstream already has a controller package for. Report the generated share
in each PR.

## What is left, in order

**Split between two GCP agents (the coordinator, 2026-11-10).** This
queue: `ComputeNetworkFirewallPolicyRule`, `IAMPrincipalAccessBoundaryPolicy`,
`DNSResponsePolicy` and its rule, then packet mirroring rules. The other
GCP agent: `LoggingBillingAccountSink` (15), Security Command Center (16)
and field gaps (decision 19). Coordinate through `main`: merge `main`
before each check run.

The queue is `docs/coverage/foundation.md` (the priority:
platform foundations first), not the older rank lists. After the
folder and organization `ServiceIdentity` (built; live pending the test
folder), the default network tier and the default service accounts
(built: `ComputeProjectDefaultNetworkTier`, `ProjectDefaultServiceAccounts`;
live pending a test project under the test folder):
organization address groups and security profiles, and so on down the
report. Its four Decision items are answered (design.md
decisions 13 to 16, 2026-11-09): alpha-only foundation kinds get our own
stable kinds (13: `IAMDenyPolicy`, `ComputeOrganizationSecurityPolicyAssociation`,
`ComputeNetworkFirewallPolicyRule`, `IAMPrincipalAccessBoundaryPolicy`,
`DNSResponsePolicy` and its rule); Cloud Router is a copy of upstream's
PR #13960 as a complete kind of ours (14); `LoggingBillingAccountSink`
now, live pending billing-account access (15); Security Command Center
kinds with organization, folder and project parents (16). Decision 5
(field gaps wait for the release) was reversed on
2026-11-10: decision 19, build foundation field gaps now in
`docs/coverage/foundation.md`'s order and keep up with upstream
mechanically. Whole-resource gaps first; any other field gap that needs the builder goes to section 10
with a recommendation. Immutability is CEL in the CRD, never a webhook (decision 20). The
builder has folderAdmin and another agent is making the `live-tests`
folder and the test estate's first generation project; keep folder-level kinds "live:
pending" until that agent reports. A deprecation path for our kinds is a
separate design; do not build it. Organization custom IAM roles (foundation item 3) are
not a gap: Config Connector's `IAMCustomRole` takes the
`cnrm.cloud.google.com/organization-id` annotation (the item says where). Org-level live tests wait for the test folder:
"live: pending, needs test folder"; never create folders, grant
organization-level IAM or touch the organization.

`ProjectDefaultServiceAccounts` (foundation item 1's last part) is
built (design.md 6.5; patch 0024); live pending a test project under the
test folder. Item 6 is built: `NetworkSecurityAddressGroup`,
`NetworkSecuritySecurityProfile` and `NetworkSecuritySecurityProfileGroup`
(design.md 6.5, patches 0025 and 0026; a project's of each passes live,
an organization's is "live: pending, needs test folder"). Decision 14
is built: `ComputeRouter` (design.md 6.5, patch 0027), and
`ComputeRouterRoutePolicy.routerRef.name` finds it (design.md section 7
item 28; a name both our and Config Connector's `ComputeRouter` have is
refused; a CEL rule allows one link per interface). Decision 13 has
begun: `IAMDenyPolicy` (`iam.gcp.hfl.dev/v1beta1`; stable kinds go in
`v1beta1`, which the charts allow), `ComputeOrganizationSecurityPolicy`
and `ComputeOrganizationSecurityPolicyAssociation` and
`ComputeOrganizationSecurityPolicyRule` and
`ComputeNetworkFirewallPolicyRule` (`compute.gcp.hfl.dev/v1beta1`)
and `IAMPrincipalAccessBoundaryPolicy` (`iam.gcp.hfl.dev/v1beta1`, patch
0030; live pending organization-level access) and `DNSResponsePolicy`
(`dns.gcp.hfl.dev/v1beta1`) are built. `IAMBinding`
moved to `v1beta1` (decision 24, K12, for the catalog's
0.4.0 charts); `v1alpha1` is no longer served (section 10, question 4)
and `CHANGELOG.md` says how to upgrade. Next the rest of decision 13:
`DNSResponsePolicyRule` (upstream has its types,
identity, reference and fuzzer but a Terraform-only controller, and
mockgcp serves no rules: write the controller after the policy's and add
a mockgcp patch; the sweeper must delete a policy's rules before it),
then 16 (Security Command Center, three parents). Packet mirroring
rules: the global network policy's is built
(`ComputeNetworkFirewallPolicyPacketMirroringRule`, patch 0032; live
passes but for the patch, design.md section 7 item 37: rerun
`task test-live PROJECT=$GCP_PROJECT KINDS=compute` and read the
patch subtest's timeout message first); the
hierarchical policy's has rule calls in the beta API only (live pending
the test folder) and the regional policy's has none in any API version.
Then on down `docs/coverage/foundation.md`, with the
field gaps (decision 19). 15 is built: `LoggingBillingAccountSink`
(design.md 6.5, patch 0029), live pending billing account access.

What `LoggingBillingAccountSink` taught: upstream's direct kinds often
already carry the identity format for a parent their spec lacks (the
log sink's identity parses `billingAccounts/...`); copy the kind and add
the reference rather than writing one. Reuse upstream's leaf types
(destination, exclusions) by import, so a later upstream fix reaches
ours. mockgcp's log sink update took `description` and `filter` only;
patch 0029 adds the rest.

What `IAMPrincipalAccessBoundaryPolicy` taught: a service mockgcp has
no generated HTTP gateway for (IAM v3) can still be served: register the
`cloud.google.com/go` gRPC server and add its REST paths to the mux with
`mux.HandlePath`, calling the server's methods (patch 0030,
`mockiam/principalaccessboundarypolicies.go`). A patch that edits a file
earlier patches edit (`mockiam/service.go`: 0006, 0007, 0010) must be
diffed against the file with those applied, not against the pin.

Decision 16 has begun: `SecurityCenterMuteConfig` (design.md 6.5, patch
0031) takes an organization, folder or project. Upstream PR #7673 has
the three parents for mute configs, BigQuery exports and custom modules
but predates `master`'s kinds: copy `master`'s kind and #7673's parent
(`apis/securitycenter/v1alpha1/parent.go`, `ResolveParent`), as the
mute config did. Next: `SecurityCenterBigQueryExport` (mockgcp already
serves all three parents), the two custom module kinds, then
`SecurityCenterNotificationConfig` (Terraform-based upstream). Live
needs Security Command Center active for the project
(`E2E_SECURITY_CENTER=true`). A new API group also goes in the manager's
ClusterRole (`config/operator/rbac.yaml`, both rules);
`TestManagerRoleCoversEveryKind` fails without it.

Field gaps (design.md decision 19, the 2026-11-10 reversal of
decision 5) are not begun. Take them in `docs/coverage/foundation.md`'s
order (items 9 to 19 and 21 to 27 that say "field"), with the attribute
lists in `docs/coverage/attributes.csv`. The accepted pattern is a
parallel kind of ours that owns the whole object (decisions 11, 13 and 14).
It suits small kinds like the log bucket and the sink fields (item 12) or
the DNS policy fields (item 15). It is costly for the big Terraform-based
kinds (`ContainerCluster`, `ComputeVPNTunnel`, `KMSCryptoKey`). The first
field-gap PR that hits one of those should propose another pattern in a
short design note, as decided, for example a field kind that
patches only its fields with an update mask beside Config Connector's
object, and why. Decision 11 rejected that for `RunService`, because
Terraform's whole-object update resets the fields.

What `DNSResponsePolicy` taught: a new group (`dns.gcp.hfl.dev`) is a
`groupversion_info.go` and `doc.go` copied, the kind added to
`pkg/controller/controllers.go` (both lists), and the group added to the
manager's ClusterRole (`config/operator/rbac.yaml`, both rules; without
it the operator's manager cannot read the kind, which only
`task operator-e2e` shows); the generator picks up the rest. Upstream's `NoProto` fuzzers (for kinds on the
`google.golang.org/api` REST types) run as a test the same way as the
proto ones (`FuzzSpec` and `FuzzStatus`). Upstream's direct DNS kinds
leave `labels` out; ours takes `metadata.labels`
(`label.NewGCPLabelsFromK8sLabels`), so live objects carry the run's
labels with no spec field. Check what the live project has before
writing a live test: `gcloud projects get-iam-policy` and `gcloud
services list` (both read-only) showed no DNS API or role, so the test
and sweeper sit behind `E2E_DNS_ADMIN`.

What `ComputeNetworkFirewallPolicyPacketMirroringRule` taught: read the
Discovery documents for which calls exist where
(`.cache/coverage/discovery/compute.{v1,beta,alpha}.json`) before taking
a queue item's list of kinds as buildable. When the pinned
`google.golang.org/api` client lacks a call and the generated
`cloud.google.com/go` client cannot send an empty list, a plain JSON
request through `htransport.NewClient(ctx, opts...)` (the same transport
and credentials, so mockgcp still answers it) is smaller than moving a
dependency. A kind with no API resource of its own needs a line in
`hack/coverage/overrides.yaml` (`subresources`), or the coverage report
counts it outside the denominator.

Field gaps (the 2026-11-10 reversal of decision 5, recorded
as design.md decision 19 in PRs #49 and #51) have begun. Take them in
`docs/coverage/foundation.md`'s order (items 9 to 19 and 21 to 27 that
say "field"), with the attribute lists in `docs/coverage/attributes.csv`.
Before building one, read design.md 6.6 ("Field gaps on large
Terraform-based kinds"), which the first field-gap PR proposed as
decided, and check how the API changes each missing field. A field
the API takes only at create gets a create-only whole-object kind
(insert, get, labels, delete; the rest immutable by CEL), as
`ComputeVPNTunnel` (item 10's `cipherSuite`) did. A field the API
changes by mask, where Config Connector's update never sends it, gets a
field kind beside Config Connector's object (not built yet; its name is
design.md section 10 question 5, so it waits on the builder). Otherwise
a whole-object kind, as decisions 11, 13 and 14. Item 10's other fields
(`ComputeInterconnectAttachment`'s IPv6 candidates and `l2Forwarding`,
`ComputeInterconnect`'s `applicationAwareInterconnect`) need a Dedicated
Interconnect to prove live, which the previous dev project has not; item 11
(`ComputeServiceAttachment`: DCL-based upstream, with direct types, a
mapper and a mockgcp service at the pin) is the next that can run live.
Upstream's `KMSCryptoKey` supports a direct controller
(`static_config.go`); read it before item 18.

What `ComputeVPNTunnel` taught: upstream's direct types and mapper for a
Terraform-based kind (here from PR #10151) copy cleanly into a
create-only kind; mockgcp had no VPN tunnel service (patch 0033). A
Config Connector reference whose `Normalize` wants its registered kind is
easier resolved with `configConnectorSelfLink` (`router_controller.go`):
`external` as given, or the named object's `status.selfLink`.
`ComputeRouterRef.Resolve` (v1alpha1) finds our router or Config
Connector's for any kind that names a router. Patch numbers: 0029 to
0032 went to #49, #52, #53 and #55, merged first, so this PR's is 0033.

What `ComputeNetworkFirewallPolicyRule` taught: upstream's kind really
was Terraform-only (no `apis/` types), so it was adapted from our
regional rule; a kind that references a Config Connector kind our
manager does not run (here `ComputeNetworkFirewallPolicy`) adds that CRD
to `pkg/test/mockenv` and the test writes the object's
`status.selfLink` as Config Connector would.

What `ComputeOrganizationSecurityPolicyRule` taught: check upstream's
`apis/` and `static_config.go`, not only the handoff: a kind called
"Terraform-only" may still have direct types, identity, reference and
mapper upstream (only the controller missing); copy those. Upstream's
mapper there maps to the wrong message (`FirewallPolicyRule`), so read
a hand-written mapper before copying it. A FIREWALL rule's fields are
only in the compute beta API, which mockgcp serves as v1 with v1 protos
and `DiscardUnknown`: fields beta has and v1 lacks vanish in the fake.
Patch 0028 keeps them beside the rule in the HTTP layer
(`organizationsecuritypolicyrules.go`); a later beta-only field can use
the same approach. `google.golang.org/api/compute/v0.beta` is already a
dependency.

What `IAMDenyPolicy` taught: an upstream alpha kind with a direct
controller copies in an hour. Copy the hand-written mappers when the
controller calls them (upstream's `mapper.generated.go` comments out the
generated ones it shadows), rename the mapper functions when the package
already has kinds, and check that the controller writes status through
`op.UpdateStatus`: upstream's deny policy set `externalRef` on a typed
copy only. A kind whose live test needs a role the live-test account
cannot get in `$GCP_PROJECT` skips live behind an `E2E_...` switch,
and so does its sweeper section, since a failed list fails the sweep.
`hack/live/sweep.sh` defined `by_run` after the security profile loop
that calls it, so profiles were never swept; fixed in this PR.

What the security profiles taught: upstream's controllers dial gRPC,
which our manager does not set up; the `cloud.google.com/go` REST
constructors (`New...RESTClient`) take `config.RESTClientOptions()` and
return operations with `Wait`, so the switch is small. gcloud's
`network-security security-profiles` and `security-profile-groups` know
only an organization's, so the sweeper lists and deletes a project's
through the REST API (`ns_api`, `ns_delete` in `hack/live/sweep.sh`,
groups before profiles, waiting on each operation); a later
networksecurity kind without gcloud support can reuse them. A kind that
references another of ours by object name in a typed reference needs the
referenced kind's identity function to read unstructured too
(`networksecuritysecurityprofile_identity.go`), since the reference
normalizer reads the object unstructured.

What `ComputeRouter` taught (decision 14, design.md 6.5):

- **Scaffolding a compute kind.** `hack/scaffold-kind.sh
  google.cloud.compute.v1 compute.gcp.hfl.dev/v1alpha2
  ComputeRouter:Router offload-artifacts/scaffold` works (v1alpha2
  because upstream has compute v1alpha1; rename to ours). The compute
  proto has no field behaviour and no resource annotations, so the
  output puts every field in the spec with the generator's JSON names
  (`enableIPV4`, `advertisedIPRanges`); keep the leaf types that fit
  unedited and name the rest after Config Connector's existing kinds.
  About a 15-minute n2d-standard-8 run.
- **A whole-object compute kind writes with `update` (PUT)**, the read
  object with the owned fields replaced, not `patch`, which keeps lists
  the body leaves out. Compare with a subset rule (what the spec sets
  must match; what it leaves out is the API's default) or every resync
  writes: `protoSubset` in `router_controller.go` is reusable.
- **Cloud Router takes one feature per write**, found only live (three
  runs): insert refuses a router with interfaces, peers, NATs and keys
  ("Creating a router with multiple features..."), update refuses more
  than one ("Updating multiple Router features..."), and a key sent
  with its value counts as a key change even when unchanged. So the
  router is inserted bare and written in ordered single-feature steps;
  keys go by name (no value) outside the key and peer steps. The exact
  rule is unknown: a live patch of an interface and a peer together was
  accepted. Patch 0027 models a stricter version.
- **Live tests after a forced resync** must wait on the cloud or the
  status, not on Ready: the object is already Ready, and live writes of
  several sequential operations take longer than the 3 s the resync
  helper sleeps.
- **Secrets the API never returns** (MD5 keys): read them every
  reconcile, send them with every write, and keep a digest of the
  Secrets' `resourceVersion`s in status (not of the values) to notice a
  rotation.
- **`go:test-envtest` and `setup-envtest` are internal tasks.** To run
  one package's tests on offload:
  `--step 'export KUBEBUILDER_ASSETS="$(go run sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20250517180713-32e5e9e948a5 use 1.33.x --bin-dir "$PWD/bin/envtest" -p path)" && E2E_GCP_TARGET=mock go test -count=1 -v -run TestX ./pkg/controller/direct/<pkg>/'`
  (the bin dir must be absolute).
- **`task coverage` needs ~225 MB of inputs**: copy `.cache/coverage`
  from another clone at the same pins rather than fetching.

Read before choosing a route: `docs/prior-art.md` in `highfivelabs/factory`
(lean on `controllerbuilder`; behaviour from Magic Modules' product YAML
into the proto overlay) and `docs/terraform-leverage.md` in
`highfivelabs/catalog` (the Terraform route is not ours, decision 2.1).

Still true from before:

- **`LoggingSettings` live and the test-folder work**, when the
  coordinator says the test folder exists. The test and the sweeper are
  ready (`mockenv.LoggingFolder`, `mockenv.LoggingKMSKey`). Everything
  under **one** dedicated test folder; every change is undone; test
  projects are deleted; the sweeper learns folders, projects and
  organization policies.
- The default network tier and default service accounts change a whole
  project: live, they run only in a test project under the test folder,
  never in `$GCP_PROJECT` (disabling or deleting its compute default
  service account would break the offload VMs and `RunService`).
- **Live runs for the other 27 `pkg/iampolicy` rows** (design.md section
  7, item 26).

## Conventions that are easy to get wrong

**Nothing heavy on the laptop.** No `task check`, `task operator-e2e`,
envtest, kind, Docker, or `go test` beyond one small package. `go build`
and `go vet` of a package or two, `hack/generate.sh` and `hack/upstream.sh`
are fine (with `GOFLAGS=-p=2 nice`). Everything else runs on offload.

**offload.** From the repository root:

```sh
../offload/offload run --hours 3 --prep 'task upstream' --step-skippable 'task check' --step 'task operator-e2e'
../offload/offload run --sa "$GCP_LIVE_SA" --hours 3 \
  --prep 'task upstream' --step-skippable 'task check' \
  --step "env GCP_PROJECT=$GCP_PROJECT task test-live KINDS='compute iam run'"
```

- **Put `task upstream` in a `--prep`.** `third_party` is gitignored and
  not uploaded, so every fresh VM needs it again; `--prep` steps rerun on
  every attempt, `--step-skippable` ones (`task check`) do not. There is
  no `tools` prep to add: `task tools` also fetches setup-envtest, which
  the tasks that need it do themselves.
- **After a preempted live run, sweep it.** The live run's own sweep only
  takes leftovers older than 2h. Sweep the interrupted run by ID before
  the next run:
  `--step "env GCP_PROJECT=$GCP_PROJECT task live-sweep RUN=<run ID>"`.
- **Start offload with the tool's background mode**, never with a shell
  `&`: a shell-backgrounded offload dies with the shell and orphans its
  VM. If that happens, `../offload/offload ls`, then
  `../offload/offload kill <ID>` (only your own VMs).
- **The source is packed when "offload: running in ..." is printed**;
  after that you can switch branches locally. Uncommitted changes go to
  the VM too and make the run "dirty" in the ledger.
- **Reading a run.** The laptop's copy of the log is whatever you
  redirected it to; `../offload/offload logs <ID>` prints a run's log from
  the bucket (`-f` follows it), and the artifacts (each kind's mock and
  live log, `summary.md`) are under the bucket path offload prints at the
  end. A live test that waits for Ready waits up to 15 minutes before it
  fails with the object's last status, which carries the API's error:
  read that before changing a wait; a longer timeout is never the fix.
- **Quota.** The project has 32 vCPUs; two 16-vCPU runs fill it. Pass
  `--machine n2d-standard-8` for a check-only run beside another.

**Patches.** `hack/upstream.sh` resets `third_party/` to the pin and
applies `patches/`; it wipes uncommitted edits there. To change a patch:
edit `third_party/`, then regenerate the patch from
`git -C third_party/k8s-config-connector diff -- <the patch's files>`
(with `git add -N` for new files), keep its header paragraph, and run
`hack/upstream.sh` to check it applies. Two patches must not touch the
same file unless they apply in order.

**Every worktree has its own `third_party/`.** It is gitignored, so a
new `git worktree` has none, and one copied or linked from another
checkout holds that checkout's patches, not this branch's. After
`git worktree add`, and after every merge of `main` that changes
`patches/` or the pin, run `GOFLAGS=-p=2 nice hack/upstream.sh` in the
worktree (it fetches the pin when missing, then resets it and applies
this branch's patches), and `hack/upstream.sh --check` before trusting a
local build. Never symlink `third_party` to another checkout's: a build
there compiles against the other branch's mockgcp and passes or fails
for the wrong reason. A long-lived worktree whose branch has merged
(for example `.worktrees/server-id`) is behind `main`'s patches until it
is moved to `main` and `hack/upstream.sh` runs again.

**Generated code.** `GOFLAGS=-p=2 nice hack/generate.sh` (controller-gen
v0.16.5) rewrites the CRDs, deepcopy and `config/crds/kustomization.yaml`.
controller-gen panics on type aliases: use the aliased type
(`refsv1beta1.FolderRefDeprecated`, not `FolderRef`).

**Live tests.** Every name from `mockenv.Name`, labels from
`mockenv.Labels()`, the project from `mockenv.Project`; the test deletes
everything it makes, abandoned resources included (`t.Cleanup`); the
sweeper (`hack/live/sweep.sh`) knows the resource type, by label or by the
run's name prefix (`by_run` for names that carry the run ID). Package-level
test helpers collide across a package's test files: keep helpers in one
`helpers_mockgcp_test.go` per package or local to the test.

**Branches, merges, PRs.** One PR per hand-written kind (generated kinds
go several to a PR), `agent/<short-name>`, cut from the latest `main`, in
your own `git worktree`, never a checkout another agent uses; if it needs
an unmerged sibling, say so and the coordinator merges first. A kind's
notes and live rows go in its own `docs/kinds/<Kind>.md`, not in
design.md 6.5, the README or the live-runs ledger, and every patch is
numbered (design.md decision 29).
What still conflicts: UPSTREAM's patch table and `upstream.yaml`,
`hack/live/sweep.sh` and `pkg/controller/controllers.go` for hand-written
kinds; when main
moves, `git merge --no-ff --signoff origin/main`, keep both sides
(renumber the later item, recount the kinds), regenerate, commit with a
full message ending in the attribution lines your session is given
(`Co-Authored-By` and the session link; `--signoff` adds the DCO line),
recheck on offload, push (never force), and tell the coordinator. Never `--no-edit` a merge: it drops the sign-off.

**Reporting.** After each PR, a short message to the coordinator: what
was built, what was copied, the real check and live results, what is
pending and why.
