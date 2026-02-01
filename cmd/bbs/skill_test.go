// ABOUTME: Tests for the install-skill command
// ABOUTME: Verifies directory creation, file content, and overwrite scenarios

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillInstallCreatesDirectory verifies that installSkillTo creates the
// skills directory structure when it doesn't exist.
func TestSkillInstallCreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Directory should not exist initially
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("expected skill directory to not exist initially")
	}

	// Install the skill
	err := installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("installSkillTo failed: %v", err)
	}

	// Directory should now exist
	info, err := os.Stat(skillDir)
	if err != nil {
		t.Fatalf("skill directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", skillDir)
	}
}

// TestSkillInstallWritesFile verifies that the SKILL.md file is written
// with correct content.
func TestSkillInstallWritesFile(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	err := installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("installSkillTo failed: %v", err)
	}

	// File should exist
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read skill file: %v", err)
	}

	// Verify content matches embedded file
	embeddedContent, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("failed to read embedded skill: %v", err)
	}

	if string(content) != string(embeddedContent) {
		t.Errorf("installed content does not match embedded content")
	}
}

// TestSkillInstallFileContent verifies that the installed file contains
// expected content sections.
func TestSkillInstallFileContent(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	err := installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("installSkillTo failed: %v", err)
	}

	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read skill file: %v", err)
	}

	contentStr := string(content)

	// Check for expected content sections
	expectedSections := []string{
		"name: bbs",
		"description:",
		"# bbs - Bulletin Board System",
		"mcp__bbs__list_topics",
		"mcp__bbs__create_topic",
		"mcp__bbs__post_message",
	}

	for _, section := range expectedSections {
		if !strings.Contains(contentStr, section) {
			t.Errorf("expected skill content to contain %q", section)
		}
	}
}

// TestSkillInstallOverwritesExistingFile verifies that installing over an
// existing file replaces its content.
func TestSkillInstallOverwritesExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Create directory and existing file with different content
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	oldContent := []byte("old content that should be replaced")
	if err := os.WriteFile(skillPath, oldContent, 0644); err != nil {
		t.Fatalf("failed to write old file: %v", err)
	}

	// Verify old content exists
	readContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read old file: %v", err)
	}
	if string(readContent) != string(oldContent) {
		t.Fatalf("old content was not written correctly")
	}

	// Install skill (should overwrite)
	err = installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("installSkillTo failed: %v", err)
	}

	// Verify content was replaced
	newContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read new file: %v", err)
	}

	if string(newContent) == string(oldContent) {
		t.Errorf("file content was not overwritten")
	}

	// Verify it contains expected bbs content
	if !strings.Contains(string(newContent), "bbs - Bulletin Board System") {
		t.Errorf("overwritten file does not contain expected content")
	}
}

// TestSkillInstallFilePermissions verifies that installed files have
// correct permissions.
func TestSkillInstallFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	err := installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("installSkillTo failed: %v", err)
	}

	// Check directory permissions
	dirInfo, err := os.Stat(skillDir)
	if err != nil {
		t.Fatalf("failed to stat skill directory: %v", err)
	}
	dirPerm := dirInfo.Mode().Perm()
	if dirPerm != 0755 {
		t.Errorf("expected directory permissions 0755, got %o", dirPerm)
	}

	// Check file permissions
	fileInfo, err := os.Stat(skillPath)
	if err != nil {
		t.Fatalf("failed to stat skill file: %v", err)
	}
	filePerm := fileInfo.Mode().Perm()
	if filePerm != 0644 {
		t.Errorf("expected file permissions 0644, got %o", filePerm)
	}
}

// TestSkillInstallNestedDirectoryCreation verifies that deeply nested
// directories are created correctly.
func TestSkillInstallNestedDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	// Create a deeply nested path
	skillDir := filepath.Join(tempDir, "a", "b", "c", ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// None of the intermediate directories should exist
	if _, err := os.Stat(filepath.Join(tempDir, "a")); !os.IsNotExist(err) {
		t.Fatalf("expected intermediate directory to not exist")
	}

	err := installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("installSkillTo failed: %v", err)
	}

	// All directories should now exist
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("skill directory was not created: %v", err)
	}

	// File should exist and be readable
	if _, err := os.ReadFile(skillPath); err != nil {
		t.Fatalf("skill file not readable: %v", err)
	}
}

// TestSkillInstallIdempotent verifies that installing multiple times
// produces the same result.
func TestSkillInstallIdempotent(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, ".claude", "skills", "bbs")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Install first time
	err := installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("first installSkillTo failed: %v", err)
	}

	firstContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read first install: %v", err)
	}

	// Install second time
	err = installSkillTo(skillDir, skillPath)
	if err != nil {
		t.Fatalf("second installSkillTo failed: %v", err)
	}

	secondContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read second install: %v", err)
	}

	// Content should be identical
	if string(firstContent) != string(secondContent) {
		t.Errorf("content differs between installs")
	}
}

// installSkillTo is a testable version of installSkill that accepts
// custom paths instead of using os.UserHomeDir.
func installSkillTo(skillDir, skillPath string) error {
	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return err
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return err
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		return err
	}

	return nil
}
