package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WorkOrgs   []string          `json:"workOrgs"`
	Identity   IdentityConfig    `json:"identity"`
	Thresholds ThresholdsConfig  `json:"thresholds"`
}

type IdentityConfig struct {
	Name          string `json:"name"`
	WorkEmail     string `json:"workEmail"`
	PersonalEmail string `json:"personalEmail"`
}

type ThresholdsConfig struct {
	StashMaxAge       Duration `json:"stashMaxAge"`
	StashMaxCount     int      `json:"stashMaxCount"`
	UncommittedMaxAge Duration `json:"uncommittedMaxAge"`
	UnpushedMaxAge    Duration `json:"unpushedMaxAge"`
}

// Duration wraps time.Duration with JSON unmarshaling from strings like "7d", "1d", "12h".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := parseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	// Support "Nd" for days, otherwise delegate to time.ParseDuration.
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func configPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "git-lint", "config.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "git-lint", "config.json")
}

func loadConfig() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}
