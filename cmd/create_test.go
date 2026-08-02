package cmd

import (
	"strings"
	"testing"

	"github.com/tedkulp/tix/internal/config"
)

func TestCreateNoAutoStashFlag(t *testing.T) {
	flag := createCmd.Flags().Lookup("no-auto-stash")
	if flag == nil {
		t.Fatal("expected --no-auto-stash flag to be registered on createCmd")
	}

	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}

	if flag.Usage == "" {
		t.Error("expected --no-auto-stash flag to have a usage description")
	}
}

func TestCreateNonInteractiveFlag(t *testing.T) {
	flag := createCmd.Flags().Lookup("non-interactive")
	if flag == nil {
		t.Fatal("expected --non-interactive flag to be registered on createCmd")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}
	if flag.Usage == "" {
		t.Error("expected --non-interactive flag to have a usage description")
	}
}

func TestCreateNonInteractiveRequiresTitle(t *testing.T) {
	orig := nonInteractive
	origTitle := title
	defer func() {
		nonInteractive = orig
		title = origTitle
	}()

	nonInteractive = true
	title = ""

	err := createCmd.RunE(createCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --non-interactive is set without --title")
	}
	if !strings.Contains(err.Error(), "--non-interactive requires -t/--title") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestCreateNonInteractiveAmbiguousRepo(t *testing.T) {
	// setupRepository errors when nonInteractive=true, no issueRepoArg, no cwd match,
	// and multiple repos exist. We test the helper directly.
	// Build a minimal config with two repos that don't match cwd.
	cfg := &config.Settings{
		Repositories: []config.Repository{
			{GithubRepo: "owner/repo-a", Directory: "/tmp/nonexistent-a"},
			{GithubRepo: "owner/repo-b", Directory: "/tmp/nonexistent-b"},
		},
	}

	orig := nonInteractive
	defer func() { nonInteractive = orig }()
	nonInteractive = true

	_, err := setupRepository(cfg, "", "")
	if err == nil {
		t.Fatal("expected error for ambiguous repo in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "--non-interactive requires an unambiguous repository") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

// TestCreateInvalidRepoConfigDoesNotLeakStructDetails reproduces issue #12:
// the validation error used "%+v" on the *config.Repository, dumping every
// field (including the Directory path) into a user-facing error message.
// It should name the repo, not print the struct.
func TestCreateInvalidRepoConfigDoesNotLeakStructDetails(t *testing.T) {
	cfg := &config.Settings{
		Repositories: []config.Repository{
			{
				Name:      "misconfigured-repo",
				Directory: "/home/someuser/secret-project-path",
				// Neither GithubRepo nor GitlabRepo set - invalid configuration.
			},
		},
	}

	_, err := setupRepository(cfg, "", "")
	if err == nil {
		t.Fatal("expected error for repo with neither github_repo nor gitlab_repo set")
	}

	msg := err.Error()
	if !strings.Contains(msg, "misconfigured-repo") {
		t.Errorf("expected error to name the repo, got: %s", msg)
	}
	if strings.Contains(msg, "/home/someuser/secret-project-path") {
		t.Errorf("error leaked repository struct details (Directory path): %s", msg)
	}
	if strings.Contains(msg, "&config.Repository") || strings.Contains(msg, "Directory:") {
		t.Errorf("error leaked raw struct dump: %s", msg)
	}
}
