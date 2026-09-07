# Weekly bug bounty hunt — GitHub Copilot cloud agent Automation

This is an alternative to the `run-weekly.sh` cron + sandbox approach in this
directory: instead of provisioning a sandbox on a schedule, use GitHub's
native **Copilot cloud agent Automations** feature, configured directly in
the repo's **Agents → Automations** page (or the Automations tab in the
GitHub Copilot app).

## Requirements

- Repo must be **private or internal** (automations aren't available on
  public repos).
- Copilot cloud agent must be enabled for the repo.
- Copilot Pro, Pro+, Max, Business, or Enterprise plan.
- Any user with **write access** can create an automation.

## Setup

**Name:** `Weekly bug bounty hunt`

**Trigger:** On a schedule → Weekly (pick day/time, e.g. Monday 11:00)

**Tools:** Read repo contents, Search/list issues, Create issue, Add labels
(skip "push"/"create PR" — keep it read-only/reporting-only, matching the
original bug-bounty-hunter skill's read-only auditor design)

**Prompt:**

```
You are a read-only security/bug bounty auditor for this repository. Do not
edit any source code or open pull requests — your only output is GitHub
issues.

Scope: review the codebase for bugs, security vulnerabilities, logic
errors, race conditions, resource leaks, and other high-confidence defects.
Ignore style/formatting nitpicks. If the repo is large, prioritize areas
you haven't covered in recent weeks (check issue history for clues) over
re-reviewing the same files repeatedly.

Before filing anything, for each candidate finding, search existing issues
(open AND closed) for a report of the same root cause — same affected
file/function or materially the same bug, even if worded differently.
Match on substance, not exact title text.
  - If an open issue already covers it, skip filing a duplicate.
  - If a closed issue covers it, only refile if the closure looks stale or
    wrong (e.g. closed without a matching code change, and the bug is
    still clearly present) — and reference the old issue with
    "See also #<number>".

File at most 10 new issues in this run. If you find more than 10 genuinely
new (non-duplicate) findings, file the 10 highest-severity ones and note
the rest briefly in your final summary so they can be picked up next week.

For each new issue: title "[bounty] <short summary>", label it "bug-bounty"
(create the label first if it doesn't exist), and include the affected
file/line, a description, severity (low/medium/high/critical), and how to
reproduce/verify it.

If you find nothing new, don't create anything — just summarize that no
new findings were confirmed this run.
```

## How this compares to the sandbox/cron approach

- No host machine, cron job, or sandbox provisioning needed — GitHub runs
  the session in its own Actions-powered environment on schedule.
- No local state file (`.bounty-hunter/state.json`) — the model dedups by
  searching GitHub issue history each run instead of reading persisted
  slice-scan state. This is a softer guarantee than the `bountyhunter` CLI's
  programmatic slice tracking + per-run issue cap, but far simpler to run.
- **Billing**: each run consumes GitHub Actions minutes + GitHub AI
  Credits, billed to the user who created the automation.
- **Attribution**: any issues/PRs it creates are attributed to that user;
  that user cannot approve PRs it opens (if PR-creation tools are ever
  enabled), preserving review separation.
- **Visibility**: the automation's configuration is private to its
  creator, but the sessions it spawns (prompt, logs, resulting
  issues/PRs) are visible to anyone with repo access.
