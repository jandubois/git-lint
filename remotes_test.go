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

// forkRepo returns a non-work repo with origin + upstream remotes and the
// fork-parent lookup cached, so RemoteCheck stays offline.
func forkRepo(t *testing.T) *testRepo {
	t.Helper()
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	r.git("remote", "add", "origin", "https://github.com/me/repo.git")
	r.git("remote", "add", "upstream", "https://github.com/acme/repo.git")
	r.git("config", "remote.origin.gh-parent", "none")
	return r
}

func TestRemoteDefaultTracksUpstreamNonWork(t *testing.T) {
	r := forkRepo(t)
	r.reload()
	if r.Work {
		t.Fatal("repo should not be classified as work")
	}

	results := (&RemoteCheck{}).Check(r.Repo)
	if got, ok := resultByName(results, "remote/tracking"); !ok || got.Status != StatusFail || !got.Fixable {
		t.Fatalf("remote/tracking = %+v, want fixable fail", results)
	}
	if got, ok := resultByName(results, "remote/push-guard"); !ok || got.Status != StatusFail || !got.Fixable {
		t.Fatalf("remote/push-guard = %+v, want fixable fail", results)
	}

	fixed := (&RemoteCheck{}).Fix(r.Repo, results)
	if got, _ := resultByName(fixed, "remote/tracking"); got.Status != StatusFix {
		t.Errorf("after fix remote/tracking = %q", got.Status)
	}
	if v := r.git("config", "--local", "branch.main.remote"); v != "upstream" {
		t.Errorf("branch.main.remote = %q, want upstream", v)
	}
	if v := r.git("config", "--local", "branch.main.merge"); v != "refs/heads/main" {
		t.Errorf("branch.main.merge = %q, want refs/heads/main", v)
	}
	if got, _ := resultByName(fixed, "remote/push-guard"); got.Status != StatusFix {
		t.Errorf("after fix remote/push-guard = %q", got.Status)
	}
	if v := r.git("config", "--local", "branch.main.pushRemote"); v != "DISABLED" {
		t.Errorf("branch.main.pushRemote = %q, want DISABLED", v)
	}
}

func TestReleaseBranchMustTrackUpstream(t *testing.T) {
	r := forkRepo(t)
	r.git("branch", "release-1.2")
	r.git("config", "branch.release-1.2.remote", "origin") // wrong: should be upstream
	r.reload()

	results := (&RemoteCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "remote/release-tracking[release-1.2]")
	if !ok || got.Status != StatusFail || !got.Fixable {
		t.Fatalf("release-tracking = %+v, want fixable fail", results)
	}

	fixed := (&RemoteCheck{}).Fix(r.Repo, results)
	if gotFix, _ := resultByName(fixed, "remote/release-tracking[release-1.2]"); gotFix.Status != StatusFix {
		t.Errorf("after fix = %q", gotFix.Status)
	}
	if v := r.git("config", "--local", "branch.release-1.2.remote"); v != "upstream" {
		t.Errorf("branch.release-1.2.remote = %q, want upstream", v)
	}
	if v := r.git("config", "--local", "branch.release-1.2.merge"); v != "refs/heads/release-1.2" {
		t.Errorf("branch.release-1.2.merge = %q, want refs/heads/release-1.2", v)
	}
}

func TestReleaseBranchExemptFromOriginTracking(t *testing.T) {
	r := forkRepo(t)
	r.git("branch", "release-1.2")
	r.git("config", "branch.release-1.2.remote", "upstream") // correct
	r.git("config", "branch.release-1.2.pushRemote", "DISABLED")
	r.reload()

	results := (&RemoteCheck{}).Check(r.Repo)
	if _, ok := resultByName(results, "remote/branch-tracking[release-1.2]"); ok {
		t.Error("release branch tracking upstream should be exempt from remote/branch-tracking")
	}
	if got, _ := resultByName(results, "remote/release-tracking[release-1.2]"); got.Status != StatusOK {
		t.Errorf("release-tracking = %q, want ok", got.Status)
	}
	if got, _ := resultByName(results, "remote/release-push-guard[release-1.2]"); got.Status != StatusOK {
		t.Errorf("release-push-guard = %q, want ok", got.Status)
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
