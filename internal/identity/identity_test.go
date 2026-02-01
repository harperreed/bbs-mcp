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

func TestParseIdentityNoAt(t *testing.T) {
	user, source := ParseIdentity("justusername")
	if user != "justusername" {
		t.Errorf("expected user 'justusername', got '%s'", user)
	}
	if source != "unknown" {
		t.Errorf("expected source 'unknown', got '%s'", source)
	}
}

func TestGetIdentityWithBBSUser(t *testing.T) {
	t.Setenv("BBS_USER", "bbsuser")
	// Clear USER to ensure BBS_USER is used
	originalUser := os.Getenv("USER")
	t.Setenv("USER", "")
	defer t.Setenv("USER", originalUser)

	got := GetIdentity("", "cli")
	if got != "bbsuser@cli" {
		t.Errorf("GetIdentity() = %v, want bbsuser@cli", got)
	}
}

func TestGetIdentityAnonymous(t *testing.T) {
	// Clear both USER and BBS_USER
	t.Setenv("BBS_USER", "")
	t.Setenv("USER", "")

	got := GetIdentity("", "cli")
	if got != "anonymous@cli" {
		t.Errorf("GetIdentity() = %v, want anonymous@cli", got)
	}
}

func TestParseIdentityMultipleAts(t *testing.T) {
	// Test with multiple @ symbols - should only split on first
	user, source := ParseIdentity("user@domain@source")
	if user != "user" {
		t.Errorf("expected user 'user', got '%s'", user)
	}
	if source != "domain@source" {
		t.Errorf("expected source 'domain@source', got '%s'", source)
	}
}
