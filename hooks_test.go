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

func TestHooksSampleFilesIgnored(t *testing.T) {
	r := newTestRepo(t)
	// git init installs *.sample hook files; these are not active hooks and
	// must not trigger a warning.
	if results := (&HooksCheck{}).Check(r.Repo); results != nil {
		t.Errorf("sample hooks: got %+v, want nil", results)
	}
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
	for _, name := range []string{"commit-msg", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooksDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s still present after fix (err = %v)", name, err)
		}
	}
}

func TestHooksFixPreservesSamples(t *testing.T) {
	r := newTestRepo(t)
	hooksDir := filepath.Join(r.dir, ".git", "hooks")
	// Keep git's installed samples; add the stale templates alongside them.
	write := func(name string, size int) {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(strings.Repeat("x", size)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("commit-msg", 635)
	write("prepare-commit-msg", 358)

	results := (&HooksCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "hooks/local")
	if !ok || !got.Fixable {
		t.Fatalf("stale templates among samples = %+v, want fixable", results)
	}

	(&HooksCheck{}).Fix(r.Repo, results)

	if _, err := os.Stat(filepath.Join(hooksDir, "commit-msg")); !os.IsNotExist(err) {
		t.Errorf("commit-msg not removed by fix (err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit.sample")); err != nil {
		t.Errorf("sample hook removed by fix: %v", err)
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
