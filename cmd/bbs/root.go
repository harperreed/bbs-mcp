// ABOUTME: Root Cobra command and global flags
// ABOUTME: Sets up CLI structure and database connection

package main

import (
	"database/sql"
	"fmt"

	"github.com/harper/bbs/internal/db"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/tui"
	"github.com/spf13/cobra"
)

var (
	dbPath       string
	dbConn       *sql.DB
	identityFlag string
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
Topics → Threads → Messages`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Launch TUI if no subcommand
		return tui.Run(dbConn, identity.GetIdentity(identityFlag, "tui"))
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB init for help commands
		if cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		// Use default path if not specified
		path := dbPath
		if path == "" {
			path = db.GetDefaultDBPath()
		}

		var err error
		dbConn, err = db.InitDB(path)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if dbConn != nil {
			return dbConn.Close()
		}
		return nil
	},
}

func init() {
	defaultPath := db.GetDefaultDBPath()
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultPath, "database file path")
	rootCmd.PersistentFlags().StringVar(&identityFlag, "as", "", "identity override (username)")
}
