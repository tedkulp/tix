package cmd

import "testing"

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
