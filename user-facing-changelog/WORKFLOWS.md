# Workflows

Both workflows apply the shared filtering rule and the shared
domain-language rewrite rule from `SKILL.md` to every candidate change
before it's written anywhere.

## Workflow A: Per-PR

1. Read the merged PR's title, description, and diff. The diff is the
   source of truth — the title may be imprecise (see "verify every
   candidate against the diff" in `SKILL.md`).
2. Apply the shared filter. If not user-facing, stop — make no changes.
3. Open `USER_CHANGELOG.md`'s `## Unreleased` section (create the file/
   section if missing). Read its existing bullet entries.
4. **Grouping**: if this PR is clearly part of the same user-facing story as
   an existing `Unreleased` entry (same feature/area/domain entity — e.g.
   several PRs that together redesign the settings page), *edit that entry*
   to reflect the fuller picture instead of adding a duplicate line. Only
   add a new line when it's a genuinely distinct change.
5. Rewrite per the shared domain-language rule and place it under the right
   subheading (`### New`, `### Improved`, `### Fixed`) within `Unreleased`.
6. Save. This should be a small, self-contained change (ideally committed
   alongside or immediately after the PR it documents).

## Workflow B: Per-release

1. Determine the range: ask the user for the tag/ref range if not given —
   don't assume a tagging convention, since it varies per repo.
2. If `USER_CHANGELOG.md` already has an `## Unreleased` section with
   entries (from Workflow A having run on each PR), promote it: rename the
   heading to `## <version/tag> — <date>` and stop — the content was already
   filtered and rewritten per-PR, don't regenerate it.
3. Otherwise (no per-PR automation has been running), gather every commit/PR
   in the range:
   - If this repo also has the `public-changelog` skill installed, you may
     reuse its `scripts/get-release-range.sh` and `scripts/get-commits.sh`
     to resolve the range and pre-filter obvious noise (chore/ci/build/
     style/test/docs) — that's a convenience, not a requirement.
   - Otherwise, use `git log --no-merges --pretty=format:'%s'` (or
     `gh pr list --state merged ...`) over the range directly.
   For each commit/PR, **read the diff** (`git show <hash>` or
   `gh pr diff <n>`) per the "verify every candidate against the diff" rule
   in `SKILL.md` — titles are not reliable enough to filter or word entries
   from. For large ranges, fan the diff reading out (e.g. subagents, one
   batch of commits each) rather than skipping it.
   Apply the shared filter and domain-language rewrite rules above to each
   surviving item, and group into New/Improved/Fixed under a new
   `## <version/tag> — <date>` heading. Expect heavier lifting here the
   longer the release cycle is — don't rush the filtering step.
4. Prepend the new section above prior history in `USER_CHANGELOG.md`
   (never overwrite past entries), and leave a fresh empty `## Unreleased`
   section above it for the next cycle.
5. If zero items survive filtering, say so instead of inventing filler
   entries, and still reset `## Unreleased` to empty.
