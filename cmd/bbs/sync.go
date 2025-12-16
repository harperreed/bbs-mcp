// ABOUTME: Sync CLI commands
// ABOUTME: Manages vault synchronization

package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/sync"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage vault synchronization",
	Long:  "Initialize, configure, and trigger vault synchronization.",
}

var syncInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize vault sync",
	RunE:  runSyncInit,
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE:  runSyncStatus,
}

var syncNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Sync now",
	RunE:  runSyncNow,
}

var syncLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to sync server",
	RunE:  runSyncLogin,
}

var syncLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from sync server",
	RunE:  runSyncLogout,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncInitCmd, syncStatusCmd, syncNowCmd, syncLoginCmd, syncLogoutCmd)
}

func runSyncInit(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.DeviceID != "" {
		fmt.Printf("Already initialized with device ID: %s\n", cfg.DeviceID[:8])
		return nil
	}

	cfg.DeviceID = ulid.Make().String()
	cfg.VaultDB = config.GetVaultDBPath()
	cfg.AutoSync = true

	if err := cfg.Save(); err != nil {
		return err
	}

	color.Green("Initialized sync")
	fmt.Printf("Device ID: %s\n", cfg.DeviceID[:8])
	fmt.Printf("Vault DB: %s\n", cfg.VaultDB)
	fmt.Println("\nRun 'bbs sync login' to authenticate.")
	return nil
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("Sync Status")
	fmt.Println("───────────")

	if cfg.DeviceID == "" {
		color.Yellow("Not initialized. Run 'bbs sync init' first.")
		return nil
	}

	fmt.Printf("Device ID: %s\n", cfg.DeviceID[:8])
	fmt.Printf("AutoSync: %v\n", cfg.AutoSync)

	if cfg.Server == "" {
		color.Yellow("Not logged in. Run 'bbs sync login' to authenticate.")
		return nil
	}

	fmt.Printf("Server: %s\n", cfg.Server)
	fmt.Printf("User: %s\n", cfg.UserID)

	syncer, err := sync.NewSyncer(dbConn)
	if err != nil {
		return err
	}
	defer syncer.Close()

	pending, err := syncer.GetPendingCount(cmd.Context())
	if err != nil {
		return err
	}
	if pending > 0 {
		color.Yellow("Pending changes: %d", pending)
	} else {
		color.Green("All synced")
	}

	return nil
}

func runSyncNow(cmd *cobra.Command, args []string) error {
	syncer, err := sync.NewSyncer(dbConn)
	if err != nil {
		return err
	}
	defer syncer.Close()

	if !syncer.IsEnabled() {
		return fmt.Errorf("sync not configured - run 'bbs sync login' first")
	}

	fmt.Println("Syncing...")
	if err := syncer.Sync(cmd.Context()); err != nil {
		return err
	}

	color.Green("Sync complete")
	return nil
}

func runSyncLogin(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.DeviceID == "" {
		return fmt.Errorf("not initialized - run 'bbs sync init' first")
	}

	// TODO: Implement interactive login flow
	// For now, just show what would happen
	fmt.Println("Login flow would:")
	fmt.Println("1. Prompt for server URL")
	fmt.Println("2. Prompt for email/password")
	fmt.Println("3. Prompt for BIP39 recovery phrase")
	fmt.Println("4. Derive encryption keys")
	fmt.Println("5. Store tokens and derived key")
	color.Yellow("\nFull vault integration pending.")

	return nil
}

func runSyncLogout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cfg.Token = ""
	cfg.RefreshToken = ""
	cfg.TokenExpires = ""
	// Keep DerivedKey for re-login

	if err := cfg.Save(); err != nil {
		return err
	}

	color.Green("Logged out")
	return nil
}
