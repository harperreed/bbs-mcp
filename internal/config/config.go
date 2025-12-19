// ABOUTME: BBS configuration management
// ABOUTME: Handles Charm server settings and preferences

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DefaultCharmHost is the default Charm server
const DefaultCharmHost = "charm.2389.dev"

// Config stores BBS configuration
type Config struct {
	// CharmHost is the Charm server URL (default: charm.2389.dev)
	CharmHost string `json:"charm_host,omitempty"`
}

// GetConfigPath returns the config file path
func GetConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configDir, "bbs", "config.json")
}

// Load reads config from disk
func Load() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes config to disk
func (c *Config) Save() error {
	path := GetConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// GetCharmHost returns the Charm host, preferring environment variable.
func (c *Config) GetCharmHost() string {
	// Environment variable takes precedence
	if host := os.Getenv("CHARM_HOST"); host != "" {
		return host
	}
	// Then config file
	if c.CharmHost != "" {
		return c.CharmHost
	}
	// Then default
	return DefaultCharmHost
}

// ApplyEnvironment sets environment variables from config.
// Call this before initializing Charm client.
func (c *Config) ApplyEnvironment() {
	if os.Getenv("CHARM_HOST") == "" {
		host := c.GetCharmHost()
		os.Setenv("CHARM_HOST", host)
	}
}
