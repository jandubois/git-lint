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

func TestProtocolCheckConvertsRemote(t *testing.T) {
	r := newTestRepo(t)
	r.git("remote", "add", "origin", "https://github.com/owner/repo.git")
	r.Config.Protocol = "ssh"
	r.reload()

	results := (&ProtocolCheck{}).Check(r.Repo)
	got, ok := resultByName(results, "remote/protocol[origin]")
	if !ok || got.Status != StatusFail || !got.Fixable {
		t.Fatalf("protocol check = %+v, want fixable fail", results)
	}

	fixed := (&ProtocolCheck{}).Fix(r.Repo, results)
	gotFix, _ := resultByName(fixed, "remote/protocol[origin]")
	if gotFix.Status != StatusFix {
		t.Errorf("after fix: status = %q, want fix", gotFix.Status)
	}
	if url := r.git("remote", "get-url", "origin"); url != "git@github.com:owner/repo.git" {
		t.Errorf("origin url = %q, want ssh form", url)
	}
}

func TestProtocolCheckDisabledWhenUnset(t *testing.T) {
	r := newTestRepo(t)
	r.git("remote", "add", "origin", "https://github.com/owner/repo.git")
	r.reload()

	if results := (&ProtocolCheck{}).Check(r.Repo); results != nil {
		t.Errorf("protocol unset: got %+v, want nil", results)
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
