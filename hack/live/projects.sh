# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=bash
#
# The only Google Cloud projects live tests may touch (docs/live-runs.md).
# Sourced by hack/live/run.sh and hack/live/sweep.sh. The projects come
# from the environment (.env locally, `env NAME=...` on an offload VM's
# command line; .env.example names them), never from the command line:
#   GCP_PROJECT                 the live-test project (required)
#   GCP_LIVE_PROJECT_PATTERN    optional: a bash regexp for the test
#                               estate's generation projects and their
#                               scratch projects (factory/estate/gcp), in
#                               the live-tests folder; the live-test
#                               account of a generation has roles only there
: "${GCP_PROJECT:?set GCP_PROJECT, the live-test project (see .env.example)}"
LIVE_PROJECTS=("${GCP_PROJECT}")
LIVE_PROJECT_PATTERN="${GCP_LIVE_PROJECT_PATTERN:-}"

live_project_allowed() {
  local p
  if [[ -z "${1:-}" ]]; then
    echo "live: no project given (PROJECT=..., else HFL_GCP_TEST_PROJECT or GCP_PROJECT)" >&2
    return 1
  fi
  for p in "${LIVE_PROJECTS[@]}"; do
    [[ "$1" == "${p}" ]] && return 0
  done
  if [[ -n "${LIVE_PROJECT_PATTERN}" && "$1" =~ ${LIVE_PROJECT_PATTERN} ]]; then return 0; fi
  echo "live: refusing project $1: live tests may touch only GCP_PROJECT (${LIVE_PROJECTS[*]})${LIVE_PROJECT_PATTERN:+ and GCP_LIVE_PROJECT_PATTERN (${LIVE_PROJECT_PATTERN})} (hack/live/projects.sh)" >&2
  return 1
}
