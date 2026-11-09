# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# config-connector-extensions: kinds for Google Cloud that Config Connector
# does not have yet, on Config Connector's own reconciler. Every target
# needs the pinned Config Connector checkout; `make upstream` fetches it
# (about 650 MB, not committed).

# The project's name, for a rename: the Go module path, the images, the
# install's object names and pkg/operator.Name carry it too.
NAME := config-connector-extensions

export GOTOOLCHAIN ?= local

BIN := $(CURDIR)/bin
ENVTEST_K8S_VERSION := 1.33.x
# setup-envtest from controller-runtime's release-0.20 branch (the
# controller-runtime Config Connector pins), as resolved on 2026-11-07.
SETUP_ENVTEST := sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20250517180713-32e5e9e948a5
# mockgcp listens on 127.0.0.2 by default, which macOS lacks
# (patches/k8s-config-connector/0002-*).
export MOCKGCP_LISTEN_ADDRESS ?= 127.0.0.1:0

.PHONY: all licenses upstream upstream-check upstream-pr generate build test test-mock test-live live-sweep live-list check clean fake-cloud-up fake-cloud-test fake-cloud-down operator-e2e

all: check

upstream:
	hack/upstream.sh

# Read-only: is upstream's direct SQLUser (PR #7148) ready to replace ours?
upstream-pr:
	hack/upstream-pr.sh

upstream-check:
	hack/upstream.sh --check
	hack/check-go-mod.sh

generate: upstream-check
	hack/generate.sh

build: upstream-check
	go build -o $(BIN)/manager ./cmd/manager
	go build -o $(BIN)/operator ./cmd/operator
	go build -o $(BIN)/fakegcp ./cmd/fakegcp
	go build -o $(BIN)/fakecloud ./cmd/fakecloud

$(BIN)/setup-envtest:
	GOBIN=$(BIN) go install $(SETUP_ENVTEST)

ENVTEST_ASSETS = $$($(BIN)/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(BIN)/envtest -p path)

# The tests against the fake cloud, whatever E2E_GCP_TARGET says in the
# environment: make check never reaches a real project.
test test-mock: upstream-check $(BIN)/setup-envtest
	E2E_GCP_TARGET=mock KUBEBUILDER_ASSETS="$(ENVTEST_ASSETS)" go test ./... -count=1

# The same end-to-end tests, each kind against the fake cloud and then a
# real project (docs/live-runs.md): make test-live PROJECT=<project>
# [KINDS="sql artifactregistry"]. Only projects in hack/live/projects.sh;
# the credentials must be a service account (an offload VM's --sa).
test-live: upstream-check $(BIN)/setup-envtest
	KUBEBUILDER_ASSETS="$(ENVTEST_ASSETS)" hack/live/run.sh --project "$(PROJECT)" $(KINDS)

# Delete live-test leftovers older than 2h (or everything of RUN=<id>).
live-sweep:
	hack/live/sweep.sh --project "$(PROJECT)" $(if $(RUN),--run $(RUN))

# List every live-test resource in PROJECT.
live-list:
	hack/live/sweep.sh --project "$(PROJECT)" --list

# Everything that must hold before a commit: the checkout is the pin plus
# our patches, generated files are current, code is formatted and vetted,
# it builds, and the tests pass.
check: upstream-check
	@test -z "$$(gofmt -l apis cmd pkg)" || { gofmt -l apis cmd pkg; echo "gofmt needed"; exit 1; }
	hack/generate.sh
	@git diff --exit-code -- apis config || { echo "generated files are stale; commit the output of hack/generate.sh"; exit 1; }
	go vet ./...
	go vet -tags fakecloud ./cmd/manager
	hack/check-no-fake-cloud.sh
	hack/check-spdx.sh
	$(MAKE) build test

# The fake cloud: kind, Config Connector's CRDs and ours, and
# bin/fakecloud (Config Connector's manager and ours against mockgcp).
# docs/fake-cloud.md.
fake-cloud-up: upstream-check
	go build -o $(BIN)/fakecloud ./cmd/fakecloud
	hack/fake-cloud/up.sh

fake-cloud-test:
	hack/fake-cloud/test.sh

fake-cloud-down:
	hack/fake-cloud/down.sh

# The operator on kind with the in-cluster fake cloud: images by ko, Config
# Connector's operator footprint made by hand. tests/operator/run.sh.
operator-e2e: upstream-check
	NAME=$(NAME) tests/operator/run.sh

# docs/third-party-licenses.csv from go-licenses; fails on a licence outside
# Apache-2.0, MIT, BSD, ISC and MPL-2.0. Needs go-licenses on PATH.
licenses: upstream-check
	hack/licenses.sh

clean:
	rm -rf $(BIN)
