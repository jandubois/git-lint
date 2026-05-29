package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"12h", 12 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"xd", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDuration(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDuration(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if err == nil && got != tt.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFormatDurationConfig(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{12 * time.Hour, "12h0m0s"},
		{25 * time.Hour, "25h0m0s"},
	}
	for _, tt := range tests {
		if got := formatDurationConfig(tt.in); got != tt.want {
			t.Errorf("formatDurationConfig(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDurationJSONRoundTrip(t *testing.T) {
	tests := []struct {
		json string
		want time.Duration
	}{
		{`"7d"`, 7 * 24 * time.Hour},
		{`"12h"`, 12 * time.Hour},
	}
	for _, tt := range tests {
		var d Duration
		if err := json.Unmarshal([]byte(tt.json), &d); err != nil {
			t.Errorf("Unmarshal(%s) error: %v", tt.json, err)
			continue
		}
		if d.Duration != tt.want {
			t.Errorf("Unmarshal(%s) = %v, want %v", tt.json, d.Duration, tt.want)
		}
	}

	// Marshal renders whole days as "Nd".
	d := Duration{7 * 24 * time.Hour}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if string(out) != `"7d"` {
		t.Errorf("Marshal(7d) = %s, want %q", out, `"7d"`)
	}
}
