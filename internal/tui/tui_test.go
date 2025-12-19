// ABOUTME: Tests for TUI components
// ABOUTME: Verifies model initialization and basic state

package tui

import (
	"testing"
)

func TestNewModel(t *testing.T) {
	// Test that model can be created with nil client
	// Full integration tests require Charm connectivity
	model := NewModel(nil, "test@tui")

	// Model is created successfully
	// Just verify it doesn't panic and returns a usable model
	if model.identity != "test@tui" {
		t.Errorf("Expected identity test@tui, got %s", model.identity)
	}
}

func TestModelInit(t *testing.T) {
	// With nil client, Init will panic when trying to load topics
	// This is expected behavior - in production, client is always set
	// Skip this test as it requires Charm connectivity
	t.Skip("Requires Charm client connectivity")
}
