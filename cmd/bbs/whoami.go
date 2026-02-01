// ABOUTME: Whoami command
// ABOUTME: Shows current identity

package main

import (
	"fmt"

	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/storage"
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
	fmt.Printf("Database: %s\n", storage.DefaultDBPath())
	return nil
}
