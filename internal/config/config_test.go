// ABOUTME: Tests for sync configuration
// ABOUTME: Verifies config load, save, and path resolution

package config

import (
	"path/filepath"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()
	if path == "" {
		t.Error("GetConfigPath returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("GetConfigPath returned non-absolute path: %s", path)
	}
}

func TestGetVaultDBPath(t *testing.T) {
	path := GetVaultDBPath()
	if path == "" {
		t.Error("GetVaultDBPath returned empty string")
	}
}

func TestLoadNonExistent(t *testing.T) {
	// Temporarily override config path
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed on non-existent config: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Server:   "https://test.example.com",
		DeviceID: "test-device",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Server != cfg.Server {
		t.Errorf("Server mismatch: got %s, want %s", loaded.Server, cfg.Server)
	}
	if loaded.DeviceID != cfg.DeviceID {
		t.Errorf("DeviceID mismatch: got %s, want %s", loaded.DeviceID, cfg.DeviceID)
	}
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected bool
	}{
		{"empty", Config{}, false},
		{"server only", Config{Server: "https://test.com"}, false},
		{"server and token", Config{Server: "https://test.com", Token: "tok"}, false},
		{"fully configured", Config{Server: "https://test.com", Token: "tok", DerivedKey: "key"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsConfigured(); got != tt.expected {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.expected)
			}
		})
	}
}
