package main

import (
	"testing"
	"time"
)

func TestBranchCleanupNoStaleBranches(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())

	results := (&BranchCleanupCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "branch/cleanup")
	if !ok || got.Status != StatusOK {
		t.Errorf("single branch: got %+v, want ok 'no stale branches'", results)
	}
}

func TestBranchCleanupMergedBranchFixable(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	r.git("checkout", "-b", "feature")
	r.commit("b.txt", "b", "feature work", time.Now())
	r.git("checkout", "main")
	r.git("merge", "feature")

	results := (&BranchCleanupCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "branch/merged[feature]")
	if !ok || got.Status != StatusWarn || !got.Fixable {
		t.Fatalf("merged branch = %+v, want fixable warn", results)
	}

	fixed := (&BranchCleanupCheck{}).Fix(r.Repo, results)
	gotFix, _ := resultByName(fixed, "branch/merged[feature]")
	if gotFix.Status != StatusFix {
		t.Errorf("after fix: status = %q, want fix (%q)", gotFix.Status, gotFix.Message)
	}
	if branches := r.git("for-each-ref", "--format=%(refname:short)", "refs/heads/"); branches != "main" {
		t.Errorf("branches after fix = %q, want only main", branches)
	}
}

func TestBranchCleanupCheckedOutBranchNotFixable(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	r.git("checkout", "-b", "feature")
	r.commit("b.txt", "b", "feature work", time.Now())
	r.git("checkout", "main")
	r.git("merge", "feature")
	// Leave feature checked out: it is merged but currently in the main
	// worktree, so deleting it must require switching branches first.
	r.git("checkout", "feature")

	results := (&BranchCleanupCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "branch/merged[feature]")
	if !ok {
		t.Fatalf("missing merged result; got %+v", results)
	}
	if got.Fixable {
		t.Errorf("checked-out branch reported fixable; message %q", got.Message)
	}
}

func TestBranchCleanupOrphanForeignAuthor(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	r.git("checkout", "-b", "foreign")
	r.commitAs("c.txt", "c", "someone else's work", "Other Dev", "other@example.com", time.Now())
	r.git("checkout", "main")

	results := (&BranchCleanupCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "branch/orphan[foreign]")
	if !ok || got.Status != StatusWarn || !got.Fixable {
		t.Fatalf("orphan branch = %+v, want fixable warn", results)
	}
}
