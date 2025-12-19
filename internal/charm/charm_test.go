// ABOUTME: Tests for Charm KV client
// ABOUTME: Requires Charm connectivity for full integration tests

package charm

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	// NewClient accepts nil KV for testing purposes
	client := NewClient(nil)
	if client == nil {
		t.Error("NewClient should return non-nil client")
	}
}

func TestKeyPrefixes(t *testing.T) {
	// Verify key prefixes are defined correctly
	prefixes := []string{TopicPrefix, ThreadPrefix, MessagePrefix, AttachmentPrefix}
	for _, p := range prefixes {
		if p == "" {
			t.Error("Key prefix should not be empty")
		}
	}
}
