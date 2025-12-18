// ABOUTME: Sync configuration management
// ABOUTME: Handles server, auth, and encryption settings

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config stores sync configuration
type Config struct {
	Server       string `json:"server"`
	UserID       string `json:"user_id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	TokenExpires string `json:"token_expires"`
	DerivedKey   string `json:"derived_key"`
	DeviceID     string `json:"device_id"`
	VaultDB      string `json:"vault_db"`
}

// GetConfigPath returns the config file path
func GetConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configDir, "bbs", "sync.json")
}

// GetVaultDBPath returns the vault database path
func GetVaultDBPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configDir, "bbs", "vault.db")
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

// IsConfigured returns true if sync is configured
func (c *Config) IsConfigured() bool {
	return c.Server != "" && c.Token != "" && c.DerivedKey != ""
}
