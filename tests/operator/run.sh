#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# The operator on kind, with the fake cloud: CRDs install, the managers run
# in Config Connector's namespace as their own service accounts carrying
# Config Connector's Workload Identity annotation, with only this project's
# rights, and gap kinds become Ready against the fake cloud, first in
# cluster mode, then in namespaced mode (one manager per
# ConfigConnectorContext, watching only its namespace). The opt-in to share
# Config Connector's service account is switched on and off.
#
# Config Connector itself is not installed: its operator's footprint that
# ours reads is made by hand (cnrm-system, its service accounts with the
# Workload Identity annotation, ConfigConnector, ConfigConnectorContext).
# Real Workload Identity cannot run on kind; what is checked is the identity
# a pod would present: its service account, and what that account may do.
#
#   tests/operator/run.sh          create the cluster, run, leave it up
#   KEEP=0 tests/operator/run.sh   delete the cluster at the end

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${REPO_ROOT}"

NAME="${NAME:-config-connector-extensions}"
MANAGER="${NAME}-manager"
CLUSTER="${CLUSTER:-${NAME}-operator}"
KEEP="${KEEP:-1}"
KCC="third_party/k8s-config-connector"
export KUBECONFIG="${REPO_ROOT}/bin/operator-e2e/kubeconfig"
mkdir -p "$(dirname "${KUBECONFIG}")"

step() { printf '\n== %s\n' "$*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok: $*"; }

# wait_for <seconds> <description> <command...>: retry until the command succeeds.
wait_for() {
  local secs="$1" what="$2"; shift 2
  for _ in $(seq 1 "${secs}"); do
    if "$@" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  fail "timed out after ${secs}s waiting for ${what}"
}
ready() { # ready <kind> <ns> <name>: Ready=True for the current generation
  local json
  json="$(kubectl get "$1" -n "$2" "$3" -o json)" || return 1
  python3 -I -c '
import json,sys
o=json.load(sys.stdin)
st=o.get("status",{})
ok=any(c.get("type")=="Ready" and c.get("status")=="True" for c in st.get("conditions",[]))
sys.exit(0 if ok and st.get("observedGeneration")==o["metadata"]["generation"] else 1)' <<<"${json}"
}

step "kind cluster ${CLUSTER}"
# grep without -q reads all its input: no SIGPIPE for kind under pipefail.
if ! kind get clusters | grep -x "${CLUSTER}" >/dev/null; then
  kind create cluster --name "${CLUSTER}" --kubeconfig "${KUBECONFIG}" --wait 60s
else
  kind export kubeconfig --name "${CLUSTER}" --kubeconfig "${KUBECONFIG}"
fi

step "Config Connector's operator CRDs and the footprint its operator would leave (cluster mode)"
kubectl apply --server-side -f "${KCC}/operator/config/crd/bases/" >/dev/null
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: Namespace
metadata:
  name: cnrm-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cnrm-controller-manager
  namespace: cnrm-system
  annotations:
    iam.gke.io/gcp-service-account: kcc@mock-project.iam.gserviceaccount.com
---
apiVersion: core.cnrm.cloud.google.com/v1beta1
kind: ConfigConnector
metadata:
  name: configconnector.core.cnrm.cloud.google.com
spec:
  mode: cluster
  googleServiceAccount: kcc@mock-project.iam.gserviceaccount.com
YAML

step "build images with ko and apply config-connector-extensions with the fake cloud"
arch="$(docker info --format '{{.Architecture}}')"
case "${arch}" in aarch64|arm64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; esac
# An empty Docker config for ko: the base image is public, and a desktop
# credential helper can block waiting on a keychain prompt.
ko_docker_config="$(mktemp -d)"
DOCKER_HOST="${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}"
export DOCKER_HOST
trap 'rm -rf "${ko_docker_config}"' EXIT
# GOFLAGS=-tags=fakecloud: the test images are the only builds with the
# manager's --fake-cloud-url (cmd/manager/fakecloud.go); a production build
# has no fake-cloud switch (hack/check-no-fake-cloud.sh).
kustomize build config/fake-cloud \
  | GOFLAGS=-tags=fakecloud DOCKER_CONFIG="${ko_docker_config}" KO_DOCKER_REPO=kind.local KIND_CLUSTER_NAME="${CLUSTER}" ko resolve --platform="linux/${arch}" -f - \
  > bin/operator-e2e/resolved.yaml
kubectl apply --server-side -f bin/operator-e2e/resolved.yaml >/dev/null
kubectl -n "${NAME}-system" rollout status deploy/fakegcp --timeout=180s
kubectl -n "${NAME}-system" rollout status deploy/"${NAME}-operator" --timeout=180s
pass "CRDs installed: $(kubectl get crd -l gcp.hfl.dev/gap=true -o name | tr '\n' ' ')"

