package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHonorsConfigFileOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yml")
	content := []byte("ready_label: from-custom-file\nrepositories: []\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	SetConfigFile(path)
	t.Cleanup(func() { SetConfigFile("") })

	settings, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if settings.ReadyLabel != "from-custom-file" {
		t.Errorf("ReadyLabel = %q, want %q", settings.ReadyLabel, "from-custom-file")
	}
}

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

func TestFindRepoForDir(t *testing.T) {
	repos := []Repository{
		{Name: "issues-only"},
		{Name: "tix", Directory: "/home/user/src/tix"},
		{Name: "tix-nested", Directory: "/home/user/src/tix/vendor/dep"},
		{Name: "other", Directory: "/home/user/src/other"},
	}

	tests := []struct {
		name string
		dir  string
		want string // repo name, "" means no match
	}{
		{
			name: "exact match on repo root",
			dir:  "/home/user/src/tix",
			want: "tix",
		},
		{
			name: "subdirectory of repo",
			dir:  "/home/user/src/tix/cmd",
			want: "tix",
		},
		{
			name: "nested repo wins over parent (longest match)",
			dir:  "/home/user/src/tix/vendor/dep/internal",
			want: "tix-nested",
		},
		{
			name: "sibling directory sharing a name prefix must not match",
			dir:  "/home/user/src/tix-other",
			want: "",
		},
		{
			name: "sibling subdirectory sharing a name prefix must not match",
			dir:  "/home/user/src/tix-other/cmd",
			want: "",
		},
		{
			name: "unrelated directory",
			dir:  "/tmp/somewhere",
			want: "",
		},
		{
			name: "repo without a directory is never matched",
			dir:  "/home/user/src",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Settings{Repositories: repos}
			got := s.FindRepoForDir(tt.dir)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("FindRepoForDir(%q) = %q, want no match", tt.dir, got.Name)
				}
				return
			}
			if got == nil {
				t.Fatalf("FindRepoForDir(%q) = no match, want %q", tt.dir, tt.want)
			}
			if got.Name != tt.want {
				t.Errorf("FindRepoForDir(%q) = %q, want %q", tt.dir, got.Name, tt.want)
			}
		})
	}
}
