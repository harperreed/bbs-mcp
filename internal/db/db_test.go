// ABOUTME: Tests for database initialization
// ABOUTME: Verifies connection and migration execution

package db

import (
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	tables := []string{"topics", "threads", "messages", "attachments"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestGetDefaultDBPath(t *testing.T) {
	path := GetDefaultDBPath()
	if path == "" {
		t.Error("expected non-empty path")
	}
	if filepath.Base(path) != "bbs.db" {
		t.Errorf("expected bbs.db, got %s", filepath.Base(path))
	}
}
