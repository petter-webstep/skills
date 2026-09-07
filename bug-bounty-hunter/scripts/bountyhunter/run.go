package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// cmdRun is the ralph loop: repeatedly ask the facilitator state for the
// next pending slice, spawn one short-lived headless `copilot -p` session
// scoped to that slice, record the result, sleep, and repeat. Each copilot
// invocation is a fresh process with no memory of prior slices, so context
// never grows across the run - the state file on disk is the only thing
// that persists between iterations, which is what makes the loop safely
// resumable across a multi-day unattended run.
func cmdRun(repo string, args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	target := fs.String("target", "", "Subpath to scan instead of the whole repo (ignored on resume)")
	maxHours := fs.Float64("max-hours", 0, "Wall-clock budget; 0 = unlimited (run until all slices done)")
	maxIssues := fs.Int("max-issues", 10, "Maximum number of new issues/entries to file across the whole run; "+
		"0 = unlimited. Checked between slices (a slice already in progress finishes and reports normally, "+
		"so the actual total for a run may exceed this slightly).")
	sleepSeconds := fs.Int("sleep-seconds", 15, "Delay between slices, to avoid rate limits")
	model := fs.String("model", "", "Model passed through to `copilot --model`")
	fresh := fs.Bool("fresh", false, "Wipe and re-init state even if it already exists")
	reportModeFlag := fs.String("report-mode", "auto", "Where findings are reported: auto|github|local. "+
		"auto = GitHub issues if `gh` is authenticated, else BUGS.md. "+
		"github = always use GitHub issues (errors out if `gh` isn't authenticated). "+
		"local = always append to BUGS.md, even if `gh` is available.")
	fs.Parse(args)

	switch *reportModeFlag {
	case "auto", "github", "local":
	default:
		fatal(fmt.Errorf("--report-mode must be one of: auto, github, local (got %q)", *reportModeFlag))
	}

	repoAbs, err := filepath.Abs(repo)
	if err != nil {
		fatal(err)
	}

	if *fresh || !fileExists(statePath(repoAbs)) {
		initArgs := []string{}
		if *target != "" {
			initArgs = append(initArgs, "--target", *target)
		}
		if *fresh {
			initArgs = append(initArgs, "--force")
		}
		cmdInit(repoAbs, initArgs)
	} else {
		fmt.Fprintf(os.Stderr, "Resuming existing state at %s\n", statePath(repoAbs))
	}

	reportFile := filepath.Join(repoAbs, "BUGS.md")
	useGithub := false
	switch *reportModeFlag {
	case "github":
		if !ghAuthenticated() {
			fatal(fmt.Errorf("--report-mode=github requires `gh` to be installed and authenticated (run `gh auth login`)"))
		}
		useGithub = true
	case "local":
		useGithub = false
	default: // auto
		useGithub = ghAuthenticated()
	}
	reportModeDesc := fmt.Sprintf("local file at %s", reportFile)
	if useGithub {
		reportModeDesc = "GitHub issues (gh CLI)"
	}
	fmt.Fprintf(os.Stderr, "Starting ralph loop for %s (reports via: %s)\n", repoAbs, reportModeDesc)

	start := time.Now()
	totalFiled := 0
	for {
		if *maxHours > 0 && time.Since(start).Hours() >= *maxHours {
			fmt.Fprintf(os.Stderr, "Max hours (%.1f) reached, stopping. Resume later by re-running `run`.\n", *maxHours)
			break
		}
		if *maxIssues > 0 && totalFiled >= *maxIssues {
			fmt.Fprintf(os.Stderr, "Max issues (%d) filed this run, stopping. Resume later by re-running `run`.\n", *maxIssues)
			break
		}

		slicePath := claimNextSlice(repoAbs)
		if slicePath == "" {
			fmt.Fprintln(os.Stderr, "All slices scanned.")
			break
		}

		rel, _ := filepath.Rel(repoAbs, slicePath)
		fmt.Fprintf(os.Stderr, "== Scanning slice: %s ==\n", rel)

		remaining := -1 // -1 means unlimited (no --max-issues cap set)
		if *maxIssues > 0 {
			remaining = *maxIssues - totalFiled
		}
		findings, issues, runErr := scanSlice(repoAbs, slicePath, reportFile, *model, useGithub, remaining)
		if runErr != nil {
			cmdFail(repoAbs, []string{"--path", slicePath, "--note", runErr.Error()})
			continue
		}
		if useGithub {
			totalFiled += len(issues)
		} else {
			totalFiled += findings
		}
		completeArgs := []string{"--path", slicePath, "--findings", strconv.Itoa(findings)}
		for _, iss := range issues {
			completeArgs = append(completeArgs, "--issue", iss)
		}
		cmdComplete(repoAbs, completeArgs)

		time.Sleep(time.Duration(*sleepSeconds) * time.Second)
	}

	cmdStatus(repoAbs, nil)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// claimNextSlice marks the next pending slice in_progress and returns its
// path, or "" if none remain. This is the same operation `bountyhunter next`
// performs, reused here so `run` doesn't need to shell out to itself.
func claimNextSlice(repo string) string {
	state := loadState(repo)
	for _, s := range state.Slices {
		if s.Status == "pending" {
			s.Status = "in_progress"
			s.Attempts++
			saveState(state.Repo, state)
			return s.Path
		}
	}
	return ""
}

func ghAuthenticated() bool {
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}
	return exec.Command("gh", "auth", "status").Run() == nil
}

var bountyResultRE = regexp.MustCompile(`BOUNTY_RESULT:\s*findings=(\d+)\s*issues=(\S*)`)

