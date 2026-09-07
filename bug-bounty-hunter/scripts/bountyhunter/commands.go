package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func cmdInit(repo string, args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	target := fs.String("target", "", "Subpath to scan instead of the whole repo")
	force := fs.Bool("force", false, "Rebuild even if state already exists")
	fs.Parse(args)

	repoAbs, _ := filepath.Abs(repo)
	targetAbs := repoAbs
	if *target != "" {
		targetAbs, _ = filepath.Abs(*target)
	}
	if _, err := os.Stat(targetAbs); err != nil {
		fatal(fmt.Errorf("target path does not exist: %s", targetAbs))
	}
	if _, err := os.Stat(statePath(repoAbs)); err == nil && !*force {
		fatal(fmt.Errorf("state already exists at %s (use --force to rebuild)", statePath(repoAbs)))
	}

	paths := discoverSlices(targetAbs, 0)
	slices := make([]*Slice, 0, len(paths))
	for _, p := range paths {
		slices = append(slices, &Slice{Path: p, Status: "pending", IssueRefs: []string{}})
	}
	state := &State{Repo: repoAbs, Target: targetAbs, CreatedAt: nowStr(), Slices: slices}
	saveState(repoAbs, state)

	fmt.Printf("Initialized %d slice(s) under %s\n", len(paths), targetAbs)
	for _, p := range paths {
		rel, _ := filepath.Rel(repoAbs, p)
		fmt.Printf("  pending  %s\n", rel)
	}
}

func cmdNext(repo string, args []string) {
	state := loadState(repo)
	for _, s := range state.Slices {
		if s.Status == "pending" {
			s.Status = "in_progress"
			s.Attempts++
			saveState(state.Repo, state)
			data, _ := json.Marshal(s)
			fmt.Println(string(data))
			return
		}
	}
	fmt.Println("DONE")
}

func cmdComplete(repo string, args []string) {
	fs := flag.NewFlagSet("complete", flag.ExitOnError)
	path := fs.String("path", "", "Slice path (required)")
	findings := fs.Int("findings", 0, "Number of findings reported")
	note := fs.String("note", "", "Optional note")
	var issues multiFlag
	fs.Var(&issues, "issue", "Issue URL/ref (repeatable)")
	fs.Parse(args)
	if *path == "" {
		fatal(fmt.Errorf("--path is required"))
	}

	state := loadState(repo)
	s := findSlice(state, *path)
	s.Status = "done"
	s.Findings = *findings
	ts := nowStr()
	s.CheckedAt = &ts
	s.IssueRefs = append(s.IssueRefs, issues...)
	if *note != "" {
		s.Note = note
	}
	saveState(state.Repo, state)
	rel, _ := filepath.Rel(state.Repo, s.Path)
	fmt.Printf("done: %s (%d finding(s))\n", rel, *findings)
}

func cmdFail(repo string, args []string) {
	fs := flag.NewFlagSet("fail", flag.ExitOnError)
	path := fs.String("path", "", "Slice path (required)")
	note := fs.String("note", "", "Reason (required)")
	fs.Parse(args)
	if *path == "" || *note == "" {
		fatal(fmt.Errorf("--path and --note are required"))
	}

	state := loadState(repo)
	s := findSlice(state, *path)
	s.Status = "blocked"
	s.Note = note
	ts := nowStr()
	s.CheckedAt = &ts
	saveState(state.Repo, state)
	rel, _ := filepath.Rel(state.Repo, s.Path)
	fmt.Printf("blocked: %s - %s\n", rel, *note)
}

func cmdStatus(repo string, args []string) {
	state := loadState(repo)
	counts := map[string]int{}
	for _, s := range state.Slices {
		counts[s.Status]++
		rel, _ := filepath.Rel(state.Repo, s.Path)
		fmt.Printf("%-12s findings=%-3d %s\n", s.Status, s.Findings, rel)
	}
	fmt.Println("---")
	summary := fmt.Sprintf("total=%d", len(state.Slices))
	for _, k := range []string{"pending", "in_progress", "done", "blocked"} {
		if counts[k] > 0 {
			summary += fmt.Sprintf(" %s=%d", k, counts[k])
		}
	}
	fmt.Println(summary)
}

func cmdReset(repo string, args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	path := fs.String("path", "", "Requeue only this slice")
	allBlocked := fs.Bool("all-blocked", false, "Requeue every blocked slice")
	fs.Parse(args)

	state := loadState(repo)
	var pathAbs string
	if *path != "" {
		pathAbs, _ = filepath.Abs(*path)
	}
	n := 0
	for _, s := range state.Slices {
		if pathAbs != "" && s.Path != pathAbs {
			continue
		}
		if *allBlocked && s.Status != "blocked" {
			continue
		}
		s.Status = "pending"
		n++
	}
	saveState(state.Repo, state)
	fmt.Printf("requeued %d slice(s)\n", n)
}

// multiFlag implements flag.Value to collect a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}
