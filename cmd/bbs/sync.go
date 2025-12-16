// ABOUTME: Sync CLI commands for vault synchronization
// ABOUTME: Provides init, login, status, now, logout, pending, and wipe commands

package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/harperreed/sweet/vault"

	"github.com/fatih/color"
	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/sync"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage vault synchronization",
	Long: `Sync your BBS data securely to the cloud using E2E encryption.

Commands:
  init    - Initialize sync configuration
  login   - Login to sync server
  status  - Show sync status
  now     - Manually trigger sync
  logout  - Clear authentication
  pending - Show changes waiting to sync
  wipe    - Clear all sync data and start fresh`,
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
	Long: `Authenticate with the sync server using email, password, and recovery phrase.

The recovery phrase is your BIP39 mnemonic that was given to you when you
registered. Only the derived key is stored locally, not the mnemonic.`,
	RunE: runSyncLogin,
}

var syncLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from sync server",
	RunE:  runSyncLogout,
}

var syncPendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "Show changes waiting to sync",
	RunE:  runSyncPending,
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe all sync data and start fresh",
	Long: `Clear all server-side sync data and local vault database.

After wipe, run 'bbs sync now' to re-push local data.`,
	RunE: runSyncWipe,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncLoginCmd.Flags().String("server", "", "sync server URL (default: https://api.storeusa.org)")
	syncNowCmd.Flags().BoolP("verbose", "v", false, "show detailed sync information")
	syncCmd.AddCommand(syncInitCmd, syncStatusCmd, syncNowCmd, syncLoginCmd, syncLogoutCmd, syncPendingCmd, syncWipeCmd)
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

	color.Green("✓ Sync initialized")
	fmt.Printf("  Device ID: %s\n", cfg.DeviceID[:8])
	fmt.Printf("  Vault DB: %s\n", cfg.VaultDB)
	fmt.Println("\nNext: Run 'bbs sync login' to authenticate")
	return nil
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("Sync Status")
	fmt.Println("───────────")

	fmt.Printf("Config:    %s\n", config.GetConfigPath())
	fmt.Printf("Server:    %s\n", valueOrNone(cfg.Server))
	fmt.Printf("User ID:   %s\n", valueOrNone(cfg.UserID))
	fmt.Printf("Device ID: %s\n", valueOrNone(cfg.DeviceID))
	fmt.Printf("Vault DB:  %s\n", valueOrNone(cfg.VaultDB))
	fmt.Printf("AutoSync:  %v\n", cfg.AutoSync)

	if cfg.DerivedKey != "" {
		fmt.Println("Keys:      " + color.GreenString("✓ configured"))
	} else {
		fmt.Println("Keys:      " + color.YellowString("(not set)"))
	}

	printTokenStatus(cfg)

	// Show sync state if configured
	if cfg.IsConfigured() {
		syncer, err := sync.NewSyncer(dbConn)
		if err == nil {
			defer syncer.Close()
			ctx := cmd.Context()

			pending, err := syncer.GetPendingCount(ctx)
			if err == nil {
				fmt.Print("\nPending:   ")
				if pending == 0 {
					color.Green("0 changes (up to date)")
				} else {
					color.Yellow("%d changes waiting to push", pending)
				}
				fmt.Println()
			}

			lastSeq, err := syncer.LastSyncedSeq(ctx)
			if err == nil && lastSeq != "0" {
				fmt.Printf("Last sync: seq %s\n", lastSeq)
			}
		}
	}

	return nil
}

func runSyncNow(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	syncer, err := sync.NewSyncer(dbConn)
	if err != nil {
		return err
	}
	defer syncer.Close()

	if !syncer.IsEnabled() {
		return fmt.Errorf("sync not configured - run 'bbs sync login' first")
	}

	ctx := cmd.Context()

	var events *vault.SyncEvents
	if verbose {
		events = &vault.SyncEvents{
			OnStart: func() {
				fmt.Println("Syncing...")
			},
			OnPush: func(pushed, remaining int) {
				fmt.Printf("  ↑ pushed %d changes (%d remaining)\n", pushed, remaining)
			},
			OnPull: func(pulled int) {
				if pulled > 0 {
					fmt.Printf("  ↓ pulled %d changes\n", pulled)
				}
			},
			OnComplete: func(pushed, pulled int) {
				fmt.Printf("  Total: %d pushed, %d pulled\n", pushed, pulled)
			},
		}
	} else {
		fmt.Println("Syncing...")
	}

	if err := syncer.SyncWithEvents(ctx, events); err != nil {
		return err
	}

	color.Green("✓ Sync complete")
	return nil
}

