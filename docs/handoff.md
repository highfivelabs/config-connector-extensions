# Handoff

This is where an agent that is new to the repository starts. It says what
is done, what is left (in order), and the working conventions that are not
obvious from the code. Written 2026-11-09 by the agent that built the
seed-catalog kinds (`docs/design.md` 6.5), when its context ran out. Read
`CLAUDE.md`, `README.md`, `docs/design.md` and `docs/live-runs.md` first;
this file only adds to them.

## Where things stand

On `main`: sixteen kinds (`README.md`, design.md's intro), mockgcp patches
0001 to 0017 (`UPSTREAM.md`; the next free number is **0018**), and the
live-test switch (`docs/live-runs.md`). From the seed catalog (6.5): ranks
3, 4, 5 built; rank 6's policy built; rank 7's `LoggingSettings` built;
rank 10's `ComputeProjectMetadataItem` built. What-is-left items in
design.md section 7 run to 25.

The live-runs PR that adds this file makes packages `compute`, `iam` and
`run` run live (and by default in `make test-live`), fixes what live
showed (a route policy's missing-policy 400, the managed identity test's
attestation rules, RunService's GPU quota and its containers' default
port and startup probe, the last two also in mockgcp) and records the
runs in the ledger. Every built kind but `LoggingSettings` now passes
live.

## What is left, in order

The builder's decisions behind items 1 and 2 (2026-11-09, through the
coordinator):

- Kind name approved: **`IAMRoleBinding`** (`iam.gcp.hfl.dev/v1alpha1`),
  in the builder's words "sounds like the right name": one role, an
  optional condition and a list of members on one resource, authoritative
  for that role and condition. This closes design.md section 10,
  question 1; record it there with those words.
- "I think I would prefer the non-authoritative, but that's fine if
  authoritative is what you've built." So: Config Connector's
  `IAMPolicyMember` stays the default in everything we write; rank 2 is
  built in the **additive** (non-authoritative) form.

1. **`IAMRoleBinding` (rank 1) is built; the additive IAM kind (rank 2)
   is next.** `IAMRoleBinding` (this PR): `pkg/iampolicy` is the shared
   engine (a table of resource types in `registry.go`, a policy client,
   the binding rules in `bindings.go` with `SetMembers` authoritative and
   `AddMembers`/`RemoveMembers` additive, and `Resolve` for a
   `resourceRef`); `pkg/controller/direct/iam/policy_common.go` is the
   plumbing a second kind reuses. The additive kind needs only: types
   (`apis/iam/v1alpha1`, one `member`/`memberFrom` and `role`, shaped like
   `IAMPolicyMember`, reusing `IAMResourceReference` and `IAMCondition`),
   a controller (`AddMembers` on create/update, `RemoveMembers` on delete,
   `Find` = the member is in the binding), a test, the `iamrefs` member
   reference, and registry rows for the rank-2 resource types (the 32 in
   `highfivelabs/catalog` `docs/gcp-inventory.md`, which are the
   `NotInIAMPolicyMember` rows plus those without a Config Connector kind:
   managed folders (Storage protocol, patch 0018 already serves them),
   log views, Cloud Run worker pools and jobs, Cloud Functions, and so
   on; compute resources need a third protocol, since Compute Engine's
   `getIamPolicy` is GET with `optionsRequestedPolicyVersion` and its
   `setIamPolicy` body is `{bindings, etag, policy}`). Working name to
   propose to the builder: **`IAMRoleMember`** (one member of one role on
   one resource; pairs with `IAMRoleBinding`); record it in design.md
   section 10 as a question with that recommendation, keep the kind name
   in the one `GVK` var. Cut it from main after this PR merges. Live test
   members must be service accounts of the project (an organization
   policy refuses `projectOwner:` and similar on secrets).

