package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

var errNotARepo = errors.New("not a git repository")

type Repo struct {
	Dir    string
	Config *Config
	Work   bool // true if any remote URL matches a work org

	mainBranch    string
	mainBranchSet bool
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

// MainBranch returns the name of the default branch. It prefers a local
// "main" or "master"; in a fork that has neither, it returns the local branch
// matching the upstream's default branch, covering custom default-branch
// names. Returns "" if none is found. The result is memoized.
func (r *Repo) MainBranch() string {
	if !r.mainBranchSet {
		r.mainBranch = r.computeMainBranch()
		r.mainBranchSet = true
	}
	return r.mainBranch
}

func (r *Repo) computeMainBranch() string {
	for _, name := range []string{"main", "master"} {
		if r.hasLocalBranch(name) {
			return name
		}
	}
	// Custom default branch: a fork whose default is neither main nor master.
	if !r.hasRemoteNamed("upstream") {
		return ""
	}
	if def := r.upstreamDefaultBranch(); def != "" && r.hasLocalBranch(def) {
		return def
	}
	return ""
}

// hasLocalBranch reports whether a local branch with the given name exists.
func (r *Repo) hasLocalBranch(name string) bool {
	return exec.Command("git", "-C", r.Dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+name).Run() == nil
}

// hasRemoteNamed reports whether a remote with the given name is configured.
func (r *Repo) hasRemoteNamed(name string) bool {
	remotes, _ := r.Remotes()
	return hasRemote(remotes, name)
}

// upstreamDefaultBranch returns the upstream remote's default branch name. It
// reads the local upstream/HEAD symref first, then a cached value, and only as
// a last resort queries the remote over the network, caching the result.
func (r *Repo) upstreamDefaultBranch() string {
	if ref, err := r.Git("symbolic-ref", "--short", "refs/remotes/upstream/HEAD"); err == nil && ref != "" {
		return strings.TrimPrefix(ref, "upstream/")
	}
	if cached := r.GitConfig("remote.upstream.lint-default"); cached != "" {
		return cached
	}
	out, err := r.Git("ls-remote", "--symref", "upstream", "HEAD")
	if err != nil {
		return ""
	}
	def := symrefHeadBranch(out)
	if def != "" {
		r.SetGitConfig("remote.upstream.lint-default", def)
	}
	return def
}

// symrefHeadBranch extracts the branch name from `git ls-remote --symref`
// output, mapping a line like "ref: refs/heads/main\tHEAD" to "main".
func symrefHeadBranch(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "ref:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strings.TrimPrefix(fields[1], "refs/heads/")
			}
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

// canonPath returns an absolute, symlink-resolved form of p with normalized
// separators. Git reports worktree paths as resolved, forward-slash strings;
// canonPath brings filepath-built paths into the same form so the two compare
// equal across platforms. On error it falls back to the cleaned input.
func canonPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

// sameDir reports whether two paths refer to the same directory, comparing
// their canonical forms.
func sameDir(a, b string) bool {
	return canonPath(a) == canonPath(b)
}
