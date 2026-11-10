# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=bash
#
# The only Google Cloud projects live tests may touch (docs/live-runs.md).
# Sourced by hack/live/run.sh and hack/live/sweep.sh; a project is added
# here by a reviewed change, never from the command line.
LIVE_PROJECTS=(our-chassis-514316-r0)
# And the test estate's generations (factory/estate/gcp): hfl-test-g1,
# hfl-test-g2, ... and their scratch projects, in the live-tests folder.
# The live-test account of a generation has roles only there.
LIVE_PROJECT_PATTERN='^hfl-test-g[0-9]+(-scratch)?$'

live_project_allowed() {
  local p
  if [[ -z "${1:-}" ]]; then
    echo "live: no project given (PROJECT=...)" >&2
    return 1
  fi
  for p in "${LIVE_PROJECTS[@]}"; do
    [[ "$1" == "${p}" ]] && return 0
  done
  [[ "$1" =~ ${LIVE_PROJECT_PATTERN} ]] && return 0
  echo "live: refusing project $1: live tests may touch only ${LIVE_PROJECTS[*]} and the test estate's ${LIVE_PROJECT_PATTERN} (hack/live/projects.sh)" >&2
  return 1
}
