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

	var details []string
	for _, name := range remotes {
		url := repo.RemoteURL(name)
		got := urlProtocol(url)
		if got != want {
			details = append(details, fmt.Sprintf("%-12s %s", name, url))
		}
	}

	if len(details) > 0 {
		return []Result{{
			Name:    "remote/protocol",
			Status:  StatusWarn,
			Message: fmt.Sprintf("%d remotes not using %s", len(details), want),
			Details: details,
		}}
	}
	return []Result{{
		Name:    "remote/protocol",
		Status:  StatusOK,
		Message: fmt.Sprintf("all remotes use %s", want),
	}}
}

func (c *ProtocolCheck) Fix(_ *Repo, results []Result) []Result {
	return results
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
