package main

import (
	"testing"
	"time"
)

func TestSplitResultName(t *testing.T) {
	tests := []struct {
		name      string
		wantRule  string
		wantParam string
	}{
		{"staleness/unpushed[bats]", "staleness/unpushed", "bats"},
		{"identity/name", "identity/name", ""},
		{"remote/protocol[origin]", "remote/protocol", "origin"},
	}
	for _, tt := range tests {
		rule, param := splitResultName(tt.name)
		if rule != tt.wantRule || param != tt.wantParam {
			t.Errorf("splitResultName(%q) = (%q, %q), want (%q, %q)",
				tt.name, rule, param, tt.wantRule, tt.wantParam)
		}
	}
}

func TestHasFailures(t *testing.T) {
	if hasFailures([]Result{{Status: StatusOK}, {Status: StatusWarn}}) {
		t.Error("hasFailures = true, want false for ok+warn")
	}
	if !hasFailures([]Result{{Status: StatusWarn}, {Status: StatusFail}}) {
		t.Error("hasFailures = false, want true when a fail is present")
	}
}

func TestHasNonOK(t *testing.T) {
	if hasNonOK([]Result{{Status: StatusOK}}) {
		t.Error("hasNonOK = true, want false for all ok")
	}
	if !hasNonOK([]Result{{Status: StatusOK}, {Status: StatusWarn}}) {
		t.Error("hasNonOK = false, want true when a warn is present")
	}
}

func TestApplyFlags(t *testing.T) {
	cfg := &Config{}
	applyFlags(cfg, "acme,globex", "ssh", "Jan", "work@x.com", "me@x.com", "7d", 3, "1d", "14d")

	if len(cfg.WorkOrgs) != 2 || cfg.WorkOrgs[0] != "acme" || cfg.WorkOrgs[1] != "globex" {
		t.Errorf("WorkOrgs = %v, want [acme globex]", cfg.WorkOrgs)
	}
	if cfg.Protocol != "ssh" {
		t.Errorf("Protocol = %q, want ssh", cfg.Protocol)
	}
	if cfg.Identity.Name != "Jan" || cfg.Identity.WorkEmail != "work@x.com" || cfg.Identity.PersonalEmail != "me@x.com" {
		t.Errorf("Identity = %+v", cfg.Identity)
	}
	if cfg.Thresholds.StashMaxAge.Duration != 7*24*time.Hour {
		t.Errorf("StashMaxAge = %v, want 7d", cfg.Thresholds.StashMaxAge.Duration)
	}
	if cfg.Thresholds.StashMaxCount != 3 {
		t.Errorf("StashMaxCount = %d, want 3", cfg.Thresholds.StashMaxCount)
	}
}

func TestApplyFlagsIgnoresInvalidDuration(t *testing.T) {
	cfg := &Config{}
	applyFlags(cfg, "", "", "", "", "", "garbage", 0, "", "")
	if cfg.Thresholds.StashMaxAge.Duration != 0 {
		t.Errorf("StashMaxAge = %v, want 0 (invalid input ignored)", cfg.Thresholds.StashMaxAge.Duration)
	}
}
