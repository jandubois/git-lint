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
		"--format=%(refname:short)|%(objectname:short)|%(authorname)|%(upstream:track)|%(upstream)",
		"refs/heads/")
	if err != nil || out == "" {
		return nil
	}

	merged := mergedBranches(repo, mainBranch)

	var results []Result
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		name, hash, author, track, upstream := parts[0], parts[1], parts[2], parts[3], parts[4]

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
		} else if reason := stalePRCheckout(repo, name, hash, author, mainBranch); reason != "" {
			results = append(results, Result{
				Name:    fmt.Sprintf("branch/pr[%s]", name),
				Status:  StatusWarn,
				Message: reason,
				Fixable: true,
			})
		} else if upstream == "" && author != repo.Config.Identity.Name {
			results = append(results, Result{
				Name:    fmt.Sprintf("branch/orphan[%s]", name),
				Status:  StatusWarn,
				Message: fmt.Sprintf("no upstream, tip by %s (%s)", author, hash),
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

// stalePRCheckout returns a non-empty reason if branch tracks a refs/pull/
// ref and is stale: either the branch is already merged into main, or the
// local commit no longer matches the remote PR head.
func stalePRCheckout(repo *Repo, branch, shortHash, author, mainBranch string) string {
	mergeRef, _ := repo.Git("config", fmt.Sprintf("branch.%s.merge", branch))
	if !strings.HasPrefix(mergeRef, "refs/pull/") {
		return ""
	}
	remote, _ := repo.Git("config", fmt.Sprintf("branch.%s.remote", branch))
	if remote == "" {
		return ""
	}

	// Extract PR number from refs/pull/<number>/head.
	pr := strings.Split(mergeRef, "/")[2]
	detail := fmt.Sprintf("(%s by %s)", shortHash, author)

	// Condition 1: branch is an ancestor of main (true merge).
	if mainBranch != "" {
		ref := mainBranch + "@{upstream}"
		_, err := repo.Git("merge-base", "--is-ancestor", branch, ref)
		if err != nil {
			_, err = repo.Git("merge-base", "--is-ancestor", branch, mainBranch)
		}
		if err == nil {
			return fmt.Sprintf("PR #%s merged %s", pr, detail)
		}
	}

	// Condition 2: local tip differs from remote PR ref.
	lsOut, err := repo.Git("ls-remote", remote, mergeRef)
	if err != nil || lsOut == "" {
		return ""
	}
	remoteHash := strings.Fields(lsOut)[0]
	if !strings.HasPrefix(remoteHash, shortHash) {
		return fmt.Sprintf("PR #%s updated since checkout %s", pr, detail)
	}

	return ""
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