can() { kubectl auth can-i "$@" 2>/dev/null || true; }
gsa_of() { kubectl -n cnrm-system get sa "$1" -o jsonpath='{.metadata.annotations.iam\.gke\.io/gcp-service-account}'; }
pod_sa() { kubectl -n cnrm-system get pod -l "$1" -o jsonpath='{.items[0].spec.serviceAccountName}'; }
GAP=secretmanagersecrets.secretmanager.gcp.hfl.dev

step "cluster mode: one manager in cnrm-system as its own service account with Config Connector's identity"
wait_for 60 "Deployment cnrm-system/${MANAGER}" kubectl -n cnrm-system get deploy "${MANAGER}"
kubectl -n cnrm-system rollout status deploy/"${MANAGER}" --timeout=180s
sa="$(pod_sa "app.kubernetes.io/name=${MANAGER}")"
[[ "${sa}" == "${MANAGER}" ]] || fail "manager pod runs as ${sa}, want its own ${MANAGER}"
[[ "$(gsa_of "${MANAGER}")" == "$(gsa_of cnrm-controller-manager)" ]] \
  || fail "own service account's Google identity '$(gsa_of "${MANAGER}")' is not Config Connector's '$(gsa_of cnrm-controller-manager)'"
pass "manager pod runs as cnrm-system/${MANAGER}, mapped like Config Connector's to $(gsa_of "${MANAGER}")"
me="system:serviceaccount:cnrm-system:${MANAGER}"
kcc="system:serviceaccount:cnrm-system:cnrm-controller-manager"
[[ "$(can create "${GAP}" -n example --as="${me}")" == yes ]] || fail "the manager may not manage gap kinds"
[[ "$(can create secrets -n example --as="${me}")" == no ]] || fail "the manager may create Secrets"
[[ "$(can patch configconnectors.core.cnrm.cloud.google.com --as="${me}")" == no ]] || fail "the manager may change ConfigConnector"
[[ "$(can create validatingwebhookconfigurations.admissionregistration.k8s.io --as="${me}")" == no ]] || fail "the manager may create webhooks"
pass "the manager may manage gap kinds, and may not create Secrets, change ConfigConnector or create webhooks"
[[ "$(can create "${GAP}" -n example --as="${kcc}")" == no ]] || fail "Config Connector's service account may manage gap kinds"
pass "Config Connector's service account has no rights on gap kinds"

step "cluster mode: a regional secret and its version become Ready"
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: Namespace
metadata:
  name: example
  annotations:
    cnrm.cloud.google.com/project-id: mock-project
---
apiVersion: v1
kind: Secret
metadata:
  name: db
  namespace: example
stringData:
  password: s3cret
---
apiVersion: secretmanager.gcp.hfl.dev/v1beta1
kind: SecretManagerSecret
metadata:
  name: app-db-password
  namespace: example
  annotations:
    cnrm.cloud.google.com/project-id: mock-project
spec:
  location: us-central1
---
apiVersion: secretmanager.gcp.hfl.dev/v1beta1
kind: SecretManagerSecretVersion
metadata:
  name: app-db-password-1
  namespace: example
  annotations:
    cnrm.cloud.google.com/project-id: mock-project
spec:
  secretRef:
    name: app-db-password
  secretData:
    valueFrom:
      secretKeyRef:
        name: db
        key: password
YAML
wait_for 90 "SecretManagerSecret example/app-db-password Ready" ready secretmanagersecrets.secretmanager.gcp.hfl.dev example app-db-password
wait_for 90 "SecretManagerSecretVersion example/app-db-password-1 Ready" ready secretmanagersecretversions.secretmanager.gcp.hfl.dev example app-db-password-1
pass "Ready: $(kubectl -n example get secretmanagersecrets.secretmanager.gcp.hfl.dev app-db-password -o jsonpath='{.status.externalRef}')"

step "the opt-in: share Config Connector's service account, then back"
kubectl annotate configconnector configconnector.core.cnrm.cloud.google.com gcp.hfl.dev/identity=config-connector --overwrite >/dev/null
wait_for 90 "the manager to run as cnrm-controller-manager" bash -c "[[ \"\$(kubectl -n cnrm-system get deploy ${MANAGER} -o jsonpath='{.spec.template.spec.serviceAccountName}')\" == cnrm-controller-manager ]]"
kubectl -n cnrm-system rollout status deploy/"${MANAGER}" --timeout=180s
wait_for 60 "the manager's own service account to go" bash -c "! kubectl -n cnrm-system get sa ${MANAGER}"
[[ "$(can create "${GAP}" -n example --as="${kcc}")" == yes ]] || fail "sharing, Config Connector's service account may not manage gap kinds"
pass "shared: the manager runs as cnrm-controller-manager, its own service account is gone"
kubectl annotate configconnector configconnector.core.cnrm.cloud.google.com gcp.hfl.dev/identity- >/dev/null
wait_for 90 "the manager to run as ${MANAGER} again" bash -c "[[ \"\$(kubectl -n cnrm-system get deploy ${MANAGER} -o jsonpath='{.spec.template.spec.serviceAccountName}')\" == ${MANAGER} ]]"
kubectl -n cnrm-system rollout status deploy/"${MANAGER}" --timeout=180s
[[ "$(can create "${GAP}" -n example --as="${kcc}")" == no ]] || fail "back to own: Config Connector's service account still manages gap kinds"
pass "back to its own service account; Config Connector's has no rights on gap kinds again"

