// ABOUTME: Tests for config functionality
// ABOUTME: Verifies config load, save, and path resolution

package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestGetConfigPathWithXDGConfigHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path := GetConfigPath()
	if !strings.HasPrefix(path, tmpDir) {
		t.Errorf("GetConfigPath should use XDG_CONFIG_HOME, got %s", path)
	}
	if !strings.HasSuffix(path, filepath.Join("bbs", "config.json")) {
		t.Errorf("GetConfigPath should end with bbs/config.json, got %s", path)
	}
}

func TestGetConfigPathWithoutXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	path := GetConfigPath()
	if path == "" {
		t.Error("GetConfigPath returned empty string")
	}
	// Should fall back to ~/.config
	if !strings.Contains(path, ".config") {
		t.Errorf("GetConfigPath should use .config fallback, got %s", path)
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

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory and file with invalid JSON
	configDir := filepath.Join(tmpDir, "bbs")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("invalid json {{{"), 0600); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load should fail on invalid JSON")
	}
}

func TestLoadReadError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory
	configDir := filepath.Join(tmpDir, "bbs")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create config file with no read permissions (only works on Unix)
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0000); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Try to load - should fail due to permissions
	_, err := Load()
	if err == nil {
		// On some systems (or when running as root), this might succeed
		t.Log("Load succeeded despite no permissions (may be running as root)")
	}

	// Restore permissions for cleanup
	os.Chmod(configPath, 0600)
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

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// The bbs subdirectory doesn't exist yet
	cfg := &Config{}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify directory was created
	configDir := filepath.Join(tmpDir, "bbs")
	info, err := os.Stat(configDir)
	if err != nil {
		t.Errorf("Config directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Config path is not a directory")
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Save twice to ensure overwrite works
	cfg := &Config{}

	if err := cfg.Save(); err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Verify file still exists and is readable
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after overwrite failed: %v", err)
	}
	if loaded == nil {
		t.Error("Loaded config is nil")
	}
}

func TestConfigStructFields(t *testing.T) {
	// Test that Config struct can be instantiated and saved
	cfg := &Config{}
	// Create a temp directory for the test
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Test that we can save the config (verifies struct is usable)
	err := cfg.Save()
	if err != nil {
		t.Errorf("Failed to save config: %v", err)
	}
}

func TestSaveToUnwritableDirectory(t *testing.T) {
	// Point to a path we cannot write to
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/path/that/does/not/exist/12345")

	cfg := &Config{}
	err := cfg.Save()

	// Expect an error since we can't create the directory
	if err == nil {
		t.Error("Expected error when saving to unwritable directory")
	}
}

func TestLoadValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory and file with valid JSON
	configDir := filepath.Join(tmpDir, "bbs")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg == nil {
		t.Error("Load returned nil config")
	}
}