// scanSlice spawns a single headless `copilot -p` session scoped to
// slicePath and parses its final BOUNTY_RESULT summary line.
func scanSlice(repo, slicePath, reportFile, model string, useGithub bool, remainingBudget int) (findings int, issues []string, err error) {
	prompt := buildPrompt(slicePath, reportFile, useGithub, remainingBudget)

	cmdArgs := []string{
		"-p", prompt,
		"--allow-all-tools",
		"--add-dir", slicePath,
		"-C", repo,
		"--silent",
	}
	if model != "" {
		cmdArgs = append(cmdArgs, "--model", model)
	}

	cmd := exec.Command("copilot", cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	output := out.String()
	tail := output
	if lines := strings.Split(output, "\n"); len(lines) > 20 {
		tail = strings.Join(lines[len(lines)-20:], "\n")
	}
	fmt.Fprintln(os.Stderr, tail)

	if runErr != nil {
		return 0, nil, fmt.Errorf("copilot exited with error: %v", runErr)
	}

	m := bountyResultRE.FindStringSubmatch(output)
	if m == nil {
		return 0, nil, fmt.Errorf("copilot session did not print a BOUNTY_RESULT summary line")
	}
	findings, _ = strconv.Atoi(m[1])
	if m[2] != "" && m[2] != "none" {
		issues = strings.Split(m[2], ",")
	}
	return findings, issues, nil
}

func buildPrompt(slicePath, reportFile string, useGithub bool, remainingBudget int) string {
	dedupInstructions := fmt.Sprintf(`Before appending anything, read the ENTIRE
    existing %s (if it exists) and check whether a finding covering the same
    root cause already has an entry there — same affected file/function, or
    materially the same bug even if worded differently. Match on substance,
    not exact wording. If a duplicate entry already exists, SKIP filing it
    again (do not append a second entry for it).`, reportFile)
	if useGithub {
		dedupInstructions = `Before filing anything, search for an existing report
    of the same root cause: run
    'gh issue list --state all --search "<key terms from the finding>" --json number,title,url,state'
    (and, if useful, also search without the "bug-bounty" label filter, since
    a human or an earlier scan may have filed it without that label). Treat
    two findings as duplicates if they point at the same file/function and
    describe materially the same bug, even if worded differently — match on
    substance, not exact title text. If a duplicate is found:
      - If it's still open, do NOT file a new issue for it. Optionally add a
        brief 'gh issue comment' noting the bug bounty scan confirmed it's
        still present, if you have something new to add (e.g. it now also
        affects this slice's code).
      - If it was closed, use your judgement: if the closure looks like a
        real fix, skip it; if it looks stale/wrong (e.g. closed without a
        corresponding code change, or the bug is clearly still present in
        the code you're reading), file a new issue and reference the old one
        with "See also #<old-number>".
    Only file a genuinely new GitHub issue with 'gh issue create' when no
    existing open (or wrongly-closed) issue already covers it.`
	}

	reportInstructions := fmt.Sprintf(`Append one entry per finding to %s. This file is
    shared across many scan sessions, so you MUST NOT overwrite, recreate, or
    truncate it — only ever add to the end. Use a shell append redirection
    (e.g. "cat >> %s <<'EOF' ... EOF") or an editor append operation that
    preserves all existing bytes; never use a "create file" operation on this
    path. Before writing, read the file's current tail (e.g. "tail -c 2000
    %s") to confirm your addition will land after existing content. If the
    file does not exist yet, create it starting with a "# Bug Bounty
    Findings" heading. Each finding gets its own "## <short summary>"
    heading, the affected path/line, a description, and severity
    (low/medium/high/critical).`, reportFile, reportFile, reportFile)
	if useGithub {
		reportInstructions = `File one GitHub issue per finding with 'gh issue create',
    titled "[bounty] <short summary>", labelled "bug-bounty" if that label
    exists (create it first with
    'gh label create bug-bounty --color FF0000 --description "Found by bug-bounty-hunter"'
    if missing), and including the file path, line numbers, a description,
    and how to reproduce/verify it. Note each created issue URL.`
	}

	budgetInstructions := ""
	if remainingBudget >= 0 {
		noun := "entries"
		if useGithub {
			noun = "issues"
		}
		budgetInstructions = fmt.Sprintf(`

This run has a budget of %d more new %s it may file in total (across all
slices, not just this one) — you can check how many you've already used by
counting prior entries/issues if relevant, but you don't need to track the
global count yourself. If this slice alone has more than %d genuinely new
(non-duplicate) findings, file the %d highest-severity ones first and stop;
skip the rest rather than exceeding the cap. Findings you skip for this
reason are not lost forever — mention them briefly in your summary so a
human can decide whether to requeue this slice later with
'bountyhunter reset --path <path>'.`, remainingBudget, noun, remainingBudget, remainingBudget)
	}

	return fmt.Sprintf(`You are a read-only security/bug bounty auditor. Investigate ONLY the
following path within this repository, do not read or modify files outside
it, and do not edit any source code:

  %s

Look for: bugs, security vulnerabilities, logic errors, race conditions,
resource leaks, and other high-confidence defects. Ignore style/formatting
nitpicks.

For each finding, BEFORE reporting it, check it isn't a duplicate of a
previously reported finding:
  %s

Then report it (skip this for anything identified as a duplicate above):
  %s
%s

If you find nothing, or everything found is a duplicate, do not create an
issue or file entry.

When completely done, print a single final line, exactly in this format
(no other text on that line):
  BOUNTY_RESULT: findings=<N> issues=<comma-separated issue URLs or "none">`, slicePath, dedupInstructions, reportInstructions, budgetInstructions)
}
