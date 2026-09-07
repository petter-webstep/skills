---
name: ai-issue-implementer
description: Implements GitHub issues labeled both "ai-generated" and "AFK" into pull requests, working in an isolated git worktree/branch per issue, with a strict daily throughput cap and open-PR backlog guard. Use when the user asks to run the AI issue loop, process AFK issues, or turn labeled GitHub issues into PRs automatically.
---

# AI Issue Implementer

Turns unattended, AI-flagged GitHub issues into pull requests, one run at a
time, using an isolated `git worktree` per issue so the user's working
directory is never disturbed.

## Requirements

- `git`, `gh` (authenticated), and `go` (to build the bundled `aitool`
  helper) must be on PATH.
- Repo must have `ai-generated` and `AFK` labels for eligible issues, and an
  `ai-in-progress` label (create it if missing) used to mark claimed issues.

## Quick start

Build the helper once per session:

```bash
cd scripts && go build -o aitool .
```

Then follow the **Workflow** below. Each invocation of this skill processes
**at most 2 issues** and then stops — it is designed to be re-run (manually,
or via a schedule) once per day, not looped internally.

## Workflow

1. **Detect the target repo from the current directory.**
   ```bash
   ./scripts/aitool detect-repo
   ```
   This prints `owner/repo<TAB>git-root-path` using the `origin` remote of
   the repo found at or above the cwd. **If it errors** (`NOT_A_GIT_REPO`,
   `NO_ORIGIN_REMOTE`, or `NOT_GITHUB_REMOTE`), do not guess — use `ask_user`
   to ask the user which repo (path or `owner/repo`) to operate on, then
   `cd` there or pass that path explicitly as an argument.

2. **Check throughput limits.**
   ```bash
   ./scripts/aitool check-limits owner/repo
   ```
   Exits `2` and prints `BLOCKED: ...` if either guard trips:
   - More than 10 open PRs in the repo were created in the last month
     (review-load guard, checked across all PRs, not just AI ones).
   - 2 PRs with a head branch starting `AI-` were already opened today
     (UTC).
   On block, stop the run and report the reason to the user. Otherwise it
   prints `REMAINING_SLOTS=N` (1 or 2) — never process more issues than
   this in the current run.

3. **Find eligible issues.**
   ```bash
   ./scripts/aitool find-issues owner/repo <REMAINING_SLOTS>
   ```
   Returns up to `REMAINING_SLOTS` open issues labeled both `ai-generated`
   and `AFK`, excluding any already labeled `ai-in-progress`, oldest first
   (FIFO). If empty, report "no eligible issues" and stop.

4. **For each issue, in order:**

   a. **Claim it immediately** so a concurrent/later run won't duplicate
      work:
      ```bash
      gh issue edit <number> --repo owner/repo --add-label ai-in-progress
      ```

   b. **Create an isolated worktree + branch.** Branch name convention:
      `AI-<repo>-<issue-number>` (e.g. `AI-winc-2143`).
      ```bash
      ./scripts/aitool create-worktree <git-root> AI-<repo>-<issue-number> <base-branch>
      ```
      `<base-branch>` is the repo's default branch unless stacking (see
      below). This prints the worktree path — `cd` into it for all
      subsequent work on this issue.

   c. **Implement the issue** in that worktree: read the issue body/comments
      (`gh issue view <number> --repo owner/repo --comments`), make the
      code changes, run the repo's existing lint/build/test commands, and
      commit.

      **If the issue is too large for one focused PR**, split the work:
      - Open sub-issues with `gh issue create` (label them `ai-generated`
        `AFK` too, and reference the parent with "Part of #<parent>"), or
      - Build a **stacked PR** sequence: branch `AI-<repo>-<issue>-2` off
        `AI-<repo>-<issue>-1` (not off the default branch), and open each
        PR with `--base <previous-branch>`. Note the stack order explicitly
        in each PR description (see `REFERENCE.md`).
      - Try to avoid comments in the code as code should be self explanatory

   d. **Push and open the PR:**
      ```bash
      git push -u origin AI-<repo>-<issue-number>
      gh pr create --repo owner/repo --title "..." \
        --body "Closes #<issue-number>" \
        --base <base-branch> --head AI-<repo>-<issue-number>
      ```

   e. **Link back on the issue and finalize labels:**
      ```bash
      gh issue comment <number> --repo owner/repo --body "Opened <PR URL>"
      gh issue edit <number> --repo owner/repo --remove-label ai-in-progress
      ```

   f. **Clean up the worktree** once pushed (the branch itself stays on the
      remote for the PR):
      ```bash
      git -C <git-root> worktree remove <worktree-path>
      ```

5. **Stop after processing `REMAINING_SLOTS` issues** — never exceed 2 PRs
   per run even if more slots were technically available.

## Details on PR stacking, sub-issues, and troubleshooting

See [REFERENCE.md](REFERENCE.md).
