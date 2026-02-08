package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Probe description and result types match the monitor's probe protocol.

type probeDescription struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Version         string         `json:"version"`
	Arguments       probeArguments `json:"arguments"`
	Output          probeOutput    `json:"output,omitempty"`
	DefaultName     string         `json:"default_name,omitempty"`
	DefaultInterval string         `json:"default_interval,omitempty"`
}

type probeArguments struct {
	Required map[string]probeArgSpec `json:"required,omitempty"`
	Optional map[string]probeArgSpec `json:"optional,omitempty"`
}

type probeArgSpec struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     any    `json:"default,omitempty"`
}

type probeOutput struct {
	Metrics map[string]probeMetricSpec `json:"metrics,omitempty"`
}

type probeMetricSpec struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type probeResult struct {
	Status  string         `json:"status"`
	Summary string         `json:"summary,omitempty"`
	Message string         `json:"message"`
	Metrics map[string]any `json:"metrics,omitempty"`
}

func probeDescribe(cfg *Config) {
	optional := map[string]probeArgSpec{
		"Work Orgs": {
			Type:        "string",
			Description: "Comma-separated GitHub work organizations",
		},
		"Protocol": {
			Type:        "string",
			Description: "Preferred git protocol (ssh or https)",
		},
		"Identity Name": {
			Type:        "string",
			Description: "Expected git user name",
		},
		"Work Email": {
			Type:        "string",
			Description: "Expected work email address",
		},
		"Personal Email": {
			Type:        "string",
			Description: "Expected personal email address",
		},
		"Stash Max Age": {
			Type:        "string",
			Description: "Max stash entry age (e.g. 7d, 12h)",
		},
		"Stash Max Count": {
			Type:        "integer",
			Description: "Max number of stash entries",
		},
		"Uncommitted Max Age": {
			Type:        "string",
			Description: "Max age for uncommitted changes (e.g. 1d)",
		},
		"Unpushed Max Age": {
			Type:        "string",
			Description: "Max age for unpushed commits (e.g. 7d)",
		},
	}

	// Populate defaults from config file values
	if len(cfg.WorkOrgs) > 0 {
		optional["Work Orgs"] = withDefault(optional["Work Orgs"], joinStrings(cfg.WorkOrgs))
	}
	if cfg.Protocol != "" {
		optional["Protocol"] = withDefault(optional["Protocol"], cfg.Protocol)
	}
	if cfg.Identity.Name != "" {
		optional["Identity Name"] = withDefault(optional["Identity Name"], cfg.Identity.Name)
	}
	if cfg.Identity.WorkEmail != "" {
		optional["Work Email"] = withDefault(optional["Work Email"], cfg.Identity.WorkEmail)
	}
	if cfg.Identity.PersonalEmail != "" {
		optional["Personal Email"] = withDefault(optional["Personal Email"], cfg.Identity.PersonalEmail)
	}
	if cfg.Thresholds.StashMaxAge.Duration > 0 {
		optional["Stash Max Age"] = withDefault(optional["Stash Max Age"], formatDurationConfig(cfg.Thresholds.StashMaxAge.Duration))
	}
	if cfg.Thresholds.StashMaxCount > 0 {
		optional["Stash Max Count"] = withDefault(optional["Stash Max Count"], cfg.Thresholds.StashMaxCount)
	}
	if cfg.Thresholds.UncommittedMaxAge.Duration > 0 {
		optional["Uncommitted Max Age"] = withDefault(optional["Uncommitted Max Age"], formatDurationConfig(cfg.Thresholds.UncommittedMaxAge.Duration))
	}
	if cfg.Thresholds.UnpushedMaxAge.Duration > 0 {
		optional["Unpushed Max Age"] = withDefault(optional["Unpushed Max Age"], formatDurationConfig(cfg.Thresholds.UnpushedMaxAge.Duration))
	}

	desc := probeDescription{
		Name:        "git-lint",
		Description: "Comprehensive git repository health checker",
		Version:     version,
		Arguments: probeArguments{
			Required: map[string]probeArgSpec{
				"Path": {
					Type:        "string",
					Description: "Root directory containing git repositories",
				},
			},
			Optional: optional,
		},
		Output: probeOutput{
			Metrics: map[string]probeMetricSpec{
				"repos_checked": {Type: "integer", Description: "Repositories scanned"},
				"repos_ok":      {Type: "integer", Description: "Repositories with no issues"},
				"repos_warned":  {Type: "integer", Description: "Repositories with warnings"},
				"repos_failed":  {Type: "integer", Description: "Repositories with failures"},
			},
		},
		DefaultName:     "Git Lint: {{Path}}",
		DefaultInterval: "1h",
	}

	_ = json.NewEncoder(os.Stdout).Encode(desc)
}

