# bountyhunter

A small Go CLI that is both the **facilitator** (tracks scan progress) and
the **driver** (spawns headless `copilot` sessions) for the
[bug-bounty-hunter skill](../../SKILL.md)'s ralph loop: it scans a
repository one folder/"slice" at a time, so no single `copilot` session
ever needs more context than one slice's worth of files, and progress is
saved to disk so the whole run is resumable across restarts or a multi-day
unattended window.

## Build

```bash
go build -o bountyhunter .
```

Requires Go 1.21+. Produces a single static binary, `bountyhunter`, with no
runtime dependencies (the `copilot` and optionally `gh` CLIs must be on
`PATH` when you actually run a scan).

## Usage

```
bountyhunter --repo <path> <command> [flags]
```

`--repo` is required for every command and is the repo root — state is
stored at `<repo>/.bounty-hunter/state.json`.

### `run` — the main entry point

Loops until every slice has been scanned (or `--max-hours` elapses),
spawning one headless `copilot -p` session per slice and recording the
outcome. Safe to re-run at any time to resume from wherever the state file
left off.

```bash
./bountyhunter --repo /path/to/repo run
```

Flags:

| Flag               | Default | Description |
|---------------------|---------|-------------|
| `--target <path>`   | (repo root) | Scan only this subpath (a folder or a single file). Ignored on resume unless combined with `--fresh`. |
| `--max-hours N`     | `0` (unlimited) | Wall-clock budget for this invocation; stops issuing new slices once exceeded. Combine with a process supervisor to cover a multi-day window. |
| `--max-issues N`    | `10` | Caps new issues/entries filed across the whole run (checked between slices); `0` = unlimited. Keeps a scheduled run from flooding the issue tracker in one pass. |
| `--sleep-seconds N` | `15` | Delay between slices, to avoid rate limits. |
| `--model <name>`    | (copilot default) | Passed through to `copilot --model`. |
| `--report-mode <mode>` | `auto` | Where findings are filed: `auto` (GitHub issues if `gh` is authenticated, else `BUGS.md`), `github` (always file issues, errors if `gh` isn't authenticated), or `local` (always write to `BUGS.md`, even if `gh` is available). |
| `--fresh`           | off | Wipe and rebuild the slice list even if state already exists. |

### Other commands

Mostly useful for debugging, scripting a custom loop, or inspecting/repairing
state without starting a full run:

```
bountyhunter --repo <path> init [--target <path>] [--force]
bountyhunter --repo <path> next
bountyhunter --repo <path> complete --path <p> [--findings N] [--issue <ref>]... [--note <text>]
bountyhunter --repo <path> fail --path <p> --note <text>
bountyhunter --repo <path> status
bountyhunter --repo <path> reset [--path <p>] [--all-blocked]
```

- `init` builds the slice list without starting the loop.
- `next` claims and prints the next pending slice as JSON (or `DONE`).
- `complete` / `fail` record the outcome of a slice.
- `status` prints a per-slice table plus totals — safe to run while a loop
  is in progress.
- `reset` requeues one slice (`--path`) or every `blocked` slice
  (`--all-blocked`) back to `pending`.

## Files

- `main.go` — argument parsing / subcommand dispatch.
- `state.go` — state file load/save, and the slicing algorithm
  (`discoverSlices`) that breaks a repo into scan-sized chunks.
- `commands.go` — `init`, `next`, `complete`, `fail`, `status`, `reset`.
- `run.go` — the `run` loop itself: claims slices, builds the per-slice
  prompt, spawns `copilot`, parses its `BOUNTY_RESULT` summary line.

See [../../REFERENCE.md](../../REFERENCE.md) for the state file schema,
slicing algorithm details, and troubleshooting.
