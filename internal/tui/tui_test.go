// ABOUTME: Tests for TUI components
// ABOUTME: Verifies model initialization and basic state

package tui

import (
	"database/sql"
	"testing"

	"github.com/harper/bbs/internal/db"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test database: %v", err)
	}
	return conn
}

func TestNewModel(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	model := NewModel(conn, "test@tui")
	// Model is created successfully - internal state is private
	// Just verify it doesn't panic and returns a usable model
	if model.db == nil {
		t.Error("Model db should not be nil")
	}
}

func TestModelInit(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	model := NewModel(conn, "test@tui")
	cmd := model.Init()
	if cmd == nil {
		t.Error("Init should return a command to load topics")
	}
}
