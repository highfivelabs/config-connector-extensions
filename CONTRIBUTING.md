# Contributing to config-connector-extensions

Thank you for helping fill Config Connector's gaps. This page says how
changes are signed off, how to test them, and the one rule that shapes
most of the code.

## Sign off your commits

config-connector-extensions uses the [Developer Certificate of Origin](https://developercertificate.org/)
(DCO) instead of a contributor licence agreement. By signing off a commit
you certify that you wrote it, or otherwise have the right to submit it,
under the project's licence (Apache-2.0). Add the line with `git commit -s`:

```text
Signed-off-by: Your Name <you@example.com>
```

Every commit in a pull request needs it, with the name and email of its
author.

## Build and test

Everything runs against a fake Google Cloud (Config Connector's mockgcp);
no real project or credentials are needed. The same end-to-end tests also
run against a real project, but only through `task test-live`, by a
maintainer, in a project set aside for it (`docs/live-runs.md`).

Install [Task](https://taskfile.dev) (`brew install go-task`, or
`go install github.com/go-task/task/v3/cmd/task@latest`); `task` lists the
tasks. The shared ones (`go:*`) are factory's `taskfiles/go.yml`, included
remotely at a tag and checksum; Task 3.54 or later fetches it with your git
credentials on the first run (factory `taskfiles/README.md`).

```sh
task upstream      # once: Config Connector at the pin (~650 MB) plus patches/
task check         # pin, go.mod pins, generated files, gofmt, vet, SPDX lines, bash -n of the shell scripts, build, tests
task operator-e2e  # the operator on a fresh kind cluster with the in-cluster fake cloud
task fake-cloud-up && task fake-cloud-test && task fake-cloud-down
                   # Config Connector's manager and ours against mockgcp on kind
```

`task check` must pass before a pull request. Each new kind needs an
end-to-end test on `pkg/test/mockenv` (a real kube-apiserver, Config
Connector's direct reconciler, and mockgcp) covering create, update,
delete and `deletion-policy: abandon`. If mockgcp lacks the API, extend it
with a patch in `patches/k8s-config-connector/` and say whether the patch
should go upstream. Patches apply in file-name order. A patch that only
adds files, or touches files no other patch touches, is named by what it
does, without a number (`mockgcp-dns-response-policies.patch`): it sorts
after the numbered ones and needs no renumbering when a sibling merges
first. Only a patch that must apply after another takes the next number. Write the test so it also runs live (names
from `mockenv.Name`, labels from `mockenv.Labels()`, everything it makes
deleted): a kind is done after one live run is recorded in its own file
in `docs/kinds/` (`docs/kinds/README.md`). A kind's pull request adds or
edits that file, not a shared list; `task check:static` checks every CRD
has one. A kind that is a standard collection is generated instead
(`hack/discokind`, `docs/design.md` section 2.4).

A new, changed or retired kind, or a moved Config Connector pin, changes the
numbers in the README's Coverage section: run `task coverage` (`hack/coverage/run.sh`) and commit
`docs/coverage/` and `README.md` with it (`task coverage:check` tells
you whether you need to; `docs/coverage/README.md` says how it works).

Generated files (`config/crds/`, `zz_generated.deepcopy.go`) come from
`hack/generate.sh`; commit its output. New Go files and shell scripts
start with:

```go
// Copyright <year> <you or your employer>
// SPDX-License-Identifier: Apache-2.0
```

## The copy-upstream rule

Where Config Connector already has the resource, on `master` or in an
open pull request, copy its code and change as little as possible:

- Keep the copied file's copyright and licence header, and add a
  "Modified by" note naming the source file and what changed.
- Record the file, its source and the change in `UPSTREAM.md`; for a pull
  request, add it to "Copied from open pull requests" so
  `hack/upstream-pr.sh` follows it.
- Never edit Config Connector's framework in this tree. It is imported
  from the pinned checkout; a needed change is a patch, meant to go
  upstream.
- Gap kinds live in our groups (`<service>.gcp.hfl.dev`), never in
  `*.cnrm.cloud.google.com`, and retire when a Config Connector release
  ships them (`docs/design.md`, "Retiring a gap kind").

Behaviour taken from the Terraform Google provider (MPL-2.0) is read and
followed, never copied.

## Choosing what to build

A gap becomes a kind when a real workload needs it and no Config
Connector kind covers it; see `docs/design.md` section 6. Open an issue
first for a new kind, naming the workload and the upstream code or pull
request, if any.