step "namespaced mode: Config Connector's operator moves to one manager per context"
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: Namespace
metadata:
  name: team-a
  annotations:
    cnrm.cloud.google.com/project-id: mock-project
---
apiVersion: v1
kind: Namespace
metadata:
  name: team-b
  annotations:
    cnrm.cloud.google.com/project-id: mock-project
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cnrm-controller-manager-team-a
  namespace: cnrm-system
  annotations:
    iam.gke.io/gcp-service-account: cnrm-system-team-a@mock-project.iam.gserviceaccount.com
---
apiVersion: core.cnrm.cloud.google.com/v1beta1
kind: ConfigConnectorContext
metadata:
  name: configconnectorcontext.core.cnrm.cloud.google.com
  namespace: team-a
spec:
  googleServiceAccount: cnrm-system-team-a@mock-project.iam.gserviceaccount.com
YAML
kubectl patch configconnector configconnector.core.cnrm.cloud.google.com --type=merge -p '{"spec":{"mode":"namespaced","googleServiceAccount":null}}' >/dev/null
wait_for 60 "the cluster-mode manager to go" bash -c "! kubectl -n cnrm-system get deploy ${MANAGER}"
wait_for 60 "Deployment cnrm-system/${MANAGER}-team-a" kubectl -n cnrm-system get deploy "${MANAGER}-team-a"
kubectl -n cnrm-system rollout status deploy/"${MANAGER}-team-a" --timeout=180s
pod_json="$(kubectl -n cnrm-system get pod -l gcp.hfl.dev/scoped-namespace=team-a -o json)"
sa="$(python3 -I -c 'import json,sys; print(json.load(sys.stdin)["items"][0]["spec"]["serviceAccountName"])' <<<"${pod_json}")"
args="$(python3 -I -c 'import json,sys; print(" ".join(json.load(sys.stdin)["items"][0]["spec"]["containers"][0]["args"]))' <<<"${pod_json}")"
[[ "${sa}" == "${MANAGER}-team-a" ]] || fail "team-a's manager runs as ${sa}"
[[ " ${args} " == *" --scoped-namespace=team-a "* ]] || fail "team-a's manager args: ${args}"
[[ "$(gsa_of "${MANAGER}-team-a")" == "$(gsa_of cnrm-controller-manager-team-a)" ]] \
  || fail "team-a's own service account's Google identity '$(gsa_of "${MANAGER}-team-a")' is not the context's"
pass "team-a's manager runs as cnrm-system/${MANAGER}-team-a, mapped like Config Connector's to $(gsa_of "${MANAGER}-team-a"), with --scoped-namespace=team-a"
me_a="system:serviceaccount:cnrm-system:${MANAGER}-team-a"
[[ "$(can create "${GAP}" -n team-a --as="${me_a}")" == yes ]] || fail "team-a's manager may not manage gap kinds in team-a"
[[ "$(can create "${GAP}" -n team-b --as="${me_a}")" == no ]] || fail "team-a's manager may manage gap kinds in team-b"
[[ "$(can create "${GAP}" -n team-a --as=system:serviceaccount:cnrm-system:cnrm-controller-manager-team-a)" == no ]] \
  || fail "Config Connector's team-a service account may manage gap kinds"
pass "team-a's manager may manage gap kinds in team-a and not in team-b; Config Connector's team-a account has no rights on them"

for ns in team-a team-b; do
kubectl apply -f - <<YAML
apiVersion: secretmanager.gcp.hfl.dev/v1beta1
kind: SecretManagerSecret
metadata:
  name: app-config
  namespace: ${ns}
  annotations:
    cnrm.cloud.google.com/project-id: mock-project
spec:
  location: us-central1
YAML
done
wait_for 90 "SecretManagerSecret team-a/app-config Ready" ready secretmanagersecrets.secretmanager.gcp.hfl.dev team-a app-config
pass "team-a/app-config Ready through team-a's manager"
sleep 15
status="$(kubectl -n team-b get secretmanagersecrets.secretmanager.gcp.hfl.dev app-config -o jsonpath='{.status}')"
[[ -z "${status}" ]] || fail "team-b has no context, yet its secret was reconciled: ${status}"
pass "team-b/app-config untouched: no context, no manager"

step "done"
if [[ "${KEEP}" == 0 ]]; then
  kind delete cluster --name "${CLUSTER}"
else
  echo "cluster ${CLUSTER} left up; KUBECONFIG=${KUBECONFIG}; delete with: kind delete cluster --name ${CLUSTER}"
fi
# The last line of the output: hack/operator-e2e.sh fails without it.
echo "all checks passed"
