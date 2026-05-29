package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReviewsNoWorktreeNoResult(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	if results := (&ReviewsCheck{}).Check(r.Repo); results != nil {
		t.Errorf("no .reviews worktree: got %+v, want nil", results)
	}
}

func TestReviewsUnpushedCommits(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())

	// A bare remote receives the reviews branch so @{upstream} resolves.
	bare := t.TempDir()
	runGit(t, bare, nil, "init", "--bare", "--initial-branch=main")
	r.git("remote", "add", "origin", bare)
	r.git("branch", "reviews")
	r.git("push", "--set-upstream", "origin", "reviews")

	// Check out reviews in a .reviews worktree and add a local commit there,
	// which is now ahead of its upstream.
	r.git("worktree", "add", ".reviews", "reviews")
	reviewsDir := filepath.Join(r.dir, ".reviews")
	stamp := time.Now().Format(time.RFC3339)
	runGit(t, reviewsDir, nil, "config", "user.name", "Test User")
	runGit(t, reviewsDir, nil, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(reviewsDir, "note.txt"), []byte("draft"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, reviewsDir, nil, "add", "note.txt")
	runGit(t, reviewsDir, []string{"GIT_AUTHOR_DATE=" + stamp, "GIT_COMMITTER_DATE=" + stamp},
		"commit", "--message", "review note")

	results := (&ReviewsCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "reviews/unpushed")
	if !ok || got.Status != StatusWarn {
		t.Fatalf("reviews check = %+v, want warn", results)
	}
	if len(got.Details) != 1 {
		t.Errorf("details = %d, want 1 unpushed commit", len(got.Details))
	}
}
