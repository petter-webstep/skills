# Enforcement (CI gate, no AI cost)

Chosen tier: **CI gate reminder only** — a GitHub Actions check fails a PR
if it changes code without touching `USER_CHANGELOG.md`'s `## Unreleased`
section, unless labeled `no-changelog`. The check itself is a pure bash
heuristic (no LLM calls, no token spend) — only the *actual changelog entry*
is written by a human using this skill (e.g. via Copilot CLI, at their own
cost), not by automation.

## First-run setup

The first time this skill is used in a given repo, check whether the gate
is already installed:

- `.github/workflows/user-changelog-check.yml`
- `.github/scripts/check-user-changelog.sh`

If missing, offer to install them (don't do it silently/unprompted):

1. Copy `templates/check-user-changelog.sh` from this skill to
   `.github/scripts/check-user-changelog.sh` in the target repo, `chmod +x`
   it, and adjust `NON_GATED_PATTERNS` for that repo's layout (e.g. its own
   docs/test/lockfile paths).
2. Copy `templates/user-changelog-check.yml` from this skill to
   `.github/workflows/user-changelog-check.yml` in the target repo as-is.
3. Ensure `USER_CHANGELOG.md` exists at the repo root with at least an
   `# Changelog` title and an empty `## Unreleased` section, so the first
   gated PR has something to append to.
4. Tell the user a `no-changelog` label should exist on the repo (create it
   via `gh label create no-changelog --description "No user-facing change" --color ededed`
   if it doesn't) — PRs with no user-facing impact use it to skip the gate.
5. Mention this is a reminder gate, not a content check: it can't tell if
   the changelog edit is *good*, only that one was made. The quality bar
   (filtering, domain language) is on whoever writes the entry — point them
   at this skill.

## Notes

- This gate only enforces that *a* human remembered to update the
  changelog — it does not run Workflow A/B itself and does not require any
  AI/API key, matching the "no org-funded automation yet" constraint.
- If org-level Copilot agents or an org-owned LLM key become available
  later, this gate can be upgraded to actually invoke Workflow A instead of
  just reminding a human to.
