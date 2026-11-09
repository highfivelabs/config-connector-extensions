# Handoff

This is where an agent that is new to the repository starts. It says what
is done, what is left (in order), and the working conventions that are not
obvious from the code. Written 2026-11-09 by the agent that built the
seed-catalog kinds (`docs/design.md` 6.5), updated by the one that built
`IAMRoleMember`. Read
`CLAUDE.md`, `README.md`, `docs/design.md` and `docs/live-runs.md` first;
this file only adds to them.

## Where things stand

On `main` after the `IAMRoleMember` PR: eighteen kinds (`README.md`,
design.md's intro), mockgcp patches 0001 to 0019 (`UPSTREAM.md`; the next
free number is **0020**), the live-test switch and the Taskfile
(`docs/live-runs.md`, `taskfiles/README.md`; use `task`, not `make`). From
the seed catalog (design.md 6.5): ranks 1 to 5 built; rank 6's policy built;
rank 7's `LoggingSettings` built (live pending a test folder); rank 10's
`ComputeProjectMetadataItem` built. Every built kind but `LoggingSettings`
passes live.

`IAMRoleMember` (rank 2) is built over `pkg/iampolicy`, whose resource
table now has the 30 rows `IAMPolicyMember` cannot reference
(`registry_more.go`) and three protocols (the `google.iam.v1` mixin, Cloud
Storage's, Compute Engine's). Three rows are proven live (a regional
secret, a managed folder, a backend service); the other 27 are checked for
names and URLs only. The kind name is a working one: design.md section 10,
question 3, waits for the builder.

## What is left, in order

1. **Rank 6's rule and association kinds** for regional firewall policies:
   the rule is its own PR (`ComputeRegionNetworkFirewallPolicyRule`, patch
   0020, a rule addressed by priority); the association is the next one
   (patch 0021; `addAssociation`, `getAssociation`, `removeAssociation`;
   it needs the policy reference type the rule PR adds,
   `ComputeRegionNetworkFirewallPolicyRef`).
2. **The rest of rank 10**: the default network tier (Terraform's
   `google_compute_project_default_network_tier`; check whether mockgcp
   serves `setDefaultNetworkTier`) and the project's default service
   accounts (`google_project_default_service_accounts`). Both change a
   whole project: live, they run only in a test project under the test
   folder (item 4), never in `hfl-dev-514205`. Disabling or deleting
   `hfl-dev-514205`'s compute default service account would break the
   offload VMs and `RunService`. Until the folder exists, mark them live:
   pending.
3. **Ranks 8 and 9** of the table in design.md 6.5, only when upstream has
   released them (recheck read-only: `v1.160.0` was the latest on
   2026-11-09, `master` at `b795cda8`).
4. **`LoggingSettings` live and the test-folder work**, when the
   coordinator says the test folder exists. The test and the sweeper are
   ready (`mockenv.LoggingFolder`, `mockenv.LoggingKMSKey`). Wider live
   testing was approved with these guardrails: everything under **one**
   dedicated test folder; organization-level policies and existing
   projects and folders are never touched; every change is undone; test
   projects are deleted; the sweeper learns folders, projects and
   organization policies. Do not try to create folders or grant
   organization-level IAM.
5. **Live runs for the other 27 `pkg/iampolicy` rows**, as the APIs and
   quotas allow (design.md section 7, item 26).

## Conventions that are easy to get wrong

**Nothing heavy on the laptop.** No `task check`, `task operator-e2e`,
envtest, kind, Docker, or `go test` beyond one small package. `go build`
and `go vet` of a package or two, `hack/generate.sh` and `hack/upstream.sh`
are fine (with `GOFLAGS=-p=2 nice`). Everything else runs on offload.

**offload.** From the repository root:

```sh
../offload/offload run --hours 3 --prep 'task upstream' --step-skippable 'task check' --step 'task operator-e2e'
../offload/offload run --sa cce-live-tests@hfl-dev-514205.iam.gserviceaccount.com --hours 3 \
  --prep 'task upstream' --step-skippable 'task check' \
  --step 'task test-live PROJECT=hfl-dev-514205 KINDS="compute iam run"'
```

- **Put `task upstream` in a `--prep`.** `third_party` is gitignored and
  not uploaded, so every fresh VM needs it again; `--prep` steps rerun on
  every attempt, `--step-skippable` ones (`task check`) do not. There is
  no `tools` prep to add: `task tools` also fetches setup-envtest, which
  the tasks that need it do themselves.
- **After a preempted live run, sweep it.** The live run's own sweep only
  takes leftovers older than 2h. Sweep the interrupted run by ID before
  the next run:
  `--step 'task live-sweep PROJECT=hfl-dev-514205 RUN=<run ID>'`.
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
