package main

import "fmt"

type IdentityCheck struct{}

func (c *IdentityCheck) Check(repo *Repo) []Result {
	var results []Result

	// Rule 1: user.name
	name := repo.GitConfig("user.name")
	want := repo.Config.Identity.Name
	if name == want {
		results = append(results, Result{
			Name:   "identity/name",
			Status: StatusOK,
			Message: name,
		})
	} else {
		msg := fmt.Sprintf("got %q, want %q", name, want)
		results = append(results, Result{
			Name:    "identity/name",
			Status:  StatusFail,
			Message: msg,
			Fixable: true,
		})
	}

	// Email rules:
	//   - Work repos must use work email.
	//   - Personal repos accept either work or personal email.
	//   - Any other email always fails.
	email := repo.GitConfig("user.email")
	workEmail := repo.Config.Identity.WorkEmail
	personalEmail := repo.Config.Identity.PersonalEmail
	isWork := email == workEmail
	isPersonal := email == personalEmail

	switch {
	case repo.Work && isWork:
		results = append(results, Result{
			Name:    "identity/email",
			Status:  StatusOK,
			Message: email,
		})
	case repo.Work && !isWork:
		results = append(results, Result{
			Name:    "identity/email",
			Status:  StatusFail,
			Message: fmt.Sprintf("got %q, want %q", email, workEmail),
			Fixable: true,
		})
	case !repo.Work && (isWork || isPersonal):
		results = append(results, Result{
			Name:    "identity/email",
			Status:  StatusOK,
			Message: email,
		})
	default:
		results = append(results, Result{
			Name:    "identity/email",
			Status:  StatusFail,
			Message: fmt.Sprintf("got %q, want %q or %q", email, workEmail, personalEmail),
			Fixable: true,
		})
	}

	return results
}

func (c *IdentityCheck) Fix(repo *Repo, results []Result) []Result {
	var fixed []Result
	for _, r := range results {
		if r.Status != StatusFail || !r.Fixable {
			fixed = append(fixed, r)
			continue
		}
		switch r.Name {
		case "identity/name":
			if err := repo.SetGitConfig("user.name", repo.Config.Identity.Name); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set to %s", repo.Config.Identity.Name),
				})
			}
		case "identity/email":
			wantEmail := repo.Config.Identity.PersonalEmail
			if repo.Work {
				wantEmail = repo.Config.Identity.WorkEmail
			}
			if err := repo.SetGitConfig("user.email", wantEmail); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set to %s", wantEmail),
				})
			}
		default:
			fixed = append(fixed, r)
		}
	}
	return fixed
}
