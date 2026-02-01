// ABOUTME: Tests for config functionality
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

	cfg := &Config{}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded == nil {
		t.Error("loaded config is nil")
	}
}
