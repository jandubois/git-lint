package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubmoduleNoModulesFileNoResult(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())
	if results := (&SubmoduleCheck{}).Check(r.Repo); results != nil {
		t.Errorf("no .gitmodules: got %+v, want nil", results)
	}
}

func TestSubmoduleUntrackedFiles(t *testing.T) {
	r := newTestRepo(t)
	r.commit("a.txt", "a", "first", time.Now())

	// A second repo serves as the submodule source.
	src, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, src, nil, "init", "--initial-branch=main")
	runGit(t, src, nil, "config", "user.name", "Test User")
	runGit(t, src, nil, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(src, "lib.txt"), []byte("lib"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, src, nil, "add", "lib.txt")
	runGit(t, src, []string{"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z"},
		"commit", "--message", "lib")

	// Local file transport is disabled by default; allow it for the test.
	r.git("-c", "protocol.file.allow=always", "submodule", "add", src, "sub")
	stamp := time.Now().Format(time.RFC3339)
	runGit(t, r.dir, []string{"GIT_AUTHOR_DATE=" + stamp, "GIT_COMMITTER_DATE=" + stamp},
		"commit", "--message", "add submodule")

	if err := os.WriteFile(filepath.Join(r.dir, "sub", "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	results := (&SubmoduleCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "submodule/untracked[sub]")
	if !ok || got.Status != StatusWarn {
		t.Fatalf("submodule untracked = %+v, want warn", results)
	}
}
