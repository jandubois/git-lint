package main

import (
	"fmt"
	"strings"
)

type ProtocolCheck struct{}

func (c *ProtocolCheck) Check(repo *Repo) []Result {
	want := repo.Config.Protocol
	if want == "" {
		return nil
	}

	remotes, _ := repo.Remotes()
	if len(remotes) == 0 {
		return nil
	}

	var results []Result
	for _, name := range remotes {
		url := repo.RemoteURL(name)
		got := urlProtocol(url)
		if got == want {
			continue
		}
		r := Result{
			Name:    fmt.Sprintf("remote/protocol[%s]", name),
			Status:  StatusWarn,
			Message: fmt.Sprintf("uses %s, want %s (%s)", got, want, url),
		}
		if converted := convertGitHubURL(url, want); converted != "" {
			r.Status = StatusFail
			r.Fixable = true
		}
		results = append(results, r)
	}

	if len(results) == 0 {
		return []Result{{
			Name:    "remote/protocol",
			Status:  StatusOK,
			Message: fmt.Sprintf("all remotes use %s", want),
		}}
	}
	return results
}

func (c *ProtocolCheck) Fix(repo *Repo, results []Result) []Result {
	want := repo.Config.Protocol
	var fixed []Result
	for _, r := range results {
		if r.Status != StatusFail || !r.Fixable {
			fixed = append(fixed, r)
			continue
		}
		// Extract remote name from "remote/protocol[name]".
		name := r.Name[len("remote/protocol[") : len(r.Name)-1]
		url := repo.RemoteURL(name)
		converted := convertGitHubURL(url, want)
		if converted == "" {
			fixed = append(fixed, r)
			continue
		}
		_, err := repo.Git("remote", "set-url", name, converted)
		if err != nil {
			fixed = append(fixed, r)
		} else {
			fixed = append(fixed, Result{
				Name:    r.Name,
				Status:  StatusFix,
				Message: fmt.Sprintf("set to %s", converted),
			})
		}
	}
	return fixed
}

// convertGitHubURL converts a GitHub URL between ssh and https.
// Returns "" if the URL is not a GitHub URL or already uses the target protocol.
func convertGitHubURL(url, target string) string {
	switch target {
	case "ssh":
		// https://github.com/org/repo.git → git@github.com:org/repo.git
		if path, ok := strings.CutPrefix(url, "https://github.com/"); ok {
			return "git@github.com:" + path
		}
	case "https":
		// git@github.com:org/repo.git → https://github.com/org/repo.git
		if path, ok := strings.CutPrefix(url, "git@github.com:"); ok {
			return "https://github.com/" + path
		}
	}
	return ""
}

// urlProtocol returns "ssh" or "https" based on the remote URL format.
func urlProtocol(url string) string {
	if strings.HasPrefix(url, "https://") {
		return "https"
	}
	// SCP-like syntax (git@host:path) or explicit ssh:// URLs.
	if strings.HasPrefix(url, "ssh://") || strings.Contains(url, "@") {
		return "ssh"
	}
	return ""
}
