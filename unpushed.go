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

	branches, err := localBranches(repo)
	if err != nil || len(branches) == 0 {
		return []Result{{
			Name:    "staleness/unpushed",
			Status:  StatusOK,
			Message: "no unpushed commits",
		}}
	}

	now := time.Now()
	var results []Result
	for _, branch := range branches {
		// Skip branches handled by BranchCleanupCheck: PR checkouts
		// and orphan branches by other authors.
		remote, _ := repo.Git("config", fmt.Sprintf("branch.%s.remote", branch))
		if remote == "" {
			author, _ := repo.Git("log", "-1", "--format=%an", branch)
			if author != "" && author != repo.Config.Identity.Name {
				continue
			}
		} else {
			mergeRef, _ := repo.Git("config", fmt.Sprintf("branch.%s.merge", branch))
			if strings.HasPrefix(mergeRef, "refs/pull/") {
				continue
			}
		}
		commits := unpushedCommits(repo, branch)
		if len(commits) == 0 {
			continue
		}
		var stale int
		var details []string
		for _, line := range commits {
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
			details = append(details, fmt.Sprintf("%s %s (%s ago)", line[:7], subject, formatDuration(now.Sub(t))))
			if now.Sub(t) > maxAge {
				stale++
			}
		}
		if stale > 0 {
			msg := fmt.Sprintf("%d/%d commits older than %s", stale, len(details), formatDuration(maxAge))
			// Show the tip author when it differs from the configured user.
			author, _ := repo.Git("log", "-1", "--format=%an", branch)
			if author != "" && author != repo.Config.Identity.Name {
				msg += fmt.Sprintf(" (by %s)", author)
			}
			results = append(results, Result{
				Name:    fmt.Sprintf("staleness/unpushed[%s]", branch),
				Status:  StatusFail,
				Message: msg,
				Details: details,
			})
		}
	}

	if len(results) == 0 {
		return []Result{{
			Name:    "staleness/unpushed",
			Status:  StatusOK,
			Message: "no unpushed commits",
		}}
	}
	return results
}

func (c *UnpushedCheck) Fix(_ *Repo, results []Result) []Result {
	return results
}

func localBranches(repo *Repo) ([]string, error) {
	out, err := repo.Git("for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// unpushedCommits returns raw log lines for commits on branch that aren't
// on any remote. Uses upstream..branch when an upstream is configured;
// falls back to branch --not --remotes otherwise.
func unpushedCommits(repo *Repo, branch string) []string {
	format := "--format=%H %ci %s"
	out, err := repo.Git("log", branch+"@{upstream}.."+branch, format)
	if err != nil {
		out, _ = repo.Git("log", branch, "--not", "--remotes", format)
	}
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}
