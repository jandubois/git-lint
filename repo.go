package main

import (
	"errors"
	"os/exec"
	"strings"
)

var errNotARepo = errors.New("not a git repository")

type Repo struct {
	Dir    string
	Config *Config
	Work   bool // true if any remote URL matches a work org
}

func NewRepo(dir string, cfg *Config) (*Repo, error) {
	r := &Repo{Dir: dir, Config: cfg}
	if _, err := r.Git("rev-parse", "--git-dir"); err != nil {
		return nil, errNotARepo
	}
	if err := r.classify(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repo) classify() error {
	remotes, err := r.Remotes()
	if err != nil {
		return err
	}
	for _, name := range remotes {
		url := r.RemoteURL(name)
		for _, org := range r.Config.WorkOrgs {
			// Match github.com/org/ in any remote URL (both HTTPS and SSH).
			if strings.Contains(url, "github.com/"+org+"/") ||
				strings.Contains(url, "github.com:"+org+"/") {
				r.Work = true
				return nil
			}
		}
	}

	// A repo using the work email is also treated as a work repo.
	// Use effective config: a globally-set work email still means work.
	email := r.GitConfigEffective("user.email")
	if email != "" && email == r.Config.Identity.WorkEmail {
		r.Work = true
	}

	return nil
}

// Git runs a git command in the repo directory and returns trimmed stdout.
func (r *Repo) Git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

// GitConfig reads a single local git config value from .git/config.
// Returns "" if unset. Ignores global, system, and environment config.
func (r *Repo) GitConfig(key string) string {
	val, _ := r.Git("config", "--local", "--get", key)
	return val
}

// GitConfigEffective reads the effective git config value from all sources.
func (r *Repo) GitConfigEffective(key string) string {
	val, _ := r.Git("config", "--get", key)
	return val
}

// SetGitConfig sets a local git config value.
func (r *Repo) SetGitConfig(key, value string) error {
	_, err := r.Git("config", key, value)
	return err
}

// UnsetGitConfig removes a local git config value.
func (r *Repo) UnsetGitConfig(key string) error {
	_, err := r.Git("config", "--unset", key)
	return err
}

// Remotes returns the list of remote names.
func (r *Repo) Remotes() ([]string, error) {
	out, err := r.Git("remote")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// RemoteURL returns the fetch URL for a remote as stored in .git/config,
// bypassing insteadOf rewriting and environment overrides.
func (r *Repo) RemoteURL(name string) string {
	return r.GitConfig("remote." + name + ".url")
}

// MainBranch returns the name of the main branch ("main" or "master").
// Returns "" if neither exists.
func (r *Repo) MainBranch() string {
	for _, name := range []string{"main", "master"} {
		if err := exec.Command("git", "-C", r.Dir, "rev-parse", "--verify", "--quiet", name).Run(); err == nil {
			return name
		}
	}
	return ""
}

// RemoteForURL returns the remote name whose fetch URL contains the given substring.
func (r *Repo) RemoteForURL(substring string) string {
	remotes, _ := r.Remotes()
	for _, name := range remotes {
		if strings.Contains(r.RemoteURL(name), substring) {
			return name
		}
	}
	return ""
}
