// ABOUTME: Tests for MCP server initialization
// ABOUTME: Verifies server creation and tool registration

package mcp

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewServerRequiresDB(t *testing.T) {
	_, err := NewServer(nil)
	if err == nil {
		t.Error("NewServer should fail with nil database")
	}
}

func TestNewServerSuccess(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	server, err := NewServer(db)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if server == nil {
		t.Error("NewServer returned nil server")
	}
}
