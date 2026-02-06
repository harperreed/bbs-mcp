// ABOUTME: Whoami command
// ABOUTME: Shows current identity and backend configuration

package main

import (
	"fmt"
	"path/filepath"

	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/identity"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current identity",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	id := identity.GetIdentity(identityFlag, "cli")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Printf("Identity: %s\n", id)
	fmt.Printf("Backend:  %s\n", cfg.GetBackend())
	fmt.Printf("Data dir: %s\n", cfg.GetDataDir())
	if cfg.GetBackend() == "sqlite" {
		fmt.Printf("Database: %s\n", filepath.Join(cfg.GetDataDir(), "bbs.db"))
	}
	return nil
}
