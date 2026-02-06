package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	path := filepath.Join(repo.Dir, settingsRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Result{{
				Name:    "claude/attribution",
				Status:  StatusFail,
				Message: fmt.Sprintf("%s missing", settingsRelPath),
				Fixable: true,
			}}
		}
		return []Result{{
			Name:    "claude/attribution",
			Status:  StatusWarn,
			Message: fmt.Sprintf("cannot read %s: %v", settingsRelPath, err),
		}}
	}

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

func (c *AttributionCheck) Fix(repo *Repo, results []Result) []Result {
	var fixed []Result
	for _, r := range results {
		if r.Status != StatusFail || !r.Fixable {
			fixed = append(fixed, r)
			continue
		}

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
	}
	return fixed
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
