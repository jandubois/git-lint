package main

import (
	"testing"
	"time"
)

func TestUnpushedDisabledWhenUnset(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	if results := (&UnpushedCheck{}).Check(r.Repo); len(results) != 0 {
		t.Errorf("max age unset: got %+v, want none", results)
	}
}

func TestUnpushedFlagsOldCommits(t *testing.T) {
	r := newTestRepo(t)
	r.Config.Thresholds.UnpushedMaxAge = Duration{7 * 24 * time.Hour}
	r.commit("old.txt", "old", "old commit", time.Now().Add(-100*24*time.Hour))
	r.commit("new.txt", "new", "new commit", time.Now())

	results := (&UnpushedCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "staleness/unpushed[main]")
	if !ok {
		t.Fatalf("missing unpushed result; got %+v", results)
	}
	if got.Status != StatusFail {
		t.Errorf("status = %q, want fail (%q)", got.Status, got.Message)
	}
	if len(got.Details) != 2 {
		t.Errorf("details = %d lines, want 2 (both commits unpushed)", len(got.Details))
	}
}

func TestUnpushedAllRecentPasses(t *testing.T) {
	r := newTestRepo(t)
	r.Config.Thresholds.UnpushedMaxAge = Duration{7 * 24 * time.Hour}
	r.commit("a.txt", "a", "recent commit", time.Now())

	results := (&UnpushedCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "staleness/unpushed")
	if !ok || got.Status != StatusOK {
		t.Errorf("recent commits: got %+v, want ok", results)
	}
}
