package main

import "testing"

func TestRemoteUpstreamPushURLFixable(t *testing.T) {
	r := newTestRepo(t)
	r.git("remote", "add", "origin", "git@github.com:me/repo.git")
	r.git("remote", "add", "upstream", "git@github.com:acme/repo.git")
	// Cache the fork-parent lookups as "none" so the check never calls gh.
	r.git("config", "remote.origin.gh-parent", "none")
	r.git("config", "remote.upstream.gh-parent", "none")
	r.reload()

	results := (&RemoteCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "remote/push-url")
	if !ok || got.Status != StatusFail || !got.Fixable {
		t.Fatalf("push-url check = %+v, want fixable fail", results)
	}

	fixed := (&RemoteCheck{}).Fix(r.Repo, results)
	gotFix, _ := resultByName(fixed, "remote/push-url")
	if gotFix.Status != StatusFix {
		t.Errorf("after fix: status = %q, want fix", gotFix.Status)
	}
	if url := r.git("config", "--local", "remote.upstream.pushurl"); url != "DISABLED" {
		t.Errorf("upstream pushurl = %q, want DISABLED", url)
	}
}

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
