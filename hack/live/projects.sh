# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=bash
#
# The only Google Cloud projects live tests may touch (docs/live-runs.md).
# Sourced by hack/live/run.sh and hack/live/sweep.sh; a project is added
# here by a reviewed change, never from the command line.
LIVE_PROJECTS=(hfl-dev-514205)

live_project_allowed() {
  local p
  if [[ -z "${1:-}" ]]; then
    echo "live: no project given (PROJECT=...)" >&2
    return 1
  fi
  for p in "${LIVE_PROJECTS[@]}"; do
    [[ "$1" == "${p}" ]] && return 0
  done
  echo "live: refusing project $1: live tests may touch only ${LIVE_PROJECTS[*]} (hack/live/projects.sh)" >&2
  return 1
}
