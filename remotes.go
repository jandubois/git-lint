package main

import (
	"fmt"
	"strings"
)

// ForkSetupCheck detects repos where origin points to someone else's GitHub
// repo and the authenticated user owns a fork. The fix renames origin to
// upstream and adds the user's fork as origin, so subsequent checks operate
// on the correct remote layout.
type ForkSetupCheck struct{}

func (c *ForkSetupCheck) Check(repo *Repo) []Result {
	remotes, _ := repo.Remotes()
	if hasRemote(remotes, "upstream") {
		return nil
	}

	originURL := repo.RemoteURL("origin")
	owner, repoName := parseGitHubRepo(originURL)
	if owner == "" {
		return nil
	}

	me, err := ghUser()
	if err != nil {
		return nil
	}
	if strings.EqualFold(owner, me) {
		return nil
	}

	if !ghHasFork(me, owner, repoName) {
		return nil
	}

	return []Result{{
		Name:    "remote/fork-setup",
		Status:  StatusFail,
		Message: fmt.Sprintf("origin is %s/%s but you own a fork", owner, repoName),
		Fixable: true,
	}}
}

func (c *ForkSetupCheck) Fix(repo *Repo, results []Result) []Result {
	var fixed []Result
	for _, r := range results {
		if r.Status != StatusFail || !r.Fixable || r.Name != "remote/fork-setup" {
			fixed = append(fixed, r)
			continue
		}

		originURL := repo.RemoteURL("origin")
		_, repoName := parseGitHubRepo(originURL)
		if repoName == "" {
			fixed = append(fixed, r)
			continue
		}

		me, err := ghUser()
		if err != nil {
			fixed = append(fixed, r)
			continue
		}

		protocol := repo.Config.Protocol
		if protocol == "" {
			if strings.HasPrefix(originURL, "git@") {
				protocol = "ssh"
			} else {
				protocol = "https"
			}
		}

		if _, err := repo.Git("remote", "rename", "origin", "upstream"); err != nil {
			fixed = append(fixed, r)
			continue
		}

		// The rename moves remote.origin.* config to remote.upstream.*.
		// Clear the stale fork-parent cache from the renamed remote.
		repo.UnsetGitConfig("remote.upstream.gh-parent")

		forkURL := githubCloneURL(me, repoName, protocol)
		if _, err := repo.Git("remote", "add", "origin", forkURL); err != nil {
			repo.Git("remote", "rename", "upstream", "origin")
			fixed = append(fixed, r)
			continue
		}

		fixed = append(fixed, Result{
			Name:    r.Name,
			Status:  StatusFix,
			Message: fmt.Sprintf("renamed origin to upstream, added fork %s/%s as origin", me, repoName),
		})
	}
	return fixed
}

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

	// upstream remote pushurl should be DISABLED.
	if hasRemote(remotes, "upstream") {
		pushURL := repo.GitConfig("remote.upstream.pushurl")
		switch {
		case pushURL == "DISABLED":
			results = append(results, Result{
				Name:    "remote/push-url",
				Status:  StatusOK,
				Message: "upstream pushurl is DISABLED",
			})
		case pushURL == "":
			results = append(results, Result{
				Name:    "remote/push-url",
				Status:  StatusFail,
				Message: "upstream has no pushurl",
				Fixable: true,
			})
		default:
			results = append(results, Result{
				Name:    "remote/push-url",
				Status:  StatusWarn,
				Message: fmt.Sprintf("upstream pushurl is %q, expected DISABLED", pushURL),
			})
		}
	}

	// mainBranch is used by the branch-tracking check below and the
	// work-repo tracking/push-guard checks further down.
	mainBranch := repo.MainBranch()

	// Non-default branches should track origin, not upstream.
	branchOut, err := repo.Git("for-each-ref", "--format=%(refname:short)", "refs/heads/")
	hasUpstream := hasRemote(remotes, "upstream")
	if err == nil && branchOut != "" {
		for _, branch := range strings.Split(branchOut, "\n") {
			if branch == mainBranch || (branch == "reviews" && hasUpstream) {
				continue
			}
			remote := repo.GitConfig(fmt.Sprintf("branch.%s.remote", branch))
			if remote != "" && remote != "origin" {
				results = append(results, Result{
					Name:    fmt.Sprintf("remote/branch-tracking[%s]", branch),
					Status:  StatusWarn,
					Message: fmt.Sprintf("tracks %s, not origin", remote),
				})
			}
		}
	}

	// reviews branch should track origin, or upstream if the upstream repo is private.
	if hasUpstream && branchExists(branchOut, "reviews") {
		remote := repo.GitConfig("branch.reviews.remote")
		expected := reviewsExpectedRemote(repo)
		if remote == expected {
			results = append(results, Result{
				Name:    "remote/reviews-tracking",
				Status:  StatusOK,
				Message: fmt.Sprintf("reviews tracks %s", remote),
			})
		} else {
			results = append(results, Result{
				Name:    "remote/reviews-tracking",
				Status:  StatusFail,
				Message: fmt.Sprintf("reviews tracks %q, should track %s", remote, expected),
				Fixable: true,
			})
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

		// Rule 6: main/master pushRemote = DISABLED.
		pushRemote := repo.GitConfig(fmt.Sprintf("branch.%s.pushRemote", mainBranch))
		if pushRemote == "DISABLED" {
			results = append(results, Result{
				Name:    "remote/push-guard",
				Status:  StatusOK,
				Message: fmt.Sprintf("%s pushRemote is DISABLED", mainBranch),
			})
		} else {
			results = append(results, Result{
				Name:    "remote/push-guard",
				Status:  StatusFail,
				Message: fmt.Sprintf("%s pushRemote is %q, should be DISABLED", mainBranch, pushRemote),
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
			if err := repo.SetGitConfig(key, "DISABLED"); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set %s pushRemote to DISABLED", mainBranch),
				})
			}

		case r.Name == "remote/push-url":
			if err := repo.SetGitConfig("remote.upstream.pushurl", "DISABLED"); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: "set upstream pushurl to DISABLED",
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

		case r.Name == "remote/reviews-tracking":
			expected := reviewsExpectedRemote(repo)
			err1 := repo.SetGitConfig("branch.reviews.remote", expected)
			err2 := repo.SetGitConfig("branch.reviews.merge", "refs/heads/reviews")
			if err1 != nil || err2 != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set reviews to track %s/reviews", expected),
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

// hasRemote reports whether name appears in the remotes list.
func hasRemote(remotes []string, name string) bool {
	for _, r := range remotes {
		if r == name {
			return true
		}
	}
	return false
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

// branchExists reports whether name appears in the for-each-ref output.
func branchExists(branchOut, name string) bool {
	for _, b := range strings.Split(branchOut, "\n") {
		if b == name {
			return true
		}
	}
	return false
}

// reviewsExpectedRemote returns which remote the reviews branch should track.
// Returns "upstream" if the upstream repo is private, "origin" otherwise.
func reviewsExpectedRemote(repo *Repo) string {
	owner, repoName := parseGitHubRepo(repo.RemoteURL("upstream"))
	if owner != "" {
		if private, ok := ghRepoPrivate(owner, repoName); ok && private {
			return "upstream"
		}
	}
	return "origin"
}
