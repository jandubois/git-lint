package main

import (
	"fmt"
	"strings"
	"time"
)

type UnpushedCheck struct{}

func (c *UnpushedCheck) Check(repo *Repo) []Result {
	maxAge := repo.Config.Thresholds.UnpushedMaxAge.Duration
	if maxAge == 0 {
		return nil
	}

	// Find commits that exist on local branches but not on any remote-tracking branch.
	out, err := repo.Git("log", "--branches", "--not", "--remotes", "--format=%H %ci")
	if err != nil || out == "" {
		return []Result{{
			Name:    "staleness/unpushed",
			Status:  StatusOK,
			Message: "no unpushed commits",
		}}
	}

	now := time.Now()
	var total, stale int
	for _, line := range strings.Split(out, "\n") {
		total++
		// Format: <hash> <date> — hash is 40 chars, then space, then date.
		if len(line) < 42 {
			continue
		}
		dateStr := line[41:]
		t, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err != nil {
			continue
		}
		if now.Sub(t) > maxAge {
			stale++
		}
	}

	if stale > 0 {
		return []Result{{
			Name:    "staleness/unpushed",
			Status:  StatusWarn,
			Message: fmt.Sprintf("%d/%d unpushed commits older than %s", stale, total, formatDuration(maxAge)),
		}}
	}
	return []Result{{
		Name:    "staleness/unpushed",
		Status:  StatusOK,
		Message: fmt.Sprintf("%d unpushed commits (all recent)", total),
	}}
}

func (c *UnpushedCheck) Fix(_ *Repo, results []Result) []Result {
	return results
}
