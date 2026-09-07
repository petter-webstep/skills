package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	maxFilesPerSlice = 300
	maxExpandDepth   = 4
)

var defaultExcludes = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true,
	"vendor": true, "dist": true, "build": true, ".venv": true, "venv": true,
	"__pycache__": true, ".next": true, ".nuxt": true, "coverage": true,
	".pytest_cache": true, ".mypy_cache": true, "target": true, ".terraform": true,
	".idea": true, ".vscode": true, ".bounty-hunter": true,
}

// Slice is one folder/file group the bounty hunter scans in a single
// headless copilot session.
type Slice struct {
	Path      string   `json:"path"`
	Status    string   `json:"status"` // pending|in_progress|done|blocked
	Attempts  int      `json:"attempts"`
	Findings  int      `json:"findings"`
	IssueRefs []string `json:"issue_refs"`
	CheckedAt *string  `json:"checked_at"`
	Note      *string  `json:"note"`
}

// State is the on-disk record of scan progress for one repo, persisted at
// <repo>/.bounty-hunter/state.json so the ralph loop can resume across
// restarts (e.g. a multi-day unattended run).
type State struct {
	Repo      string   `json:"repo"`
	Target    string   `json:"target"`
	CreatedAt string   `json:"created_at"`
	Slices    []*Slice `json:"slices"`
}

func statePath(repo string) string {
	return filepath.Join(repo, ".bounty-hunter", "state.json")
}

func loadState(repo string) *State {
	p := statePath(repo)
	data, err := os.ReadFile(p)
	if err != nil {
		fatal(fmt.Errorf("no state file at %s (run `bountyhunter init` first)", p))
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		fatal(fmt.Errorf("failed to parse state file: %w", err))
	}
	return &s
}

func saveState(repo string, s *State) {
	p := statePath(repo)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		fatal(err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func nowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// countFiles walks a directory (skipping excluded/hidden dirs) and returns
// the number of files found, short-circuiting once it exceeds the slice size
// threshold since the exact count beyond that point doesn't matter.
func countFiles(path string) int {
	total := 0
	var walk func(dir string) bool // returns false to stop early
	walk = func(dir string) bool {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return true
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				if defaultExcludes[name] || (len(name) > 0 && name[0] == '.') {
					continue
				}
				if !walk(filepath.Join(dir, name)) {
					return false
				}
			} else {
				total++
				if total > maxFilesPerSlice {
					return false
				}
			}
		}
		return true
	}
	walk(path)
	return total
}

// discoverSlices returns a list of absolute directory/file paths sized under
// maxFilesPerSlice, expanding large directories one level deeper (up to
// maxExpandDepth) instead of treating them as a single giant slice.
func discoverSlices(target string, depth int) []string {
	info, err := os.Stat(target)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{target}
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return []string{target}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var subdirs []string
	looseFiles := false
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if defaultExcludes[name] || (len(name) > 0 && name[0] == '.') {
				continue
			}
			subdirs = append(subdirs, filepath.Join(target, name))
		} else {
			looseFiles = true
		}
	}

	var slices []string
	if looseFiles {
		slices = append(slices, target) // scanning `target` non-recursively covers loose files
	}
	for _, d := range subdirs {
		if depth < maxExpandDepth && countFiles(d) > maxFilesPerSlice {
			slices = append(slices, discoverSlices(d, depth+1)...)
		} else {
			slices = append(slices, d)
		}
	}
	if len(slices) == 0 {
		slices = append(slices, target) // empty/leaf dir, still track it
	}
	return slices
}

func findSlice(s *State, path string) *Slice {
	abs, _ := filepath.Abs(path)
	for _, sl := range s.Slices {
		if sl.Path == abs {
			return sl
		}
	}
	fatal(fmt.Errorf("no slice found for path: %s", abs))
	return nil
}
