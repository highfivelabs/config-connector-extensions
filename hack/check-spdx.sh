#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
#
# Every Go file and shell script in the repository names its licence with
# an SPDX line near the top. Files copied from Config Connector keep
# Google's header and carry the line after it.
set -o errexit -o nounset -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"
missing=0
while IFS= read -r f; do
  [[ -f "${f}" ]] || continue
  if ! head -n 25 "${f}" | grep -q 'SPDX-License-Identifier: Apache-2.0'; then
    echo "no SPDX-License-Identifier line: ${f}" >&2
    missing=$((missing + 1))
  fi
done < <(git ls-files --cached --others --exclude-standard -- '*.go' '*.sh')
if [[ ${missing} -gt 0 ]]; then
  echo "${missing} file(s) without an SPDX line; add '// SPDX-License-Identifier: Apache-2.0'" >&2
  exit 1
fi
echo "ok: every Go file and shell script has an SPDX line"
