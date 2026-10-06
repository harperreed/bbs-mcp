// ABOUTME: Tests root command storage lifecycle classification and initialization
// ABOUTME: Verifies stateless commands do not create BBS config or data

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootExecutionClosesGlobalStoreWhenCommandFails(t *testing.T) {
	oldStore := globalStore
	oldIn := rootCmd.InOrStdin()
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	oldArgs := os.Args
	t.Cleanup(func() {
		if globalStore != nil {
			_ = globalStore.Close()
		}
		globalStore = oldStore
		rootCmd.SetIn(oldIn)
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		os.Args = oldArgs
	})

	globalStore = nil
	os.Args = []string{"bbs", "topic", "show", "missing-topic"}
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := executeRoot()
	if err == nil {
		t.Fatal("expected missing topic command to fail")
	}
	if globalStore != nil {
		t.Error("global store remains initialized after command failure")
	}
}

func TestPersistentPreRunInitializesStorageOnlyForGlobalStoreCommands(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	helpCmd, _, err := rootCmd.Find([]string{"help"})
	if err != nil {
		t.Fatalf("find help command: %v", err)
	}
	completionCmd, _, err := rootCmd.Find([]string{"completion", "bash"})
	if err != nil {
		t.Fatalf("find completion command: %v", err)
	}

	tests := []struct {
		name      string
		command   *cobra.Command
		wantStore bool
	}{
		{name: "root TUI", command: rootCmd},
		{name: "MCP", command: mcpCmd},
		{name: "export namespace", command: exportCmd},
		{name: "export markdown", command: exportMarkdownCmd},
		{name: "export YAML", command: exportYAMLCmd},
		{name: "export JSON", command: exportJSONCmd},
		{name: "import namespace", command: importCmd},
		{name: "import YAML", command: importYAMLCmd},
		{name: "migrate", command: migrateCmd},
		{name: "whoami", command: whoamiCmd},
		{name: "completion", command: completionCmd},
		{name: "version", command: versionCmd},
		{name: "help", command: helpCmd},
		{name: "install skill", command: installSkillCmd},
		{name: "topic namespace", command: topicCmd},
		{name: "topic list", command: topicListCmd, wantStore: true},
		{name: "topic new", command: topicNewCmd, wantStore: true},
		{name: "topic archive", command: topicArchiveCmd, wantStore: true},
		{name: "topic show", command: topicShowCmd, wantStore: true},
		{name: "thread namespace", command: threadCmd},
		{name: "thread list", command: threadListCmd, wantStore: true},
		{name: "thread new", command: threadNewCmd, wantStore: true},
		{name: "thread show", command: threadShowCmd, wantStore: true},
		{name: "thread sticky", command: threadStickyCmd, wantStore: true},
		{name: "post", command: postCmd, wantStore: true},
		{name: "edit", command: editCmd, wantStore: true},
	}

	oldStore := globalStore
	globalStore = nil
	t.Cleanup(func() {
		if globalStore != nil {
			_ = globalStore.Close()
		}
		globalStore = oldStore
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if globalStore != nil {
				_ = globalStore.Close()
				globalStore = nil
			}

			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("XDG_DATA_HOME", t.TempDir())

			if err := rootCmd.PersistentPreRunE(tt.command, nil); err != nil {
				t.Fatalf("persistent pre-run: %v", err)
			}
			if gotStore := globalStore != nil; gotStore != tt.wantStore {
				t.Errorf("global store initialized = %t, want %t", gotStore, tt.wantStore)
			}
		})
	}
}

func TestRunnableCommandsDeclarePositionalArgs(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		generatedHelp := command.Parent() == rootCmd && command.Use == "help [command]"
		if command.Runnable() && command.Args == nil && !generatedHelp {
			t.Errorf("runnable command %q has no positional argument validator", command.CommandPath())
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(rootCmd)
}

func TestStatelessCommandsDoNotCreateBBSState(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "bbs")
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	t.Run("completion", func(t *testing.T) {
		homeDir := t.TempDir()
		configDir := t.TempDir()
		dataDir := t.TempDir()
		command := exec.Command(binaryPath, "completion", "bash")
		command.Env = isolatedCLIEnv(homeDir, configDir, dataDir)

		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("run completion: %v\n%s", err, output)
		}
		if !strings.Contains(string(output), "__start_bbs") {
			t.Errorf("completion output does not contain generated bash entrypoint")
		}
		assertNoBBSState(t, configDir, dataDir)
	})

	t.Run("install skill", func(t *testing.T) {
		homeDir := t.TempDir()
		configDir := t.TempDir()
		dataDir := t.TempDir()
		command := exec.Command(binaryPath, "install-skill", "--yes")
		command.Env = isolatedCLIEnv(homeDir, configDir, dataDir)

		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("run install-skill: %v\n%s", err, output)
		}
		skillPath := filepath.Join(homeDir, ".claude", "skills", "bbs", "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			t.Fatalf("installed skill: %v", err)
		}
		assertNoBBSState(t, configDir, dataDir)
	})
}

func isolatedCLIEnv(homeDir, configDir, dataDir string) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOME=") ||
			strings.HasPrefix(entry, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(entry, "XDG_DATA_HOME=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+configDir,
		"XDG_DATA_HOME="+dataDir,
	)
}

func assertNoBBSState(t *testing.T, configDir, dataDir string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(configDir, "bbs"),
		filepath.Join(dataDir, "bbs"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("BBS state path %s exists or could not be checked: %v", path, err)
		}
	}
}
