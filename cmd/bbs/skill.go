// ABOUTME: Install Claude Code skill for bbs
// ABOUTME: Embeds and installs the skill definition to ~/.claude/skills/

package main

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed skill/SKILL.md
var skillFS embed.FS

var skillSkipConfirm bool

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "Install Claude Code skill",
	Long: `Install the bbs skill for Claude Code.

This copies the skill definition to ~/.claude/skills/bbs/
so Claude Code can use bbs commands contextually.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installSkill()
	},
}

func init() {
	installSkillCmd.Flags().BoolVarP(&skillSkipConfirm, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(installSkillCmd)
}

func installSkill() error {
	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Show explanation
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│               BBS Skill for Claude Code                     │")
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("This will install the bbs skill, enabling Claude Code to:")
	fmt.Println()
	fmt.Println("  • Create and manage discussion topics")
	fmt.Println("  • Post and reply to threaded messages")
	fmt.Println("  • Collaborate with other agents and humans")
	fmt.Println("  • Use the /bbs slash command")
	fmt.Println()
	fmt.Println("Destination:")
	fmt.Printf("  %s\n", skillPath)
	fmt.Println()

	// Check if already installed
	if _, err := os.Stat(skillPath); err == nil {
		fmt.Println("Note: A skill file already exists and will be overwritten.")
		fmt.Println()
	}

	// Ask for confirmation unless --yes flag is set
	if !skillSkipConfirm {
		fmt.Print("Install the bbs skill? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Installation cancelled.")
			return nil
		}
		fmt.Println()
	}

	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return fmt.Errorf("failed to read embedded skill: %w", err)
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	fmt.Println("✓ Installed bbs skill successfully!")
	fmt.Println()
	fmt.Println("Claude Code will now recognize /bbs commands.")
	fmt.Println("Try asking Claude: \"Start a discussion about the new feature\" or \"Check the board\"")
	return nil
}
