// ABOUTME: Package test entry point that sandboxes HOME and XDG directories for CLI tests.
// ABOUTME: Keeps tests from reading or writing the developer's real bbs config and board data.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain points HOME, XDG_CONFIG_HOME, and XDG_DATA_HOME at a fresh sandbox, so config.Load
// can never read the developer's real config or save a first-run default over it. Tests that
// need specific settings still override these with t.Setenv.
func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "bbs-cmd-test-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create test sandbox: %v\n", err)
		os.Exit(1)
	}
	for _, env := range []struct{ name, dir string }{
		{"HOME", "home"},
		{"XDG_CONFIG_HOME", "config"},
		{"XDG_DATA_HOME", "data"},
	} {
		dir := filepath.Join(sandbox, env.dir)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "create test sandbox %s: %v\n", env.name, err)
			os.Exit(1)
		}
		if err := os.Setenv(env.name, dir); err != nil {
			fmt.Fprintf(os.Stderr, "set %s: %v\n", env.name, err)
			os.Exit(1)
		}
	}

	code := m.Run()
	_ = os.RemoveAll(sandbox)
	os.Exit(code)
}
