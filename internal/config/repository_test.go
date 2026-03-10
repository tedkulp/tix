package config

import "testing"

func TestResolveWorktreePath(t *testing.T) {
	tests := []struct {
		name    string
		global  WorktreeConfig
		perRepo WorktreeConfig
		repoDir string
		want    string
	}{
		{
			name:    "default fallback",
			repoDir: "/home/user/src/myrepo",
			want:    "/home/user/src/myrepo/.worktrees",
		},
		{
			name:    "global path set",
			global:  WorktreeConfig{Path: "/tmp/worktrees"},
			repoDir: "/home/user/src/myrepo",
			want:    "/tmp/worktrees",
		},
		{
			name:    "per-repo overrides global",
			global:  WorktreeConfig{Path: "/tmp/worktrees"},
			perRepo: WorktreeConfig{Path: "/tmp/repo-worktrees"},
			repoDir: "/home/user/src/myrepo",
			want:    "/tmp/repo-worktrees",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Settings{Worktree: tt.global}
			repo := &Repository{Directory: tt.repoDir, Worktree: tt.perRepo}
			got := s.ResolveWorktreePath(repo)
			if got != tt.want {
				t.Errorf("ResolveWorktreePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDefaultBranch(t *testing.T) {
	tests := []struct {
		name              string
		global            WorktreeConfig
		perRepo           WorktreeConfig
		repoDefaultBranch string
		want              string
	}{
		{name: "default fallback", want: "main"},
		{
			name:   "global branch set",
			global: WorktreeConfig{DefaultBranch: "master"},
			want:   "master",
		},
		{
			name:    "per-repo worktree overrides global",
			global:  WorktreeConfig{DefaultBranch: "master"},
			perRepo: WorktreeConfig{DefaultBranch: "develop"},
			want:    "develop",
		},
		{
			name:              "repo DefaultBranch used as fallback",
			global:            WorktreeConfig{DefaultBranch: "master"},
			repoDefaultBranch: "staging",
			want:              "staging",
		},
		{
			name:              "worktree per-repo overrides repo DefaultBranch",
			repoDefaultBranch: "staging",
			perRepo:           WorktreeConfig{DefaultBranch: "develop"},
			want:              "develop",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Settings{Worktree: tt.global}
			repo := &Repository{DefaultBranch: tt.repoDefaultBranch, Worktree: tt.perRepo}
			got := s.ResolveDefaultBranch(repo)
			if got != tt.want {
				t.Errorf("ResolveDefaultBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}
