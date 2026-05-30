package main

import (
	"testing"
	"time"
)

func TestSymrefHeadBranch(t *testing.T) {
	out := "ref: refs/heads/develop\tHEAD\n0123456789\tHEAD"
	if got := symrefHeadBranch(out); got != "develop" {
		t.Errorf("symrefHeadBranch = %q, want develop", got)
	}
	if got := symrefHeadBranch("no symref here"); got != "" {
		t.Errorf("symrefHeadBranch(garbage) = %q, want empty", got)
	}
}

func TestMainBranchPrefersLocalMain(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	if got := r.MainBranch(); got != "main" {
		t.Errorf("MainBranch() = %q, want main", got)
	}
}

func TestMainBranchCustomDefaultFromUpstreamHead(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	// Rename so neither main nor master exists locally.
	r.git("branch", "-m", "main", "trunk")
	r.git("remote", "add", "upstream", "https://github.com/acme/repo.git")
	// Record the upstream's default branch locally, as a clone would.
	sha := r.git("rev-parse", "HEAD")
	r.git("update-ref", "refs/remotes/upstream/trunk", sha)
	r.git("symbolic-ref", "refs/remotes/upstream/HEAD", "refs/remotes/upstream/trunk")
	r.reload()

	if got := r.MainBranch(); got != "trunk" {
		t.Errorf("MainBranch() = %q, want trunk (custom default)", got)
	}
}

func TestMainBranchCustomDefaultFromCachedConfig(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	r.git("branch", "-m", "main", "trunk")
	r.git("remote", "add", "upstream", "https://github.com/acme/repo.git")
	// No upstream/HEAD symref; fall back to the cached config value.
	r.git("config", "remote.upstream.lint-default", "trunk")
	r.reload()

	if got := r.MainBranch(); got != "trunk" {
		t.Errorf("MainBranch() = %q, want trunk (from cached config)", got)
	}
}
