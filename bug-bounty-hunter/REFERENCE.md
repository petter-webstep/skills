# Reference: bug-bounty-hunter internals

## Why a ralph loop

Auditing a large repo in one conversation blows the context window. Instead,
`bountyhunter run` acts as a facilitator that hands off exactly one "slice"
(folder or small file group) to a brand-new, disposable `copilot -p` session
at a time. Each session:

1. Is scoped to a single slice path (`--add-dir <slice>`).
2. Is told to investigate read-only and not touch code outside that path.
3. Reports its own findings (GitHub issue or `BUGS.md` entry) before exiting.
4. Prints one machine-parseable summary line so the driver can record the
   outcome without needing to read the whole transcript.

Because state lives on disk (not in any one conversation/session), the loop
can be stopped and restarted freely — this is what makes a genuine
multi-day, unattended "workweek" run possible.

## CLI reference

All commands take a required `--repo <path>` (the repo root; state lives at
`<repo>/.bounty-hunter/state.json`).

```
bountyhunter --repo <path> run [--target <path>] [--max-hours N]
                                [--max-issues N] [--sleep-seconds N] [--model <name>]
                                [--report-mode auto|github|local] [--fresh]
bountyhunter --repo <path> init [--target <path>] [--force]
bountyhunter --repo <path> next
bountyhunter --repo <path> complete --path <p> [--findings N] [--issue <ref>]... [--note <text>]
bountyhunter --repo <path> fail --path <p> --note <text>
bountyhunter --repo <path> status
bountyhunter --repo <path> reset [--path <p>] [--all-blocked]
```

- `run` is the normal entry point. `init`/`next`/`complete`/`fail`/`reset`
  exist mainly for debugging or scripting a custom loop.
- `--target` scopes the *initial* slice discovery to a subpath; it's ignored
  on resume (state already exists) unless combined with `--fresh`.
- `--max-hours` bounds wall-clock time per invocation of `run`; 0 (default)
  means "run until every slice is done." Combine with a process supervisor
  (`nohup`, `tmux`, `systemd`/`launchd` timer, cron) plus re-running `run`
  to cover a multi-day window in daily chunks.
- `--max-issues` (default `10`) caps how many new issues/entries can be
  filed across an entire `run` invocation, checked between slices — use `0`
  for unlimited. Each per-slice session is also told its remaining budget
  and prioritizes highest-severity findings first if a single slice would
  otherwise exceed it. This keeps a scheduled (e.g. weekly) run from
  flooding the issue tracker in one pass.
- `--sleep-seconds` (default 15) pauses between slices to avoid hammering
  rate limits.
- `--model` is passed straight through to `copilot --model`.
- `--report-mode` controls where findings are filed: `auto` (default, GitHub
  issues if `gh` is authenticated else `BUGS.md`), `github` (force GitHub
  issues, error if `gh` isn't authenticated), or `local` (always write to
  `BUGS.md`, even when `gh` is available).
- Every per-slice session is instructed to check for duplicates before
  filing: it searches existing GitHub issues (or reads the existing
  `BUGS.md`) for a report of the same root cause and skips filing again if
  one already exists (open, or closed by what looks like a real fix).

## State file schema

`<repo>/.bounty-hunter/state.json`:

```json
{
  "repo": "/abs/path/to/repo",
  "target": "/abs/path/scanned (may equal repo)",
  "created_at": "2024-01-01T00:00:00Z",
  "slices": [
    {
      "path": "/abs/path/to/repo/src/api",
      "status": "pending|in_progress|done|blocked",
      "attempts": 0,
      "findings": 0,
      "issue_refs": [],
      "checked_at": null,
      "note": null
    }
  ]
}
```

Treat this file (and `.bounty-hunter/`) as local/ephemeral — add it to
`.gitignore` in the target repo rather than committing it.

## Slicing algorithm

`discoverSlices` (in `scripts/bountyhunter/state.go`):

1. Starts at the repo root or `--target` path.
2. Lists immediate subdirectories, skipping common noise
   (`.git`, `node_modules`, `vendor`, `dist`, `build`, `.venv`, `venv`,
   `__pycache__`, `.next`, `.nuxt`, `coverage`, caches, `.idea`, `.vscode`,
   hidden dirs, `.bounty-hunter` itself).
3. Loose files directly in a directory are covered by scanning that
   directory non-recursively as its own slice.
4. Any subdirectory containing more than 300 files is expanded one level
   deeper recursively (up to 4 levels) instead of becoming one giant slice —
   this keeps each headless session's file set small enough to stay well
   within context limits.
5. A single file passed as `--target` becomes a one-slice run (the "user
   points at a specific part of the repo" case).

## Troubleshooting

- **Slice stuck `blocked`**: check the `note` field (`status` output shows
  findings but not notes — inspect `.bounty-hunter/state.json` directly, or
  re-run with `reset --path <p>` after fixing the underlying issue, e.g. an
  auth problem with `copilot` or `gh`). Use `reset --all-blocked` to retry
  every blocked slice at once.
- **No `BOUNTY_RESULT` line / slice marked blocked with "did not print a
  BOUNTY_RESULT summary line"**: the model didn't follow the final-line
  instruction (rare, but can happen with weaker models or truncated output).
  Retry with `reset --path <p>`, or pin a stronger `--model`.
- **Findings not showing up as issues**: confirm `gh auth status` succeeds
  in the same environment the loop runs in; otherwise it silently falls
  back to `BUGS.md`, which is the intended behavior, not a bug.
- **Rebuilding the binary**: `run` does not auto-rebuild. After editing the
  Go source, `cd scripts/bountyhunter && go build -o bountyhunter .` again.
