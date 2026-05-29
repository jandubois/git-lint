package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{48 * time.Hour, "2d"},
		{24 * time.Hour, "1d"},
		{25 * time.Hour, "1d"},
		{1 * time.Hour, "1h0m0s"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.in); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStalenessCleanRepo(t *testing.T) {
	r := newTestRepo(t)
	r.commit("file.txt", "hello", "initial", time.Now())

	results := (&StalenessCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "staleness/uncommitted")
	if !ok {
		t.Fatalf("no staleness/uncommitted result; got %+v", results)
	}
	if got.Status != StatusOK {
		t.Errorf("clean repo: status = %q, want %q (%q)", got.Status, StatusOK, got.Message)
	}
}

func TestStalenessWorktreeSuffix(t *testing.T) {
	r := newTestRepo(t)
	r.commit("file.txt", "hello", "initial", time.Now())

	wtPath := filepath.Join(t.TempDir(), "linked")
	r.git("worktree", "add", "-b", "feature", wtPath)

	results := (&StalenessCheck{}).Check(r.Repo)

	// The main worktree's result carries no suffix; the linked worktree's
	// does. This exercises the path comparison that differs across platforms.
	if _, ok := resultByName(results, "staleness/uncommitted"); !ok {
		var names []string
		for _, res := range results {
			names = append(names, res.Name)
		}
		t.Errorf("main worktree result missing unsuffixed name; got names %v", names)
	}
}
