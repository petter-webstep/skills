---
name: bug-bounty-hunter
description: Runs a "ralph loop" bug bounty hunt over a repository, scanning it one folder/slice at a time via short-lived headless copilot sessions to keep context small, tracking progress across a resumable multi-day run, and reporting findings to GitHub Issues (or a local BUGS.md fallback). Use when the user asks to hunt for bugs/vulnerabilities across a repo, wants a continuous/background security audit, or mentions "bug bounty", "ralph loop", or scanning a codebase folder-by-folder.
---

# Bug Bounty Hunter

A `bountyhunter` Go binary (source in `scripts/bountyhunter/`) drives the
whole workflow: it is the **facilitator** (tracks which folders/"slices" of
the repo have been scanned in `.bounty-hunter/state.json`) and the **driver**
(spawns one short-lived, headless `copilot -p` session per slice, waits for
it to finish and report, then moves to the next slice). Each spawned session
gets only that slice's files, investigates read-only, reports findings
itself, and exits — so context never grows across the run, and the run can
be killed and resumed at any time.

## Quick start

Build once (binary is git-ignored / rebuilt automatically if missing):

```bash
cd scripts/bountyhunter && go build -o bountyhunter .
```

Full-repo continuous hunt, one folder at a time, until everything is scanned:

```bash
./scripts/bountyhunter/bountyhunter --repo /path/to/repo run
```

Targeted hunt over just one part of the repo (a single subfolder or file),
useful when the user points at a specific area:

```bash
./scripts/bountyhunter/bountyhunter --repo /path/to/repo run --target src/payments
```

Unattended multi-day run (e.g. "run continuously for a workweek"): launch
under a supervisor so it survives terminal/session exit, cap wall-clock time,
and re-run the same command any time to resume from where it left off:

```bash
nohup ./scripts/bountyhunter/bountyhunter --repo /path/to/repo run --max-hours 168 \
  > bounty.log 2>&1 &
```

Check progress at any time (safe to run mid-loop):

```bash
./scripts/bountyhunter/bountyhunter --repo /path/to/repo status
```

## How reporting works

Each per-slice `copilot` session is instructed to file findings itself,
before it exits, according to `--report-mode` (default `auto`):
- `auto` (default): GitHub issues (labelled `bug-bounty`) if `gh` is
  installed and authenticated, else falls back to `BUGS.md` at the repo root.
- `github`: always file GitHub issues; errors out up front if `gh` isn't
  authenticated.
- `local`: always append findings to `BUGS.md`, even if `gh` is available —
  use this to start with a local, file-based findings log (e.g. to review
  before anything is filed publicly, or when you don't want to create
  issues yet).

```bash
./scripts/bountyhunter/bountyhunter --repo /path/to/repo run --report-mode local
```

No findings → nothing is created for that slice.

## Workflow when invoked as this skill

1. Ask the user (if not already clear): scan the **whole repo** or a
   **specific path**? Continuous/unattended, or a single pass now?
2. Confirm `copilot` non-interactive mode is usable (`--allow-all-tools`
   requires the user to be OK with fully autonomous tool use per slice).
3. Run `bountyhunter run` (with `--target`/`--max-hours` as appropriate).
   Tail its stderr for progress; each slice prints its own summary line.
4. On completion (or when asked), run `bountyhunter status` and summarize
   findings/issues for the user.
5. If slices are `blocked` (a `copilot` invocation errored), surface the
   note and offer `bountyhunter reset --all-blocked` to retry them.

See [REFERENCE.md](REFERENCE.md) for the state file schema, slicing
algorithm, all CLI flags, and troubleshooting. See
[scripts/bountyhunter/README.md](scripts/bountyhunter/README.md) for the
binary's own usage docs.
