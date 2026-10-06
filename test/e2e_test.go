// ABOUTME: End-to-end tests for the compiled BBS command-line interface.
// ABOUTME: Verifies persistent workflows in isolated real storage across processes.

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIWorkflowPersistsAcrossProcesses(t *testing.T) {
	testRoot := t.TempDir()
	binaryPath := buildCLI(t, testRoot)

	homeDir := filepath.Join(testRoot, "home")
	configDir := filepath.Join(testRoot, "config")
	dataDir := filepath.Join(testRoot, "data")
	if err := os.MkdirAll(homeDir, 0o750); err != nil {
		t.Fatalf("create isolated home: %v", err)
	}
	env := isolatedEnvironment(homeDir, configDir, dataDir)

	const (
		topicName   = "e2e-persistence-topic"
		description = "isolated end-to-end storage"
		threadName  = "persistence across CLI processes"
		message     = "this post survives a separate read process"
	)

	runCLI(t, binaryPath, testRoot, env, "--as", "e2e", "topic", "new", topicName, description)
	threadOutput := runCLI(t, binaryPath, testRoot, env, "--as", "e2e", "thread", "new", topicName, threadName)
	threadID := outputValue(t, threadOutput, "ID: ")
	runCLI(t, binaryPath, testRoot, env, "--as", "e2e", "post", threadID, message)
	output := runCLI(t, binaryPath, testRoot, env, "thread", "show", threadID)

	for _, expected := range []string{threadName, message} {
		if !strings.Contains(output, expected) {
			t.Errorf("thread output missing %q:\n%s", expected, output)
		}
	}

	storagePath := filepath.Join(dataDir, "bbs")
	if _, err := os.Stat(storagePath); err != nil {
		t.Errorf("isolated storage was not created at %s: %v", storagePath, err)
	}
}

func TestCLIRejectsUndeclaredPositionalArguments(t *testing.T) {
	testRoot := t.TempDir()
	binaryPath := buildCLI(t, testRoot)

	cases := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{name: "root", args: []string{"unexpected"}, expectedErr: `unknown command "unexpected" for "bbs"`},
		{name: "topic namespace", args: []string{"topic", "bogus"}, expectedErr: `unknown command "bogus" for "bbs topic"`},
		{name: "thread namespace", args: []string{"thread", "bogus"}, expectedErr: `unknown command "bogus" for "bbs thread"`},
		{name: "export namespace", args: []string{"export", "bogus"}, expectedErr: `unknown command "bogus" for "bbs export"`},
		{name: "import namespace", args: []string{"import", "bogus"}, expectedErr: `unknown command "bogus" for "bbs import"`},
		{name: "topic list", args: []string{"topic", "list", "extra"}, expectedErr: `unknown command "extra" for "bbs topic list"`},
		{name: "version", args: []string{"version", "extra"}, expectedErr: `unknown command "extra" for "bbs version"`},
		{name: "whoami", args: []string{"whoami", "extra"}, expectedErr: `unknown command "extra" for "bbs whoami"`},
		{name: "mcp", args: []string{"mcp", "extra"}, expectedErr: `unknown command "extra" for "bbs mcp"`},
		{name: "migrate", args: []string{"migrate", "extra"}, expectedErr: `unknown command "extra" for "bbs migrate"`},
		{name: "install skill", args: []string{"install-skill", "extra"}, expectedErr: `unknown command "extra" for "bbs install-skill"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			homeDir := filepath.Join(caseRoot, "home")
			configDir := filepath.Join(caseRoot, "config")
			dataDir := filepath.Join(caseRoot, "data")
			if err := os.MkdirAll(homeDir, 0o750); err != nil {
				t.Fatalf("create isolated home: %v", err)
			}

			output, err := runCLIResult(binaryPath, caseRoot, isolatedEnvironment(homeDir, configDir, dataDir), tc.args...)
			if err == nil {
				t.Fatalf("bbs %q unexpectedly succeeded:\n%s", tc.args, output)
			}
			if count := strings.Count(output, tc.expectedErr); count != 1 {
				t.Errorf("error %q occurred %d times, want exactly once:\n%s", tc.expectedErr, count, output)
			}

			for _, path := range []string{
				configDir,
				dataDir,
				filepath.Join(homeDir, ".claude", "skills", "bbs"),
			} {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Errorf("invalid invocation created state at %s: %v", path, statErr)
				}
			}
		})
	}
}

func TestCLINamespacesShowHelpWithoutArguments(t *testing.T) {
	testRoot := t.TempDir()
	binaryPath := buildCLI(t, testRoot)

	for _, namespace := range []string{"topic", "thread", "export", "import"} {
		t.Run(namespace, func(t *testing.T) {
			caseRoot := t.TempDir()
			homeDir := filepath.Join(caseRoot, "home")
			configDir := filepath.Join(caseRoot, "config")
			dataDir := filepath.Join(caseRoot, "data")
			if err := os.MkdirAll(homeDir, 0o750); err != nil {
				t.Fatalf("create isolated home: %v", err)
			}

			output := runCLI(t, binaryPath, caseRoot, isolatedEnvironment(homeDir, configDir, dataDir), namespace)
			expected := "bbs " + namespace + " [command]"
			if !strings.Contains(output, expected) {
				t.Errorf("namespace help missing %q:\n%s", expected, output)
			}
			for _, path := range []string{configDir, dataDir} {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Errorf("namespace help created state at %s: %v", path, statErr)
				}
			}

			output = runCLI(t, binaryPath, caseRoot, isolatedEnvironment(homeDir, os.DevNull, os.DevNull), namespace)
			if !strings.Contains(output, expected) {
				t.Errorf("namespace help with unavailable storage missing %q:\n%s", expected, output)
			}
		})
	}
}

func buildCLI(t *testing.T, outputDir string) string {
	t.Helper()

	binaryPath := filepath.Join(outputDir, "bbs")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/bbs")
	build.Dir = repositoryRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binaryPath
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E test source")
	}
	return filepath.Dir(filepath.Dir(filename))
}

func isolatedEnvironment(homeDir, configDir, dataDir string) []string {
	isolationKeys := map[string]struct{}{
		"HOME":            {},
		"XDG_CONFIG_HOME": {},
		"XDG_DATA_HOME":   {},
	}

	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		key, _, found := strings.Cut(entry, "=")
		if _, isolated := isolationKeys[key]; found && isolated {
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

func runCLI(t *testing.T, binaryPath, workingDir string, env []string, args ...string) string {
	t.Helper()

	output, err := runCLIResult(binaryPath, workingDir, env, args...)
	if err != nil {
		t.Fatalf("run bbs %q: %v\n%s", args, err, output)
	}
	return output
}

func runCLIResult(binaryPath, workingDir string, env []string, args ...string) (string, error) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = workingDir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func outputValue(t *testing.T, output, prefix string) string {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		if value, found := strings.CutPrefix(line, prefix); found {
			return value
		}
	}
	t.Fatalf("output missing %q:\n%s", prefix, output)
	return ""
}
