package main

import (
	"strings"
	"testing"
)

func TestClassifyResults(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    string
	}{
		{"empty", nil, "ok"},
		{"all ok", []Result{{Status: StatusOK}}, "ok"},
		{"warn", []Result{{Status: StatusOK}, {Status: StatusWarn}}, "warning"},
		{"fail", []Result{{Status: StatusWarn}, {Status: StatusFail}}, "critical"},
	}
	for _, tt := range tests {
		if got := classifyResults(tt.results); got != tt.want {
			t.Errorf("classifyResults(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFormatRepoSection(t *testing.T) {
	// ok and fix results produce no section.
	if got := formatRepoSection("repo", []Result{{Name: "a", Status: StatusOK}, {Name: "b", Status: StatusFix}}); got != "" {
		t.Errorf("clean repo section = %q, want empty", got)
	}

	section := formatRepoSection("repo", []Result{
		{Name: "branch/merged[old]", Status: StatusWarn, Message: "merged", Fixable: true},
		{Name: "staleness/uncommitted", Status: StatusFail, Message: "stale"},
	})
	for _, want := range []string{"**repo**", "branch/merged[old]: merged [fixable]", "staleness/uncommitted: stale"} {
		if !strings.Contains(section, want) {
			t.Errorf("section %q missing %q", section, want)
		}
	}
}
