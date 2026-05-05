package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type StalenessCheck struct{}

type stashEntry struct {
	date    time.Time
	display string
}

func (c *StalenessCheck) Check(repo *Repo) []Result {
	var results []Result

	maxAge := repo.Config.Thresholds.StashMaxAge.Duration
	maxCount := repo.Config.Thresholds.StashMaxCount

	entries, err := stashEntries(repo)
	if err == nil {
		// Stash age.
		now := time.Now()
		var oldDetails []string
		for _, e := range entries {
			if now.Sub(e.date) > maxAge {
				oldDetails = append(oldDetails, e.display)
			}
		}
		if len(oldDetails) > 0 {
			results = append(results, Result{
				Name:    "staleness/stash-age",
				Status:  StatusFail,
				Message: fmt.Sprintf("%d stash entries older than %s", len(oldDetails), formatDuration(maxAge)),
				Details: oldDetails,
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/stash-age",
				Status:  StatusOK,
				Message: "no stale stash entries",
			})
		}

		// Stash count.
		if len(entries) > maxCount {
			var allDetails []string
			for _, e := range entries {
				allDetails = append(allDetails, e.display)
			}
			results = append(results, Result{
				Name:    "staleness/stash-count",
				Status:  StatusFail,
				Message: fmt.Sprintf("%d entries (max %d)", len(entries), maxCount),
				Details: allDetails,
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/stash-count",
				Status:  StatusOK,
				Message: fmt.Sprintf("%d entries", len(entries)),
			})
		}
	}

	// Uncommitted changes and untracked files (per worktree).
	maxUncommitted := repo.Config.Thresholds.UncommittedMaxAge.Duration
	worktrees := listWorktrees(repo)
	if len(worktrees) == 0 {
		worktrees = []string{repo.Dir}
	}
	for _, wt := range worktrees {
		results = append(results, worktreeStaleness(repo, wt, maxUncommitted)...)
	}

	return results
}

// worktreeStaleness reports uncommitted/untracked staleness for one worktree.
// Result names are suffixed with [<relpath>] for non-main worktrees so each
// worktree appears as a separate row in the output.
func worktreeStaleness(repo *Repo, wt string, maxUncommitted time.Duration) []Result {
	suffix := ""
	if wt != repo.Dir {
		rel, err := filepath.Rel(repo.Dir, wt)
		if err != nil {
			rel = wt
		}
		suffix = fmt.Sprintf("[%s]", rel)
	}

	porcelain, _ := gitInDir(wt, "status", "--porcelain")
	var uncommittedLines, untrackedLines []string
	for _, line := range strings.Split(porcelain, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "?? ") {
			untrackedLines = append(untrackedLines, line)
		} else {
			uncommittedLines = append(uncommittedLines, line)
		}
	}

	age := uncommittedAge(wt)
	stale := age > maxUncommitted

	var results []Result
	if len(uncommittedLines) > 0 {
		if stale {
			results = append(results, Result{
				Name:    "staleness/uncommitted" + suffix,
				Status:  StatusFail,
				Message: fmt.Sprintf("uncommitted changes for %s (max %s)", formatDuration(age), formatDuration(maxUncommitted)),
				Details: uncommittedLines,
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/uncommitted" + suffix,
				Status:  StatusOK,
				Message: "uncommitted changes are recent",
			})
		}
	}

	if len(untrackedLines) > 0 {
		if stale {
			results = append(results, Result{
				Name:    "staleness/untracked" + suffix,
				Status:  StatusFail,
				Message: fmt.Sprintf("%d untracked files for %s (max %s)", len(untrackedLines), formatDuration(age), formatDuration(maxUncommitted)),
				Details: untrackedLines,
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/untracked" + suffix,
				Status:  StatusOK,
				Message: "untracked files are recent",
			})
		}
	}

	if len(uncommittedLines) == 0 && len(untrackedLines) == 0 {
		results = append(results, Result{
			Name:    "staleness/uncommitted" + suffix,
			Status:  StatusOK,
			Message: "working tree clean",
		})
	}

	return results
}

// listWorktrees returns paths of all worktrees attached to the repo.
func listWorktrees(repo *Repo) []string {
	out, err := repo.Git("worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths
}

func (c *StalenessCheck) Fix(_ *Repo, results []Result) []Result {
	// Staleness checks have no automated fix.
	return results
}

// stashEntries returns each stash entry with its date and display string.
func stashEntries(repo *Repo) ([]stashEntry, error) {
	// %ci = committer date ISO, %gd = reflog selector, %s = subject
	out, err := repo.Git("stash", "list", "--format=%ci %gd: %s")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var entries []stashEntry
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 26 {
			continue
		}
		t, err := time.Parse("2006-01-02 15:04:05 -0700", line[:25])
		if err != nil {
			continue
		}
		entries = append(entries, stashEntry{date: t, display: line[26:]})
	}
	return entries, nil
}

// uncommittedAge returns how long ago the working tree at dir last changed,
// approximated by the time since its HEAD's last commit.
func uncommittedAge(dir string) time.Duration {
	out, err := gitInDir(dir, "log", "-1", "--format=%ci")
	if err != nil || out == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02 15:04:05 -0700", out)
	if err != nil {
		return 0
	}
	return time.Since(t)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	return d.Round(time.Hour).String()
}
