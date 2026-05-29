package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearHooks empties the .git/hooks directory that git init populates with
// sample files, so a test controls exactly which hooks are present.
func clearHooks(t *testing.T, repoDir string) string {
	t.Helper()
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.RemoveAll(hooksDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return hooksDir
}

func TestHooksEmptyDirNoResult(t *testing.T) {
	r := newTestRepo(t)
	clearHooks(t, r.dir)

	if results := (&HooksCheck{}).Check(r.Repo); results != nil {
		t.Errorf("empty hooks dir: got %+v, want nil", results)
	}
}

func TestHooksStaleTemplatesFixable(t *testing.T) {
	r := newTestRepo(t)
	hooksDir := clearHooks(t, r.dir)

	// The stale templates are recognized by exact name and byte size.
	write := func(name string, size int) {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(strings.Repeat("x", size)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("commit-msg", 635)
	write("prepare-commit-msg", 358)

	results := (&HooksCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "hooks/local")
	if !ok || got.Status != StatusWarn || !got.Fixable {
		t.Fatalf("stale templates check = %+v, want fixable warn", results)
	}

	fixed := (&HooksCheck{}).Fix(r.Repo, results)
	gotFix, _ := resultByName(fixed, "hooks/local")
	if gotFix.Status != StatusFix {
		t.Errorf("after fix: status = %q, want fix", gotFix.Status)
	}
	if _, err := os.Stat(hooksDir); !os.IsNotExist(err) {
		t.Errorf("hooks dir still present after fix (err = %v)", err)
	}
}

func TestHooksForeignHookNotFixable(t *testing.T) {
	r := newTestRepo(t)
	hooksDir := clearHooks(t, r.dir)
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	results := (&HooksCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "hooks/local")
	if !ok || got.Status != StatusWarn || got.Fixable {
		t.Fatalf("foreign hook check = %+v, want non-fixable warn", results)
	}
}
