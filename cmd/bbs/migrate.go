// ABOUTME: Migration command placeholder
// ABOUTME: For future migrations if needed

package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/storage"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration utilities",
	Long: `Database migration utilities for BBS.

The Charm KV migration is no longer available as the Charm dependency
has been removed. If you need to migrate data from Charm KV, use an
older version of BBS to export your data first, then import it using
the 'bbs import yaml' command.`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Check if database exists
	dbPath := storage.DefaultDBPath()
	color.Yellow("Migration from Charm KV is no longer available.")
	fmt.Println()
	fmt.Println("To migrate from an older version of BBS:")
	fmt.Println("  1. Use the old version to run 'bbs export yaml backup.yaml'")
	fmt.Println("  2. Update to this version")
	fmt.Println("  3. Run 'bbs import yaml backup.yaml'")
	fmt.Println()
	fmt.Printf("Database path: %s\n", dbPath)
	return nil
}
