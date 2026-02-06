package main

import (
	"fmt"
	"strings"
)

type BranchCleanupCheck struct{}

func (c *BranchCleanupCheck) Check(repo *Repo) []Result {
	currentBranch, _ := repo.Git("symbolic-ref", "--short", "HEAD")
	mainBranch := repo.MainBranch()

	out, err := repo.Git("for-each-ref",
		"--format=%(refname:short)|%(objectname:short)|%(authorname)|%(upstream:track)",
		"refs/heads/")
	if err != nil || out == "" {
		return nil
	}

	merged := mergedBranches(repo, mainBranch)

	var results []Result
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		name, hash, author, track := parts[0], parts[1], parts[2], parts[3]

		if name == currentBranch || name == mainBranch {
			continue
		}

		if strings.Contains(track, "gone") {
			results = append(results, Result{
				Name:    fmt.Sprintf("branch/gone[%s]", name),
				Status:  StatusWarn,
				Message: fmt.Sprintf("upstream deleted (%s by %s)", hash, author),
				Fixable: true,
			})
		} else if merged[name] {
			results = append(results, Result{
				Name:    fmt.Sprintf("branch/merged[%s]", name),
				Status:  StatusWarn,
				Message: fmt.Sprintf("merged into %s (%s by %s)", mainBranch, hash, author),
				Fixable: true,
			})
		}
	}

	if len(results) == 0 {
		return []Result{{
			Name:    "branch/cleanup",
			Status:  StatusOK,
			Message: "no stale branches",
		}}
	}
	return results
}

func (c *BranchCleanupCheck) Fix(repo *Repo, results []Result) []Result {
	var fixed []Result
	for _, r := range results {
		if !r.Fixable {
			fixed = append(fixed, r)
			continue
		}
		_, param := splitResultName(r.Name)
		if param == "" {
			fixed = append(fixed, r)
			continue
		}
		_, err := repo.Git("branch", "-D", param)
		if err != nil {
			fixed = append(fixed, r)
		} else {
			fixed = append(fixed, Result{
				Name:    r.Name,
				Status:  StatusFix,
				Message: fmt.Sprintf("deleted %s", param),
			})
		}
	}
	return fixed
}

// mergedBranches returns names of local branches fully merged into the
// remote-tracking main branch (or local main if no upstream).
func mergedBranches(repo *Repo, mainBranch string) map[string]bool {
	if mainBranch == "" {
		return nil
	}
	ref := mainBranch + "@{upstream}"
	out, err := repo.Git("branch", "--merged", ref, "--format=%(refname:short)")
	if err != nil {
		out, _ = repo.Git("branch", "--merged", mainBranch, "--format=%(refname:short)")
	}
	if out == "" {
		return nil
	}
	m := make(map[string]bool)
	for _, name := range strings.Split(out, "\n") {
		m[name] = true
	}
	return m
}
