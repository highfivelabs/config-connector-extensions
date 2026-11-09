# Handoff

This is where an agent that is new to the repository starts. It says what
is done, what is left (in order), and the working conventions that are not
obvious from the code. Written 2026-11-09 by the agent that built the
seed-catalog kinds (`docs/design.md` 6.5), updated by the one that built
`IAMRoleMember`. Read
`CLAUDE.md`, `README.md`, `docs/design.md` and `docs/live-runs.md` first;
this file only adds to them.

## Where things stand

On `main` after the association PR: twenty kinds (`README.md`, design.md's
intro), mockgcp patches 0001 to 0021 (`UPSTREAM.md`; 0022 is
`ServiceIdentity`'s, PR #28, and 0023 `ComputeProjectDefaultNetworkTier`'s;
the next free number is **0024**), the live-test switch and the
Taskfile (`docs/live-runs.md`, `taskfiles/README.md`; use `task`, not
`make`). Every built kind but `LoggingSettings` passes live.

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

## What is left, in order

The queue is `docs/coverage/foundation.md` (the builder's priority:
platform foundations first), not the older rank lists. After the
association kind: folder and organization `ServiceIdentity`, default
service accounts, default network tier (built: `ComputeProjectDefaultNetworkTier`,
live pending a test project under the test folder), organization custom IAM roles,
organization address groups and security profiles, and so on down the
report. The builder answered its four Decision items (design.md
decisions 13 to 16, 2026-11-09): alpha-only foundation kinds get our own
stable kinds (13: `IAMDenyPolicy`, `ComputeOrganizationSecurityPolicyAssociation`,
`ComputeNetworkFirewallPolicyRule`, `IAMPrincipalAccessBoundaryPolicy`,
`DNSResponsePolicy` and its rule); Cloud Router is a copy of upstream's
PR #13960 as a complete kind of ours (14); `LoggingBillingAccountSink`
now, live pending billing-account access (15); Security Command Center
kinds with organization, folder and project parents (16). Decision 5
stands for other field gaps (wait for the release). Whole-resource gaps
first; any other field gap that needs the builder goes to section 10
with a recommendation. A deprecation path for our kinds is the builder's
separate design; do not build it. Org-level live tests wait for the test folder:
"live: pending, needs test folder"; never create folders, grant
organization-level IAM or touch the organization.

Next, **`ProjectDefaultServiceAccounts`** (`resourcemanager.gcp.hfl.dev`,
foundation item 1's last part), researched 2026-11-09, not started.
Terraform's `google_project_default_service_accounts` (google-beta
`24c6f5fd`, read): it lists the project's service accounts and picks the
defaults by display name ("Compute Engine default service account", "App
Engine default service account"); `DELETE` deletes, `DISABLE` disables,
`DEPRIVILEGE` removes `serviceAccount:<email>` from every project binding;
it records unique ID to email in state, and on destroy, unless
`restorePolicy: NONE`, undeletes (by unique ID) or enables them
(`REVERT_AND_IGNORE_FAILURE` logs instead of failing); `DEPRIVILEGE` is
never reverted; `action` is ForceNew. mockgcp lacks
`serviceAccounts.list` and `undelete`, and its delete, disable and enable
take an email, not a unique ID: the patch is the work. Live: a test
project under the test folder only (`mockenv.FolderProject`), never
`hfl-dev-514205`. Then organization address groups and security
profiles (item 6), decisions 13 to 16, packet mirroring rules.

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
  never in `hfl-dev-514205` (disabling or deleting its compute default
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
