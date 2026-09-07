# Reference: stacking, sub-issues, and troubleshooting

## When to split work

Split an issue instead of forcing it into one PR when:
- The change touches unrelated subsystems that could be reviewed/merged
  independently.
- Implementation naturally has sequential phases (e.g. "add schema" →
  "add API" → "add UI").
- The diff would be large enough to slow down review meaningfully.

Prefer **sub-issues** when the pieces are independently useful and could
each consume their own future daily PR slot. Prefer a **stacked PR**
sequence when the pieces are only meaningful together and should land as
an ordered set relatively close together.

## Creating sub-issues

```bash
gh issue create --repo owner/repo \
  --title "Part 1: <slice of original issue>" \
  --body "Split from #<parent-number> because <reason>.\n\nScope: ..." \
  --label ai-generated --label AFK
```

Each sub-issue re-enters the normal FIFO queue and is subject to the same
daily cap and backlog guard — do not create more sub-issues than can
realistically be processed within the 2/day limit; prefer 2-3 sub-issues
max per parent.

## Creating a PR stack

Branch and PR chain for a 3-part stack on issue 2143 in `winc`:

```
main
 └─ AI-winc-2143-1  (PR #1: base=main,              head=AI-winc-2143-1)
     └─ AI-winc-2143-2  (PR #2: base=AI-winc-2143-1, head=AI-winc-2143-2)
         └─ AI-winc-2143-3  (PR #3: base=AI-winc-2143-2, head=AI-winc-2143-3)
```

Steps per stack level:
```bash
./scripts/aitool create-worktree <git-root> AI-winc-2143-2 AI-winc-2143-1
# ... implement, commit ...
git push -u origin AI-winc-2143-2
gh pr create --repo owner/repo --base AI-winc-2143-1 --head AI-winc-2143-2 \
  --title "Part 2/3: ..." \
  --body "Stacked on #<PR1 number>. Part of #2143.\n\n<description>"
```

Mention the stack explicitly in every PR body (which PR it's based on, and
its position in the stack) per GitHub's stacked-PR guidance:
https://docs.github.com/en/pull-requests/get-started/about-stacked-prs

Only the **bottom** PR of a stack counts toward the "2 PRs/day" cap for
that issue's *first* level; each additional stacked PR opened the same day
still counts against the same daily cap computed by `check-limits` (it
counts all PRs with head `AI-*` created today, stacked or not) — budget
accordingly, e.g. don't start a 3-PR stack if only 1 slot remains today
without a plan to open the rest on subsequent days.

## Troubleshooting

- **`check-limits` blocks every run**: confirm the repo doesn't already
  have >10 open PRs younger than a month for unrelated reasons; this guard
  is intentionally global (all authors), not AI-specific.
- **`find-issues` returns issues that were already actioned**: verify the
  `ai-in-progress` label wasn't accidentally left off during labeling, or
  removed too early in step 4(e).
- **`detect-repo` fails inside a worktree-of-a-worktree**: run it from the
  main repo checkout, not from a previously created worktree directory.
- **Branch name collisions**: if `AI-<repo>-<issue-number>` already exists
  (e.g. a retried run), reuse the existing branch/worktree rather than
  creating a duplicate — check with `git worktree list` and
  `gh pr list --repo owner/repo --head AI-<repo>-<issue-number>` first.
