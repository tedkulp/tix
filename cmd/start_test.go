package cmd

import (
	"strings"
	"testing"
)

func TestStartNoAutoStashFlag(t *testing.T) {
	flag := startCmd.Flags().Lookup("no-auto-stash")
	if flag == nil {
		t.Fatal("expected --no-auto-stash flag to be registered on startCmd")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}
	if flag.Usage == "" {
		t.Error("expected --no-auto-stash flag to have a usage description")
	}
}

func TestStartNonInteractiveFlag(t *testing.T) {
	flag := startCmd.Flags().Lookup("non-interactive")
	if flag == nil {
		t.Fatal("expected --non-interactive flag to be registered on startCmd")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}
	if flag.Usage == "" {
		t.Error("expected --non-interactive flag to have a usage description")
	}
}

func TestStartNonInteractiveRequiresIssueNumber(t *testing.T) {
	orig := startNonInteractive
	defer func() { startNonInteractive = orig }()

	startNonInteractive = true

	err := startCmd.RunE(startCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --non-interactive is set without an issue number argument")
	}
	if !strings.Contains(err.Error(), "--non-interactive requires an issue number argument") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}
