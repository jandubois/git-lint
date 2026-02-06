package main

import (
	"os/exec"
	"strings"
)

// parseGitHubRepo extracts owner and repo from a GitHub URL.
// Returns "", "" if the URL is not a GitHub URL.
func parseGitHubRepo(url string) (owner, repo string) {
	var path string
	switch {
	case strings.HasPrefix(url, "https://github.com/"):
		path = url[len("https://github.com/"):]
	case strings.HasPrefix(url, "git@github.com:"):
		path = url[len("git@github.com:"):]
	default:
		return "", ""
	}
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git")
}

// ghForkParent queries the GitHub API for the fork parent of owner/repo.
// Returns (parent, true) on success: parent is "owner/repo" or "" if not a fork.
// Returns ("", false) on any error (no gh CLI, network, 404, private repo).
func ghForkParent(owner, repo string) (parent string, ok bool) {
	cmd := exec.Command("gh", "api", "repos/"+owner+"/"+repo, "--jq", `.parent.full_name // empty`)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// ForkParent returns the "owner/repo" of origin's fork parent on GitHub.
// Caches the result in remote.origin.gh-parent to avoid repeated API calls.
// Returns "" if origin is not a GitHub fork or if the lookup fails transiently.
func (r *Repo) ForkParent() string {
	cached := r.GitConfig("remote.origin.gh-parent")
	if cached == "none" {
		return ""
	}
	if cached != "" {
		return cached
	}

	owner, repo := parseGitHubRepo(r.RemoteURL("origin"))
	if owner == "" {
		return ""
	}

	parent, ok := ghForkParent(owner, repo)
	if !ok {
		return ""
	}
	if parent == "" {
		r.SetGitConfig("remote.origin.gh-parent", "none")
		return ""
	}
	r.SetGitConfig("remote.origin.gh-parent", parent)
	return parent
}

// ForkParentRemote returns the remote name whose GitHub owner/repo matches
// origin's fork parent. Returns "" if no matching remote is found.
func (r *Repo) ForkParentRemote() string {
	parent := r.ForkParent()
	if parent == "" {
		return ""
	}
	remotes, _ := r.Remotes()
	for _, name := range remotes {
		if name == "origin" {
			continue
		}
		owner, repo := parseGitHubRepo(r.RemoteURL(name))
		if owner == "" {
			continue
		}
		if owner+"/"+repo == parent {
			return name
		}
	}
	return ""
}
