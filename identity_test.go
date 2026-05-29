package main

import "testing"

func TestIdentityPersonalRepoPasses(t *testing.T) {
	r := newTestRepo(t)
	results := (&IdentityCheck{}).Check(r.Repo)

	for _, want := range []string{"identity/name", "identity/email"} {
		got, ok := resultByName(results, want)
		if !ok {
			t.Fatalf("missing %s result", want)
		}
		if got.Status != StatusOK {
			t.Errorf("%s status = %q, want ok (%q)", want, got.Status, got.Message)
		}
	}
}

func TestIdentityNameMismatchFixable(t *testing.T) {
	r := newTestRepo(t)
	r.Config.Identity.Name = "Expected Name"

	results := (&IdentityCheck{}).Check(r.Repo)
	got, _ := resultByName(results, "identity/name")
	if got.Status != StatusFail || !got.Fixable {
		t.Fatalf("name check = %+v, want fixable fail", got)
	}

	fixed := (&IdentityCheck{}).Fix(r.Repo, results)
	gotFix, _ := resultByName(fixed, "identity/name")
	if gotFix.Status != StatusFix {
		t.Errorf("after fix: status = %q, want fix (%q)", gotFix.Status, gotFix.Message)
	}
	if name := r.git("config", "user.name"); name != "Expected Name" {
		t.Errorf("local user.name = %q, want %q", name, "Expected Name")
	}
}

func TestIdentityWorkRepoRequiresLocalWorkEmail(t *testing.T) {
	r := newTestRepo(t)
	r.git("remote", "add", "origin", "git@github.com:acme/repo.git")
	r.Config.WorkOrgs = []string{"acme"}
	r.Config.Identity.WorkEmail = "jan@acme.com"
	r.reload()
	if !r.Work {
		t.Fatal("repo not classified as work")
	}

	// Local email is the personal address, so the work-email check fails.
	results := (&IdentityCheck{}).Check(r.Repo)
	got, _ := resultByName(results, "identity/email")
	if got.Status != StatusFail || !got.Fixable {
		t.Fatalf("work email check = %+v, want fixable fail", got)
	}

	fixed := (&IdentityCheck{}).Fix(r.Repo, results)
	gotFix, _ := resultByName(fixed, "identity/email")
	if gotFix.Status != StatusFix {
		t.Errorf("after fix: status = %q, want fix (%q)", gotFix.Status, gotFix.Message)
	}
	if email := r.git("config", "--local", "user.email"); email != "jan@acme.com" {
		t.Errorf("local user.email = %q, want %q", email, "jan@acme.com")
	}
}
