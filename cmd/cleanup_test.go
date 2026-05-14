package cmd

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestListWorktreeBranches(t *testing.T) {
	t.Run("lists directories in worktree base", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create some worktree directories
		if err := os.MkdirAll(filepath.Join(tmpDir, "123-feature-a"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmpDir, "456-feature-b"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmpDir, "789-feature-c"), 0755); err != nil {
			t.Fatal(err)
		}
		// Hidden directory should be excluded
		if err := os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0755); err != nil {
			t.Fatal(err)
		}
		// Regular file should be excluded
		f, err := os.Create(filepath.Join(tmpDir, "not-a-dir"))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		got, err := listWorktreeBranches(tmpDir)
		if err != nil {
			t.Fatalf("listWorktreeBranches() error: %v", err)
		}

		want := []string{"123-feature-a", "456-feature-b", "789-feature-c"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("listWorktreeBranches() = %v, want %v", got, want)
		}
	})

	t.Run("returns empty slice when no worktrees exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		got, err := listWorktreeBranches(tmpDir)
		if err != nil {
			t.Fatalf("listWorktreeBranches() error: %v", err)
		}

		if len(got) != 0 {
			t.Errorf("listWorktreeBranches() = %v, want empty slice", got)
		}
	})

	t.Run("returns error for nonexistent directory", func(t *testing.T) {
		_, err := listWorktreeBranches("/nonexistent/path")
		if err == nil {
			t.Error("listWorktreeBranches() expected error for nonexistent path")
		}
	})

	t.Run("returns sorted branches regardless of creation order", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create directories in reverse alphabetical order
		for _, name := range []string{"z-feature", "m-feature", "a-feature"} {
			if err := os.MkdirAll(filepath.Join(tmpDir, name), 0755); err != nil {
				t.Fatal(err)
			}
		}

		got, err := listWorktreeBranches(tmpDir)
		if err != nil {
			t.Fatalf("listWorktreeBranches() error: %v", err)
		}

		want := []string{"a-feature", "m-feature", "z-feature"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("listWorktreeBranches() = %v, want alphabetically sorted %v", got, want)
		}
	})
}
