# Worktree Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace per-repo `worktree.enabled` config toggle with an opt-in `--worktree` flag on `tix create` and `tix start`, backed by real `git worktree add`, and add a `tix cleanup` command to remove worktrees.

**Architecture:** Config gains a shared `WorktreeConfig` struct at both global and per-repo level with a resolution helper on `Settings`. The git layer is rewritten to use `exec.Command("git", "worktree", ...)`. Both `create` and `start` grow a `--worktree` flag that skips the clean-check and calls the new git layer. A new `cleanup` command detects the current worktree from cwd and removes it.

**Tech Stack:** Go 1.23, `github.com/spf13/cobra`, `github.com/pterm/pterm`, `os/exec`, standard `testing` package with table-driven tests.

---

### Task 1: Update config struct

**Context:** `internal/config/repository.go` currently has a `Worktree struct { Enabled bool; DefaultBranch string }`. We need to replace it with `WorktreeConfig { Path string; DefaultBranch string }` used at both global (`Settings`) and per-repo (`Repository`) level. The `Enabled` field is removed entirely. We also need resolution helpers on `Settings`.

**Files:**
- Modify: `internal/config/repository.go`
- Create: `internal/config/repository_test.go`

---

**Step 1: Write failing tests for config resolution**

Create `internal/config/repository_test.go`:

```go
package config

import (
	"testing"
)

func TestResolveWorktreePath(t *testing.T) {
	tests := []struct {
		name     string
		global   WorktreeConfig
		perRepo  WorktreeConfig
		repoDir  string
		want     string
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
		name    string
		global  WorktreeConfig
		perRepo WorktreeConfig
		want    string
	}{
		{
			name: "default fallback",
			want: "main",
		},
		{
			name:   "global branch set",
			global: WorktreeConfig{DefaultBranch: "master"},
			want:   "master",
		},
		{
			name:    "per-repo overrides global",
			global:  WorktreeConfig{DefaultBranch: "master"},
			perRepo: WorktreeConfig{DefaultBranch: "develop"},
			want:    "develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Settings{Worktree: tt.global}
			repo := &Repository{Worktree: tt.perRepo}
			got := s.ResolveDefaultBranch(repo)
			if got != tt.want {
				t.Errorf("ResolveDefaultBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /Users/tedkulp/src/tix && go test ./internal/config/... -v
```

Expected: compile error — `WorktreeConfig`, `ResolveWorktreePath`, `ResolveDefaultBranch` not defined.

**Step 3: Update `internal/config/repository.go`**

Replace the `Worktree` struct and update `Repository` and `Settings`. The full updated file:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// WorktreeConfig represents worktree configuration (used at global and per-repo level)
type WorktreeConfig struct {
	Path          string `yaml:"path" mapstructure:"path"`
	DefaultBranch string `yaml:"default_branch" mapstructure:"default_branch"`
}

// Repository represents a single repository configuration
type Repository struct {
	Name          string         `yaml:"name" mapstructure:"name"`
	Directory     string         `yaml:"directory" mapstructure:"directory"`
	DefaultLabels string         `yaml:"default_labels" mapstructure:"default_labels"`
	ReadyLabel    string         `yaml:"ready_label" mapstructure:"ready_label"`
	ReadyStatus   string         `yaml:"ready_status" mapstructure:"ready_status"`
	UnreadyLabel  string         `yaml:"unready_label" mapstructure:"unready_label"`
	UnreadyStatus string         `yaml:"unready_status" mapstructure:"unready_status"`
	GithubRepo    string         `yaml:"github_repo" mapstructure:"github_repo"`
	GitlabRepo    string         `yaml:"gitlab_repo" mapstructure:"gitlab_repo"`
	DefaultBranch string         `yaml:"default_branch" mapstructure:"default_branch"`
	Worktree      WorktreeConfig `yaml:"worktree,omitempty" mapstructure:"worktree"`
}

