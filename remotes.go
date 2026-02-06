package main

import (
	"fmt"
	"strings"
)

type RemoteCheck struct{}

func (c *RemoteCheck) Check(repo *Repo) []Result {
	remotes, _ := repo.Remotes()
	if len(remotes) < 2 {
		return nil
	}

	var results []Result

	// gh-resolved checks apply to all repos where origin is a fork.
	parentRemote := repo.ForkParentRemote()
	if parentRemote != "" {
		resolved := repo.GitConfig(fmt.Sprintf("remote.%s.gh-resolved", parentRemote))
		if resolved == "base" {
			results = append(results, Result{
				Name:    "remote/gh-resolved",
				Status:  StatusOK,
				Message: fmt.Sprintf("%s gh-resolved is base", parentRemote),
			})
		} else {
			results = append(results, Result{
				Name:    "remote/gh-resolved",
				Status:  StatusFail,
				Message: fmt.Sprintf("%s gh-resolved is %q, should be base", parentRemote, resolved),
				Fixable: true,
			})
		}

		// Flag stale gh-resolved on other remotes.
		for _, name := range remotes {
			if name == parentRemote {
				continue
			}
			resolved := repo.GitConfig(fmt.Sprintf("remote.%s.gh-resolved", name))
			if resolved != "" {
				results = append(results, Result{
					Name:    fmt.Sprintf("remote/gh-resolved[%s]", name),
					Status:  StatusFail,
					Message: fmt.Sprintf("%s has stale gh-resolved=%q", name, resolved),
					Fixable: true,
				})
			}
		}
	}

	if !repo.Work {
		return results
	}

	// Rule 4: origin should point to personal fork, not work org.
	originURL := repo.RemoteURL("origin")
	if org := workOrgInURL(originURL, repo.Config.WorkOrgs); org != "" {
		results = append(results, Result{
			Name:    "remote/origin",
			Status:  StatusFail,
			Message: fmt.Sprintf("origin points to work org %s (expected personal fork)", org),
		})
	} else {
		results = append(results, Result{
			Name:    "remote/origin",
			Status:  StatusOK,
			Message: "origin points to personal fork",
		})
	}

	// Rules 5-6 require a main branch.
	mainBranch := repo.MainBranch()
	if mainBranch != "" {
		// Rule 5: main/master should track a non-origin remote.
		upstream := repo.GitConfig(fmt.Sprintf("branch.%s.remote", mainBranch))
		upstreamRemote := upstreamFor(repo, remotes)
		if upstreamRemote == "" {
			// No non-origin work remote found; skip tracking check.
			results = append(results, Result{
				Name:    "remote/tracking",
				Status:  StatusOK,
				Message: fmt.Sprintf("%s tracks %s", mainBranch, upstream),
			})
		} else if upstream == upstreamRemote {
			results = append(results, Result{
				Name:    "remote/tracking",
				Status:  StatusOK,
				Message: fmt.Sprintf("%s tracks %s", mainBranch, upstream),
			})
		} else {
			results = append(results, Result{
				Name:    "remote/tracking",
				Status:  StatusFail,
				Message: fmt.Sprintf("%s tracks %q, should track %q", mainBranch, upstream, upstreamRemote),
				Fixable: true,
			})
		}

		// Rule 6: main/master pushRemote = no_push.
		pushRemote := repo.GitConfig(fmt.Sprintf("branch.%s.pushRemote", mainBranch))
		if pushRemote == "no_push" {
			results = append(results, Result{
				Name:    "remote/push-guard",
				Status:  StatusOK,
				Message: fmt.Sprintf("%s pushRemote is no_push", mainBranch),
			})
		} else {
			results = append(results, Result{
				Name:    "remote/push-guard",
				Status:  StatusFail,
				Message: fmt.Sprintf("%s pushRemote is %q, should be no_push", mainBranch, pushRemote),
				Fixable: true,
			})
		}
	}

	return results
}

func (c *RemoteCheck) Fix(repo *Repo, results []Result) []Result {
	var fixed []Result
	mainBranch := repo.MainBranch()

	for _, r := range results {
		if r.Status != StatusFail || !r.Fixable {
			fixed = append(fixed, r)
			continue
		}
		switch {
		case r.Name == "remote/tracking" && mainBranch != "":
			remotes, _ := repo.Remotes()
			upstream := upstreamFor(repo, remotes)
			if upstream == "" {
				fixed = append(fixed, r)
				continue
			}
			// Set upstream tracking: branch.<main>.remote and branch.<main>.merge.
			err1 := repo.SetGitConfig(fmt.Sprintf("branch.%s.remote", mainBranch), upstream)
			err2 := repo.SetGitConfig(fmt.Sprintf("branch.%s.merge", mainBranch), fmt.Sprintf("refs/heads/%s", mainBranch))
			if err1 != nil || err2 != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set %s to track %s/%s", mainBranch, upstream, mainBranch),
				})
			}

		case r.Name == "remote/push-guard" && mainBranch != "":
			key := fmt.Sprintf("branch.%s.pushRemote", mainBranch)
			if err := repo.SetGitConfig(key, "no_push"); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set %s pushRemote to no_push", mainBranch),
				})
			}

		case r.Name == "remote/gh-resolved":
			parentRemote := repo.ForkParentRemote()
			if parentRemote == "" {
				// Fall back to main's tracking remote.
				if mainBranch != "" {
					parentRemote = repo.GitConfig(fmt.Sprintf("branch.%s.remote", mainBranch))
				}
			}
			if parentRemote == "" || parentRemote == "origin" {
				fixed = append(fixed, r)
				continue
			}
			key := fmt.Sprintf("remote.%s.gh-resolved", parentRemote)
			if err := repo.SetGitConfig(key, "base"); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set %s gh-resolved to base", parentRemote),
				})
			}

		case strings.HasPrefix(r.Name, "remote/gh-resolved["):
			name := r.Name[len("remote/gh-resolved[") : len(r.Name)-1]
			key := fmt.Sprintf("remote.%s.gh-resolved", name)
			if err := repo.UnsetGitConfig(key); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("removed gh-resolved from %s", name),
				})
			}

		default:
			fixed = append(fixed, r)
		}
	}
	return fixed
}

// workOrgInURL returns the work org name found in the URL, or "".
func workOrgInURL(url string, orgs []string) string {
	for _, org := range orgs {
		if strings.Contains(url, "github.com/"+org+"/") ||
			strings.Contains(url, "github.com:"+org+"/") {
			return org
		}
	}
	return ""
}

// upstreamFor finds the upstream remote: prefers the fork parent remote,
// falls back to the first non-origin remote whose URL matches a work org.
func upstreamFor(repo *Repo, remotes []string) string {
	if parent := repo.ForkParentRemote(); parent != "" {
		return parent
	}
	for _, name := range remotes {
		if name == "origin" {
			continue
		}
		url := repo.RemoteURL(name)
		if workOrgInURL(url, repo.Config.WorkOrgs) != "" {
			return name
		}
	}
	return ""
}
