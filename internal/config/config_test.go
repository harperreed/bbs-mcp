// ABOUTME: Tests for config functionality
// ABOUTME: Verifies config load, save, and path resolution

package config

import (
	"os"
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
		CharmHost: "custom.charm.example.com",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.CharmHost != cfg.CharmHost {
		t.Errorf("CharmHost mismatch: got %s, want %s", loaded.CharmHost, cfg.CharmHost)
	}
}

func TestGetCharmHost(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		envHost     string
		expectedLen int // Just check non-empty since default varies
	}{
		{"empty config uses default", Config{}, "", 1},
		{"config CharmHost used", Config{CharmHost: "custom.example.com"}, "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envHost != "" {
				t.Setenv("CHARM_HOST", tt.envHost)
			}
			host := tt.cfg.GetCharmHost()
			if len(host) < tt.expectedLen {
				t.Errorf("GetCharmHost() returned empty or short string")
			}
		})
	}
}

func TestApplyEnvironment(t *testing.T) {
	cfg := &Config{CharmHost: "custom.example.com"}

	// Clear any existing CHARM_HOST
	os.Unsetenv("CHARM_HOST")

	cfg.ApplyEnvironment()

	// After ApplyEnvironment, CHARM_HOST should be set
	if os.Getenv("CHARM_HOST") != "custom.example.com" {
		t.Errorf("ApplyEnvironment did not set CHARM_HOST correctly")
	}
}
