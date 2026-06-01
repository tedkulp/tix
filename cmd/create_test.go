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
