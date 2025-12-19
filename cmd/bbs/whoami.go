// ABOUTME: Whoami command
// ABOUTME: Shows current identity

package main

import (
	"fmt"

	"github.com/harper/bbs/internal/charm"
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

	client, err := charm.Global()
	if err != nil {
		fmt.Println("Sync: not initialized")
		return nil
	}

	charmID, err := client.ID()
	if err != nil {
		fmt.Println("Sync: not linked")
	} else {
		fmt.Printf("Charm ID: %s\n", charmID[:8])
		fmt.Printf("Sync: enabled (host: %s)\n", charm.DefaultCharmHost)
	}

	return nil
}
