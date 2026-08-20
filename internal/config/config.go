// Package config keeps the application preferences between sessions.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the preferences that survive a restart. The zero value of each
// field is the default, so a missing or partial file still works.
type Config struct {
	// Transparent lets the terminal background show instead of painting one.
	Transparent bool `json:"transparent"`
	// Seen records that the startup checks have already been shown once.
	Seen bool `json:"seen"`
}

// Path is where the file lives, honouring XDG_CONFIG_HOME.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cupstui", "config.json"), nil
}

// Load reads the preferences. Any problem — missing, unreadable or corrupt
// file — yields the defaults: a look and feel preference cannot stop the
// application from starting.
func Load() Config {
	var c Config

	path, err := Path()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}
	}
	return c
}

// Save writes the preferences, creating the directory if needed.
func Save(c Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
