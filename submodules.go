package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SubmoduleCheck struct{}

func (c *SubmoduleCheck) Check(repo *Repo) []Result {
	if _, err := os.Stat(filepath.Join(repo.Dir, ".gitmodules")); err != nil {
		return nil
	}

	paths, prefixes, err := submoduleStatus(repo)
	if err != nil {
		return []Result{{
			Name:    "submodule/status",
			Status:  StatusWarn,
			Message: fmt.Sprintf("cannot read submodule status: %v", err),
		}}
	}
	if len(paths) == 0 {
		return nil
	}

	var results []Result
	for i, path := range paths {
		results = append(results, c.checkSubmodule(repo, path, prefixes[i])...)
	}
	return results
}

func (c *SubmoduleCheck) checkSubmodule(repo *Repo, path string, prefix byte) []Result {
	var results []Result

	// Not initialized: submodule isn't cloned. Git commands in that
	// directory would fall through to the parent repo, so skip the
	// remaining checks.
	if prefix == '-' {
		results = append(results, Result{
			Name:    fmt.Sprintf("submodule/init[%s]", path),
			Status:  StatusWarn,
			Message: "submodule not initialized",
			Fixable: true,
		})
		return results
	}

	// Out of sync: checked-out commit differs from what the parent records.
	if prefix == '+' {
		results = append(results, Result{
			Name:    fmt.Sprintf("submodule/sync[%s]", path),
			Status:  StatusWarn,
			Message: "checked-out commit differs from parent",
		})
	}

	// Uncommitted and untracked: run git status inside the submodule.
	absPath := filepath.Join(repo.Dir, path)
	porcelain, err := gitInDir(absPath, "status", "--porcelain")
	if err == nil && porcelain != "" {
		var uncommittedDetails, untrackedDetails []string
		for _, line := range strings.Split(porcelain, "\n") {
			if strings.HasPrefix(line, "?? ") {
				untrackedDetails = append(untrackedDetails, line[3:])
			} else {
				uncommittedDetails = append(uncommittedDetails, line)
			}
		}
		if len(uncommittedDetails) > 0 {
			results = append(results, Result{
				Name:    fmt.Sprintf("submodule/uncommitted[%s]", path),
				Status:  StatusWarn,
				Message: fmt.Sprintf("%d uncommitted changes", len(uncommittedDetails)),
				Details: uncommittedDetails,
			})
		}
		if len(untrackedDetails) > 0 {
			results = append(results, Result{
				Name:    fmt.Sprintf("submodule/untracked[%s]", path),
				Status:  StatusWarn,
				Message: fmt.Sprintf("%d untracked files", len(untrackedDetails)),
				Details: untrackedDetails,
			})
		}
	}

	// Unpushed: commits ahead of upstream. Skip if no upstream configured.
	unpushed, err := gitInDir(absPath, "log", "@{upstream}..HEAD", "--oneline")
	if err == nil && unpushed != "" {
		lines := strings.Split(unpushed, "\n")
		results = append(results, Result{
			Name:    fmt.Sprintf("submodule/unpushed[%s]", path),
			Status:  StatusWarn,
			Message: fmt.Sprintf("%d unpushed commits", len(lines)),
			Details: lines,
		})
	}

	return results
}

func (c *SubmoduleCheck) Fix(repo *Repo, results []Result) []Result {
	// Collect uninitialized submodule paths and init them in one call.
	var paths []string
	for _, r := range results {
		if !r.Fixable {
			continue
		}
		_, param := splitResultName(r.Name)
		if param != "" {
			paths = append(paths, param)
		}
	}
	if len(paths) == 0 {
		return results
	}

	args := append([]string{"submodule", "update", "--init", "--recursive", "--"}, paths...)
	_, err := repo.Git(args...)

	var fixed []Result
	for _, r := range results {
		if !r.Fixable {
			fixed = append(fixed, r)
			continue
		}
		_, param := splitResultName(r.Name)
		if err != nil {
			fixed = append(fixed, r)
		} else {
			fixed = append(fixed, Result{
				Name:    r.Name,
				Status:  StatusFix,
				Message: fmt.Sprintf("initialized %s", param),
			})
		}
	}
	return fixed
}

// submoduleStatus parses `git submodule status` into paths and prefix characters.
// Each line has format: <prefix><sha> <path> [(<describe>)]
func submoduleStatus(repo *Repo) (paths []string, prefixes []byte, err error) {
	out, err := repo.Git("submodule", "status")
	if err != nil {
		return nil, nil, err
	}
	if out == "" {
		return nil, nil, nil
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 2 {
			continue
		}
		prefix := line[0]
		// After the prefix+sha, the path is the next space-delimited field.
		rest := line[1:] // skip prefix
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		paths = append(paths, fields[1])
		prefixes = append(prefixes, prefix)
	}
	return paths, prefixes, nil
}

// gitInDir runs a git command in the given directory and returns trimmed stdout.
func gitInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}
