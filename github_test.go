package main

import "testing"

func TestParseGitHubRepo(t *testing.T) {
	tests := []struct {
		in        string
		wantOwner string
		wantRepo  string
	}{
		{"https://github.com/owner/repo", "owner", "repo"},
		{"https://github.com/owner/repo.git", "owner", "repo"},
		{"git@github.com:owner/repo.git", "owner", "repo"},
		{"git@github.com:owner/repo", "owner", "repo"},
		{"owner/repo", "owner", "repo"},
		{"https://github.com/owner/repo/pull/5", "owner", "repo"},
		{"https://gitlab.com/owner/repo", "", ""},
		{"git@gitlab.com:owner/repo", "", ""},
		{"https://github.com/owner/", "", ""},
		{"owner", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		owner, repo := parseGitHubRepo(tt.in)
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("parseGitHubRepo(%q) = (%q, %q), want (%q, %q)",
				tt.in, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestGithubCloneURL(t *testing.T) {
	tests := []struct {
		owner    string
		repo     string
		protocol string
		want     string
	}{
		{"owner", "repo", "ssh", "git@github.com:owner/repo.git"},
		{"owner", "repo", "https", "https://github.com/owner/repo.git"},
		{"owner", "repo", "", "https://github.com/owner/repo.git"},
	}
	for _, tt := range tests {
		got := githubCloneURL(tt.owner, tt.repo, tt.protocol)
		if got != tt.want {
			t.Errorf("githubCloneURL(%q, %q, %q) = %q, want %q",
				tt.owner, tt.repo, tt.protocol, got, tt.want)
		}
	}
}
