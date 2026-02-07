package main

import "fmt"

type IdentityCheck struct{}

func (c *IdentityCheck) Check(repo *Repo) []Result {
	var results []Result

	// user.name: check effective value; fix sets it locally.
	name := repo.GitConfigEffective("user.name")
	want := repo.Config.Identity.Name
	if name == want {
		results = append(results, Result{
			Name:    "identity/name",
			Status:  StatusOK,
			Message: name,
		})
	} else {
		results = append(results, Result{
			Name:    "identity/name",
			Status:  StatusFail,
			Message: fmt.Sprintf("got %q, want %q", name, want),
			Fixable: true,
		})
	}

	// Email rules:
	//   - Work repos require the work email set in local config.
	//   - Personal repos accept either configured email from any source.
	workEmail := repo.Config.Identity.WorkEmail
	personalEmail := repo.Config.Identity.PersonalEmail

	if repo.Work {
		// Work repos: require work email in local .git/config.
		localEmail := repo.GitConfig("user.email")
		if localEmail == workEmail {
			results = append(results, Result{
				Name:    "identity/email",
				Status:  StatusOK,
				Message: localEmail,
			})
		} else {
			results = append(results, Result{
				Name:    "identity/email",
				Status:  StatusFail,
				Message: fmt.Sprintf("got %q, want %q", localEmail, workEmail),
				Fixable: true,
			})
		}
	} else {
		// Personal repos: effective value from any config source suffices.
		email := repo.GitConfigEffective("user.email")
		if email == workEmail || email == personalEmail {
			results = append(results, Result{
				Name:    "identity/email",
				Status:  StatusOK,
				Message: email,
			})
		} else {
			results = append(results, Result{
				Name:    "identity/email",
				Status:  StatusFail,
				Message: fmt.Sprintf("got %q, want %q or %q", email, workEmail, personalEmail),
				Fixable: true,
			})
		}
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
