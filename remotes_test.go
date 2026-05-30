package main

import (
	"testing"
	"time"
)

func TestBranchTrackingSuppressedForOrphan(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	r.git("remote", "add", "origin", "git@github.com:me/repo.git")
	r.git("remote", "add", "nirs", "https://github.com/nirs/repo.git")
	// Cache the fork-parent lookup so RemoteCheck stays offline.
	r.git("config", "remote.origin.gh-parent", "none")
	r.reload()

	// An orphan branch by another author that tracks the non-origin remote.
	// Setting only branch.<name>.remote (no merge) leaves it without an
	// upstream ref, so the cleanup check takes the offline no-upstream path.
	r.git("checkout", "-b", "direct-io")
	r.commitAs("c.txt", "c", "their work", "Other Dev", "other@example.com", time.Now())
	r.git("config", "branch.direct-io.remote", "nirs")
	r.git("checkout", "main")

	// Both checks flag direct-io before suppression.
	combined := (&RemoteCheck{}).Check(r.Repo)
	combined = append(combined, (&BranchCleanupCheck{}).Check(r.Repo)...)
	if _, ok := resultByName(combined, "remote/branch-tracking[direct-io]"); !ok {
		t.Fatalf("expected remote/branch-tracking warning before suppression; got %+v", combined)
	}
	if _, ok := resultByName(combined, "branch/orphan[direct-io]"); !ok {
		t.Fatalf("expected branch/orphan flag; got %+v", combined)
	}

	got := suppressRedundantTracking(combined)
	if _, ok := resultByName(got, "remote/branch-tracking[direct-io]"); ok {
		t.Error("tracking warning should be suppressed once the branch is flagged for cleanup")
	}
	if _, ok := resultByName(got, "branch/orphan[direct-io]"); !ok {
		t.Error("orphan flag should survive suppression")
	}
}

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