func runSyncLogin(cmd *cobra.Command, args []string) error {
	server, _ := cmd.Flags().GetString("server")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	serverURL := server
	if serverURL == "" {
		serverURL = cfg.Server
	}
	if serverURL == "" {
		serverURL = "https://api.storeusa.org"
	}

	// Ensure device ID exists BEFORE login (v0.3.0 requirement)
	if cfg.DeviceID == "" {
		cfg.DeviceID = ulid.Make().String()
	}

	reader := bufio.NewReader(os.Stdin)

	// Get email
	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email required")
	}

	// Get password
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(syscall.Stdin)
	fmt.Println()
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	password := string(passwordBytes)

	// Get mnemonic
	fmt.Print("\nEnter your recovery phrase:\n> ")
	mnemonic, _ := reader.ReadString('\n')
	mnemonic = strings.TrimSpace(mnemonic)

	// Validate mnemonic
	if _, err := vault.ParseMnemonic(mnemonic); err != nil {
		return fmt.Errorf("invalid recovery phrase: %w", err)
	}

	// Login to server with device registration (v0.3.0 API)
	fmt.Printf("\nLogging in to %s...\n", serverURL)
	fmt.Printf("Registering device: %s\n", cfg.DeviceID[:8])
	client := vault.NewPBAuthClient(serverURL)
	result, err := client.Login(context.Background(), email, password, cfg.DeviceID)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Derive key from mnemonic (we only store the derived key, not the mnemonic)
	seed, err := vault.ParseSeedPhrase(mnemonic)
	if err != nil {
		return fmt.Errorf("parse mnemonic: %w", err)
	}
	derivedKeyHex := hex.EncodeToString(seed.Raw)

	// Save config
	cfg.Server = serverURL
	cfg.UserID = result.UserID
	cfg.Token = result.Token.Token
	cfg.RefreshToken = result.RefreshToken
	cfg.TokenExpires = result.Token.Expires.Format(time.RFC3339)
	cfg.DerivedKey = derivedKeyHex
	if cfg.VaultDB == "" {
		cfg.VaultDB = config.GetVaultDBPath()
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	color.Green("\n✓ Logged in to BBS sync")
	fmt.Printf("Device ID: %s\n", cfg.DeviceID[:8])
	fmt.Printf("Token expires: %s\n", result.Token.Expires.Format(time.RFC3339))

	return nil
}

func runSyncLogout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Token == "" {
		fmt.Println("Not logged in")
		return nil
	}

	cfg.Token = ""
	cfg.RefreshToken = ""
	cfg.TokenExpires = ""
	// Keep DerivedKey for re-login

	if err := cfg.Save(); err != nil {
		return err
	}

	color.Green("✓ Logged out")
	return nil
}

func runSyncPending(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if !cfg.IsConfigured() {
		fmt.Println("Sync not configured. Run 'bbs sync login' first.")
		return nil
	}

	syncer, err := sync.NewSyncer(dbConn)
	if err != nil {
		return fmt.Errorf("create syncer: %w", err)
	}
	defer syncer.Close()

	items, err := syncer.PendingChanges(cmd.Context())
	if err != nil {
		return fmt.Errorf("get pending: %w", err)
	}

	if len(items) == 0 {
		color.Green("✓ No pending changes - everything is synced!")
		return nil
	}

	fmt.Printf("Pending changes (%d):\n\n", len(items))
	for _, item := range items {
		fmt.Printf("  %s  %-10s  %s\n",
			color.New(color.Faint).Sprint(item.ChangeID[:8]),
			item.Entity,
			item.TS.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("\nRun 'bbs sync now' to push these changes.\n")

	return nil
}

func runSyncWipe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if !cfg.IsConfigured() {
		return fmt.Errorf("sync not configured - run 'bbs sync login' first")
	}

	// Confirm with user
	fmt.Println("This will DELETE local vault database.")
	fmt.Println("Your local BBS data will NOT be affected.")
	fmt.Print("\nType 'wipe' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(confirmation)

	if confirmation != "wipe" {
		fmt.Println("Aborted.")
		return nil
	}

	// Remove local vault.db
	fmt.Println("\nRemoving local vault database...")
	if err := os.Remove(cfg.VaultDB); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove vault.db: %w", err)
	}
	color.Green("✓ Local vault.db removed")

	fmt.Println("\nSync data cleared. Run 'bbs sync now' to re-push local data.")
	return nil
}

// valueOrNone returns "(not set)" if the string is empty
func valueOrNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

// printTokenStatus displays token validity information
func printTokenStatus(cfg *config.Config) {
	if cfg.Token == "" {
		fmt.Println("\nStatus:    " + color.YellowString("Not logged in"))
		return
	}

	fmt.Println()
	if cfg.TokenExpires == "" {
		fmt.Println("Token:     valid (no expiry info)")
		return
	}

	expires, err := time.Parse(time.RFC3339, cfg.TokenExpires)
	if err != nil {
		fmt.Printf("Token:     valid (invalid expiry: %v)\n", err)
		return
	}

	now := time.Now()
	if expires.Before(now) {
		fmt.Print("Token:     ")
		color.Red("EXPIRED (%s ago)", now.Sub(expires).Round(time.Second))
		fmt.Println()
		if cfg.RefreshToken != "" {
			fmt.Println("           (has refresh token - run 'bbs sync now' to refresh)")
		}
	} else {
		fmt.Print("Token:     ")
		color.Green("valid")
		fmt.Printf(" (expires in %s)\n", formatDuration(expires.Sub(now)))
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
