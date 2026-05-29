package main

import "testing"

func TestHasRemote(t *testing.T) {
	remotes := []string{"origin", "upstream"}
	if !hasRemote(remotes, "upstream") {
		t.Error("hasRemote(upstream) = false, want true")
	}
	if hasRemote(remotes, "fork") {
		t.Error("hasRemote(fork) = true, want false")
	}
}

func TestWorkOrgInURL(t *testing.T) {
	orgs := []string{"acme"}
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/acme/repo.git", "acme"},
		{"git@github.com:acme/repo.git", "acme"},
		{"https://github.com/personal/repo.git", ""},
	}
	for _, tt := range tests {
		if got := workOrgInURL(tt.url, orgs); got != tt.want {
			t.Errorf("workOrgInURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestBranchExists(t *testing.T) {
	out := "main\nfeature\nreviews"
	if !branchExists(out, "feature") {
		t.Error("branchExists(feature) = false, want true")
	}
	if branchExists(out, "missing") {
		t.Error("branchExists(missing) = true, want false")
	}
}
