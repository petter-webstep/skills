#!/usr/bin/env bash
# Fails if this PR changes code but doesn't touch USER_CHANGELOG.md, unless
# the PR is labeled to skip the check. Pure heuristic, no AI/API calls.
#
# Usage: check-user-changelog.sh <base-ref> <head-ref> <space-separated-labels>
#
# Customize NON_GATED_PATTERNS below for this repo (paths that never need a
# changelog entry: CI config, docs, tests, lockfiles, etc).
set -euo pipefail

BASE="${1:?Usage: check-user-changelog.sh <base-ref> <head-ref> <labels>}"
HEAD="${2:?Usage: check-user-changelog.sh <base-ref> <head-ref> <labels>}"
LABELS="${3:-}"

SKIP_LABEL="no-changelog"
CHANGELOG_FILE="USER_CHANGELOG.md"

# Labels that exempt a PR from this check entirely.
for label in $LABELS; do
  if [ "$label" = "$SKIP_LABEL" ]; then
    echo "Skipping: PR labeled '${SKIP_LABEL}'."
    exit 0
  fi
done

# Paths that never require a changelog entry on their own.
NON_GATED_PATTERNS=(
  '^\.github/'
  '^docs/'
  '\.md$'
  '^tests?/'
  '^spec/'
  '(^|/)package-lock\.json$'
  '(^|/)yarn\.lock$'
  '(^|/)pnpm-lock\.yaml$'
)

mapfile -t CHANGED < <(git diff --name-only "${BASE}" "${HEAD}")

if [ "${#CHANGED[@]}" -eq 0 ]; then
  echo "No changed files detected; nothing to check."
  exit 0
fi

GATED=()
for f in "${CHANGED[@]}"; do
  skip=0
  for pat in "${NON_GATED_PATTERNS[@]}"; do
    if [[ "$f" =~ $pat ]]; then
      skip=1
      break
    fi
  done
  [ "$skip" -eq 0 ] && GATED+=("$f")
done

if [ "${#GATED[@]}" -eq 0 ]; then
  echo "Only non-gated paths changed; no changelog entry required."
  exit 0
fi

for f in "${CHANGED[@]}"; do
  if [ "$f" = "$CHANGELOG_FILE" ]; then
    echo "${CHANGELOG_FILE} was updated. OK."
    exit 0
  fi
done

echo "::error::This PR changes code but does not update ${CHANGELOG_FILE}." >&2
echo "If this change has no user-facing impact, add the '${SKIP_LABEL}' label instead." >&2
echo "Otherwise, run the user-facing-changelog skill to add an entry under '## Unreleased'." >&2
exit 1
