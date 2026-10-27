---
name: user-facing-changelog
description: Writes customer-facing changelog entries in plain, non-technical language for end users, filtering out everything except new features, major visual overhauls, and bugs that blocked users from completing a task. Supports two triggers — per merged PR (preferred, needs an automated/org-level trigger) and per release (batch fallback). Use when the user wants a changelog, release notes, or "what's new" entry meant to be shown to end users/customers, as opposed to a developer/commit changelog.
---

# User-Facing Changelog

Turns changes into a short list of changelog entries written **for end
users**, using the product's own domain vocabulary instead of coding terms.
Maintains a `USER_CHANGELOG.md` with an `## Unreleased` section that entries
accumulate into, which gets stamped with a version/date at release time.

## Which workflow to use

- **Per-PR** (preferred): one PR just merged and you have an automated
  trigger (e.g. a GitHub Actions step or an org-level Copilot agent hook
  invoked on PR merge).
- **Per-release** (fallback): no per-PR automation exists, or the user asks
  for a changelog covering a range/tag directly.

If unsure which applies, ask the user. Step-by-step instructions for both
are in [WORKFLOWS.md](WORKFLOWS.md).

## First run in a repo: offer to set up the CI reminder gate

Before (or instead of) running a workflow, check if
`.github/workflows/user-changelog-check.yml` exists in the current repo. If
not, offer to install the no-AI-cost CI gate described in
[ENFORCEMENT.md](ENFORCEMENT.md), which fails a PR if code changed without a
`USER_CHANGELOG.md` update (skippable via a `no-changelog` label). This
keeps the habit enforced without needing org-funded automation.

## Shared rule: only truly user-facing changes survive

Keep an item only if it is one of:
- **A new feature or capability** the user can now do that they couldn't before.
- **A major visual/UX overhaul** — a redesigned screen, flow, or interaction
  pattern. Not a color tweak, spacing adjustment, or icon swap.
- **A bug fix that blocked or hindered a user task** — stopped them from
  completing work, caused data loss, crashed, or produced wrong results
  they'd notice.

Drop everything else: internal refactors, dependency bumps, CI/build/test/
tooling, performance work with no noticeable effect, minor visual polish,
backend-only/API/schema changes. When in doubt: "Would an end user notice
and care, unprompted?" If no, drop it.

## Shared rule: rewrite in domain language, not technical language

Read the project's domain vocabulary before writing, if available:
- `docs/glossary.md` — preferred terms for product concepts.
- `docs/domain-model.md` — the entities/relationships the product models.

Use these to name things the way users do (domain nouns like "invoice",
"shipment", "booking" are fine) instead of internal names (table/class
names, variable names, ticket IDs). Avoid engineering language entirely: no
"refactor", "endpoint", "API", "schema", "null check", "cache", "backend/
frontend", "deploy", "migration", commit hashes, or file paths. One short,
active-voice sentence per entry, from the user's point of view.

Examples:
- `fix(checkout): handle null discount code on retry` → "Fixed an issue where applying a discount code twice could cause checkout to fail."
- `feat(reports): add PDF export` → "You can now export your reports as PDF."
- `style: adjust button padding by 2px` / `refactor: extract invoice service` → **drop**.

## Shared rule: verify every candidate against the diff

PR titles and commit subjects are often imprecise, overstated, or wrong
(e.g. "fix login" that only touches a log message, or a "refactor" that
also adds a user-visible option). Never decide to keep, drop, or word an
entry from the title alone. For **every** candidate, before filtering and
rewriting:

1. Read the actual diff (`git show <hash>` / `gh pr diff <n>`), plus the PR
   body if there is one.
2. Work out what changed *for the user*: which screen, flow, or behavior,
   and what they can now do or no longer hit.
3. Base the filter decision and the wording on the diff. If the title and
   the diff disagree, the diff wins.
4. If the diff is too large or unclear to judge confidently, say so in your
   output and flag the item for the user to confirm instead of guessing.

Only skip the diff for items that are obviously noise by type (chore/ci/
build/test/docs) **and** whose diff stat confirms they touch no
user-facing code.
