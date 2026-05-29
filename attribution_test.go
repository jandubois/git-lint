package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAttributionPersonalRepoNoResults(t *testing.T) {
	r := newTestRepo(t)
	if results := (&AttributionCheck{}).Check(r.Repo); len(results) != 0 {
		t.Errorf("personal single-remote repo: got %+v, want none", results)
	}
}

func TestAttributionWorkRepoFixCreatesSettingsAndExcludes(t *testing.T) {
	r := newTestRepo(t)
	r.git("remote", "add", "origin", "git@github.com:acme/repo.git")
	r.Config.WorkOrgs = []string{"acme"}
	r.reload()

	results := (&AttributionCheck{}).Check(r.Repo)
	attr, ok := resultByName(results, "claude/attribution")
	if !ok || attr.Status != StatusFail || !attr.Fixable {
		t.Fatalf("attribution = %+v, want fixable fail", results)
	}
	excl, ok := resultByName(results, "local/exclude")
	if !ok || excl.Status != StatusFail || !excl.Fixable {
		t.Fatalf("exclude = %+v, want fixable fail", results)
	}

	(&AttributionCheck{}).Fix(r.Repo, results)

	// Re-check: both should now pass.
	after := (&AttributionCheck{}).Check(r.Repo)
	if got, _ := resultByName(after, "claude/attribution"); got.Status != StatusOK {
		t.Errorf("attribution after fix = %q (%q), want ok", got.Status, got.Message)
	}
	if got, _ := resultByName(after, "local/exclude"); got.Status != StatusOK {
		t.Errorf("exclude after fix = %q (%q), want ok", got.Status, got.Message)
	}

	if _, err := os.Stat(filepath.Join(r.dir, settingsRelPath)); err != nil {
		t.Errorf("settings file not created: %v", err)
	}
}
