package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ghUser returns the authenticated GitHub user login.
func ghUser() (string, error) {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh api user: %w (is gh installed and authenticated?)", err)
	}
	login := strings.TrimSpace(string(out))
	if login == "" {
		return "", fmt.Errorf("gh api user returned empty login")
	}
	return login, nil
}

// ghHasFork checks whether user has a fork of owner/repo.
// It queries user/repo and checks if its parent is owner/repo.
func ghHasFork(user, owner, repo string) bool {
	parent, ok := ghForkParent(user, repo)
	return ok && parent == owner+"/"+repo
}

// githubCloneURL builds a GitHub clone URL from owner/repo and protocol.
func githubCloneURL(owner, repo, protocol string) string {
	if protocol == "ssh" {
		return "git@github.com:" + owner + "/" + repo + ".git"
	}
	return "https://github.com/" + owner + "/" + repo + ".git"
}

// cloneRepo clones a GitHub repo and configures it via lintRepo --fix.
func cloneRepo(cfg *Config, arg string) error {
	owner, repo := parseGitHubRepo(arg)
	if owner == "" || repo == "" {
		return fmt.Errorf("cannot parse GitHub repo from %q", arg)
	}

	dest := repo
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("directory %q already exists", dest)
	}

	me, err := ghUser()
	if err != nil {
		return err
	}

	protocol := cfg.Protocol

	var cloneOwner, cloneRepo string
	var upstreamOwner, upstreamRepo string

	if strings.EqualFold(owner, me) {
		// I own this repo; clone it as origin.
		cloneOwner, cloneRepo = owner, repo
		// If it's a fork, add the parent as upstream.
		if parent, ok := ghForkParent(owner, repo); ok && parent != "" {
			parts := strings.SplitN(parent, "/", 2)
			upstreamOwner, upstreamRepo = parts[0], parts[1]
		}
	} else {
		// Someone else's repo. Check if I have a fork.
		if ghHasFork(me, owner, repo) {
			cloneOwner, cloneRepo = me, repo
			upstreamOwner, upstreamRepo = owner, repo
		} else {
			cloneOwner, cloneRepo = owner, repo
		}
	}

	cloneURL := githubCloneURL(cloneOwner, cloneRepo, protocol)
	fmt.Printf("Cloning %s/%s ...\n", cloneOwner, cloneRepo)
	cmd := exec.Command("git", "clone", cloneURL, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	if upstreamOwner != "" {
		upstreamURL := githubCloneURL(upstreamOwner, upstreamRepo, protocol)
		fmt.Printf("Adding upstream %s/%s ...\n", upstreamOwner, upstreamRepo)
		cmd = exec.Command("git", "-C", dest, "remote", "add", "upstream", upstreamURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git remote add upstream: %w", err)
		}
	}

	absDir, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	fmt.Printf("\nLinting %s ...\n", dest)
	opts := lintOptions{
		cfg:     cfg,
		fix:     true,
		verbose: true,
	}
	lintRepo(absDir, opts)
	return nil
}
