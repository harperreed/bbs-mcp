// ABOUTME: Tests for sync engine
// ABOUTME: Verifies change queuing and sync state

package sync

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewSyncer(t *testing.T) {
	// Use temp dir to avoid interfering with real config
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

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
	defer syncer.Close()
}

func TestSyncerDisabledByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()

	syncer, _ := NewSyncer(db)
	defer syncer.Close()

	if syncer.IsEnabled() {
		t.Error("Syncer should be disabled when not configured")
	}
}

func TestGetPendingCount(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()

	syncer, _ := NewSyncer(db)
	defer syncer.Close()

	count, err := syncer.GetPendingCount(context.Background())
	if err != nil {
		t.Fatalf("GetPendingCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 pending, got %d", count)
	}
}

func TestLastSyncedSeq(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()

	syncer, _ := NewSyncer(db)
	defer syncer.Close()

	seq, err := syncer.LastSyncedSeq(context.Background())
	if err != nil {
		t.Fatalf("LastSyncedSeq failed: %v", err)
	}
	if seq != "0" {
		t.Errorf("Expected seq '0', got '%s'", seq)
	}
}
