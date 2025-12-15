// ABOUTME: Root Cobra command and global flags
// ABOUTME: Sets up CLI structure and database connection

package main

import (
	"database/sql"

	"github.com/spf13/cobra"
)

var (
	dbPath   string
	dbConn   *sql.DB
	identity string
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Database init will be added in Task 4
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
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database file path")
	rootCmd.PersistentFlags().StringVar(&identity, "as", "", "identity override (username)")
}
