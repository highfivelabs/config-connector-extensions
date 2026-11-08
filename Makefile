# gaps-gcp. Every target needs the pinned Config Connector checkout;
# `make upstream` fetches it (about 650 MB, not committed).

export GOTOOLCHAIN ?= local

BIN := $(CURDIR)/bin
ENVTEST_K8S_VERSION := 1.33.x
# setup-envtest from controller-runtime's release-0.20 branch (the
# controller-runtime Config Connector pins), as resolved on 2026-11-07.
SETUP_ENVTEST := sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20250517180713-32e5e9e948a5
# mockgcp listens on 127.0.0.2 by default, which macOS lacks
# (patches/k8s-config-connector/0002-*).
export MOCKGCP_LISTEN_ADDRESS ?= 127.0.0.1:0

.PHONY: all upstream upstream-check upstream-pr generate build test check clean fake-cloud-up fake-cloud-test fake-cloud-down operator-e2e

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

test: upstream-check $(BIN)/setup-envtest
	KUBEBUILDER_ASSETS="$$($(BIN)/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(BIN)/envtest -p path)" \
		go test ./... -count=1

# Everything that must hold before a commit: the checkout is the pin plus
# our patches, generated files are current, code is formatted and vetted,
# it builds, and the tests pass.
check: upstream-check
	@test -z "$$(gofmt -l apis cmd pkg)" || { gofmt -l apis cmd pkg; echo "gofmt needed"; exit 1; }
	hack/generate.sh
	@git diff --exit-code -- apis config || { echo "generated files are stale; commit the output of hack/generate.sh"; exit 1; }
	go vet ./...
	$(MAKE) build test

# The fake cloud for Bundt's L2: kind, Config Connector's CRDs and ours, and
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
	tests/operator/run.sh

clean:
	rm -rf $(BIN)
