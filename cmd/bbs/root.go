// ABOUTME: Root Cobra command and global flags
// ABOUTME: Sets up CLI structure and config-driven storage connection

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/storage"
	"github.com/harper/bbs/internal/tui"
)

var identityFlag string

// globalStore holds the database connection for CLI commands
var globalStore storage.Storage

// commandRequiresGlobalStore reports whether cmd reads globalStore: post, edit, and every command
// beneath the topic and thread namespaces. The bare namespaces only print help.
func commandRequiresGlobalStore(cmd *cobra.Command) bool {
	if cmd == postCmd || cmd == editCmd {
		return true
	}
	for ancestor := cmd.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		if ancestor == topicCmd || ancestor == threadCmd {
			return true
		}
	}
	return false
}

func closeGlobalStore() {
	if globalStore == nil {
		return
	}
	_ = globalStore.Close()
	globalStore = nil
}

func runNamespaceHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func setNoArgsDefaults(commands ...*cobra.Command) {
	for _, command := range commands {
		if command.Runnable() && command.Args == nil {
			command.Args = cobra.NoArgs
		}
	}
}

var rootCmd = &cobra.Command{
	Use:           "bbs",
	Short:         "A lightweight message board for humans and agents",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
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

Data is stored locally.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Launch TUI if no subcommand
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		store, err := cfg.OpenStorage()
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer store.Close()
		return tui.Run(store, identity.GetIdentity(identityFlag, "tui"))
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !commandRequiresGlobalStore(cmd) {
			return nil
		}

		// Load config and initialize the shared store for storage-backed commands.
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		store, err := cfg.OpenStorage()
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		globalStore = store

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		closeGlobalStore()
		return nil
	},
}

func init() {
	setNoArgsDefaults(topicListCmd, versionCmd, whoamiCmd, mcpCmd, migrateCmd, installSkillCmd)
	rootCmd.PersistentFlags().StringVar(&identityFlag, "as", "", "identity override (username)")
}
