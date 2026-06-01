package cmd

import (
	"strings"
	"testing"
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
