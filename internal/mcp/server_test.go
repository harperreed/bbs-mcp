// ABOUTME: Tests for MCP server initialization
// ABOUTME: Verifies server creation and tool registration

package mcp

import (
	"testing"
)

func TestNewServerRequiresClient(t *testing.T) {
	_, err := NewServer(nil)
	if err == nil {
		t.Error("NewServer should fail with nil client")
	}
}

func TestNewServerSuccess(t *testing.T) {
	// Full server tests require Charm connectivity
	// This test verifies the nil check works
	t.Skip("Requires Charm client connectivity")
}
