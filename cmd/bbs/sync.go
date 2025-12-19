// ABOUTME: Sync CLI commands for Charm cloud synchronization
// ABOUTME: Provides status, link, and wipe commands using SSH key auth

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	charmclient "github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/ui/link"
	"github.com/charmbracelet/charm/ui/linkgen"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/charm"
	"github.com/harper/bbs/internal/config"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage Charm cloud synchronization",
	Long: `Sync your BBS data to the cloud using Charm.

Authentication is automatic via SSH keys - no passwords needed.
Your data is encrypted end-to-end before leaving your device.

Commands:
  status  - Show sync status and user info
  link    - Link this device to your Charm account
  wipe    - Clear local data and re-sync from cloud`,
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE:  runSyncStatus,
}

var syncLinkCmd = &cobra.Command{
	Use:   "link [code]",
	Short: "Link this device to your Charm account",
	Long: `Link multiple machines to your Charm account.

Run without arguments to generate a link code.
Run with a code to link to an existing account.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSyncLink,
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe local data and re-sync from cloud",
	Long: `Clear local Charm KV database and re-sync from cloud.

This is useful if your local data is corrupted or out of sync.
Your cloud data will NOT be affected.`,
	RunE: runSyncWipe,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncStatusCmd, syncLinkCmd, syncWipeCmd)
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("Sync Status")
	fmt.Println("───────────")

	// Show config
	fmt.Printf("Config:     %s\n", config.GetConfigPath())
	fmt.Printf("Charm Host: %s\n", cfg.GetCharmHost())

	// Check if charm is initialized
	client, err := charm.Global()
	if err != nil {
		fmt.Print("\nStatus:     ")
		color.Yellow("Not initialized")
		fmt.Println()
		fmt.Println("\nRun any BBS command to initialize, or 'bbs sync link' to link devices.")
		return nil
	}

	// Get user info
	userID, err := client.ID()
	if err != nil {
		fmt.Print("\nStatus:     ")
		color.Yellow("Not linked")
		fmt.Println()
		fmt.Println("\nRun 'bbs sync link' to link this device.")
		return nil
	}

	// Show user info
	fmt.Printf("User ID:    %s\n", strings.TrimSpace(userID))
	fmt.Print("\nStatus:     ")
	color.Green("Connected")
	fmt.Println()

	// Get authorized keys
	cc := client.CharmClient()
	keys, err := cc.AuthorizedKeys()
	if err == nil && keys != "" {
		lines := strings.Split(strings.TrimSpace(keys), "\n")
		fmt.Printf("Devices:    %d linked\n", len(lines))
	}

	return nil
}

func runSyncLink(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Apply environment (set CHARM_HOST)
	cfg.ApplyEnvironment()

	// Get charm config
	charmCfg, err := getCharmConfig()
	if err != nil {
		return fmt.Errorf("get charm config: %w", err)
	}

	var p *tea.Program
	if len(args) == 0 {
		// Generate a link code
		fmt.Println("Generating link code...")
		fmt.Println("Share this code with another device to link it to your account.")
		fmt.Println()
		p = linkgen.NewProgram(charmCfg, "bbs")
	} else {
		// Join a link session
		fmt.Println("Linking to existing account...")
		fmt.Println()
		p = link.NewProgram(charmCfg, args[0])
	}

	if _, err := p.Run(); err != nil {
		return err
	}

	color.Green("\n✓ Device linked successfully")
	return nil
}

func runSyncWipe(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return fmt.Errorf("charm not initialized: %w", err)
	}

	// Confirm with user
	fmt.Println("This will DELETE your local BBS data and re-sync from the cloud.")
	fmt.Println("Your cloud data will NOT be affected.")
	fmt.Print("\nType 'wipe' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(confirmation)

	if confirmation != "wipe" {
		fmt.Println("Aborted.")
		return nil
	}

	fmt.Println("\nWiping local data...")
	if err := client.Reset(); err != nil {
		return fmt.Errorf("reset failed: %w", err)
	}

	color.Green("✓ Local data wiped and re-synced from cloud")
	return nil
}

// getCharmConfig returns the charm client configuration.
func getCharmConfig() (*charmclient.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Apply environment first
	cfg.ApplyEnvironment()

	// Load charm config from environment
	charmCfg, err := charmclient.ConfigFromEnv()
	if err != nil {
		return nil, err
	}

	// Override host if set in our config
	if cfg.CharmHost != "" {
		charmCfg.Host = cfg.CharmHost
	}

	return charmCfg, nil
}
