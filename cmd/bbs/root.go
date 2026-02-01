// ABOUTME: Root Cobra command and global flags
// ABOUTME: Sets up CLI structure and SQLite storage connection

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/storage"
	"github.com/harper/bbs/internal/tui"
)

var identityFlag string

// globalStore holds the database connection for CLI commands
var globalStore *storage.Store

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

Data is stored locally in SQLite.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Launch TUI if no subcommand
		store, err := storage.NewStore(storage.DefaultDBPath())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer store.Close()
		return tui.Run(store, identity.GetIdentity(identityFlag, "tui"))
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip init for help commands
		if cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		// Initialize global store for subcommands that need it
		// Note: Commands that use globalStore should handle nil check
		store, err := storage.NewStore(storage.DefaultDBPath())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		globalStore = store

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if globalStore != nil {
			globalStore.Close()
			globalStore = nil
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&identityFlag, "as", "", "identity override (username)")
}