// Settings represents the root configuration
type Settings struct {
	ReadyLabel    string         `yaml:"ready_label" mapstructure:"ready_label"`
	ReadyStatus   string         `yaml:"ready_status" mapstructure:"ready_status"`
	UnreadyLabel  string         `yaml:"unready_label" mapstructure:"unready_label"`
	UnreadyStatus string         `yaml:"unready_status" mapstructure:"unready_status"`
	Worktree      WorktreeConfig `yaml:"worktree,omitempty" mapstructure:"worktree"`
	Repositories  []Repository   `yaml:"repositories" mapstructure:"repositories"`
}

// ResolveWorktreePath returns the worktree base path for a repo.
// Resolution order: per-repo > global > default (<repo-dir>/.worktrees)
func (s *Settings) ResolveWorktreePath(repo *Repository) string {
	if repo.Worktree.Path != "" {
		return expandHomeDir(repo.Worktree.Path)
	}
	if s.Worktree.Path != "" {
		return expandHomeDir(s.Worktree.Path)
	}
	return filepath.Join(repo.Directory, ".worktrees")
}

// ResolveDefaultBranch returns the default branch for a repo.
// Resolution order: per-repo > global > "main"
func (s *Settings) ResolveDefaultBranch(repo *Repository) string {
	if repo.Worktree.DefaultBranch != "" {
		return repo.Worktree.DefaultBranch
	}
	if s.Worktree.DefaultBranch != "" {
		return s.Worktree.DefaultBranch
	}
	return "main"
}

// Load reads the configuration from the specified file
func Load() (*Settings, error) {
	v := viper.New()
	v.SetConfigName(".tix")
	v.SetConfigType("yaml")
	v.AddConfigPath("$HOME")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var settings Settings
	decoderConfig := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &settings,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	for i := range settings.Repositories {
		repo := &settings.Repositories[i]
		if repo.Directory != "" {
			repo.Directory = expandHomeDir(repo.Directory)
		}
	}

	return &settings, nil
}

func expandHomeDir(path string) string {
	if path == "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Clean(filepath.Join(home, path[2:]))
	}
	return path
}

// IsCodeRepo returns true if the repository has a directory configured
func (r *Repository) IsCodeRepo() bool {
	return r.Directory != ""
}

// GetRepoNames returns a list of repository names
func (s *Settings) GetRepoNames() []string {
	names := make([]string, len(s.Repositories))
	for i, repo := range s.Repositories {
		names[i] = repo.Name
	}
	return names
}

// GetRepo returns a repository by name
func (s *Settings) GetRepo(name string) *Repository {
	for i := range s.Repositories {
		if s.Repositories[i].Name == name {
			return &s.Repositories[i]
		}
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /Users/tedkulp/src/tix && go test ./internal/config/... -v
```

Expected: PASS — both `TestResolveWorktreePath` and `TestResolveDefaultBranch` pass.

**Step 5: Verify the project still compiles**

```bash
cd /Users/tedkulp/src/tix && go build ./...
```

Expected: compile errors in `cmd/create.go` and `cmd/start.go` referencing `repo.Worktree.Enabled` — that's fine, we'll fix those in later tasks. If there are other unexpected errors, fix them before continuing.

---

### Task 2: Rewrite git worktree layer

**Context:** `internal/git/worktree.go` currently has a broken implementation that uses go-git to checkout a branch in the main worktree — not a real `git worktree add`. Replace `AddWorktree` with a real exec-based implementation, add `RemoveWorktree`, and remove the unused `ListWorktrees` and `GetWorktreePath`.

**Files:**
- Modify: `internal/git/worktree.go`

**Note on testing:** These functions shell out to `git` and require a real git repo on disk. Unit tests would need a temp git repo setup. Given the project has no existing git layer tests and the functions are thin wrappers around exec, skip unit tests here — the integration will be tested manually end-to-end.

---

**Step 1: Rewrite `internal/git/worktree.go`**

```go
package git

import (
	"fmt"
	"os/exec"
)

// AddWorktree creates a new git worktree at worktreePath with a new branch branchName.
// Runs: git worktree add <worktreePath> -b <branchName>
func (r *Repository) AddWorktree(worktreePath, branchName string) error {
	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branchName)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}
	return nil
}

