package cmd

import (
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
