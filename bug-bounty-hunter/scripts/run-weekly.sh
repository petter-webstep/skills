#!/usr/bin/env bash
# Weekly cron entry point for the bug-bounty-hunter skill.
#
# This script runs on the HOST (via `crontab -e`), not inside a sandbox
# session. Each run creates a FRESH, disposable sandbox (like the
# ai-issue-implementer's daily job) and destroys it when done — there is no
# need to keep a sandbox alive between runs, because the only thing that
# must persist across weeks (bountyhunter's `.bounty-hunter/state.json`,
# plus `BUGS.md` if using --report-mode=local) is kept in a plain directory
# on the HOST and injected into the sandbox as a volume/bind mount. That
# also means state survives even if a run crashes mid-way or the sandbox is
# deleted out from under it.
#
# The target repo itself is mounted directly (NOT cloned) so the mounted
# .bounty-hunter/BUGS.md volumes below line up at the exact same absolute
# path inside the sandbox as on the host, and so the scan always sees the
# latest working tree without needing a separate git-fetch/sync step.
# bountyhunter is a read-only auditor (see SKILL.md) and never edits source
# files, so mounting the real repo directly is safe.
#
# Setup:
#   1. Edit REPO_PATH and STATE_DIR below.
#   2. chmod +x this script.
#   3. Verify the `sbx create`/`sbx exec`/`sbx rm` invocations against
#      `sbx --help` and `sbx create --help` on your host — exact flag names
#      for "direct mode" (no --clone) and for bind-mounting a host path into
#      the sandbox (`--volume`/`-v`/`--mount`, HOST:CONTAINER syntax) may
#      differ across sbx versions. Adjust the `sbx create` call accordingly.
#   4. Add a crontab entry (host machine, `crontab -e`) for Monday 11:00
#      Europe/Oslo time. Most Linux cron implementations (cronie/vixie-cron)
#      support a per-line `CRON_TZ=` prefix so the schedule stays correct
#      across CET/CEST DST changes without manual adjustment:
#        CRON_TZ=Europe/Oslo
#        0 11 * * 1 /absolute/path/to/run-weekly.sh >> ~/.bug-bounty-hunter-logs/cron.log 2>&1
#      If your cron doesn't support CRON_TZ (e.g. macOS's built-in cron),
#      set TZ for the whole crontab instead:
#        TZ=Europe/Oslo
#        0 11 * * 1 /absolute/path/to/run-weekly.sh >> ~/.bug-bounty-hunter-logs/cron.log 2>&1
#   5. Add `.bounty-hunter/` and `BUGS.md` to the target repo's .gitignore
#      (see REFERENCE.md) — this state is local/host-side, never committed.
set -euo pipefail

# ---- Configuration (edit before enabling) ----
REPO_PATH="/path/to/your/repo"   # PLACEHOLDER: absolute path to the target repo on the host
SANDBOX_NAME="bug-bounty-hunter-weekly"
LOG_DIR="${HOME}/.bug-bounty-hunter-logs"
STATE_DIR="${HOME}/.bug-bounty-hunter-state/$(basename "${REPO_PATH}")"  # host-side, persists across runs
MAX_HOURS="6"
MAX_ISSUES="10"       # cap on new issues/entries filed in this run
REPORT_MODE="auto"    # auto | github | local — see skill SKILL.md
# -----------------------------------------------

mkdir -p "${LOG_DIR}"
mkdir -p "${STATE_DIR}/.bounty-hunter"
if [[ "${REPORT_MODE}" == "local" ]]; then
  touch "${STATE_DIR}/BUGS.md"   # bind mounts of a file generally require it to pre-exist on the host
fi
LOG_FILE="${LOG_DIR}/$(date -u +%Y-%m-%dT%H-%M-%SZ).log"

echo "=== bug-bounty-hunter weekly run: $(date -u) ===" | tee -a "${LOG_FILE}"

# Clean up any stale sandbox left over from a previous failed run.
sbx rm "${SANDBOX_NAME}" >/dev/null 2>&1 || true

# Create a disposable sandbox with the target repo mounted directly (same
# path as the host), plus the persistent state directory bind-mounted at
# exactly <repo>/.bounty-hunter (and BUGS.md, if local report mode), so
# bountyhunter reads/writes its state on the host, not inside the sandbox.
VOLUME_ARGS=(
  --volume "${REPO_PATH}:${REPO_PATH}"
  --volume "${STATE_DIR}/.bounty-hunter:${REPO_PATH}/.bounty-hunter"
)
if [[ "${REPORT_MODE}" == "local" ]]; then
  VOLUME_ARGS+=(--volume "${STATE_DIR}/BUGS.md:${REPO_PATH}/BUGS.md")
fi

sbx create "${SANDBOX_NAME}" --dir "${REPO_PATH}" "${VOLUME_ARGS[@]}" >> "${LOG_FILE}" 2>&1

cleanup() {
  sbx rm "${SANDBOX_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Ask copilot to run the skill itself: it locates/builds the bountyhunter
# binary from the skill's own scripts/bountyhunter/ dir and points --repo at
# the mounted repo. --max-hours bounds the run so it can't overrun into next
# week's cron tick; --max-issues caps how many new issues/entries get filed
# in one run so a single scan can't flood the tracker. bountyhunter resumes
# from the host-mounted .bounty-hunter/state.json next time either way.
if sbx exec "${SANDBOX_NAME}" -- copilot -p \
  "Run the bug-bounty-hunter skill against the repository at ${REPO_PATH}: build scripts/bountyhunter if needed, then run 'bountyhunter --repo \"${REPO_PATH}\" run --max-hours ${MAX_HOURS} --max-issues ${MAX_ISSUES} --report-mode ${REPORT_MODE}' and let it drive the scan to completion or until a cap is hit." \
  --allow-all-tools >> "${LOG_FILE}" 2>&1; then
  echo "Run succeeded" | tee -a "${LOG_FILE}"
else
  echo "Run FAILED — see ${LOG_FILE}" | tee -a "${LOG_FILE}"
  exit 1
fi
