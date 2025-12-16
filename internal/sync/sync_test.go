// ABOUTME: Tests for sync engine
// ABOUTME: Verifies change queuing and sync state

package sync

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewSyncer(t *testing.T) {
	// Use temp dir to avoid interfering with real config
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	syncer, err := NewSyncer(db)
	if err != nil {
		t.Fatalf("NewSyncer failed: %v", err)
	}
	if syncer == nil {
		t.Error("NewSyncer returned nil")
	}
}

func TestSyncerDisabledByDefault(t *testing.T) {
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()

	syncer, _ := NewSyncer(db)
	if syncer.IsEnabled() {
		t.Error("Syncer should be disabled when not configured")
	}
}

func TestGetPendingCount(t *testing.T) {
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()

	syncer, _ := NewSyncer(db)
	count := syncer.GetPendingCount()
	if count != 0 {
		t.Errorf("Expected 0 pending, got %d", count)
	}
}
