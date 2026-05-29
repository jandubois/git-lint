package main

import "testing"

func TestURLProtocol(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/owner/repo.git", "https"},
		{"git@github.com:owner/repo.git", "ssh"},
		{"ssh://git@github.com/owner/repo.git", "ssh"},
		{"/local/path/repo", ""},
	}
	for _, tt := range tests {
		if got := urlProtocol(tt.url); got != tt.want {
			t.Errorf("urlProtocol(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestConvertGitHubURL(t *testing.T) {
	tests := []struct {
		url    string
		target string
		want   string
	}{
		{"https://github.com/owner/repo.git", "ssh", "git@github.com:owner/repo.git"},
		{"git@github.com:owner/repo.git", "https", "https://github.com/owner/repo.git"},
		{"git@github.com:owner/repo.git", "ssh", ""},
		{"https://github.com/owner/repo.git", "https", ""},
		{"https://gitlab.com/owner/repo.git", "ssh", ""},
	}
	for _, tt := range tests {
		if got := convertGitHubURL(tt.url, tt.target); got != tt.want {
			t.Errorf("convertGitHubURL(%q, %q) = %q, want %q", tt.url, tt.target, got, tt.want)
		}
	}
}
