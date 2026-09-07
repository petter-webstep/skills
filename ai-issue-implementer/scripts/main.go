// Command aitool provides small, dependency-free helpers for the
// ai-issue-implementer skill. It shells out to `git` and `gh` (which must
// already be installed and authenticated) rather than reimplementing the
// GitHub API client.
//
// Subcommands:
//
//	detect-repo   [start-path]
//	check-limits  <owner/repo>
//	find-issues   <owner/repo> [limit]
//	create-worktree <git-root> <branch-name> [base-branch]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: aitool <detect-repo|check-limits|find-issues|create-worktree> [args...]")
	}

	var err error
	switch os.Args[1] {
	case "detect-repo":
		err = cmdDetectRepo(os.Args[2:])
	case "check-limits":
		err = cmdCheckLimits(os.Args[2:])
	case "find-issues":
		err = cmdFindIssues(os.Args[2:])
	case "create-worktree":
		err = cmdCreateWorktree(os.Args[2:])
	default:
		fatal(fmt.Sprintf("unknown subcommand %q", os.Args[1]))
	}
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// ---------------------------------------------------------------------
// detect-repo: finds the enclosing git repo and its owner/repo slug.
// ---------------------------------------------------------------------

var githubRemoteRE = regexp.MustCompile(`^(?:git@github\.com:|https?://github\.com/)([^/]+/[^/]+?)(?:\.git)?$`)

func cmdDetectRepo(args []string) error {
	start := "."
	if len(args) > 0 {
		start = args[0]
	}

	root, err := run("git", "-C", start, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("NOT_A_GIT_REPO: no .git directory found at or above %q", start)
	}

	remoteURL, err := run("git", "-C", root, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("NO_ORIGIN_REMOTE: %q has no 'origin' remote", root)
	}

	m := githubRemoteRE.FindStringSubmatch(remoteURL)
	if m == nil {
		return fmt.Errorf("NOT_GITHUB_REMOTE: origin remote %q is not a github.com URL", remoteURL)
	}

	fmt.Printf("%s\t%s\n", m[1], root)
	return nil
}

// ---------------------------------------------------------------------
// check-limits: enforces the daily PR cap and open-PR backlog guard.
// ---------------------------------------------------------------------

type ghItem struct {
	Number int `json:"number"`
}

func ghJSONList(args ...string) ([]ghItem, error) {
	out, err := run("gh", args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var items []ghItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w\n%s", err, out)
	}
	return items, nil
}

func cmdCheckLimits(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: check-limits <owner/repo>")
	}
	repo := args[0]

	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	oneMonthAgo := now.AddDate(0, -1, 0).Format("2006-01-02")

	// Rule: skip entirely if more than 10 open PRs (any author) were
	// created within the last month (review-load guard).
	recentOpen, err := ghJSONList("pr", "list", "--repo", repo, "--state", "open",
		"--search", "created:>="+oneMonthAgo, "--json", "number")
	if err != nil {
		return err
	}
	if len(recentOpen) > 10 {
		fmt.Printf("BLOCKED: %d open PRs created since %s (limit: 10). Skipping this run.\n", len(recentOpen), oneMonthAgo)
		os.Exit(2)
	}

	// Rule: at most 2 PRs from this skill (head branch prefix "AI-") per
	// UTC day.
	todayAI, err := ghJSONList("pr", "list", "--repo", repo, "--state", "all",
		"--search", "created:>="+today+" head:AI-", "--json", "number")
	if err != nil {
		return err
	}
	remaining := 2 - len(todayAI)
	if remaining <= 0 {
		fmt.Printf("BLOCKED: already opened %d AI-* PR(s) today (limit: 2). Skipping this run.\n", len(todayAI))
		os.Exit(2)
	}

	fmt.Printf("OK: %d slot(s) remaining today; %d open PRs younger than 1 month (limit 10).\n", remaining, len(recentOpen))
	fmt.Printf("REMAINING_SLOTS=%d\n", remaining)
	return nil
}

// ---------------------------------------------------------------------
// find-issues: lists eligible issues (ai-generated + AFK, not claimed),
// oldest first.
// ---------------------------------------------------------------------

type issue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func cmdFindIssues(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: find-issues <owner/repo> [limit]")
	}
	repo := args[0]
	limit := 2
	if len(args) > 1 {
		if _, err := fmt.Sscanf(args[1], "%d", &limit); err != nil {
			return fmt.Errorf("invalid limit %q: %w", args[1], err)
		}
	}

	out, err := run("gh", "issue", "list", "--repo", repo,
		"--label", "ai-generated", "--label", "AFK", "--state", "open",
		"--json", "number,title,createdAt,labels")
	if err != nil {
		return err
	}

	var issues []issue
	if out != "" {
		if err := json.Unmarshal([]byte(out), &issues); err != nil {
			return fmt.Errorf("parsing gh output: %w\n%s", err, out)
		}
	}

	filtered := issues[:0]
	for _, is := range issues {
		claimed := false
		for _, l := range is.Labels {
			if l.Name == "ai-in-progress" {
				claimed = true
				break
			}
		}
		if !claimed {
			filtered = append(filtered, is)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt < filtered[j].CreatedAt
	})

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	enc, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(enc))
	return nil
}

// ---------------------------------------------------------------------
// create-worktree: adds a new git worktree + branch in a sibling
// directory, e.g. ../<repo>-worktrees/<branch-name>.
// ---------------------------------------------------------------------

func cmdCreateWorktree(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: create-worktree <git-root> <branch-name> [base-branch]")
	}
	gitRoot := args[0]
	branch := args[1]
	base := "HEAD"
	if len(args) > 2 {
		base = args[2]
	}

	repoName := filepath.Base(gitRoot)
	parent := filepath.Dir(gitRoot)
	worktreeDir := filepath.Join(parent, repoName+"-worktrees", branch)

	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		return fmt.Errorf("creating worktree parent dir: %w", err)
	}

	// Make sure we branch off an up-to-date base ref when it's a remote branch.
	_, _ = run("git", "-C", gitRoot, "fetch", "origin", base)

	if _, err := run("git", "-C", gitRoot, "worktree", "add", "-b", branch, worktreeDir, base); err != nil {
		return err
	}

	fmt.Println(worktreeDir)
	return nil
}
