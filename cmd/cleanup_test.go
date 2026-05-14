package cmd

import (
	"strings"
	"testing"
)

func TestDetectWorktreeBranch(t *testing.T) {
	tests := []struct {
		name         string
		cwd          string
		worktreePath string
		want         string
	}{
		{
			name:         "cwd is directly inside worktree",
			cwd:          "/home/user/src/myrepo/.worktrees/123-my-feature",
			worktreePath: "/home/user/src/myrepo/.worktrees",
			want:         "123-my-feature",
		},
		{
			name:         "cwd is a subdirectory inside worktree",
			cwd:          "/home/user/src/myrepo/.worktrees/123-my-feature/src/pkg",
			worktreePath: "/home/user/src/myrepo/.worktrees",
			want:         "123-my-feature",
		},
		{
			name:         "cwd is not inside worktree path",
			cwd:          "/home/user/src/myrepo",
			worktreePath: "/home/user/src/myrepo/.worktrees",
			want:         "",
		},
		{
			name:         "cwd is the worktrees base itself",
			cwd:          "/home/user/src/myrepo/.worktrees",
			worktreePath: "/home/user/src/myrepo/.worktrees",
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectWorktreeBranch(tt.cwd, tt.worktreePath)
			if got != tt.want {
				t.Errorf("detectWorktreeBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanupForceFlag(t *testing.T) {
	// Verify the --force flag is registered on the cleanup command
	flag := cleanupCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("expected --force flag to be registered on cleanupCmd")
	}

	if flag.Shorthand != "f" {
		t.Errorf("expected shorthand 'f', got %q", flag.Shorthand)
	}

	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}

	if flag.Usage == "" {
		t.Error("expected --force flag to have a usage description")
	}
}

func TestCleanupUseString(t *testing.T) {
	// Verify the Use string includes "[branch]" for the optional arg
	if cleanupCmd.Use != "cleanup [branch]" {
		t.Errorf("expected Use 'cleanup [branch]', got %q", cleanupCmd.Use)
	}
}

func TestCleanupLongDescription(t *testing.T) {
	// Verify the Long description mentions --force behavior
	if !strings.Contains(cleanupCmd.Long, "--force") {
		t.Error("expected Long description to mention --force")
	}
	if !strings.Contains(cleanupCmd.Long, "auto-detected") {
		t.Error("expected Long description to mention auto-detection")
	}
}
