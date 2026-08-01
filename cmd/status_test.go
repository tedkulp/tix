package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tedkulp/tix/internal/config"
)

// TestStatusJSONNonTicketBranchReturnsError verifies that `tix status --json`
// on a non-ticket branch returns an error (so Execute()/main() control the
// exit code and any deferred cleanup still runs) instead of calling
// os.Exit(1) directly from within RunE.
func TestStatusJSONNonTicketBranchReturnsError(t *testing.T) {
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

	configPath := filepath.Join(dir, ".tix.yml")
	configContents := "repositories:\n" +
		"  - name: test-repo\n" +
		"    directory: " + dir + "\n" +
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

	if err := statusCmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("set --json flag: %v", err)
	}
	defer func() { _ = statusCmd.Flags().Set("json", "false") }()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := statusCmd.RunE(statusCmd, []string{})

	_ = w.Close()
	os.Stdout = origStdout

	var buf []byte
	buf, readErr := readAll(r)
	if readErr != nil {
		t.Fatalf("read captured stdout: %v", readErr)
	}

	if runErr == nil {
		t.Fatal("expected RunE to return an error for a non-ticket branch, got nil (should not call os.Exit directly)")
	}

	var out statusJSON
	if err := json.Unmarshal(buf, &out); err != nil {
		t.Fatalf("expected valid JSON on stdout even on error, got %q: %v", buf, err)
	}
	if out.Branch != "main" {
		t.Errorf("expected branch %q in JSON output, got %q", "main", out.Branch)
	}
}

func readAll(r *os.File) ([]byte, error) {
	var buf []byte
	chunk := make([]byte, 4096)
	for {
		n, err := r.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}
