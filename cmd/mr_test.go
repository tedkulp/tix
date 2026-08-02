package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tedkulp/tix/internal/config"
)

// TestMrNonInteractiveFlag verifies the --non-interactive/-n flag exists on
// mrCmd, mirroring the flag other subcommands (create, start, setdesc) expose.
func TestMrNonInteractiveFlag(t *testing.T) {
	flag := mrCmd.Flags().Lookup("non-interactive")
	if flag == nil {
		t.Fatal("expected --non-interactive flag to be registered on mrCmd")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}
	if flag.Shorthand != "n" {
		t.Errorf("expected shorthand 'n', got %q", flag.Shorthand)
	}
	if flag.Usage == "" {
		t.Error("expected --non-interactive flag to have a usage description")
	}
}

// TestMrNonInteractiveAmbiguousRepo reproduces issue #10: when the working
// directory doesn't match any configured repo, `tix mr` used to fall back to
// an interactive pterm selector with no way to opt out, which hangs
// non-interactive contexts like CI. With --non-interactive it must instead
// return a clear error.
func TestMrNonInteractiveAmbiguousRepo(t *testing.T) {
	dir := t.TempDir()

	runGit := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "init")

	// Config points at a directory that isn't the temp repo, so
	// FindRepoForDir won't match and the ambiguous-repo path fires.
	configPath := filepath.Join(dir, ".tix.yml")
	configContents := "repositories:\n" +
		"  - name: other-repo\n" +
		"    directory: /tmp/nonexistent-mr-test-repo\n" +
		"    github_repo: someone/somewhere\n"
	if err := os.WriteFile(configPath, []byte(configContents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	config.SetConfigFile(configPath)
	defer config.SetConfigFile("")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	orig := mrNonInteractive
	defer func() { mrNonInteractive = orig }()
	mrNonInteractive = true

	err = mrCmd.RunE(mrCmd, []string{})
	if err == nil {
		t.Fatal("expected error for ambiguous repo in non-interactive mode, got nil")
	}
	if !strings.Contains(err.Error(), "--non-interactive requires an unambiguous repository") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}
