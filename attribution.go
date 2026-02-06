package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AttributionCheck struct{}

const settingsRelPath = ".claude/settings.local.json"

// claudeSettings represents the parts of settings.local.json we care about.
type claudeSettings struct {
	Attribution *claudeAttribution `json:"attribution,omitempty"`
	// Preserve other fields during round-trip.
	Other map[string]json.RawMessage `json:"-"`
}

type claudeAttribution struct {
	Commit string `json:"commit"`
	PR     string `json:"pr"`
}

func (c *AttributionCheck) Check(repo *Repo) []Result {
	if !repo.Work {
		return nil
	}

	var results []Result

	path := filepath.Join(repo.Dir, settingsRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			results = append(results, Result{
				Name:    "claude/attribution",
				Status:  StatusFail,
				Message: fmt.Sprintf("%s missing", settingsRelPath),
				Fixable: true,
			})
		} else {
			results = append(results, Result{
				Name:    "claude/attribution",
				Status:  StatusWarn,
				Message: fmt.Sprintf("cannot read %s: %v", settingsRelPath, err),
			})
		}
	} else {
		results = append(results, c.checkAttribution(data)...)
	}

	results = append(results, c.checkExclude(repo)...)
	return results
}

func (c *AttributionCheck) checkAttribution(data []byte) []Result {
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return []Result{{
			Name:    "claude/attribution",
			Status:  StatusWarn,
			Message: fmt.Sprintf("cannot parse %s: %v", settingsRelPath, err),
		}}
	}

	raw, ok := settings["attribution"]
	if !ok {
		return []Result{{
			Name:    "claude/attribution",
			Status:  StatusFail,
			Message: "attribution not configured",
			Fixable: true,
		}}
	}

	var attr claudeAttribution
	if err := json.Unmarshal(raw, &attr); err != nil {
		return []Result{{
			Name:    "claude/attribution",
			Status:  StatusWarn,
			Message: fmt.Sprintf("cannot parse attribution: %v", err),
		}}
	}

	if attr.Commit != "" || attr.PR != "" {
		return []Result{{
			Name:    "claude/attribution",
			Status:  StatusFail,
			Message: fmt.Sprintf("attribution not empty (commit=%q, pr=%q)", attr.Commit, attr.PR),
			Fixable: true,
		}}
	}

	return []Result{{
		Name:    "claude/attribution",
		Status:  StatusOK,
		Message: "attribution is empty",
	}}
}

// Patterns that should be in .git/info/exclude for work repos.
var claudeExcludePatterns = []string{"CLAUDE.md", ".claude/"}

func (c *AttributionCheck) checkExclude(repo *Repo) []Result {
	excludePath := filepath.Join(repo.Dir, ".git", "info", "exclude")
	existing := readLines(excludePath)

	var missing []string
	for _, pattern := range claudeExcludePatterns {
		if !containsLine(existing, pattern) {
			missing = append(missing, pattern)
		}
	}

	if len(missing) > 0 {
		return []Result{{
			Name:    "claude/exclude",
			Status:  StatusFail,
			Message: fmt.Sprintf(".git/info/exclude missing: %s", strings.Join(missing, ", ")),
			Fixable: true,
		}}
	}
	return []Result{{
		Name:    "claude/exclude",
		Status:  StatusOK,
		Message: "claude files excluded",
	}}
}

func (c *AttributionCheck) Fix(repo *Repo, results []Result) []Result {
	var fixed []Result
	for _, r := range results {
		if !r.Fixable {
			fixed = append(fixed, r)
			continue
		}

		switch {
		case r.Name == "claude/attribution":
			path := filepath.Join(repo.Dir, settingsRelPath)
			if err := ensureAttribution(path); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: fmt.Sprintf("set empty attribution in %s", settingsRelPath),
				})
			}
		case r.Name == "claude/exclude":
			excludePath := filepath.Join(repo.Dir, ".git", "info", "exclude")
			if err := ensureExcludePatterns(excludePath); err != nil {
				fixed = append(fixed, r)
			} else {
				fixed = append(fixed, Result{
					Name:    r.Name,
					Status:  StatusFix,
					Message: "added claude patterns to .git/info/exclude",
				})
			}
		default:
			fixed = append(fixed, r)
		}
	}
	return fixed
}

// ensureExcludePatterns appends missing claude patterns to the exclude file.
func ensureExcludePatterns(path string) error {
	existing := readLines(path)

	var toAdd []string
	for _, pattern := range claudeExcludePatterns {
		if !containsLine(existing, pattern) {
			toAdd = append(toAdd, pattern)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Ensure we start on a new line if the file doesn't end with one.
	if len(existing) > 0 {
		last := existing[len(existing)-1]
		if last != "" {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	}
	for _, pattern := range toAdd {
		if _, err := fmt.Fprintln(f, pattern); err != nil {
			return err
		}
	}
	return nil
}

// readLines returns all lines from a file, or nil if unreadable.
func readLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// containsLine returns true if any line matches the target exactly.
func containsLine(lines []string, target string) bool {
	for _, line := range lines {
		if line == target {
			return true
		}
	}
	return false
}

// ensureAttribution reads (or creates) the settings file and sets empty attribution.
func ensureAttribution(path string) error {
	var settings map[string]json.RawMessage

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	} else if os.IsNotExist(err) {
		settings = make(map[string]json.RawMessage)
	} else {
		return err
	}

	attr := claudeAttribution{Commit: "", PR: ""}
	raw, err := json.Marshal(attr)
	if err != nil {
		return err
	}
	settings["attribution"] = raw

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
