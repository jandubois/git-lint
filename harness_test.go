package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testRepo wraps a Repo built in a temporary directory with an isolated git
// configuration, so checks run against real git without reading the
// developer's global or system config.
type testRepo struct {
	*Repo
	t   *testing.T
	dir string
}

// newTestRepo initializes a git repository in a temp dir with a fixed local
// identity and a hermetic config. The returned Config presets the identity
// fields so IdentityCheck passes by default; tests adjust it as needed.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	dir := t.TempDir()

	// Isolate from the developer's real git config. GIT_CONFIG_GLOBAL points
	// at an empty file and GIT_CONFIG_NOSYSTEM drops the system config, so
	// results never depend on the host machine.
	cfgFile := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfgFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfgFile)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	r := &testRepo{t: t, dir: dir}
	r.git("init", "--initial-branch=main")
	r.git("config", "user.name", "Test User")
	r.git("config", "user.email", "test@example.com")

	cfg := &Config{
		Identity: IdentityConfig{
			Name:          "Test User",
			PersonalEmail: "test@example.com",
		},
	}
	repo, err := NewRepo(dir, cfg)
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	r.Repo = repo
	return r
}

// reload rebuilds the Repo from the current config, re-running the work/fork
// classification after a test changes remotes or config.
func (r *testRepo) reload() {
	r.t.Helper()
	repo, err := NewRepo(r.dir, r.Config)
	if err != nil {
		r.t.Fatalf("NewRepo: %v", err)
	}
	r.Repo = repo
}

// git runs a git command in the repo and fails the test on error.
func (r *testRepo) git(args ...string) string {
	r.t.Helper()
	return runGit(r.t, r.dir, nil, args...)
}

// commit writes a file and commits it, stamping author and committer dates
// so age-based checks are deterministic.
func (r *testRepo) commit(name, content, message string, date time.Time) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
	r.git("add", name)
	stamp := date.Format(time.RFC3339)
	runGit(r.t, r.dir, []string{"GIT_AUTHOR_DATE=" + stamp, "GIT_COMMITTER_DATE=" + stamp},
		"commit", "--message", message)
}

// commitAs writes a file and commits it under a different author identity,
// for exercising orphan-branch and foreign-author checks.
func (r *testRepo) commitAs(name, content, message, author, email string, date time.Time) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
	r.git("add", name)
	stamp := date.Format(time.RFC3339)
	runGit(r.t, r.dir, []string{
		"GIT_AUTHOR_DATE=" + stamp,
		"GIT_COMMITTER_DATE=" + stamp,
		"GIT_AUTHOR_NAME=" + author,
		"GIT_AUTHOR_EMAIL=" + email,
	}, "commit", "--message", message)
}

// runGit runs git in dir with optional extra environment variables, failing
// the test on error. It returns trimmed stdout+stderr.
func runGit(t *testing.T, dir string, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if extraEnv != nil {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// resultByName returns the first result whose Name matches, or a zero Result
// and false. Tests use it to assert on a specific check output.
func resultByName(results []Result, name string) (Result, bool) {
	for _, r := range results {
		if r.Name == name {
			return r, true
		}
	}
	return Result{}, false
}
