// ABOUTME: Whoami command
// ABOUTME: Shows current identity

package main

import (
	"fmt"

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
	fmt.Printf("Identity: %s\n", id)

	cfg, _ := config.Load()
	if cfg != nil && cfg.DeviceID != "" {
		fmt.Printf("Device: %s\n", cfg.DeviceID[:8])
	}
	if cfg != nil && cfg.IsConfigured() {
		fmt.Printf("Sync: enabled (server: %s)\n", cfg.Server)
	} else {
		fmt.Println("Sync: not configured")
	}

	return nil
}