func withDefault(spec probeArgSpec, value any) probeArgSpec {
	spec.Default = value
	return spec
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}

func probeRun(path string, cfg *Config) int {
	absPath, err := filepath.Abs(path)
	if err != nil {
		outputProbeResult(probeResult{
			Status:  "critical",
			Message: fmt.Sprintf("invalid path: %v", err),
		})
		return 0
	}

	if err := os.Chdir(absPath); err != nil {
		outputProbeResult(probeResult{
			Status:  "critical",
			Message: fmt.Sprintf("cannot access %s: %v", path, err),
		})
		return 0
	}

	opts := lintOptions{cfg: cfg}

	entries, err := os.ReadDir(".")
	if err != nil {
		outputProbeResult(probeResult{
			Status:  "critical",
			Message: fmt.Sprintf("cannot read directory: %v", err),
		})
		return 0
	}

	var (
		reposChecked int
		reposOK      int
		reposWarned  int
		reposFailed  int
		worstStatus  string = "ok"
		message      string
	)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(entry.Name(), ".git")); err != nil {
			continue
		}

		absDir, err := filepath.Abs(entry.Name())
		if err != nil {
			continue
		}

		results, _ := runChecks(absDir, opts)
		reposChecked++

		repoStatus := classifyResults(results)
		section := formatRepoSection(entry.Name(), results)

		switch repoStatus {
		case "critical":
			reposFailed++
			message += section
			worstStatus = "critical"
		case "warning":
			reposWarned++
			message += section
			if worstStatus == "ok" {
				worstStatus = "warning"
			}
		default:
			reposOK++
		}
	}

	if reposChecked == 0 {
		outputProbeResult(probeResult{
			Status:  "ok",
			Message: "no git repositories found",
		})
		return 0
	}

	needAttention := reposWarned + reposFailed
	var summary string
	if needAttention > 0 {
		summary = fmt.Sprintf("%d of %d repos need attention", needAttention, reposChecked)
	} else {
		summary = fmt.Sprintf("%d repos clean", reposChecked)
	}

	if message == "" {
		message = summary
	}

	outputProbeResult(probeResult{
		Status:  worstStatus,
		Summary: summary,
		Message: message,
		Metrics: map[string]any{
			"repos_checked": reposChecked,
			"repos_ok":      reposOK,
			"repos_warned":  reposWarned,
			"repos_failed":  reposFailed,
		},
	})
	return 0
}

// classifyResults maps git-lint result statuses to probe statuses.
// fail → critical, warn → warning, ok/fix → ok.
// Returns the worst status across all results.
func classifyResults(results []Result) string {
	worst := "ok"
	for _, r := range results {
		switch r.Status {
		case StatusFail:
			return "critical"
		case StatusWarn:
			worst = "warning"
		}
	}
	return worst
}

// formatRepoSection builds a Markdown section for a repo with issues.
func formatRepoSection(name string, results []Result) string {
	var section string
	for _, r := range results {
		if r.Status == StatusOK || r.Status == StatusFix {
			continue
		}
		fix := ""
		if r.Fixable {
			fix = " [fixable]"
		}
		section += fmt.Sprintf("- %s: %s%s\n", r.Name, r.Message, fix)
	}
	if section == "" {
		return ""
	}
	return fmt.Sprintf("**%s**\n%s\n", name, section)
}

func outputProbeResult(r probeResult) {
	_ = json.NewEncoder(os.Stdout).Encode(r)
}
