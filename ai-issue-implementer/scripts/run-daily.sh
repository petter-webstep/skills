#!/usr/bin/env bash
# Daily cron entry point for the ai-issue-implementer skill.
#
# This script is meant to run on the HOST (via `crontab -e`), not inside a
# sandbox session — it spins up a fresh, isolated docker sandbox, runs the
# skill headlessly inside it once, then tears the sandbox down. It exists
# because sandbox sessions are ephemeral and cannot schedule their own
# recurrence. A disposable sandbox is fine here because the skill itself
# tracks throughput (open-PR/day guards) via GitHub issue/PR state, not any
# local file, so nothing needs to persist between runs.
#
# Setup:
#   1. Edit REPO_PATH below to point at the target repo's absolute path.
#   2. chmod +x this script.
#   3. Verify the `sbx create`/`sbx exec`/`sbx rm` invocations below against
#      `sbx --help` on your host — flag names may differ across sbx versions.
#   4. Add a crontab entry (host machine, `crontab -e`) for daily at 11:00
#      Europe/Oslo time. Most Linux cron implementations (cronie/vixie-cron)
#      support a per-line `CRON_TZ=` prefix so the schedule stays correct
#      across CET/CEST DST changes without manual adjustment:
#        CRON_TZ=Europe/Oslo
#        0 11 * * * /absolute/path/to/run-daily.sh >> ~/.ai-issue-implementer-logs/cron.log 2>&1
#      If your cron doesn't support CRON_TZ (e.g. macOS's built-in cron),
#      set TZ for the whole crontab instead:
#        TZ=Europe/Oslo
#        0 11 * * * /absolute/path/to/run-daily.sh >> ~/.ai-issue-implementer-logs/cron.log 2>&1
#
set -euo pipefail

# ---- Configuration (edit before enabling) ----
REPO_PATH="/Users/elveblest/code/equinor/winc"   # PLACEHOLDER: absolute path to the target repo on the host
SANDBOX_NAME="ai-issue-implementer-daily"
LOG_DIR="${HOME}/.ai-issue-implementer-logs"
# -----------------------------------------------

mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_DIR}/$(date -u +%Y-%m-%dT%H-%M-%SZ).log"

echo "=== ai-issue-implementer daily run: $(date -u) ===" | tee -a "${LOG_FILE}"

# Clean up any stale sandbox left over from a previous failed run.
sbx rm "${SANDBOX_NAME}" >/dev/null 2>&1 || true

# Spin up a fresh sandbox cloned from the target repo.
sbx create copilot "${REPO_PATH}" --name "${SANDBOX_NAME}" --clone >> "${LOG_FILE}" 2>&1

cleanup() {
  sbx rm "${SANDBOX_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Run the skill headlessly inside the sandbox. copilot builds scripts/aitool
# itself on first use inside the sandbox (see skill Quick start).
if sbx exec "${SANDBOX_NAME}" -- copilot -p \
  "Run the ai-issue-implementer skill against this repository: build scripts/aitool if needed, detect the repo, check throughput limits, find eligible issues, and implement up to the allowed number of issues as PRs, following the skill's workflow exactly." \
  --allow-all-tools >> "${LOG_FILE}" 2>&1; then
  echo "Run succeeded" | tee -a "${LOG_FILE}"
else
  echo "Run FAILED — see ${LOG_FILE}" | tee -a "${LOG_FILE}"
  exit 1
fi