// RemoveWorktree removes the git worktree at worktreePath.
// Runs: git worktree remove <worktreePath>
func (r *Repository) RemoveWorktree(worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %s: %w", string(output), err)
	}
	return nil
}
```

**Step 2: Verify the package compiles**

```bash
cd /Users/tedkulp/src/tix && go build ./internal/git/...
```

Expected: SUCCESS.

---

### Task 3: Update `tix create`

**Context:** `cmd/create.go` has `createBranch` that checks `repo.Worktree.Enabled`. Replace this with a `--worktree` flag. When the flag is set, skip the clean check and use `git worktree add` into the resolved path. Pass `cfg *config.Settings` into `createBranch` so it can resolve the worktree path.

**Files:**
- Modify: `cmd/create.go`

---

**Step 1: Add `useWorktree` package-level var and update `init()`**

In `cmd/create.go`, add a `useWorktree bool` var alongside the existing `title` and `selfAssign` vars, and register the flag in `init()`:

```go
var (
	title       string
	selfAssign  bool
	useWorktree bool
)
```

In `init()`, add:
```go
createCmd.Flags().BoolVarP(&useWorktree, "worktree", "w", false, "Create a git worktree instead of checking out a branch")
```

**Step 2: Update `RunE` to conditionally skip clean check**

In the `RunE` body, replace the current:
```go
// Open Git repository and validate it's clean BEFORE any user interaction
gitRepo, err := openAndValidateRepo(repoSettings.Directory)
if err != nil {
    ...
}
```

With:
```go
// Open Git repository
gitRepo, err := git.Open(repoSettings.Directory)
if err != nil {
    return fmt.Errorf("couldn't open git repository at %s", repoSettings.Directory)
}

