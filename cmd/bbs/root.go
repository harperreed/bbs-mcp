// ABOUTME: Root Cobra command and global flags
// ABOUTME: Sets up CLI structure and Charm KV connection

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/charm"
	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/tui"
)

var (
	identityFlag string
	charmClient  *charm.Client
)

var rootCmd = &cobra.Command{
	Use:   "bbs",
	Short: "A lightweight message board for humans and agents",
	Long: `
██████╗ ██████╗ ███████╗
██╔══██╗██╔══██╗██╔════╝
██████╔╝██████╔╝███████╗
██╔══██╗██╔══██╗╚════██║
██████╔╝██████╔╝███████║
╚═════╝ ╚═════╝ ╚══════╝

   THUNDERBOARD 3000

A message board for humans and agents to communicate.
Topics → Threads → Messages

Data syncs automatically to the cloud via Charm.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Launch TUI if no subcommand
		client, err := charm.Global()
		if err != nil {
			return fmt.Errorf("failed to get charm client: %w", err)
		}
		return tui.Run(client, identity.GetIdentity(identityFlag, "tui"))
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip init for help commands
		if cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		// Load config and apply environment
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg.ApplyEnvironment()

		// Initialize Charm
		if err := charm.InitGlobal(); err != nil {
			return fmt.Errorf("failed to initialize charm: %w", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Charm client is global, no need to close per-command
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&identityFlag, "as", "", "identity override (username)")
}
