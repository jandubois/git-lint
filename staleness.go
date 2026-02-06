package main

import (
	"fmt"
	"strings"
	"time"
)

type StalenessCheck struct{}

func (c *StalenessCheck) Check(repo *Repo) []Result {
	var results []Result

	maxAge := repo.Config.Thresholds.StashMaxAge.Duration
	maxCount := repo.Config.Thresholds.StashMaxCount

	// Rules 9-10: stash age and count.
	stashDates, err := stashEntryDates(repo)
	if err == nil {
		// Rule 9: stash age.
		now := time.Now()
		var old int
		for _, d := range stashDates {
			if now.Sub(d) > maxAge {
				old++
			}
		}
		if old > 0 {
			results = append(results, Result{
				Name:    "staleness/stash-age",
				Status:  StatusWarn,
				Message: fmt.Sprintf("%d stash entries older than %s", old, formatDuration(maxAge)),
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/stash-age",
				Status:  StatusOK,
				Message: "no stale stash entries",
			})
		}

		// Rule 10: stash count.
		if len(stashDates) > maxCount {
			results = append(results, Result{
				Name:    "staleness/stash-count",
				Status:  StatusWarn,
				Message: fmt.Sprintf("%d entries (max %d)", len(stashDates), maxCount),
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/stash-count",
				Status:  StatusOK,
				Message: fmt.Sprintf("%d entries", len(stashDates)),
			})
		}
	}

	// Rule 11: uncommitted changes age.
	maxUncommitted := repo.Config.Thresholds.UncommittedMaxAge.Duration
	if hasUncommitted(repo) {
		age := uncommittedAge(repo)
		if age > maxUncommitted {
			results = append(results, Result{
				Name:    "staleness/uncommitted",
				Status:  StatusWarn,
				Message: fmt.Sprintf("uncommitted changes for %s (max %s)", formatDuration(age), formatDuration(maxUncommitted)),
			})
		} else {
			results = append(results, Result{
				Name:    "staleness/uncommitted",
				Status:  StatusOK,
				Message: "uncommitted changes are recent",
			})
		}
	} else {
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

// stashEntryDates returns the date of each stash entry.
func stashEntryDates(repo *Repo) ([]time.Time, error) {
	// %ci = committer date, ISO format
	out, err := repo.Git("stash", "list", "--format=%ci")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var dates []time.Time
	for _, line := range strings.Split(out, "\n") {
		t, err := time.Parse("2006-01-02 15:04:05 -0700", line)
		if err != nil {
			continue
		}
		dates = append(dates, t)
	}
	return dates, nil
}

// hasUncommitted checks for staged or unstaged changes.
func hasUncommitted(repo *Repo) bool {
	out, _ := repo.Git("status", "--porcelain")
	return out != ""
}

// uncommittedAge returns how long ago the working tree last changed,
// approximated by the most recent mtime reported by git status.
// Falls back to time since last commit if we can't determine mtime.
func uncommittedAge(repo *Repo) time.Duration {
	// Use the last commit time as a proxy for when changes started accumulating.
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