// Validate clean working tree only for regular (non-worktree) branches
if !useWorktree {
    isClean, err := gitRepo.IsClean()
    if err != nil {
        return fmt.Errorf("failed to check repository status: %w", err)
    }
    if !isClean {
        return fmt.Errorf("git repository has uncommitted changes - commit or stash them first")
    }
}
```

Also update the `createBranch` call to pass `cfg` and `useWorktree`:
```go
if err := createBranch(gitRepo, repoSettings.CodeRepo, cfg, issueResult.Number, issueResult.Title, projectPrefix, useWorktree); err != nil {
```

Note: `cfg` is already loaded earlier in `setupRepository` but not returned. You need to either return it from `setupRepository` or reload it. The simplest fix: load cfg at the top of `RunE` before calling `setupRepository`, then pass it to `setupRepository`. Update `setupRepository` to accept `cfg *config.Settings` instead of loading it internally.

**Step 3: Update `setupRepository` to accept cfg**

Change signature:
```go
func setupRepository(cfg *config.Settings, issueRepoArg, codeRepoArg string) (*RepoSettings, error) {
```

Remove the `cfg, err := config.Load()` lines from inside `setupRepository`.

In `RunE`, load cfg first:
```go
cfg, err := config.Load()
if err != nil {
    return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
}

repoSettings, err := setupRepository(cfg, issueRepoArg, codeRepoArg)
```

**Step 4: Update `createBranch` function**

New signature:
```go
func createBranch(gitRepo *git.Repository, repo *config.Repository, cfg *config.Settings, issueNumber int, issueTitle string, projectPrefix string, useWorktree bool) error {
```

Replace the body's worktree section. Remove the `if repo.Worktree.Enabled` block entirely. New logic:

```go
if useWorktree {
    worktreeBase := cfg.ResolveWorktreePath(repo)
    worktreeDir := filepath.Join(worktreeBase, branchName)
    logger.Info("Creating worktree", map[string]interface{}{
        "branch":    branchName,
        "directory": worktreeDir,
    })

    if err := gitRepo.AddWorktree(worktreeDir, branchName); err != nil {
        return fmt.Errorf("failed to create worktree: %w", err)
    }

    fmt.Printf("Created worktree: %s\n", worktreeDir)
} else {
    logger.Info("Creating and checking out branch", map[string]interface{}{
        "branch": branchName,
    })

    if err := gitRepo.CreateBranch(branchName); err != nil {
        return fmt.Errorf("failed to create branch: %w", err)
    }
    if err := gitRepo.CheckoutBranch(branchName); err != nil {
        return fmt.Errorf("failed to checkout branch: %w", err)
    }

    fmt.Printf("Created and checked out branch: %s\n", branchName)
}
```

Also remove `openAndValidateRepo` function since it's no longer used (the clean check is now inline).

**Step 5: Verify the project compiles**

```bash
cd /Users/tedkulp/src/tix && go build ./...
```

Expected: SUCCESS (or only errors in `cmd/start.go` which is handled next).

---

### Task 4: Update `tix start`

**Context:** `cmd/start.go` has the same worktree pattern as `create.go` — it checks `codeRepo.Worktree.Enabled` inline and also does a clean check unconditionally. Apply the same `--worktree` flag pattern.

**Files:**
- Modify: `cmd/start.go`

---

**Step 1: Add `startUseWorktree` var and flag**

Add a package-level var (use a distinct name to avoid conflict with create.go's `useWorktree`):

```go
var startUseWorktree bool
```

In `init()`:
```go
startCmd.Flags().BoolVarP(&startUseWorktree, "worktree", "w", false, "Create a git worktree instead of checking out a branch")
```

**Step 2: Load cfg at top of `RunE`**

Add at the start of `RunE`, replacing the existing `config.Load()` call:
```go
cfg, err := config.Load()
if err != nil {
    return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
}
```

(This is already there — just make sure `cfg` is kept in scope for later use.)

**Step 3: Replace clean check and branch creation**

Find the section in `start.go`'s `RunE` that currently reads:

```go
isClean, err := gitRepo.IsClean()
if err != nil {
    return fmt.Errorf("failed to check repository status: %w", err)
}
if !isClean {
    return fmt.Errorf("repository is not clean - commit or stash changes first")
}

// Create and checkout branch
if codeRepo.Worktree.Enabled {
    ...
} else {
    ...
}
```

Replace with:

```go
if !startUseWorktree {
    isClean, err := gitRepo.IsClean()
    if err != nil {
        return fmt.Errorf("failed to check repository status: %w", err)
    }
    if !isClean {
        return fmt.Errorf("repository is not clean - commit or stash changes first")
    }
}

if startUseWorktree {
    worktreeBase := cfg.ResolveWorktreePath(codeRepo)
    worktreeDir := filepath.Join(worktreeBase, branchName)
    logger.Info("Creating worktree", map[string]interface{}{
        "branch":    branchName,
        "directory": worktreeDir,
    })

    if err := gitRepo.AddWorktree(worktreeDir, branchName); err != nil {
        return fmt.Errorf("failed to create worktree: %w", err)
    }

    fmt.Printf("Created worktree: %s\n", worktreeDir)
} else {
    logger.Info("Creating and checking out branch", map[string]interface{}{
        "branch": branchName,
    })

    if err := gitRepo.CreateBranch(branchName); err != nil {
        return fmt.Errorf("failed to create branch: %w", err)
    }
    if err := gitRepo.CheckoutBranch(branchName); err != nil {
        return fmt.Errorf("failed to checkout branch: %w", err)
    }

    fmt.Printf("Created and checked out branch: %s\n", branchName)
}
```

**Step 4: Verify the project compiles**

```bash
cd /Users/tedkulp/src/tix && go build ./...
```

Expected: SUCCESS with no errors.

**Step 5: Run all tests**

```bash
cd /Users/tedkulp/src/tix && go test ./...
```

Expected: all existing tests pass.

---

### Task 5: Add `tix cleanup` command

**Context:** New command that detects the current worktree from cwd, prompts with the detected branch name as default, then calls `git worktree remove`. We also need a pure helper function `detectWorktreeBranch` that can be tested without any git operations.

**Files:**
- Create: `cmd/cleanup.go`
- Create: `cmd/cleanup_test.go`

---

**Step 1: Write failing test for detection helper**

Create `cmd/cleanup_test.go`:

```go
package cmd

import (
	"testing"
)

func TestDetectWorktreeBranch(t *testing.T) {
	tests := []struct {
		name          string
		cwd           string
		worktreePath  string
		want          string
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
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/tedkulp/src/tix && go test ./cmd/... -run TestDetectWorktreeBranch -v
```

Expected: compile error — `detectWorktreeBranch` not defined.

**Step 3: Create `cmd/cleanup.go`**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove a git worktree",
	Long:  `Remove a git worktree directory. The branch is left intact.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
		}

		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Find the code repo whose worktree path contains cwd
		var codeRepo *config.Repository
		var detectedBranch string
		var worktreeBase string

		for i := range cfg.Repositories {
			repo := &cfg.Repositories[i]
			if !repo.IsCodeRepo() {
				continue
			}
			base := cfg.ResolveWorktreePath(repo)
			branch := detectWorktreeBranch(wd, base)
			if branch != "" {
				codeRepo = repo
				detectedBranch = branch
				worktreeBase = base
				break
			}
		}

		// If we couldn't detect from cwd, pick first code repo
		if codeRepo == nil {
			for i := range cfg.Repositories {
				if cfg.Repositories[i].IsCodeRepo() {
					codeRepo = &cfg.Repositories[i]
					worktreeBase = cfg.ResolveWorktreePath(codeRepo)
					break
				}
			}
		}

		if codeRepo == nil {
			return fmt.Errorf("no code repositories configured")
		}

		// Prompt with detected branch as default
		branchName, err := pterm.DefaultInteractiveTextInput.
			WithDefaultText("Worktree branch to remove").
			WithDefaultValue(detectedBranch).
			Show()
		if err != nil || strings.TrimSpace(branchName) == "" {
			return fmt.Errorf("cleanup cancelled")
		}
		branchName = strings.TrimSpace(branchName)

		worktreeDir := filepath.Join(worktreeBase, branchName)

		logger.Info("Removing worktree", map[string]interface{}{
			"branch":    branchName,
			"directory": worktreeDir,
		})

		gitRepo, err := git.Open(codeRepo.Directory)
		if err != nil {
			return fmt.Errorf("failed to open git repository: %w", err)
		}

		if err := gitRepo.RemoveWorktree(worktreeDir); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		fmt.Printf("Removed worktree: %s\n", worktreeDir)
		fmt.Printf("Project root: %s\n", codeRepo.Directory)

		return nil
	},
}

// detectWorktreeBranch returns the branch name if cwd is inside worktreePath,
// or empty string if not.
func detectWorktreeBranch(cwd, worktreePath string) string {
	// Normalize: ensure worktreePath ends with separator for prefix matching
	base := worktreePath
	if !strings.HasSuffix(base, string(filepath.Separator)) {
		base += string(filepath.Separator)
	}

	if !strings.HasPrefix(cwd, base) {
		return ""
	}

	rel := cwd[len(base):]
	if rel == "" {
		return ""
	}

	// First path segment is the branch name
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if parts[0] == "" {
		return ""
	}
	return parts[0]
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /Users/tedkulp/src/tix && go test ./cmd/... -run TestDetectWorktreeBranch -v
```

Expected: PASS.

**Step 5: Verify full build and all tests**

```bash
cd /Users/tedkulp/src/tix && go build ./... && go test ./...
```

Expected: build succeeds, all tests pass.

---

### Task 6: End-to-end smoke test

**Context:** Manual verification that the happy path works. Use the existing `test-repo-for-tix` if present, or a fresh temp git repo.

**Step 1: Build the binary**

```bash
cd /Users/tedkulp/src/tix && go build -o dist/tix .
```

**Step 2: Smoke test `tix create --worktree`**

In a repo configured in `.tix.yaml`:
```bash
dist/tix create --worktree --title "test worktree feature"
```

Expected output includes:
```
Created worktree: /path/to/.worktrees/NNN-test-worktree-feature
```

Verify with:
```bash
git worktree list
```

**Step 3: Smoke test `tix cleanup`**

```bash
cd /path/to/.worktrees/NNN-test-worktree-feature
dist/tix cleanup
# Prompt should pre-fill with "NNN-test-worktree-feature"
# Confirm
```

Expected:
```
Removed worktree: /path/to/.worktrees/NNN-test-worktree-feature
Project root: /path/to/repo
```

Verify with:
```bash
git worktree list
```

The worktree should no longer appear.

**Step 4: Verify branch still exists**

```bash
git branch --list NNN-test-worktree-feature
```

Expected: branch still exists.
