# Throughput: why kinds come slowly, and what changed

Measured 2026-11-11 from `git log`, the pull requests, CI runs and
`docs/coverage/` (pinned 2026-11-09).

## Where it stands

- **Kinds on `main`: 34** (35 with this change's 3: 37). Four more wait in
  open pull requests since 2026-11-10 (#53 `SecurityCenterMuteConfig`, #54
  `DNSResponsePolicy`, #55 a packet mirroring rule, #56 `ComputeVPNTunnel`).
- **Remaining whole-resource gaps** (a create in the API, no Config
  Connector kind, none of ours), from `docs/coverage/resources.csv`:
  tier 1 (the foundation) 62, tier 2 198, tier 3 234: 494 in all.
  The MVP's own services (projects, folders, IAM, VPC, GKE, Cloud SQL,
  Secret Manager, Artifact Registry, Cloud Run, logging, monitoring) are
  almost all covered by Config Connector already; what is left for them
  is field gaps and a few whole resources (saved queries, OAuth clients,
  log bucket links, Cloud Run domain mappings, monitoring snoozes).
- **Kinds per day**, by merged pull request: 2026-11-09 about 21,
  2026-11-10 6, 2026-11-11 2 (and 4 queued). The first day ran several
  agents in parallel on easy gaps; then it fell off.

## Where the time goes, per kind

A kind's pull request adds 1,700 to 2,900 lines (#35, #38, #44, #49, #52).
Of that, hand-written:

| Part | Lines | Notes |
| --- | ---: | --- |
| controller | 260 to 370 | copied from upstream's template where one exists, then changed |
| identity, reference | 150 to 300 | per kind, the same shape every time |
| end-to-end test | 220 to 460 | the same five subtests every time: create, resync writes nothing, update, immutable fields, delete, abandon |
| mockgcp patch | 90 to 370 | a fake service per kind (patches 0011 to 0030), its own number |
| docs | 15 files | design.md, handoff.md, README, UPSTREAM.md, upstream.yaml, live-runs.md, coverage (10 generated files), controllers.go, sweep.sh |

Generated: the types and mapper, when `hack/scaffold-kind.sh` was used
(2 of 34 kinds: it needs protoc and a googleapis clone, so one kind per
15-minute VM run, and its output needs an overlay for field behaviour).

Then the queue: every kind touches the same 15 files, so each merge makes
every other open pull request stale; it merges `main` back, renumbers its
mockgcp patch (#52's went from 0029 to 0030), and CI runs again (15 to 76
minutes a run, `task check` when `pkg/` or `patches/` change). Merges are
serial; the four kinds above have waited a day. A live run per kind needs
an offload VM as the live-test account; the project's 32 vCPUs fit two.

## Why generation is not further along

1. **The generator only speaks protos.** `controllerbuilder` reads
   googleapis protos and the published Go clients. Many remaining gaps
   are newer than the Go clients (the logging client at our pin has no
   saved queries; IAM OAuth clients and workforce provider keys have no
   gapic client at all): they exist only in the APIs' Discovery
   documents, which `docs/coverage` already pins and reads.
2. **It stops at the types.** The controller, identity, test and fake
   service are written by hand for each kind, though for a standard
   resource collection they differ only in names.
3. **The fake is per kind.** mockgcp serves what Config Connector tests;
   our gaps are by definition what it lacks, so every kind wrote a fake
   service as a numbered patch to `third_party/`.
4. **One kind per pull request, many shared files.** Doc hotspots and
   sequential patch numbers turn parallel work into serial merges.

## What changed (this pull request)

**`discokind`: a kind from a Discovery document, with nothing per kind
but a config entry and two fixtures.** For an API resource that is a
standard collection (create with an ID, get, patch with an update mask,
delete; AIP-121 to 135):

- `hack/discokind` reads `hack/discokind/kinds.yaml` and the pinned
  Discovery document and writes the CRD's Go types (spec from the writable
  fields, `status.observedState` from the read-only ones; `Required.` and
  `Immutable.` in the API's descriptions become CRD rules), a `Kind`
  descriptor (base URL, name template, parents, ID parameter, which calls
  are long-running), the kind's test function and the sweeper's row.
- `pkg/discokind` is one controller for every such kind: the spec, minus
  its parent reference and the fields that make up the name, is the
  request body as it stands (the CRD's field names are the API's), so
  there is no mapper; update sends the top-level fields that differ, by
  update mask.
- `pkg/discokind/fake` serves every such collection in front of mockgcp,
  from the same descriptors: no mockgcp patch.
- `pkg/discokind/kindtest` is the end-to-end test: create, a resync
  writes nothing, update, delete, abandon, on the fake or live, from
  `testdata/discokind/<Kind>/create.yaml` and `update.yaml`.
- The kind list (`pkg/controller/zz_discokinds.go`) and the sweeper's
  list (`hack/live/discokinds.tsv`) are generated, so a kind touches no
  hand-kept shared file; several kinds go in one pull request.
- `hack/live/run.sh` takes `LIVE_RUN` (a `go test -run` regexp), so one
  kind of a package runs live without the rest.

Per kind, by hand: about 10 lines of `kinds.yaml` and 30 of fixtures.
Reach: of the remaining whole-resource gaps that have a Discovery
document, 26 in tier 1, 59 in tier 2 and 93 in tier 3 have this shape
(178 of 494). The others: 129 create without an ID parameter (compute's
insert style, server-assigned IDs), 127 lack a create, get or delete
(singletons, settings, policies set by a verb), 60 have no Discovery
document (proto only).

What it does not do: field gaps on Config Connector's kinds (decision 19
keeps those on the copy-upstream route), references beyond the parent
(other parts of the name are plain IDs), secrets (a read-only secret
would land in status, so a kind like `IAMOauthClientCredential` is not
generated yet), prerequisites in the test (a kind whose parent is
another resource, like a log bucket's link, needs that made first), and
validation in the fake (required fields, enums, cross-field rules: that is
what a live run is for; `docs/testing.md`).

## Still to fix

- **Docs per kind, not per file.** A kind's design notes and ledger row in
  their own files (`docs/kinds/<Kind>.md`) would end the merge conflicts
  in design.md, handoff.md and live-runs.md for hand-written kinds too.
- **mockgcp patches named, not numbered**, so siblings do not renumber.
- **The compute insert shape** (`insert`, `get`, `patch`, `delete` with
  the name in the body and global or regional operations) in the same
  generator: 25 of the 62 tier 1 gaps are Compute Engine's.
- **A live job** that runs the generated kinds' tests in the test project
  on a schedule (`docs/testing.md`).
