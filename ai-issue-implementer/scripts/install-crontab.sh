#!/usr/bin/env bash
# Installs (or updates) the ai-issue-implementer weekly cron job into the
# current user's crontab on the HOST machine.
#
# Usage:
#   ./install-crontab.sh
#
# Safe to re-run: it strips any previously-installed block for this job
# (marked by the BEGIN/END markers below) before re-adding it, so it won't
# duplicate entries.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRONTAB_FRAGMENT="${SCRIPT_DIR}/ai-issue-implementer.crontab"
BEGIN_MARKER="# >>> ai-issue-implementer weekly cron >>>"
END_MARKER="# <<< ai-issue-implementer weekly cron <<<"

if [[ ! -f "${CRONTAB_FRAGMENT}" ]]; then
  echo "ERROR: ${CRONTAB_FRAGMENT} not found" >&2
  exit 1
fi

# Grab the existing crontab (empty if none set yet), then drop any block
# from a previous install of this job.
EXISTING="$(crontab -l 2>/dev/null || true)"
STRIPPED="$(printf '%s\n' "${EXISTING}" | awk -v b="${BEGIN_MARKER}" -v e="${END_MARKER}" '
  $0 == b {skip=1; next}
  $0 == e {skip=0; next}
  skip {next}
  {print}
')"

{
  printf '%s\n' "${STRIPPED}"
  echo "${BEGIN_MARKER}"
  cat "${CRONTAB_FRAGMENT}"
  echo "${END_MARKER}"
} | crontab -

echo "Installed. Current crontab:"
crontab -l
