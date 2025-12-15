// ABOUTME: Tests for identity resolution
// ABOUTME: Verifies username@source format handling

package identity

import (
	"os"
	"testing"
)

func TestGetIdentity(t *testing.T) {
	tests := []struct {
		name     string
		override string
		source   string
		want     string
	}{
		{"with override", "mybot", "cli", "mybot@cli"},
		{"without override", "", "mcp", os.Getenv("USER") + "@mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIdentity(tt.override, tt.source)
			if got != tt.want {
				t.Errorf("GetIdentity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseIdentity(t *testing.T) {
	user, source := ParseIdentity("harper@cli")
	if user != "harper" {
		t.Errorf("expected user 'harper', got '%s'", user)
	}
	if source != "cli" {
		t.Errorf("expected source 'cli', got '%s'", source)
	}
}
