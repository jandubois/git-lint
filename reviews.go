package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ReviewsCheck struct{}

func (c *ReviewsCheck) Check(repo *Repo) []Result {
	worktree := filepath.Join(repo.Dir, ".reviews")
	if _, err := os.Stat(worktree); err != nil {
		return nil
	}

	// Only warn when the branch tracks a remote (otherwise there's
	// nowhere to push).
	remote := repo.GitConfig("branch.reviews.remote")
	if remote == "" {
		return nil
	}

	unpushed, err := gitInDir(worktree, "log", "@{upstream}..HEAD", "--oneline")
	if err != nil || unpushed == "" {
		return nil
	}

	lines := strings.Split(unpushed, "\n")
	return []Result{{
		Name:    "reviews/unpushed",
		Status:  StatusWarn,
		Message: fmt.Sprintf("%d unpushed commits in .reviews worktree", len(lines)),
		Details: lines,
	}}
}

func (c *ReviewsCheck) Fix(_ *Repo, results []Result) []Result {
	return results
}
