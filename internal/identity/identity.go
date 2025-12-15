// ABOUTME: Identity resolution for BBS users
// ABOUTME: Handles username@source format

package identity

import (
	"os"
	"strings"
)

// GetIdentity returns the identity string for a user.
// If override is provided, uses that as username.
// Otherwise uses $USER or $BBS_USER environment variable.
func GetIdentity(override, source string) string {
	username := override
	if username == "" {
		username = os.Getenv("BBS_USER")
	}
	if username == "" {
		username = os.Getenv("USER")
	}
	if username == "" {
		username = "anonymous"
	}
	return username + "@" + source
}

// ParseIdentity splits an identity string into username and source.
func ParseIdentity(id string) (username, source string) {
	parts := strings.SplitN(id, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return id, "unknown"
}