2. **Taskfile instead of make.** Not started; do it after the in-flight
   PRs merge, as one dedicated PR. Replace the `Makefile` with
   `Taskfile.yml` keeping the task names (`check`, `operator-e2e`,
   `test-mock`, `test-live`, `live-sweep`, `live-list`, `upstream`,
   `upstream-check`, `tools`, `generate`, and the rest), and update
   `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, the docs and the
   live-runs page (the ledger keeps its history; new rows say `task`).
   Put the tasks other repositories could share in a separate common
   Taskfile that this one includes, and in the PR recommend how to share
   it across repositories (Task's remote includes from a private
   repository, or a vendored copy with a sync script), with the trade-offs.
   Do not create a GitHub repository; say what is needed and the
   coordinator asks the builder. The offload VM image is said to have
   `task` installed: verify that first, then use `task check` in offload
   `--step`s from then on.

3. **`LoggingSettings` live.** Built and merged; the test and the sweeper
   are ready (`mockenv.LoggingFolder`, `mockenv.LoggingKMSKey` in
   `pkg/test/mockenv/target.go`). Live is pending until the coordinator
   passes `E2E_LOGGING_FOLDER=folders/<id>` and a KMS key
   (`E2E_LOGGING_KMS_KEY`) the folder's Log Router can use. A live run
   never changes an organization's settings: that subtest skips live.
   Wider live testing was approved by the builder with these guardrails
   (coordinator, 2026-11-09): everything under **one** dedicated test
   folder; organization-level policies and existing projects and folders
   are never touched; every change is undone; test projects are deleted;
   the sweeper learns folders, projects and organization policies. The
   folder does not exist yet and the live-test account cannot create
   folders: do not try. Prepare the sweeper coverage for folders,
   projects and org policies under that folder in the meantime.

4. **Rank 6's rule and association kinds.** Not started; cut from main
   (the policy kind is merged). Pointers: Terraform's
   `resource_compute_region_network_firewall_policy_rule.go` and
   `..._association.go` under
   `third_party/k8s-config-connector/third_party/github.com/hashicorp/terraform-provider-google-beta/google-beta/services/compute/`
   (read, not copied); Config Connector's direct `ComputeFirewallPolicyRule`
   (`apis/compute/v1beta1/computefirewallpolicyrule_types.go`,
   `pkg/controller/direct/compute/computefirewallpolicyrule_controller.go`,
   `firewallpolicyrule_resolverefs.go`) is the nearest code to copy for
   the rule's match fields. The API is
   `regionNetworkFirewallPolicies.{addRule,getRule,patchRule,removeRule}`
   and `{addAssociation,getAssociation,removeAssociation}`. Patch 0014's
   regional mock (`mockcompute/regionnetworkfirewallpolicies.go`) serves
   only the policy: add the rule and association methods in a new patch,
   adapted from the global mock's (`networkfirewallpolicies.go`, which
   uses `mockFieldValuesForRule`). One PR per kind; each needs a live run
   (`roles/compute.securityAdmin` is granted; an association needs a
   network of the run's own, as `newNetwork` in the compute tests makes).

5. **The rest of rank 10.** The default network tier
   (Terraform's `google_compute_project_default_network_tier`; mockgcp's
   compute project has `defaultNetworkTier`, check whether it serves
   `setDefaultNetworkTier`) and the project's default service accounts
   (`google_project_default_service_accounts`). Both change a whole
   project: live, they run only in a test project under the test folder
   (item 3), never in `hfl-dev-514205`. Disabling or deleting
   `hfl-dev-514205`'s compute default service account would break the
   offload VMs and `RunService`.

6. **Live: pending.** Only `LoggingSettings` (item 3). Everything else
   passed live on 2026-11-09 (`docs/live-runs.md`). RunService's GPU
   checks are skipped live: the project has no Cloud Run GPU quota without
   zonal redundancy; `E2E_RUN_GPU=true` runs them where there is. Every
   new kind needs its own live run before it is done.

## Conventions that are easy to get wrong

**Nothing heavy on the laptop.** No `make check`, `make operator-e2e`,
envtest, kind, Docker, or `go test` beyond one small package. `go build`
and `go vet` of a package or two, `hack/generate.sh` and `hack/upstream.sh`
are fine (with `GOFLAGS=-p=2 nice`). Everything else runs on offload.

**offload.** From the repository root:

```sh
../offload/offload run --hours 3 --step 'make upstream' --step 'make check' --step 'make operator-e2e'
../offload/offload run --sa cce-live-tests@hfl-dev-514205.iam.gserviceaccount.com --hours 3 \
  --step 'make upstream && make check' \
  --step 'make upstream && make test-live PROJECT=hfl-dev-514205 KINDS="compute iam run"'
```

- **Batch `make upstream` into every step that needs it.** When a Spot VM
  is preempted, offload resumes the run on a fresh VM and skips the steps
  that passed, `make upstream` included, so the next step finds no
  `third_party/k8s-config-connector` ("is missing; run hack/upstream.sh")
  and fails at once. `make upstream && make test-live ...` in one step
  survives a resume.
- **After a preempted live run, sweep it.** The live run's own sweep only
  takes leftovers older than 2h. Sweep the interrupted run by ID before
  the next run:
  `--step 'make live-sweep PROJECT=hfl-dev-514205 RUN=<run ID>'`.
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

**Branches, merges, PRs.** One PR per kind, `agent/<short-name>`, cut
from the latest `main`; if it needs an unmerged sibling, say so and the
coordinator merges first. Sibling PRs conflict in the same doc spots
(design.md's intro count and kind list, the 6.5 notes, section 7's
numbered items, the README table, UPSTREAM's patch table, the live-runs
ledger, `hack/live/sweep.sh`, `pkg/controller/controllers.go`): when main
moves, `git merge --no-ff --signoff origin/main`, keep both sides
(renumber the later item, recount the kinds), regenerate, commit with a
full message ending in the attribution lines your session is given
(`Co-Authored-By` and the session link; `--signoff` adds the DCO line),
recheck on offload, push (never force), and tell the coordinator. Never `--no-edit` a merge: it drops the sign-off.

**Reporting.** After each PR, a short message to the coordinator: what
was built, what was copied, the real check and live results, what is
pending and why.
