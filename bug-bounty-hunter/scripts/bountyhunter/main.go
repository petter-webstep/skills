// Command bountyhunter is the ralph-loop driver and state tracker for the
// bug-bounty-hunter skill. It owns .bounty-hunter/state.json inside the
// target repo, tracking which "slices" (folders/files) have been scanned,
// and drives short-lived headless `copilot -p` sessions - one per slice -
// so no single session ever needs more than one slice's worth of context.
//
// Subcommands:
//
//	run       The main entry point: loops until every slice is scanned (or
//	          --max-hours elapses, or --max-issues new issues have been
//	          filed), spawning one headless copilot session per slice and
//	          recording results. Resumable - re-run to continue from
//	          wherever state.json left off.
//	init      Build (or rebuild) the slice list without starting the loop.
//	next      Print the next pending slice as JSON, mark it in_progress.
//	          Prints "DONE" when nothing is left. (Mostly for debugging;
//	          `run` claims slices internally.)
//	complete  Mark a slice done, recording findings count + issue refs.
//	fail      Mark a slice blocked, so it can be retried later.
//	status    Print a human-readable summary table.
//	reset     Requeue one slice (or all blocked slices) back to pending.
package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: bountyhunter --repo <path> <command> [flags]

Commands:
  run       [--target <path>] [--max-hours N] [--max-issues N]
            [--sleep-seconds N] [--model <name>]
            [--report-mode auto|github|local] [--fresh]
  init      [--target <path>] [--force]
  next
  complete  --path <path> [--findings N] [--issue <ref>]... [--note <text>]
  fail      --path <path> --note <text>
  status
  reset     [--path <path>] [--all-blocked]`)
}

func main() {
	args := os.Args[1:]
	var repo string
	var rest []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}

	if repo == "" || len(rest) == 0 {
		usage()
		os.Exit(2)
	}

	cmd, cmdArgs := rest[0], rest[1:]
	switch cmd {
	case "run":
		cmdRun(repo, cmdArgs)
	case "init":
		cmdInit(repo, cmdArgs)
	case "next":
		cmdNext(repo, cmdArgs)
	case "complete":
		cmdComplete(repo, cmdArgs)
	case "fail":
		cmdFail(repo, cmdArgs)
	case "status":
		cmdStatus(repo, cmdArgs)
	case "reset":
		cmdReset(repo, cmdArgs)
	default:
		usage()
		os.Exit(2)
	}
}
