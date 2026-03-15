package main

import (
	"os"
	"path/filepath"
	"strings"
)

// DependabotCheck warns when a non-fork repo owned by the authenticated user
// has a .github directory but no Dependabot configuration.
type DependabotCheck struct{}

func (c *DependabotCheck) Check(repo *Repo) []Result {
	originURL := repo.RemoteURL("origin")
	owner, _ := parseGitHubRepo(originURL)
	if owner == "" {
		return nil
	}

	me, err := ghUser()
	if err != nil {
		return nil
	}
	if !strings.EqualFold(owner, me) {
		return nil
	}

	if repo.ForkParent() != "" {
		return nil
	}

	dotGithub := filepath.Join(repo.Dir, ".github")
	if _, err := os.Stat(dotGithub); err != nil {
		return nil
	}

	for _, name := range []string{"dependabot.yml", "dependabot.yaml"} {
		if _, err := os.Stat(filepath.Join(dotGithub, name)); err == nil {
			return []Result{{
				Name:    "github/dependabot",
				Status:  StatusOK,
				Message: "dependabot configured",
			}}
		}
	}

	return []Result{{
		Name:    "github/dependabot",
		Status:  StatusWarn,
		Message: ".github exists but has no dependabot config",
	}}
}

func (c *DependabotCheck) Fix(_ *Repo, results []Result) []Result {
	return results
}
