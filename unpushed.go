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
	// Format: <40-char hash> <25-char date> <subject>
	out, err := repo.Git("log", "--branches", "--not", "--remotes", "--format=%H %ci %s")
	if err != nil || out == "" {
		return []Result{{
			Name:    "staleness/unpushed",
			Status:  StatusOK,
			Message: "no unpushed commits",
		}}
	}

	now := time.Now()
	var total, stale int
	var details []string
	for _, line := range strings.Split(out, "\n") {
		total++
		if len(line) < 42 {
			continue
		}
		dateStr := line[41:66]
		subject := ""
		if len(line) > 67 {
			subject = line[67:]
		}
		t, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err != nil {
			continue
		}
		detail := fmt.Sprintf("%s %s (%s ago)", line[:7], subject, formatDuration(now.Sub(t)))
		if now.Sub(t) > maxAge {
			stale++
		}
		details = append(details, detail)
	}

	if stale > 0 {
		return []Result{{
			Name:    "staleness/unpushed",
			Status:  StatusWarn,
			Message: fmt.Sprintf("%d/%d unpushed commits older than %s", stale, total, formatDuration(maxAge)),
			Details: details,
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
