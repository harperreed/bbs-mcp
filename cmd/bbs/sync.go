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
	"github.com/charmbracelet/charm/kv"
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
  repair  - Repair database integrity issues
  reset   - Clear local data and re-sync from cloud
  wipe    - Delete all local and cloud data`,
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

var syncRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair database integrity issues",
	Long: `Run database repair operations to fix corruption or sync issues.

Performs WAL checkpoint, removes shared memory, runs integrity check, and vacuums.
Use --force to attempt repair even if initial checks fail.`,
	RunE: runSyncRepair,
}

var syncResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clear local data and re-sync from cloud",
	Long: `Clear local Charm KV database and re-sync from cloud.

This is useful if your local data is corrupted or out of sync.
Your cloud data will NOT be affected.`,
	RunE: runSyncReset,
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Delete all local and cloud data",
	Long: `Delete all BBS data from both local storage and Charm Cloud.

WARNING: This permanently deletes your cloud backups!
This cannot be undone.`,
	RunE: runSyncWipe,
}

var forceRepair bool

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncStatusCmd, syncLinkCmd, syncRepairCmd, syncResetCmd, syncWipeCmd)
	syncRepairCmd.Flags().BoolVar(&forceRepair, "force", false, "Force repair even if initial checks fail")
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

func runSyncRepair(cmd *cobra.Command, args []string) error {
	_, err := charm.Global()
	if err != nil {
		return fmt.Errorf("charm not initialized: %w", err)
	}

	fmt.Println("Running database repair...")
	fmt.Println()

	result, err := kv.Repair(charm.DBName, forceRepair)
	if err != nil {
		color.Red("✗ Repair failed: %v", err)
		return err
	}

	// Show repair results
	fmt.Println("Repair Results:")
	fmt.Println("───────────────")

	if result.WalCheckpointed {
		color.Green("✓ WAL checkpointed")
	} else {
		color.Yellow("○ WAL checkpoint not needed")
	}

	if result.ShmRemoved {
		color.Green("✓ Shared memory removed")
	} else {
		color.Yellow("○ Shared memory cleanup not needed")
	}

	if result.IntegrityOK {
		color.Green("✓ Integrity check passed")
	} else {
		color.Red("✗ Integrity check failed")
	}

	if result.Vacuumed {
		color.Green("✓ Database vacuumed")
	} else {
		color.Yellow("○ Vacuum not needed")
	}

	fmt.Println()
	if result.IntegrityOK {
		color.Green("✓ Database repaired successfully")
	} else {
		color.Yellow("⚠ Repair completed but integrity issues remain")
		fmt.Println("\nConsider using 'bbs sync reset' to re-sync from cloud.")
	}

	return nil
}

func runSyncReset(cmd *cobra.Command, args []string) error {
	_, err := charm.Global()
	if err != nil {
		return fmt.Errorf("charm not initialized: %w", err)
	}

	// Confirm with user
	fmt.Println("This will DELETE your local BBS data and re-sync from the cloud.")
	fmt.Println("Your cloud data will NOT be affected.")
	fmt.Print("\nContinue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.ToLower(strings.TrimSpace(confirmation))

	if confirmation != "y" && confirmation != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	fmt.Println("\nResetting local data...")
	if err := kv.Reset(charm.DBName); err != nil {
		return fmt.Errorf("reset failed: %w", err)
	}

	color.Green("✓ Local data reset and re-synced from cloud")
	return nil
}

func runSyncWipe(cmd *cobra.Command, args []string) error {
	_, err := charm.Global()
	if err != nil {
		return fmt.Errorf("charm not initialized: %w", err)
	}

	// Confirm with user
	fmt.Println("WARNING: This will PERMANENTLY DELETE all BBS data!")
	fmt.Println()
	fmt.Println("This includes:")
	fmt.Println("  • All local database files")
	fmt.Println("  • All cloud backups on Charm servers")
	fmt.Println()
	color.Red("THIS CANNOT BE UNDONE!")
	fmt.Println()
	fmt.Print("Type 'wipe' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(confirmation)

	if confirmation != "wipe" {
		fmt.Println("Aborted.")
		return nil
	}

	fmt.Println("\nWiping all data...")
	result, err := kv.Wipe(charm.DBName)
	if err != nil {
		return fmt.Errorf("wipe failed: %w", err)
	}

	// Show wipe results
	fmt.Println()
	fmt.Println("Wipe Results:")
	fmt.Println("─────────────")

	if result.CloudBackupsDeleted > 0 {
		color.Green("✓ Cloud backups deleted: %d", result.CloudBackupsDeleted)
	} else {
		color.Yellow("○ No cloud backups to delete")
	}

	if result.LocalFilesDeleted > 0 {
		color.Green("✓ Local files deleted: %d", result.LocalFilesDeleted)
	} else {
		color.Yellow("○ No local files to delete")
	}

	fmt.Println()
	color.Green("✓ All BBS data permanently deleted")
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
