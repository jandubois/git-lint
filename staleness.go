package main

import (
	"fmt"
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
				Status:  StatusWarn,
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
				Status:  StatusWarn,
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

	// Uncommitted changes and untracked files.
	maxUncommitted := repo.Config.Thresholds.UncommittedMaxAge.Duration
	porcelain, _ := repo.Git("status", "--porcelain")
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

	age := uncommittedAge(repo)
	stale := age > maxUncommitted

	if len(uncommittedLines) > 0 {
		if stale {
			results = append(results, Result{
				Name:    "staleness/uncommitted",
				Status:  StatusWarn,
				Message: fmt.Sprintf("uncommitted changes for %s (max %s)", formatDuration(age), formatDuration(maxUncommitted)),
				Details: uncommittedLines,
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/uncommitted",
				Status:  StatusOK,
				Message: "uncommitted changes are recent",
			})
		}
	}

	if len(untrackedLines) > 0 {
		if stale {
			results = append(results, Result{
				Name:    "staleness/untracked",
				Status:  StatusWarn,
				Message: fmt.Sprintf("%d untracked files for %s (max %s)", len(untrackedLines), formatDuration(age), formatDuration(maxUncommitted)),
				Details: untrackedLines,
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/untracked",
				Status:  StatusOK,
				Message: "untracked files are recent",
			})
		}
	}

	if len(uncommittedLines) == 0 && len(untrackedLines) == 0 {
		results = append(results, Result{
			Name:    "staleness/uncommitted",
			Status:  StatusOK,
			Message: "working tree clean",
		})
	}

	return results
}

func (c *StalenessCheck) Fix(_ *Repo, results []Result) []Result {
	// Staleness checks are warn-only; nothing to fix.
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

// uncommittedAge returns how long ago the working tree last changed,
// approximated by the time since the last commit.
func uncommittedAge(repo *Repo) time.Duration {
	out, err := repo.Git("log", "-1", "--format=%ci")
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
