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
run against a real project, but only through `make test-live`, by a
maintainer, in a project set aside for it (`docs/live-runs.md`).

```sh
make upstream      # once: Config Connector at the pin (~650 MB) plus patches/
make check         # pin, go.mod pins, generated files, gofmt, vet, SPDX lines, build, tests
make operator-e2e  # the operator on a fresh kind cluster with the in-cluster fake cloud
make fake-cloud-up && make fake-cloud-test && make fake-cloud-down
                   # Config Connector's manager and ours against mockgcp on kind
```

`make check` must pass before a pull request. Each new kind needs an
end-to-end test on `pkg/test/mockenv` (a real kube-apiserver, Config
Connector's direct reconciler, and mockgcp) covering create, update,
delete and `deletion-policy: abandon`. If mockgcp lacks the API, extend it
with a numbered patch in `patches/k8s-config-connector/` and say whether
the patch should go upstream. Write the test so it also runs live (names
from `mockenv.Name`, labels from `mockenv.Labels()`, everything it makes
deleted): a kind is done after one live run is recorded in
`docs/live-runs.md`.

A new, changed or retired kind, or a moved Config Connector pin, changes the
numbers in the README's Coverage section: run `hack/coverage/run.sh` and commit
`docs/coverage/` and `README.md` with it (`hack/coverage/run.sh --check` tells
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
