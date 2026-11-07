#!/usr/bin/env bash
# Watch Config Connector PR #7148 (upstream's direct SQLUser), the change we
# would copy instead of our own SQLUser controller once it does what we need.
# Read-only: fetches the PR head into the checkout's object store and reports.
#
# The PR number and the head we last read are the "Watched PR" rows of
# UPSTREAM.md. The PR is adequate when its SQLUser types carry databaseRoles
# uncommented (it was `/*NOTYET ... */` at eb5d176a). When it is:
#   1. set "PR head" in UPSTREAM.md to the new SHA;
#   2. follow "Switching to PR #7148" in docs/plan.md.
#
#   hack/upstream-pr.sh   fetch refs/pull/<n>/head and report what changed

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

PR="$(sed -n 's/^| Watched PR | #\([0-9]*\).*/\1/p' UPSTREAM.md | head -1)"
RECORDED="$(sed -n 's/^| PR head | `\([0-9a-f]\{40\}\)`.*/\1/p' UPSTREAM.md | head -1)"
DEST="third_party/k8s-config-connector"
TYPES="apis/sql/v1beta1/sqluser_types.go"

if [[ -z "${PR}" || -z "${RECORDED}" ]]; then
  echo "could not read the watched PR and its head from UPSTREAM.md" >&2
  exit 1
fi
if [[ ! -d "${DEST}/.git" ]]; then
  echo "${DEST} is missing; run hack/upstream.sh" >&2
  exit 1
fi

ref="refs/remotes/pr/${PR}"
git -C "${DEST}" fetch --quiet --depth 1 origin "+refs/pull/${PR}/head:${ref}"
head="$(git -C "${DEST}" rev-parse "${ref}")"
echo "PR #${PR}: head ${head} (UPSTREAM.md records ${RECORDED})"
[[ "${head}" == "${RECORDED}" ]] && echo "  unchanged since it was recorded"

types="$(git -C "${DEST}" show "${ref}:${TYPES}" 2>/dev/null || true)"
if [[ -z "${types}" ]]; then
  echo "  ${TYPES} is not in the PR head: re-read the PR by hand"
  exit 0
fi
# databaseRoles is adequate when its field is declared outside a /*NOTYET block.
if awk '/\/\*NOTYET/{c=1} /\*\//{c=0; next} !c && /DatabaseRoles +\[\]string/{f=1} END{exit !f}' <<<"${types}"; then
  echo "  databaseRoles is live in the PR: it may now do what we need (docs/plan.md, 'Switching to PR #${PR}')"
else
  echo "  databaseRoles is still commented out (/*NOTYET*/): keep our controller"
fi
