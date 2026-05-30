package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// staleHookTemplates defines the exact files (name and size) that can be
// auto-removed. These are ancient hook templates that were copied into
// many local clones by mistake.
var staleHookTemplates = map[string]int64{
	"commit-msg":         635,
	"prepare-commit-msg": 358,
}

type HooksCheck struct{}

func (c *HooksCheck) Check(repo *Repo) []Result {
	hooksDir := filepath.Join(repo.Dir, ".git", "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return nil
	}

	// Sample hooks that git installs are inert; only active hooks (those
	// without the .sample suffix) override global config.
	var files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".sample") {
			continue
		}
		files = append(files, e)
	}
	if len(files) == 0 {
		return nil
	}

	fixable := isStaleTemplates(files)

	var details []string
	for _, f := range files {
		info, err := f.Info()
		if err == nil {
			details = append(details, fmt.Sprintf("%s (%d bytes)", f.Name(), info.Size()))
		} else {
			details = append(details, f.Name())
		}
	}

	msg := "local hooks override global config"
	if fixable {
		msg = "stale hook templates"
	}

	return []Result{{
		Name:    "hooks/local",
		Status:  StatusWarn,
		Message: msg,
		Details: details,
		Fixable: fixable,
	}}
}

func (c *HooksCheck) Fix(repo *Repo, results []Result) []Result {
	out := make([]Result, len(results))
	for i, r := range results {
		if r.Status != StatusWarn || !r.Fixable {
			out[i] = r
			continue
		}
		hooksDir := filepath.Join(repo.Dir, ".git", "hooks")
		failed := false
		for name := range staleHookTemplates {
			if err := os.Remove(filepath.Join(hooksDir, name)); err != nil && !os.IsNotExist(err) {
				failed = true
			}
		}
		if failed {
			out[i] = r
			continue
		}
		out[i] = Result{
			Name:    r.Name,
			Status:  StatusFix,
			Message: "removed stale hook templates",
		}
	}
	return out
}

// isStaleTemplates returns true when files match exactly the known stale
// hook templates by name and size.
func isStaleTemplates(files []os.DirEntry) bool {
	if len(files) != len(staleHookTemplates) {
		return false
	}
	for _, f := range files {
		expected, ok := staleHookTemplates[f.Name()]
		if !ok {
			return false
		}
		info, err := f.Info()
		if err != nil || info.Size() != expected {
			return false
		}
	}
	return true
}
